package tui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type ContentWidthStyle struct {
	MaxWidth unit.Dp
}

func ContentWidth(th *material.Theme) ContentWidthStyle {
	return ContentWidthStyle{
		MaxWidth: unit.Dp(th.TextSize * 15), // TODO:
	}
}

func (maxWidth ContentWidthStyle) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	max := gtx.Dp(maxWidth.MaxWidth)
	if gtx.Constraints.Max.X > max {
		gtx.Constraints.Max.X = max
	}
	if gtx.Constraints.Min.X > max {
		gtx.Constraints.Min.X = max
	}
	return w(gtx)
}
