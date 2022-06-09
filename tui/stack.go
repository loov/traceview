package tui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
)

type StackStyle struct {
	Gap Gap
}

func Stack(gap Gap) StackStyle {
	return StackStyle{
		Gap: gap,
	}
}

func (stack StackStyle) Layout(gtx layout.Context, ws ...layout.Widget) layout.Dimensions {
	defer op.Offset(image.Point{}).Push(gtx.Ops).Pop()

	gap := gtx.Dp(stack.Gap)
	dims := layout.Dimensions{
		Size: image.Point{X: gtx.Constraints.Max.X},
	}

	for i, w := range ws {
		wdims := w(gtx)
		dims.Size.Y += wdims.Size.Y

		if i+1 < len(ws) {
			dims.Size.Y += gap
			op.Offset(image.Point{Y: wdims.Size.Y + gap}).Add(gtx.Ops)
		}
	}

	return dims
}
