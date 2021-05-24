package tui

import (
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type Spin struct {
	Axis  layout.Axis
	Delta float32
	last  float32
	drag  gesture.Drag
}

func (spin *Spin) Dragging() bool { return spin.drag.Dragging() }

func (spin *Spin) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max

	var de *pointer.Event
	for _, e := range spin.drag.Events(gtx.Metric, gtx, gesture.Axis(spin.Axis)) {
		if e.Type == pointer.Press || e.Type == pointer.Drag {
			de = &e
		}
	}

	if de != nil {
		if de.Type == pointer.Press {
			spin.last = de.Position.X
		}
		spin.Delta = de.Position.X - spin.last
		spin.last = de.Position.X
	}

	defer op.Save(gtx.Ops).Load()

	pointer.Rect(image.Rectangle{Max: size}).Add(gtx.Ops)
	pointer.CursorNameOp{
		Name: pointer.CursorColResize,
	}.Add(gtx.Ops)

	spin.drag.Add(gtx.Ops)

	return layout.Dimensions{Size: size}
}

type SpinnerStyle struct {
	Color  color.NRGBA
	Active color.NRGBA
	Spin   *Spin
}

func Spinner(col, active color.NRGBA, spin *Spin) SpinnerStyle {
	return SpinnerStyle{
		Color:  col,
		Active: active,
		Spin:   spin,
	}
}

func (spin SpinnerStyle) Layout(gtx layout.Context) layout.Dimensions {
	defer op.Save(gtx.Ops).Load()

	col := spin.Color
	if spin.Spin.Dragging() {
		col = spin.Active
	}

	dims := RoundBox(col).Layout(gtx,
		layout.Spacer{
			Height: unit.Dp(12),
			Width:  unit.Dp(4),
		}.Layout,
	)

	gtx.Constraints.Min = dims.Size
	gtx.Constraints.Max = dims.Size

	spin.Spin.Layout(gtx)

	return dims
}
