package trace

import (
	"image"
	"math"
	"sort"
)

type Timeline struct {
	Traces   []*Trace
	SpanByID map[TraceSpanID]*Span
	TimeRange
}

type TraceID int64
type SpanID int64

type TraceSpanID struct {
	TraceID TraceID
	SpanID  SpanID
}

func (id TraceID) IsZero() bool     { return id == 0 }
func (id SpanID) IsZero() bool      { return id == 0 }
func (id TraceSpanID) IsZero() bool { return id == TraceSpanID{} }

type Trace struct {
	TraceID
	TimeRange

	Spans []*Span
	Order []*Span
}

type Span struct {
	TraceSpanID

	Caption string
	TimeRange

	Parents  []*Span
	Children []*Span

	FollowsFrom []*Span
	FollowedBy  []*Span

	Visible bool
	Anchor  image.Point
}

type TimeRange struct {
	Start  Time
	Finish Time
}

var InvalidRange = TimeRange{
	Start:  math.MaxInt64,
	Finish: math.MinInt64,
}

func (a TimeRange) Duration() Time {
	return a.Finish - a.Start
}

func (a TimeRange) Less(b TimeRange) bool {
	if a.Start == b.Start {
		return a.Finish < b.Finish
	}
	return a.Start < b.Start
}

func (a TimeRange) Expand(b TimeRange) TimeRange {
	return TimeRange{
		Start:  a.Start.Min(b.Start),
		Finish: a.Finish.Max(b.Finish),
	}
}

func (timeline *Timeline) Sort() {
	sort.Slice(timeline.Traces, func(i, k int) bool {
		a := timeline.Traces[i]
		b := timeline.Traces[k]
		return a.TimeRange.Less(b.TimeRange)
	})

	for _, t := range timeline.Traces {
		sort.Slice(t.Spans, func(i, k int) bool {
			a := t.Spans[i]
			b := t.Spans[k]
			return a.TimeRange.Less(b.TimeRange)
		})

		roots := []*Span{}
		for _, span := range t.Spans {
			if len(span.Parents) == 0 {
				roots = append(roots, span)
			}
		}

		t.Order = []*Span{}
		seen := make(map[*Span]struct{})
		var include func(*Span)
		include = func(span *Span) {
			if _, ok := seen[span]; ok {
				return
			}
			t.Order = append(t.Order, span)
			for _, child := range span.Children {
				include(child)
			}
		}
		for _, root := range roots {
			include(root)
		}
	}

}
