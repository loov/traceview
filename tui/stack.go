package tui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

type StackStyle struct {
	Gap unit.Value
}

func Stack(gap Gap) StackStyle {
	return StackStyle{
		Gap: gap,
	}
}

func (stack StackStyle) Layout(gtx layout.Context, ws ...layout.Widget) layout.Dimensions {
	defer op.Save(gtx.Ops).Load()

	gap := gtx.Px(stack.Gap)
	dims := layout.Dimensions{
		Size: image.Point{X: gtx.Constraints.Max.X},
	}

	for i, w := range ws {
		wdims := w(gtx)
		dims.Size.Y += wdims.Size.Y

		if i+1 < len(ws) {
			dims.Size.Y += gap
			op.Offset(f32.Point{Y: float32(wdims.Size.Y)}).Add(gtx.Ops)
			op.Offset(f32.Point{Y: float32(gap)}).Add(gtx.Ops)
		}
	}

	return dims
}
