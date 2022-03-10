package tui

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/widget/material"
)

type PanelStyle struct {
	Caption material.LabelStyle
}

func Panel(th *material.Theme, caption string) PanelStyle {
	cap := material.Body2(th, caption)
	cap.Color = color.NRGBA{R: 0xB0, G: 0xB0, B: 0xB4, A: 0xFF}
	return PanelStyle{
		Caption: cap,
	}
}

func (p PanelStyle) Layout(gtx layout.Context, ws ...layout.Widget) layout.Dimensions {
	bg := color.NRGBA{R: 0x30, G: 0x30, B: 0x38, A: 0xFF}
	return Stack(Small).Layout(gtx,
		layout.Spacer{Height: Small}.Layout,
		p.Caption.Layout,
		func(gtx layout.Context) layout.Dimensions {
			return RoundBox(bg).Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return Stack(Tiny).Layout(gtx, ws...)
				})
		},
	)
}

type SidePanelStyle struct {
	ContentWidth ContentWidthStyle
}

func SidePanel(th *material.Theme) SidePanelStyle {
	return SidePanelStyle{
		ContentWidth: ContentWidth(th),
	}
}

func (p SidePanelStyle) Layout(gtx layout.Context, ws ...layout.Widget) layout.Dimensions {
	bg := color.NRGBA{R: 0x20, G: 0x20, B: 0x28, A: 0xFF}
	return p.ContentWidth.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return Box(bg).Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return Stack(Medium).Layout(gtx, ws...)
				})
		})
}
