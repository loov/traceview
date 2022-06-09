package tui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type BoxStyle struct {
	Background color.NRGBA

	Gap Gap
}

func Box(col color.NRGBA) BoxStyle {
	return BoxStyle{
		Background: col,
		Gap:        Small,
	}
}

func (box BoxStyle) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := image.Point{
				X: gtx.Constraints.Min.X,
				Y: gtx.Constraints.Max.Y,
			}
			paint.FillShape(gtx.Ops, box.Background, clip.Rect{Max: size}.Op())
			return layout.Dimensions{
				Size: size,
			}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(box.Gap).Layout(gtx, w)
		}),
	)
}

type RoundBoxStyle struct {
	Background   color.NRGBA
	CornerRadius unit.Dp
	Padding      Gap
}

func RoundBox(bg color.NRGBA) RoundBoxStyle {
	return RoundBoxStyle{
		Background:   bg,
		CornerRadius: Small,
		Padding:      Small,
	}
}

func (b RoundBoxStyle) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	padding := gtx.Dp(b.Padding)

	gtx.Constraints.Min.X -= 2 * padding
	gtx.Constraints.Min.Y -= 2 * padding
	gtx.Constraints.Max.X -= 2 * padding
	gtx.Constraints.Max.Y -= 2 * padding

	gtx.Constraints.Min = positive(gtx.Constraints.Min)
	gtx.Constraints.Max = positive(gtx.Constraints.Max)

	rec := op.Record(gtx.Ops)
	stack := op.Offset(image.Point{X: padding, Y: padding}).Push(gtx.Ops)
	dims := w(gtx)
	stack.Pop()
	wrec := rec.Stop()

	sz := dims.Size

	rr := gtx.Dp(b.CornerRadius)
	sz.X += 2 * padding
	sz.Y += 2 * padding

	paint.FillShape(gtx.Ops,
		b.Background,
		clip.UniformRRect(image.Rectangle{Max: sz}, rr).Op(gtx.Ops),
	)
	wrec.Add(gtx.Ops)

	dims.Size.X += 2 * padding
	dims.Size.Y += 2 * padding

	return dims
}

func positive(v image.Point) image.Point {
	if v.X < 0 {
		v.X = 0
	}
	if v.Y < 0 {
		v.Y = 0
	}
	return v
}
