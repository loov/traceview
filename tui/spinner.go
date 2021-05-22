package tui

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type SpinnerStyle struct {
	Color color.NRGBA
}

func Spinner(col color.NRGBA) SpinnerStyle {
	return SpinnerStyle{
		Color: col,
	}
}

func (spin SpinnerStyle) Layout(gtx layout.Context) layout.Dimensions {
	defer op.Save(gtx.Ops).Load()

	dims := RoundBox(spin.Color).Layout(gtx,
		layout.Spacer{
			Height: unit.Dp(12),
			Width:  unit.Dp(4),
		}.Layout,
	)

	pointer.Rect(image.Rectangle{
		Max: dims.Size,
	}).Add(gtx.Ops)
	pointer.CursorNameOp{
		Name: pointer.CursorColResize,
	}.Add(gtx.Ops)
	pointer.InputOp{
		Tag:   1,
		Types: pointer.Drag,
	}.Add(gtx.Ops)

	return dims
}
