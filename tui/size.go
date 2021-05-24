package tui

import (
	"fmt"
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Px struct {
	Value  float32
	valid  bool
	spin   Spin
	editor widget.Editor
}

func (px *Px) SetValue(value float32) {
	if px.Value == value {
		return
	}

	px.Value = value
	px.valid = true
	px.editor.SetText(fmt.Sprintf("%.0f", value))
}

type PxEditorStyle struct {
	Caption material.LabelStyle
	Value   *Px
	Editor  material.EditorStyle

	Min float32
	Max float32
}

func PxEditor(theme *material.Theme, value *Px, caption string, min, max float32) PxEditorStyle {
	cap := material.Body2(theme, caption)
	cap.Color = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	cap.Alignment = text.End

	editor := material.Editor(theme, &value.editor, "")
	editor.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	editor.Font = cap.Font

	editor.TextSize = cap.TextSize.Scale(0.8)
	cap.TextSize = cap.TextSize.Scale(0.8)

	return PxEditorStyle{
		Caption: cap,
		Value:   value,
		Editor:  editor,

		Min: min,
		Max: max,
	}
}

func (edit PxEditorStyle) setValue(value float32) {
	if value < edit.Min {
		value = edit.Min
	}
	if value > edit.Max {
		value = edit.Max
	}

	edit.Value.SetValue(value)
}

func (edit PxEditorStyle) Layout(gtx layout.Context) layout.Dimensions {
	if edit.Value.spin.Dragging() && edit.Value.spin.Delta != 0 {
		edit.setValue(edit.Value.Value + edit.Value.spin.Delta*0.1)
	}

	return layout.Flex{
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
}
