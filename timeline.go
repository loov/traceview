package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"loov.dev/traceview/trace"
)

// Viewport tracks scroll and zoom state for the timeline view.
type Viewport struct {
	ScrollY        int
	ScrollX        int
	scrollTag      bool
	clickTag       bool
	ZoomOffset     trace.Time
	SpansViewportH int

	Clicked  bool
	ClickPos f32.Point
}

// RenderOrder groups visible spans into non-overlapping rows for rendering.
type RenderOrder struct {
	Rows  []RenderSpan
	Spans []*trace.Span

	lastRow  *RenderSpan
	lastSpan *trace.Span
}

// RenderSpan defines a range of spans within a single row.
type RenderSpan struct {
	Low, High int
}

func (order *RenderOrder) Add(span *trace.Span) {
	order.Spans = append(order.Spans, span)
	if order.lastSpan == nil {
		order.Rows = append(order.Rows, RenderSpan{Low: 0, High: 1})
		order.lastSpan = span
		order.lastRow = &order.Rows[len(order.Rows)-1]
		return
	}

	if order.lastSpan.Finish <= span.Start {
		order.lastRow.High++
	} else {
		order.Rows = append(order.Rows, RenderSpan{
			Low:  len(order.Spans) - 1,
			High: len(order.Spans),
		})
		order.lastRow = &order.Rows[len(order.Rows)-1]
	}

	order.lastSpan = span
}

// TimelineView holds the state needed for a single frame of timeline rendering.
type TimelineView struct {
	UI    *UI
	Theme *material.Theme

	*trace.Timeline
	Visible RenderOrder

	RowHeight   unit.Dp
	RowGap      unit.Dp
	SpanCaption unit.Sp

	ZoomStart  trace.Time
	ZoomFinish trace.Time
}

func (view *TimelineView) Minimap(gtx layout.Context) layout.Dimensions {
	height := gtx.Dp(1) * len(view.Visible.Rows)
	if smallestSize := gtx.Dp(view.Theme.FingerSize); height < smallestSize {
		height = smallestSize
	}
	size := image.Point{
		X: gtx.Constraints.Max.X,
		Y: height,
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()

	rowHeight := max(int(float32(size.Y)/float32(len(view.Visible.Rows))), 1)

	topY := 0

	durationToPx := float64(size.X) / float64(view.Duration())
	for _, row := range view.Visible.Rows {
		for _, span := range view.Visible.Spans[row.Low:row.High] {
			x0 := int(durationToPx * float64(span.Start-view.Start))
			x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.Start))))

			view.drawSpan(gtx, span, clip.Rect{
				Min: image.Point{X: x0, Y: topY},
				Max: image.Point{X: x1, Y: topY + rowHeight},
			})
		}
		topY += rowHeight
	}

	// Draw visible region overlay.
	{
		x0 := int(durationToPx * float64(view.ZoomStart-view.Start))
		x1 := int(math.Ceil(durationToPx * float64(view.ZoomFinish-view.Start)))
		if x0 < 0 {
			x0 = 0
		}
		if x1 > size.X {
			x1 = size.X
		}

		// Compute vertical visible region.
		totalRows := len(view.Visible.Rows)
		spansRowHeight := gtx.Dp(view.RowHeight)
		totalContentH := totalRows * spansRowHeight
		y0, y1 := 0, size.Y
		if totalContentH > 0 {
			scale := float64(size.Y) / float64(totalContentH)
			y0 = int(float64(view.UI.Viewport.ScrollY) * scale)
			y1 = int(float64(view.UI.Viewport.ScrollY+view.UI.Viewport.SpansViewportH) * scale)
			if y0 < 0 {
				y0 = 0
			}
			if y1 > size.Y {
				y1 = size.Y
			}
		}

		overlay := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x20}
		paint.FillShape(gtx.Ops, overlay, clip.Rect{
			Min: image.Point{X: x0, Y: y0},
			Max: image.Point{X: x1, Y: y1},
		}.Op())
		// Draw edges.
		border := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x60}
		for _, r := range []clip.Rect{
			{Min: image.Point{X: x0, Y: y0}, Max: image.Point{X: x0 + 1, Y: y1}},
			{Min: image.Point{X: x1 - 1, Y: y0}, Max: image.Point{X: x1, Y: y1}},
			{Min: image.Point{X: x0, Y: y0}, Max: image.Point{X: x1, Y: y0 + 1}},
			{Min: image.Point{X: x0, Y: y1 - 1}, Max: image.Point{X: x1, Y: y1}},
		} {
			paint.FillShape(gtx.Ops, border, r.Op())
		}
	}

	return layout.Dimensions{
		Size: size,
	}
}

// tickLayout computes tick positions for the current zoom window.
func (view *TimelineView) tickLayout(width int) (tickNs, firstTick trace.Time, pxPerNs float64) {
	zoomDuration := view.ZoomFinish - view.ZoomStart
	if zoomDuration <= 0 {
		return 0, 0, 0
	}

	pxPerNs = float64(width) / float64(zoomDuration)
	minTickNs := float64(80) / pxPerNs

	interval := tickIntervals[len(tickIntervals)-1]
	for _, ti := range tickIntervals {
		if float64(ti.Nanoseconds()) >= minTickNs {
			interval = ti
			break
		}
	}

	tickNs = trace.NewTime(interval)
	firstTick = ((view.ZoomStart - view.Timeline.Start) / tickNs) * tickNs
	if firstTick < view.ZoomStart-view.Timeline.Start {
		firstTick += tickNs
	}
	return tickNs, firstTick, pxPerNs
}

func (view *TimelineView) tickPx(t trace.Time, pxPerNs float64) int {
	absTime := view.Timeline.Start + t
	return int(float64(absTime-view.ZoomStart) * pxPerNs)
}

func (view *TimelineView) Ruler(gtx layout.Context) layout.Dimensions {
	rulerHeight := gtx.Dp(unit.Dp(24))
	size := image.Point{X: gtx.Constraints.Max.X, Y: rulerHeight}

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, color.NRGBA{0x30, 0x30, 0x38, 0xFF}, clip.Rect{Max: size}.Op())

	tickNs, firstTick, pxPerNs := view.tickLayout(size.X)
	if tickNs <= 0 {
		return layout.Dimensions{Size: size}
	}

	tickColor := color.NRGBA{R: 0x80, G: 0x80, B: 0x88, A: 0xFF}
	labelColor := color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}

	// Draw a baseline.
	paint.FillShape(gtx.Ops, tickColor, clip.Rect{
		Min: image.Point{X: 0, Y: rulerHeight - 1},
		Max: image.Point{X: size.X, Y: rulerHeight},
	}.Op())

	for t := firstTick; t <= view.ZoomFinish-view.Timeline.Start; t += tickNs {
		px := view.tickPx(t, pxPerNs)
		if px < 0 || px >= size.X {
			continue
		}

		// Draw tick mark.
		paint.FillShape(gtx.Ops, tickColor, clip.Rect{
			Min: image.Point{X: px, Y: rulerHeight - 6},
			Max: image.Point{X: px + 1, Y: rulerHeight},
		}.Op())

		// Draw label.
		func() {
			defer op.Offset(image.Point{X: px + 3, Y: 2}).Push(gtx.Ops).Pop()
			lbl := material.Label(view.Theme, unit.Sp(10), formatDuration(t.Std()))
			lbl.Color = labelColor
			lbl.Layout(gtx)
		}()
	}

	return layout.Dimensions{Size: size}
}

func (view *TimelineView) drawGridLines(gtx layout.Context, size image.Point) {
	tickNs, firstTick, pxPerNs := view.tickLayout(size.X)
	if tickNs <= 0 {
		return
	}

	gridColor := color.NRGBA{R: 0x50, G: 0x50, B: 0x58, A: 0xFF}

	for t := firstTick; t <= view.ZoomFinish-view.Timeline.Start; t += tickNs {
		px := view.tickPx(t, pxPerNs)
		if px < 0 || px >= size.X {
			continue
		}
		paint.FillShape(gtx.Ops, gridColor, clip.Rect{
			Min: image.Point{X: px, Y: 0},
			Max: image.Point{X: px + 1, Y: size.Y},
		}.Op())
	}
}

func (view *TimelineView) Spans(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()

	view.drawGridLines(gtx, size)
	view.UI.Viewport.SpansViewportH = size.Y

	totalRows := len(view.Visible.Rows)
	rowHeight := gtx.Dp(view.RowHeight)
	rowAdvance := rowHeight
	totalHeight := totalRows * rowAdvance

	// Handle scroll events for both axes.
	event.Op(gtx.Ops, &view.UI.Viewport.scrollTag)
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target:  &view.UI.Viewport.scrollTag,
			Kinds:   pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -size.X, Max: size.X},
			ScrollY: pointer.ScrollRange{Min: -totalHeight, Max: totalHeight},
		})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Scroll {
			view.UI.Viewport.ScrollY += int(e.Scroll.Y)
			view.UI.Viewport.ScrollX += int(e.Scroll.X)
		}
	}

	// Register for click events (processed in UI.Layout).
	event.Op(gtx.Ops, &view.UI.Viewport.clickTag)

	// Clamp vertical scroll.
	maxScrollY := totalHeight - size.Y
	if maxScrollY < 0 {
		maxScrollY = 0
	}
	view.UI.Viewport.ScrollY = max(0, min(view.UI.Viewport.ScrollY, maxScrollY))

	// Apply horizontal scroll as zoom offset.
	zoomDuration := view.ZoomFinish - view.ZoomStart
	pxToDuration := float64(zoomDuration) / float64(size.X)
	view.UI.Viewport.ZoomOffset += trace.Time(float64(view.UI.Viewport.ScrollX) * pxToDuration)
	view.UI.Viewport.ScrollX = 0

	// Clamp zoom offset.
	maxOffset := view.UI.Timeline.Finish - view.UI.Timeline.Start - trace.NewTime(view.UI.ZoomLevel.Value)
	if maxOffset < 0 {
		maxOffset = 0
	}
	if view.UI.Viewport.ZoomOffset < 0 {
		view.UI.Viewport.ZoomOffset = 0
	}
	if view.UI.Viewport.ZoomOffset > maxOffset {
		view.UI.Viewport.ZoomOffset = maxOffset
	}
	view.ZoomStart = view.UI.Timeline.Start + view.UI.Viewport.ZoomOffset
	view.ZoomFinish = view.ZoomStart + trace.NewTime(view.UI.ZoomLevel.Value)

	topY := -view.UI.Viewport.ScrollY
	durationToPx := float64(size.X) / float64(view.ZoomFinish-view.ZoomStart)

	// Hit-test click against spans.
	if view.UI.Viewport.Clicked {
		prev := view.UI.Selected
		view.UI.Selected = nil
		cx, cy := int(view.UI.Viewport.ClickPos.X), int(view.UI.Viewport.ClickPos.Y)
		hitY := topY
		for _, row := range view.Visible.Rows {
			for _, span := range view.Visible.Spans[row.Low:row.High] {
				x0 := int(durationToPx * float64(span.Start-view.ZoomStart))
				x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.ZoomStart))))
				if cx >= x0 && cx < x1 && cy >= hitY && cy < hitY+rowHeight {
					view.UI.Selected = span
				}
			}
			hitY += rowAdvance
		}
		if view.UI.Selected != prev {
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	for _, row := range view.Visible.Rows {
		for _, span := range view.Visible.Spans[row.Low:row.High] {
			x0 := int(durationToPx * float64(span.Start-view.ZoomStart))
			x1 := int(math.Ceil(float64(durationToPx * float64(span.Finish-view.ZoomStart))))

			span.Anchor = image.Point{X: x0, Y: topY + rowHeight/2}

			if topY+rowHeight < 0 || size.Y < topY {
				continue
			}
			if span.Finish < view.ZoomStart || view.ZoomFinish < span.Start {
				continue
			}
			view.drawSpanCaption(gtx, span, clip.Rect{
				Min: image.Point{X: x0, Y: topY},
				Max: image.Point{X: x1, Y: topY + rowHeight},
			})
		}
		topY += rowAdvance
	}

	func() {
		var links clip.Path
		links.Begin(gtx.Ops)

		for _, row := range view.Visible.Rows {
			for _, parent := range view.Visible.Spans[row.Low:row.High] {
				if !parent.Visible {
					continue
				}
				// Skip parents entirely off-screen.
				if parent.Anchor.Y+rowHeight < 0 || size.Y < parent.Anchor.Y-rowHeight {
					continue
				}
				if parent.Finish < view.ZoomStart || view.ZoomFinish < parent.Start {
					continue
				}
				for _, child := range parent.Children {
					if !child.Visible || parent.Anchor.Y > child.Anchor.Y {
						continue
					}
					// Skip links where both endpoints are off-screen in the same direction.
					if child.Anchor.Y < 0 {
						continue
					}
					if parent.Anchor.Y > size.Y {
						continue
					}

					midx := float32(parent.Anchor.X+child.Anchor.X) / 2
					midy := float32(parent.Anchor.Y+child.Anchor.Y) / 2

					links.MoveTo(layout.FPt(parent.Anchor))
					links.CubeTo(
						f32.Point{X: float32(parent.Anchor.X), Y: midy},
						f32.Point{X: midx, Y: float32(child.Anchor.Y)},
						layout.FPt(child.Anchor),
					)
				}
			}
		}

		paint.FillShape(gtx.Ops,
			color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xAA},
			clip.Stroke{Path: links.End(), Width: 2}.Op(),
		)
	}()

	return layout.Dimensions{
		Size: size,
	}
}

func (view *TimelineView) drawSpan(gtx layout.Context, span *trace.Span, bounds clip.Rect) {
	paint.FillShape(gtx.Ops, spanColor(int64(span.SpanID), int64(span.TraceID)), bounds.Op())
}

func (view *TimelineView) drawSpanCaption(gtx layout.Context, span *trace.Span, bounds clip.Rect) {
	bg := spanColor(int64(span.SpanID), int64(span.TraceID))
	if view.UI.Selected == span {
		bg = brighten(bg)
	}
	paint.FillShape(gtx.Ops, bg, bounds.Op())

	// Draw selection border.
	if view.UI.Selected == span {
		border := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xCC}
		b := bounds
		paint.FillShape(gtx.Ops, border, clip.Rect{Min: b.Min, Max: image.Point{X: b.Max.X, Y: b.Min.Y + 1}}.Op())
		paint.FillShape(gtx.Ops, border, clip.Rect{Min: image.Point{X: b.Min.X, Y: b.Max.Y - 1}, Max: b.Max}.Op())
		paint.FillShape(gtx.Ops, border, clip.Rect{Min: b.Min, Max: image.Point{X: b.Min.X + 1, Y: b.Max.Y}}.Op())
		paint.FillShape(gtx.Ops, border, clip.Rect{Min: image.Point{X: b.Max.X - 1, Y: b.Min.Y}, Max: b.Max}.Op())
	}

	defer bounds.Op().Push(gtx.Ops).Pop()

	defer op.Offset(image.Point{X: bounds.Min.X + 2, Y: bounds.Min.Y}).Push(gtx.Ops).Pop()
	size := bounds.Max.Sub(bounds.Min)
	gtx.Constraints.Min = size
	gtx.Constraints.Max = size

	label := material.Label(view.Theme, view.SpanCaption, span.Caption)
	label.Color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xDD}
	label.Layout(gtx)
}
