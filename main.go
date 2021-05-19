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
	_ "loov.dev/traceview/import/jaeger"
	_ "loov.dev/traceview/import/monkit"
	"loov.dev/traceview/trace"
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

	timeline, err := jaeger.Convert(tracefile.Data...)
	if err != nil {
		return fmt.Errorf("failed to convert jaeger %q: %w", infile, err)
	}

	ui := NewUI(timeline)
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

func NewUI(timeline trace.Timeline) *UI {
	ui := &UI{}
	ui.Theme = material.NewTheme(gofont.Collection())
	ui.Timeline = NewTimeline(timeline)

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

		ZoomStart:  ui.Timeline.Start,
		ZoomFinish: ui.Timeline.Finish,
	}

	for _, node := range ui.Timeline.RenderOrder {
		if node.Duration().Std().Seconds() < float64(ui.SkipSpans.Value) {
			continue
		}
		view.Visible = append(view.Visible, node)
	}

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(
			DurationSlider(ui.Theme, &ui.ZoomLevel, "Zoom", time.Microsecond, ui.Timeline.Duration().Std()).Layout),
		layout.Rigid(
			DurationSlider(ui.Theme, &ui.SkipSpans, "Skip", 0, 5*time.Second).Layout),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(8)).Layout(gtx, view.Minimap)
		}),

		layout.Flexed(1, view.Spans),
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
	Visible []*trace.Span

	RowHeight unit.Value
	RowGap    unit.Value

	ZoomStart  trace.Time
	ZoomFinish trace.Time
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

	durationToPx := float64(size.X) / float64(view.Duration())
	for _, span := range view.Visible {
		x0 := int(durationToPx * float64(span.Start-view.Start))
		x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.Start))))

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

	durationToPx := float64(size.X) / float64(view.ZoomFinish-view.ZoomStart)
	for _, span := range view.Visible {
		x0 := int(durationToPx * float64(span.Start-view.ZoomStart))
		x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.ZoomStart))))

		paint.FillShape(gtx.Ops, view.SpanColor(span), clip.Rect{
			Min: image.Pt(x0, topY),
			Max: image.Pt(x1, topY+rowHeight),
		}.Op())

		topY += rowAdvance
	}

	return layout.Dimensions{
		Size: size,
	}
}

func (view *TimelineView) SpanColor(span *trace.Span) color.NRGBA {
	p := int64(span.SpanID) ^ int64(span.TraceID)
	return color.NRGBA{R: byte(p), G: byte(p >> 8), B: byte(p >> 16), A: 0xFF}
}

type Timeline struct {
	trace.Timeline
	RenderOrder []*trace.Span
}

func NewTimeline(timeline trace.Timeline) *Timeline {
	roots := []*trace.Span{}

	for _, tr := range timeline.Traces {
		for _, span := range tr.Spans {
			if len(span.Parents) == 0 {
				roots = append(roots, span)
			}
		}
	}

	sort.Slice(roots, func(i, k int) bool {
		a, b := roots[i], roots[k]
		return a.TimeRange.Less(b.TimeRange)
	})

	seen := make(map[*trace.Span]struct{})
	renderOrder := []*trace.Span{}
	var include func(span *trace.Span)
	include = func(span *trace.Span) {
		if _, ok := seen[span]; ok {
			return
		}
		seen[span] = struct{}{}
		renderOrder = append(renderOrder, span)
		for _, child := range span.Children {
			include(child)
		}
	}
	for _, root := range roots {
		include(root)
	}

	return &Timeline{
		Timeline:    timeline,
		RenderOrder: renderOrder,
	}
}
