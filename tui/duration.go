package tui

import (
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Duration struct {
	Value  time.Duration
	valid  bool
	editor widget.Editor
	spin   Spin
}

func (dur *Duration) SetValue(value time.Duration) {
	if dur.Value == value {
		return
	}
	dur.Value = value
	dur.valid = true
	dur.editor.SetText(value.String())
}

type DurationEditorStyle struct {
	Caption material.LabelStyle
	Value   *Duration
	Editor  material.EditorStyle

	Min time.Duration
	Max time.Duration
}

func DurationEditor(theme *material.Theme, value *Duration, caption string, min, max time.Duration) DurationEditorStyle {
	cap := material.Body2(theme, caption)
	cap.Color = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	cap.Alignment = text.End

	editor := material.Editor(theme, &value.editor, "")
	editor.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	editor.Font = cap.Font

	editor.TextSize = cap.TextSize.Scale(0.8)
	cap.TextSize = cap.TextSize.Scale(0.8)

	return DurationEditorStyle{
		Caption: cap,
		Value:   value,
		Editor:  editor,

		Min: min,
		Max: max,
	}
}

func (edit DurationEditorStyle) setValue(value time.Duration) {
	if value < edit.Min {
		value = edit.Min
	}
	if value > edit.Max {
		value = edit.Max
	}

	edit.Value.SetValue(value)
}

func (edit DurationEditorStyle) Layout(gtx layout.Context) layout.Dimensions {
	if edit.Value.spin.Dragging() && edit.Value.spin.Delta != 0 {
		changed := float64(edit.Value.Value) + float64(edit.Value.spin.Delta)*float64(time.Second/10)
		edit.setValue(time.Duration(changed))
	}

	dim := layout.Flex{
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Flexed(1, edit.Caption.Layout),
		layout.Rigid(layout.Spacer{Width: Small}.Layout),
		layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
			return RoundBox(color.NRGBA{0x40, 0x40, 0x40, 0xFF}).Layout(gtx.Disabled(), edit.Editor.Layout)
		}),
		layout.Rigid(layout.Spacer{Width: Small}.Layout),
		layout.Rigid(Spinner(color.NRGBA{0x60, 0x60, 0x60, 0xFF}, color.NRGBA{0x80, 0x80, 0x80, 0xFF}, &edit.Value.spin).Layout),
	)

	edit.setValue(edit.Value.Value)

	return dim
}
