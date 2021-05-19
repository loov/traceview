package monkit

import (
	"loov.dev/traceview/trace"
)

func Convert(files ...File) (*trace.Timeline, error) {
	var timeline trace.Timeline

	traceByID := make(map[trace.TraceID]*trace.Trace)
	timeline.SpanByID = make(map[trace.TraceSpanID]*trace.Span)
	timeline.TimeRange = trace.InvalidRange

	// span may be nil
	ensure := func(spanID SpanID, traceID TraceID, span *Span) (*trace.Span, error) {
		id := trace.TraceSpanID{
			SpanID:  trace.SpanID(spanID),
			TraceID: trace.TraceID(traceID),
		}

		node, ok := timeline.SpanByID[id]
		if !ok {
			node = &trace.Span{}
			timeline.SpanByID[id] = node
		}
		updateSpanContent(node, id, span)

		if span != nil {
			tr, ok := traceByID[id.TraceID]
			if !ok {
				tr = &trace.Trace{
					TraceID:   id.TraceID,
					TimeRange: trace.InvalidRange,
				}
				timeline.Traces = append(timeline.Traces, tr)
				traceByID[id.TraceID] = tr
			}
			tr.Spans = append(tr.Spans, node)
			tr.TimeRange = tr.TimeRange.Expand(node.TimeRange)
		}

		return node, nil
	}

	for i := range files {
		file := files[i]
		for k := range file {
			span := &file[k]

			node, err := ensure(span.ID, span.Trace.ID, span)
			if err != nil {
				return nil, err
			}

			timeline.TimeRange = timeline.TimeRange.Expand(node.TimeRange)

			if span.ParentID != nil && TraceID(*span.ParentID) != span.Trace.ID {
				parent, err := ensure(*span.ParentID, span.Trace.ID, nil)
				if err != nil {
					return nil, err
				}
				parent.Children = append(parent.Children, node)
				node.Parents = append(node.Parents, parent)
			}
		}
	}

	timeline.Sort()

	return &timeline, nil
}

func updateSpanContent(node *trace.Span, id trace.TraceSpanID, span *Span) {
	if node.TraceSpanID.IsZero() {
		node.TraceSpanID = id
	}
	if span == nil {
		return
	}

	node.Caption = span.Func.Package + " " + span.Func.Name
	node.Start = span.Start.Time()
	node.Finish = span.Finish.Time()
}
