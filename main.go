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
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"loov.dev/traceview/import/jaeger"
	"loov.dev/traceview/import/monkit"
	"loov.dev/traceview/trace"
	"loov.dev/traceview/tui"
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

		_, err := env.Run(ctx, func(cmds clingy.Commands) {
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

func (cmd *cmdMonkit) Setup(params clingy.Parameters) {
	cmd.source = params.Arg("trace", "trace file").(string)
}

func (cmd *cmdJaeger) Setup(params clingy.Parameters) {
	cmd.source = params.Arg("trace", "trace file").(string)
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
		w := new(app.Window)
		w.Option(app.Title("traceview"))
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

	SkipSpans tui.Duration
	ZoomLevel tui.Duration
	RowHeight tui.Px

	clickAlpha widget.Clickable
	clickBeta  widget.Clickable
	clickGamma widget.Clickable
}

func NewUI(timeline *trace.Timeline) *UI {
	ui := &UI{}
	ui.Theme = material.NewTheme()
	ui.Theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	ui.Timeline = timeline

	ui.SkipSpans.SetValue(100 * time.Millisecond)
	ui.ZoomLevel.SetValue(time.Second)
	ui.RowHeight.SetValue(12)
	return ui
}

func (ui *UI) Run(w *app.Window) error {
	var ops op.Ops

	for {
		e := w.Event()
		switch e := e.(type) {
		case app.FrameEvent:

			gtx := app.NewContext(&ops, e)
			ui.Layout(gtx)
			e.Frame(gtx.Ops)

		case key.Event:
			switch e.Name {
			case key.NameEscape:
				return nil
			}

		case app.DestroyEvent:
			return e.Err
		}
	}

	return nil
}

func (ui *UI) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{
		Axis: layout.Horizontal,
	}.Layout(gtx,
		layout.Rigid(ui.LayoutControls),
		layout.Flexed(1, ui.LayoutTimeline),
	)
}

func (ui *UI) LayoutTimeline(gtx layout.Context) layout.Dimensions {
	view := &TimelineView{
		Theme:    ui.Theme,
		Timeline: ui.Timeline,

		RowHeight:   unit.Dp(ui.RowHeight.Value),
		RowGap:      unit.Dp(1),
		SpanCaption: unit.Sp(ui.RowHeight.Value - 2),

		ZoomStart:  ui.Timeline.Start,
		ZoomFinish: ui.Timeline.Start + trace.NewTime(ui.ZoomLevel.Value),
	}

	for _, tr := range ui.Timeline.Traces {
		for _, span := range tr.Order {
			span.Visible = span.Duration().Std() > ui.SkipSpans.Value
			if !span.Visible {
				continue
			}
			view.Visible.Add(span)
		}
	}

	return layout.Flex{
		Axis: layout.Horizontal,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return tui.Box(color.NRGBA{0x40, 0x40, 0x48, 0xFF}).Layout(gtx, view.Spans)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return tui.ContentWidthStyle{
				MaxWidth: unit.Dp(100),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return tui.Box(color.NRGBA{R: 0x20, G: 0x20, B: 0x28, A: 0xFF}).Layout(gtx, view.Minimap)
			})
		}),
	)
}

func (ui *UI) LayoutControls(gtx layout.Context) layout.Dimensions {
	th := ui.Theme
	return tui.SidePanel(th).Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return tui.Panel(th, "Filter").Layout(gtx,
				tui.DurationEditor(th, &ui.SkipSpans, "Skip Spans", 0, 5*time.Second).Layout,
			)
		},
		func(gtx layout.Context) layout.Dimensions {
			return tui.Panel(th, "View").Layout(gtx,
				tui.DurationEditor(th, &ui.ZoomLevel, "Zoom", time.Second/10, nextSecond(ui.Timeline.Duration().Std())).Layout,
				tui.PxEditor(th, &ui.RowHeight, "Row Height", 6, 24).Layout,
			)
		},
	)
}

func nextSecond(s time.Duration) time.Duration {
	return time.Second * ((s + time.Second - 1) / time.Second)
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

type TimelineView struct {
	Theme *material.Theme

	*trace.Timeline
	Visible RenderOrder

	RowHeight   unit.Dp
	RowGap      unit.Dp
	SpanCaption unit.Sp

	ZoomStart  trace.Time
	ZoomFinish trace.Time
}

func (view *TimelineView) Minimap(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(1) * len(view.Visible.Rows)
	if smallestSize := gtx.Dp(view.Theme.FingerSize); height < smallestSize {
		height = smallestSize
	}
	size := image.Point{
		X: gtx.Constraints.Max.X,
		Y: height,
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()

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
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()

	rowHeight := gtx.Dp(view.RowHeight)
	rowAdvance := rowHeight

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
			if span.Finish < view.Start || view.Finish < span.Start {
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

					midx := float32(parent.Anchor.X+child.Anchor.X) / 2
					midy := float32(parent.Anchor.Y+child.Anchor.Y) / 2

					links.MoveTo(layout.FPt(parent.Anchor))
					links.CubeTo(
						f32.Point{X: float32(parent.Anchor.X), Y: midy},
						f32.Point{X: midx, Y: float32(child.Anchor.Y)},
						layout.FPt(child.Anchor),
					)
				}
			}
		}

		defer clip.Stroke{
			Path:  links.End(),
			Width: 2,
		}.Op().Push(gtx.Ops).Pop()

		paint.ColorOp{
			Color: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xAA},
		}.Add(gtx.Ops)

		paint.PaintOp{}.Add(gtx.Ops)
	}()

	return layout.Dimensions{
		Size: size,
	}
}

func (view *TimelineView) drawSpan(gtx layout.Context, span *trace.Span, bounds clip.Rect) {
	bg := view.SpanColor(span)
	defer bounds.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (view *TimelineView) drawSpanCaption(gtx layout.Context, span *trace.Span, bounds clip.Rect) {
	bg := view.SpanColor(span)
	defer bounds.Op().Push(gtx.Ops).Pop()
	paint.ColorOp{Color: bg}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	defer op.Offset(image.Point{X: bounds.Min.X + 2, Y: bounds.Min.Y}).Push(gtx.Ops).Pop()
	size := bounds.Max.Sub(bounds.Min)
	gtx.Constraints.Min = size
	gtx.Constraints.Max = size

	material.Label(view.Theme, view.SpanCaption, span.Caption).Layout(gtx)
}

func (view *TimelineView) SpanColor(span *trace.Span) color.NRGBA {
	p := int64(span.SpanID) ^ int64(span.TraceID)
	return color.NRGBA{R: byte(p), G: byte(p >> 8), B: byte(p >> 16), A: 0xFF}
}
