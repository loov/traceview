package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"loov.dev/traceview/trace"
	"loov.dev/traceview/tui"
)

// DetailPanel displays information about a selected span.
type DetailPanel struct {
	Span *trace.Span

	TagScroll widget.List
	LogScroll widget.List
}

func NewDetailPanel() DetailPanel {
	return DetailPanel{
		TagScroll: widget.List{List: layout.List{Axis: layout.Vertical}},
		LogScroll: widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

const detailPanelHeight = unit.Dp(150)

func (d *DetailPanel) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if d.Span == nil {
		return layout.Dimensions{}
	}

	bg := color.NRGBA{R: 0x20, G: 0x20, B: 0x28, A: 0xFF}
	borderColor := color.NRGBA{R: 0x50, G: 0x50, B: 0x58, A: 0xFF}

	height := gtx.Dp(detailPanelHeight)
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height

	span := d.Span

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := image.Point{X: gtx.Constraints.Max.X, Y: height}
			paint.FillShape(gtx.Ops, borderColor, clip.Rect{Max: image.Point{X: size.X, Y: 1}}.Op())
			paint.FillShape(gtx.Ops, bg, clip.Rect{Min: image.Point{Y: 1}, Max: size}.Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceEnd}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body1(th, span.Caption)
								lbl.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								dur := formatDuration(span.Duration().Std())
								info := fmt.Sprintf("Duration: %s  |  Children: %d  |  Parents: %d",
									dur, len(span.Children), len(span.Parents))
								lbl := material.Caption(th, info)
								lbl.Color = color.NRGBA{R: 0xA0, G: 0xA0, B: 0xA8, A: 0xFF}
								return lbl.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(24)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return d.layoutTags(gtx, th, span.Tags)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(24)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return d.layoutLogs(gtx, th, span.Logs)
					}),
				)
			})
		}),
	)
}

func (d *DetailPanel) layoutTags(gtx layout.Context, th *material.Theme, tags []trace.Tag) layout.Dimensions {
	if len(tags) == 0 {
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Tags")
			lbl.Color = color.NRGBA{R: 0xB0, G: 0xB0, B: 0xB4, A: 0xFF}
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: tui.Tiny}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &d.TagScroll).Layout(gtx, len(tags), func(gtx layout.Context, i int) layout.Dimensions {
				tag := tags[i]
				lbl := material.Caption(th, tag.Key+": "+tag.Value)
				lbl.Color = color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
	)
}

func (d *DetailPanel) layoutLogs(gtx layout.Context, th *material.Theme, logs []trace.Log) layout.Dimensions {
	if len(logs) == 0 {
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, "Logs")
			lbl.Color = color.NRGBA{R: 0xB0, G: 0xB0, B: 0xB4, A: 0xFF}
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: tui.Tiny}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &d.LogScroll).Layout(gtx, len(logs), func(gtx layout.Context, i int) layout.Dimensions {
				log := logs[i]
				text := formatDuration(log.Timestamp.Std())
				for _, f := range log.Fields {
					text += " " + f.Key + "=" + f.Value
				}
				lbl := material.Caption(th, text)
				lbl.Color = color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			})
		}),
	)
}
