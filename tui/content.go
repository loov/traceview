package tui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type ContentWidthStyle struct {
	MaxWidth unit.Value
}

func ContentWidth(th *material.Theme) ContentWidthStyle {
	return ContentWidthStyle{
		MaxWidth: th.TextSize.Scale(15),
	}
}

func (maxWidth ContentWidthStyle) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	max := gtx.Px(maxWidth.MaxWidth)
	if gtx.Constraints.Max.X > max {
		gtx.Constraints.Max.X = max
	}
	if gtx.Constraints.Min.X > max {
		gtx.Constraints.Min.X = max
	}
	return w(gtx)
}
