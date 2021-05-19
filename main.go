package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"sort"
	"time"
	"unsafe"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"loov.dev/traceview/import/jaeger"
)

func main() {
	flag.Parse()
	infile := flag.Arg(0)
	if infile == "" {
		return
	}
	if err := run(context.Background(), infile); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run(ctx context.Context, infile string) error {
	data, err := os.ReadFile(infile)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", infile, err)
	}

	var tracefile jaeger.File
	err = json.Unmarshal(data, &tracefile)
	if err != nil {
		return fmt.Errorf("failed to parse file %q: %w", infile, err)
	}

	ui := NewUI(tracefile.Data)
	go func() {
		w := app.NewWindow(app.Title("Jaeger UI"))
		if err := ui.Run(w); err != nil {
			log.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	app.Main()
	return nil
}

var defaultMargin = unit.Dp(10)

type UI struct {
	Theme    *material.Theme
	Timeline *Timeline

	SkipSpans widget.Float
	ZoomLevel widget.Float
}

func NewUI(traces []jaeger.Trace) *UI {
	ui := &UI{}
	ui.Theme = material.NewTheme(gofont.Collection())
	ui.Timeline = NewTimeline(traces)

	ui.SkipSpans.Value = 0.0
	ui.ZoomLevel.Value = 1.0
	return ui
}

func (ui *UI) Run(w *app.Window) error {
	var ops op.Ops

	for e := range w.Events() {
		switch e := e.(type) {
		case system.FrameEvent:

			gtx := layout.NewContext(&ops, e)
			ui.Layout(gtx)
			e.Frame(gtx.Ops)

		case key.Event:
			switch e.Name {
			case key.NameEscape:
				return nil
			}

		case system.DestroyEvent:
			return e.Err
		}
	}

	return nil
}

func (ui *UI) Layout(gtx layout.Context) layout.Dimensions {
	view := &TimelineView{
		Theme:    ui.Theme,
		Timeline: ui.Timeline,

		RowHeight: ui.Theme.FingerSize,
		RowGap:    ui.Theme.FingerSize.Scale(0.2),

		ZoomStart: ui.Timeline.StartTime.Std(),
		ZoomEnd:   ui.Timeline.StartTime.Std() + ui.Timeline.Duration.Std(),
	}

	for _, node := range ui.Timeline.RenderOrder {
		if node.Duration.Std().Seconds() < float64(ui.SkipSpans.Value) {
			continue
		}
		view.Visible = append(view.Visible, node)
	}

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(
			DurationSlider(ui.Theme, &ui.ZoomLevel, "Zoom", time.Microsecond, ui.Timeline.Duration.Std()).Layout),
		layout.Rigid(
			DurationSlider(ui.Theme, &ui.SkipSpans, "Skip", 0, 5*time.Second).Layout),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, view.Minimap)
		}),

		layout.Flexed(1,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, view.Spans),
					layout.Flexed(2, view.Spans),
				)
			}),
	)
}

type DurationSliderStyle struct {
	Theme *material.Theme
	Value *widget.Float
	Text  string
	Min   time.Duration
	Max   time.Duration
}

func DurationSlider(theme *material.Theme, value *widget.Float, text string, min, max time.Duration) DurationSliderStyle {
	return DurationSliderStyle{
		Theme: theme,
		Value: value,
		Text:  text,
		Min:   min,
		Max:   max,
	}
}

func (slider DurationSliderStyle) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Rigid(
			material.Body1(slider.Theme,
				slider.Text+fmt.Sprintf(" %.2f sec", slider.Value.Value)).Layout,
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
		layout.Flexed(1,
			material.Slider(slider.Theme,
				slider.Value,
				float32(slider.Min.Seconds()),
				float32(slider.Max.Seconds())).Layout,
		),
	)
}

type TimelineView struct {
	Theme *material.Theme

	*Timeline
	Visible []*SpanNode

	RowHeight unit.Value
	RowGap    unit.Value

	ZoomStart time.Duration
	ZoomEnd   time.Duration
}

func (view *TimelineView) Minimap(gtx layout.Context) layout.Dimensions {
	height := gtx.Px(unit.Dp(1)) * len(view.Visible)
	if smallestSize := gtx.Px(view.Theme.FingerSize); height < smallestSize {
		height = smallestSize
	}
	size := image.Point{
		X: gtx.Constraints.Max.X,
		Y: height,
	}

	rowHeight := int(float32(size.Y) / float32(len(view.Visible)))
	if rowHeight < 1 {
		rowHeight = 1
	}

	topY := 0

	durationToPx := float32(size.X) / float32(view.Duration)
	for _, span := range view.Visible {
		x0 := int(durationToPx * float32(span.StartTime-view.StartTime))
		x1 := x0 + int(math.Ceil(float64(durationToPx*float32(span.Duration))))

		paint.FillShape(gtx.Ops, view.SpanColor(span), clip.Rect{
			Min: image.Point{X: x0, Y: topY},
			Max: image.Point{X: x1, Y: topY + rowHeight},
		}.Op())

		topY += rowHeight
	}

	return layout.Dimensions{
		Size: size,
	}
}

func (view *TimelineView) Spans(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max

	rowHeight := gtx.Px(view.RowHeight)
	rowAdvance := rowHeight + gtx.Px(view.RowGap)

	topY := 0

	durationToPx := float32(size.X) / float32((view.ZoomEnd - view.ZoomStart).Microseconds())
	for _, node := range view.Visible {
		x0 := int(durationToPx * float32(node.StartTime-jaeger.Duration(view.ZoomStart.Microseconds())))
		x1 := int(math.Ceil(float64(durationToPx * float32(node.Duration))))

		paint.FillShape(gtx.Ops, view.SpanColor(node), clip.Rect{
			Min: image.Pt(x0, topY),
			Max: image.Pt(x1, topY+rowHeight),
		}.Op())

		topY += rowAdvance
	}

	return layout.Dimensions{
		Size: size,
	}
}

func (view *TimelineView) SpanColor(span *SpanNode) color.NRGBA {
	p := uintptr(unsafe.Pointer(span))
	return color.NRGBA{R: byte(p), G: byte(p >> 8), B: byte(p >> 16), A: 0xFF}
}

func (ui *UI) LayoutTimeline(gtx layout.Context) layout.Dimensions {
	topY := 0
	timelineWidth := gtx.Constraints.Max.X

	rowHeight := gtx.Px(unit.Sp(12))
	rowAdvance := rowHeight + gtx.Px(unit.Px(2))

	timeline := ui.Timeline
	durationToPx := float32(timelineWidth) / (ui.ZoomLevel.Value * float32(time.Second/time.Microsecond))

	for _, node := range timeline.RenderOrder {
		if node.Duration.Std().Seconds() < float64(ui.SkipSpans.Value) {
			continue
		}

		x0 := int(durationToPx * float32(node.StartTime-timeline.StartTime))
		x1 := x0 + int(math.Ceil(float64(durationToPx*float32(node.Duration))))

		paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, A: 0xFF}, clip.Rect{
			Min: image.Pt(x0, topY),
			Max: image.Pt(x1, topY+rowHeight),
		}.Op())

		topY += rowAdvance
	}

	return layout.Dimensions{
		Size: gtx.Constraints.Max,
	}
}

type Timeline struct {
	Traces   []jaeger.Trace
	NodeByID map[jaeger.TraceSpanID]*SpanNode

	StartTime jaeger.Duration
	Duration  jaeger.Duration

	RenderOrder []*SpanNode
}

type SpanNode struct {
	*jaeger.Span

	Parents  []*SpanNode
	Children []*SpanNode

	FollowsFrom []*SpanNode
	FollowedBy  []*SpanNode
}

func NewTimeline(traces []jaeger.Trace) *Timeline {
	nodeByID := make(map[jaeger.TraceSpanID]*SpanNode)

	var startTime jaeger.Duration = math.MaxInt64
	var endTime jaeger.Duration = math.MinInt64

	ensure := func(id jaeger.TraceSpanID, span *jaeger.Span) *SpanNode {
		node, ok := nodeByID[id]
		if !ok {
			if span == nil {
				span = &jaeger.Span{
					TraceSpanID: id,
				}
			}
			node = &SpanNode{
				Span: span,
			}
			nodeByID[id] = node
		} else {
			if span != nil {
				node.Span = span
			}
		}
		return node
	}

	roots := []*SpanNode{}
	for i := range traces {
		trace := &traces[i]
		for k := range trace.Spans {
			span := &trace.Spans[k]
			startTime = startTime.Min(span.StartTime)
			endTime = endTime.Max(span.StartTime + span.Duration)

			node := ensure(span.TraceSpanID, span)
			for _, ref := range span.References {
				switch ref.RefType {
				case jaeger.ChildOf:
					parent := ensure(ref.TraceSpanID, nil)
					parent.Children = append(parent.Children, node)
					node.Parents = append(node.Parents, parent)
				case jaeger.FollowsFrom:
					parent := ensure(ref.TraceSpanID, nil)
					parent.FollowedBy = append(parent.FollowedBy, node)
					node.FollowsFrom = append(node.FollowsFrom, parent)
				}
			}

			if len(span.References) == 0 {
				roots = append(roots, node)
			}
		}
	}

	sort.Slice(roots, func(i, k int) bool {
		a, b := roots[i], roots[k]
		if a.StartTime == b.StartTime {
			return a.Duration > b.Duration
		}
		return a.StartTime < b.StartTime
	})

	seen := make(map[jaeger.TraceSpanID]struct{})
	renderOrder := []*SpanNode{}
	var include func(node *SpanNode)
	include = func(node *SpanNode) {
		if _, ok := seen[node.TraceSpanID]; ok {
			return
		}
		seen[node.TraceSpanID] = struct{}{}
		renderOrder = append(renderOrder, node)
		for _, child := range node.Children {
			include(child)
		}
	}
	for _, root := range roots {
		include(root)
	}

	return &Timeline{
		Traces:   traces,
		NodeByID: nodeByID,

		StartTime: startTime,
		Duration:  endTime - startTime,

		RenderOrder: renderOrder,
	}
}
