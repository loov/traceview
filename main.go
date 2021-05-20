package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"os/signal"
	"time"

	"github.com/zeebo/clingy"

	"gioui.org/app"
	"gioui.org/f32"
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
	"loov.dev/traceview/import/monkit"
	"loov.dev/traceview/trace"
)

func main() {
	os.Exit(func() int {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		name := ""
		if len(os.Args) > 0 {
			name = os.Args[0]
		} else if exename, err := os.Executable(); err == nil {
			name = exename
		}

		env := clingy.Environment{
			Name: name,
			Args: os.Args[1:],
		}

		_, err := env.Run(ctx, func(cmds clingy.Commands, flags clingy.Flags) {
			cmds.New("jaeger", "load jaeger .json trace", new(cmdJaeger))
			cmds.New("monkit", "load monkit .json trace", new(cmdMonkit))
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}())
}

type cmdMonkit struct{ source string }
type cmdJaeger struct{ source string }

func (cmd *cmdMonkit) Setup(args clingy.Arguments, flags clingy.Flags) {
	cmd.source = args.New("trace", "trace file").(string)
}

func (cmd *cmdJaeger) Setup(args clingy.Arguments, flags clingy.Flags) {
	cmd.source = args.New("trace", "trace file").(string)
}

func (cmd *cmdMonkit) Execute(ctx clingy.Context) error {
	data, err := os.ReadFile(cmd.source)
	if err != nil {
		return fmt.Errorf("failed to read trace: %w", err)
	}

	var tracefile monkit.File
	err = json.Unmarshal(data, &tracefile)
	if err != nil {
		return fmt.Errorf("failed to parse file %q: %w", cmd.source, err)
	}

	timeline, err := monkit.Convert(tracefile)
	if err != nil {
		return fmt.Errorf("failed to convert monkit %q: %w", cmd.source, err)
	}

	return run(ctx, timeline)
}

func (cmd *cmdJaeger) Execute(ctx clingy.Context) error {
	data, err := os.ReadFile(cmd.source)
	if err != nil {
		return fmt.Errorf("failed to read trace: %w", err)
	}

	var tracefile jaeger.File
	err = json.Unmarshal(data, &tracefile)
	if err != nil {
		return fmt.Errorf("failed to parse file %q: %w", cmd.source, err)
	}

	timeline, err := jaeger.Convert(tracefile.Data...)
	if err != nil {
		return fmt.Errorf("failed to convert jaeger %q: %w", cmd.source, err)
	}

	return run(ctx, timeline)
}

func run(ctx context.Context, timeline *trace.Timeline) error {
	ui := NewUI(timeline)
	go func() {
		w := app.NewWindow(app.Title("traceview"))
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
	Timeline *trace.Timeline

	SkipSpans widget.Float
	ZoomLevel widget.Float
}

func NewUI(timeline *trace.Timeline) *UI {
	ui := &UI{}
	ui.Theme = material.NewTheme(gofont.Collection())
	ui.Timeline = timeline

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

		RowHeight:   ui.Theme.TextSize,
		RowGap:      unit.Px(1),
		SpanCaption: ui.Theme.TextSize.Scale(0.8),

		ZoomStart:  ui.Timeline.Start,
		ZoomFinish: ui.Timeline.Finish,
	}

	for _, tr := range ui.Timeline.Traces {
		for _, span := range tr.Order {
			span.Visible = span.Duration().Std().Seconds() > float64(ui.SkipSpans.Value)
			if !span.Visible {
				continue
			}
			view.Visible.Add(span)
		}
	}

	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(
			DurationSlider(ui.Theme, &ui.ZoomLevel, "Zoom", time.Microsecond, ui.Timeline.Duration().Std()).Layout),
		layout.Rigid(
			DurationSlider(ui.Theme, &ui.SkipSpans, "Skip", 0, 5*time.Second).Layout),

		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis: layout.Horizontal,
			}.Layout(gtx,
				layout.Flexed(1, view.Spans),
				layout.Rigid(view.Minimap),
			)
		}),
	)
}

type RenderOrder struct {
	Rows  []RenderSpan
	Spans []*trace.Span

	lastRow  *RenderSpan
	lastSpan *trace.Span
}

type RenderSpan struct {
	Low, High int
}

func (order *RenderOrder) Add(span *trace.Span) {
	order.Spans = append(order.Spans, span)
	if order.lastSpan == nil {
		order.Rows = append(order.Rows, RenderSpan{Low: 0, High: 1})
		order.lastSpan = span
		order.lastRow = &order.Rows[len(order.Rows)-1]
		return
	}

	if order.lastSpan.Finish <= span.Start {
		order.lastRow.High++
	} else {
		order.Rows = append(order.Rows, RenderSpan{
			Low:  len(order.Spans) - 1,
			High: len(order.Spans),
		})
		order.lastRow = &order.Rows[len(order.Rows)-1]
	}

	order.lastSpan = span
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

	*trace.Timeline
	Visible RenderOrder

	RowHeight   unit.Value
	RowGap      unit.Value
	SpanCaption unit.Value

	ZoomStart  trace.Time
	ZoomFinish trace.Time
}

func (view *TimelineView) Minimap(gtx layout.Context) layout.Dimensions {
	height := gtx.Px(unit.Dp(1)) * len(view.Visible.Rows)
	if smallestSize := gtx.Px(view.Theme.FingerSize); height < smallestSize {
		height = smallestSize
	}
	size := image.Point{
		X: gtx.Px(view.Theme.FingerSize) * 2,
		Y: height,
	}

	rowHeight := int(float32(size.Y) / float32(len(view.Visible.Rows)))
	if rowHeight < 1 {
		rowHeight = 1
	}

	topY := 0

	durationToPx := float64(size.X) / float64(view.Duration())
	for _, row := range view.Visible.Rows {
		for _, span := range view.Visible.Spans[row.Low:row.High] {
			x0 := int(durationToPx * float64(span.Start-view.Start))
			x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.Start))))

			view.drawSpan(gtx, span, clip.Rect{
				Min: image.Point{X: x0, Y: topY},
				Max: image.Point{X: x1, Y: topY + rowHeight},
			})
		}
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
	for _, row := range view.Visible.Rows {
		for _, span := range view.Visible.Spans[row.Low:row.High] {
			x0 := int(durationToPx * float64(span.Start-view.ZoomStart))
			x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.ZoomStart))))

			span.Anchor = image.Point{X: x0, Y: topY + rowHeight/2}

			if topY+rowHeight < 0 || gtx.Constraints.Max.Y < topY {
				continue
			}
			view.drawSpanCaption(gtx, span, clip.Rect{
				Min: image.Point{X: x0, Y: topY},
				Max: image.Point{X: x1, Y: topY + rowHeight},
			})
		}
		topY += rowAdvance
	}

	func() {
		defer op.Save(gtx.Ops).Load()

		var links clip.Path
		links.Begin(gtx.Ops)

		for _, row := range view.Visible.Rows {
			for _, parent := range view.Visible.Spans[row.Low:row.High] {
				if !parent.Visible {
					continue
				}
				for _, child := range parent.Children {
					if !child.Visible || parent.Anchor.Y > child.Anchor.Y {
						continue
					}

					links.MoveTo(layout.FPt(parent.Anchor))
					links.LineTo(layout.FPt(child.Anchor))
				}
			}
		}

		clip.Stroke{
			Path: links.End(),
			Style: clip.StrokeStyle{
				Width: 1,
			},
		}.Op().Add(gtx.Ops)

		paint.ColorOp{
			Color: color.NRGBA{R: 0, G: 0, B: 0, A: 0x80},
		}.Add(gtx.Ops)

		paint.PaintOp{}.Add(gtx.Ops)
	}()

	return layout.Dimensions{
		Size: size,
	}
}

func (view *TimelineView) drawSpan(gtx layout.Context, span *trace.Span, bounds clip.Rect) {
	defer op.Save(gtx.Ops).Load()

	bg := view.SpanColor(span)
	bounds.Op().Add(gtx.Ops)
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (view *TimelineView) drawSpanCaption(gtx layout.Context, span *trace.Span, bounds clip.Rect) {
	defer op.Save(gtx.Ops).Load()

	bg := view.SpanColor(span)
	bounds.Op().Add(gtx.Ops)
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	op.Offset(f32.Point{
		X: float32(bounds.Min.X) + 2,
		Y: float32(bounds.Min.Y),
	}).Add(gtx.Ops)
	size := bounds.Max.Sub(bounds.Min)
	gtx.Constraints.Min = size
	gtx.Constraints.Max = size

	material.Label(view.Theme, view.SpanCaption, span.Caption).Layout(gtx)
}

func (view *TimelineView) SpanColor(span *trace.Span) color.NRGBA {
	p := int64(span.SpanID) ^ int64(span.TraceID)
	return color.NRGBA{R: byte(p), G: byte(p >> 8), B: byte(p >> 16), A: 0xFF}
}
