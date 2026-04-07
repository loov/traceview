package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/zeebo/clingy"

	"gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	tvfont "loov.dev/traceview/font"

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

func (cmd *cmdMonkit) Execute(ctx context.Context) error {
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

func (cmd *cmdJaeger) Execute(ctx context.Context) error {
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

type UI struct {
	Theme    *material.Theme
	Timeline *trace.Timeline

	SkipSpans tui.Duration
	ZoomLevel tui.Duration
	RowHeight tui.Px

	Viewport Viewport
	Selected *trace.Span
	Detail   DetailPanel
}

func NewUI(timeline *trace.Timeline) *UI {
	ui := &UI{}
	ui.Theme = material.NewTheme()
	ui.Theme.Shaper = text.NewShaper(text.WithCollection(tvfont.Collection()))
	ui.Timeline = timeline

	ui.SkipSpans.SetValue(100 * time.Millisecond)
	ui.ZoomLevel.SetValue(time.Second)
	ui.RowHeight.SetValue(12)

	ui.Detail = NewDetailPanel()

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
				if ui.Selected != nil {
					ui.Selected = nil
					w.Invalidate()
				} else {
					return nil
				}
			}

		case app.DestroyEvent:
			return e.Err
		}
	}
}

func (ui *UI) Layout(gtx layout.Context) layout.Dimensions {
	// Process click events early, before layout, so both
	// the timeline and detail panel see the same selection.
	ui.Viewport.Clicked = false
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: &ui.Viewport.clickTag,
			Kinds:  pointer.Press,
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
			ui.Viewport.ClickPos = e.Position
			ui.Viewport.Clicked = true
		}
	}

	return layout.Flex{
		Axis: layout.Horizontal,
	}.Layout(gtx,
		layout.Rigid(ui.LayoutControls),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, ui.LayoutTimeline),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ui.Detail.Span = ui.Selected
					return ui.Detail.Layout(gtx, ui.Theme)
				}),
			)
		}),
	)
}

func (ui *UI) LayoutTimeline(gtx layout.Context) layout.Dimensions {
	view := &TimelineView{
		UI:       ui,
		Theme:    ui.Theme,
		Timeline: ui.Timeline,

		RowHeight:   unit.Dp(ui.RowHeight.Value),
		RowGap:      unit.Dp(1),
		SpanCaption: unit.Sp(ui.RowHeight.Value - 2),

		ZoomStart:  ui.Timeline.Start + ui.Viewport.ZoomOffset,
		ZoomFinish: ui.Timeline.Start + ui.Viewport.ZoomOffset + trace.NewTime(ui.ZoomLevel.Value),
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
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(view.Ruler),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					size := gtx.Constraints.Max
					paint.FillShape(gtx.Ops, color.NRGBA{0x40, 0x40, 0x48, 0xFF}, clip.Rect{Max: size}.Op())
					return view.Spans(gtx)
				}),
			)
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
