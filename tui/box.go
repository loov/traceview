package tui

import (
	"image"
	"image/color"

	"gioui.org/f32"
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
	CornerRadius unit.Value
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
	defer op.Save(gtx.Ops).Load()

	padding := gtx.Px(b.Padding)

	gtx.Constraints.Min.X -= 2 * padding
	gtx.Constraints.Min.Y -= 2 * padding
	gtx.Constraints.Max.X -= 2 * padding
	gtx.Constraints.Max.Y -= 2 * padding

	gtx.Constraints.Min = positive(gtx.Constraints.Min)
	gtx.Constraints.Max = positive(gtx.Constraints.Max)

	rec := op.Record(gtx.Ops)
	op.Offset(f32.Point{
		X: float32(padding),
		Y: float32(padding),
	}).Add(gtx.Ops)
	dims := w(gtx)
	wrec := rec.Stop()

	sz := layout.FPt(dims.Size)

	rr := float32(gtx.Px(b.CornerRadius))
	sz.X += float32(2 * padding)
	sz.Y += float32(2 * padding)

	r := f32.Rectangle{Max: sz}

	paint.FillShape(gtx.Ops,
		b.Background,
		clip.UniformRRect(r, rr).Op(gtx.Ops),
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
