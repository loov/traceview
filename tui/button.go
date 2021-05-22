package tui

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type ButtonStyle struct {
	Text         string
	Color        color.NRGBA
	Font         text.Font
	TextSize     unit.Value
	Background   color.NRGBA
	CornerRadius unit.Value
	Inset        layout.Inset
	Button       *widget.Clickable
	shaper       text.Shaper
}

func Button(th *material.Theme, button *widget.Clickable, txt string) ButtonStyle {
	return ButtonStyle{
		Text:         txt,
		Color:        color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF},
		CornerRadius: Small,
		Background:   color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xFF},
		TextSize:     unit.Dp(12),
		Inset: layout.Inset{
			Top: Tiny, Bottom: Tiny,
			Left: Small, Right: Small,
		},
		Button: button,
		shaper: th.Shaper,
	}
}

func (b ButtonStyle) Layout(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return material.ButtonLayoutStyle{
		Background:   b.Background,
		CornerRadius: b.CornerRadius,
		Button:       b.Button,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return b.Inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			paint.ColorOp{Color: b.Color}.Add(gtx.Ops)
			return widget.Label{Alignment: text.Middle}.Layout(gtx, b.shaper, b.Font, b.TextSize, b.Text)
		})
	})
}
