package jaeger

import (
	"fmt"
	"strconv"

	"loov.dev/traceview/trace"
)

func convertTags(tags []Tag) []trace.Tag {
	if len(tags) == 0 {
		return nil
	}
	out := make([]trace.Tag, len(tags))
	for i, t := range tags {
		out[i] = trace.Tag{
			Key:   t.Key,
			Value: fmt.Sprint(t.Value),
		}
	}
	return out
}

func convertLogs(logs []Log) []trace.Log {
	if len(logs) == 0 {
		return nil
	}
	out := make([]trace.Log, len(logs))
	for i, l := range logs {
		out[i] = trace.Log{
			Timestamp: l.Timestamp.Time(),
			Fields:    convertTags(l.Fields),
		}
	}
	return out
}

func Convert(traces ...Trace) (*trace.Timeline, error) {
	var timeline trace.Timeline

	traceByID := make(map[trace.TraceID]*trace.Trace)

	timeline.SpanByID = make(map[trace.TraceSpanID]*trace.Span)
	timeline.TimeRange = trace.InvalidRange

	// span may be nil
	ensure := func(refid TraceSpanID, span *Span, processes map[ProcessID]Process) (*trace.Span, error) {
		id, err := convertTraceSpanID(refid)
		if err != nil {
			return nil, err
		}

		node, ok := timeline.SpanByID[id]
		if !ok {
			node = &trace.Span{}
			timeline.SpanByID[id] = node
		}
		updateSpanContent(node, id, span, processes)

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

	for i := range traces {
		trace := &traces[i]
		for k := range trace.Spans {
			span := &trace.Spans[k]

			node, err := ensure(span.TraceSpanID, span, trace.Processes)
			if err != nil {
				return nil, err
			}

			timeline.TimeRange = timeline.TimeRange.Expand(node.TimeRange)

			for _, ref := range span.References {
				switch ref.RefType {
				case ChildOf:
					parent, err := ensure(ref.TraceSpanID, nil, nil)
					if err != nil {
						return nil, err
					}
					parent.Children = append(parent.Children, node)
					node.Parents = append(node.Parents, parent)
				case FollowsFrom:
					parent, err := ensure(ref.TraceSpanID, nil, nil)
					if err != nil {
						return nil, err
					}
					parent.FollowedBy = append(parent.FollowedBy, node)
					node.FollowsFrom = append(node.FollowsFrom, parent)
				}
			}
		}
	}

	timeline.Sort()
	return &timeline, nil
}

func updateSpanContent(node *trace.Span, id trace.TraceSpanID, span *Span, processes map[ProcessID]Process) {
	if node.TraceSpanID.IsZero() {
		node.TraceSpanID = id
	}
	if span == nil {
		return
	}

	node.Caption = span.OperationName
	node.Start = span.StartTime.Time()
	node.Finish = node.Start + span.Duration.Time()

	node.Tags = convertTags(span.Tags)
	node.Logs = convertLogs(span.Logs)

	if proc, ok := processes[span.ProcessID]; ok {
		node.Tags = append([]trace.Tag{{Key: "service", Value: proc.ServiceName}}, node.Tags...)
		node.Tags = append(node.Tags, convertTags(proc.Tags)...)
	}
	for _, w := range span.Warnings {
		node.Tags = append(node.Tags, trace.Tag{Key: "warning", Value: w})
	}
}

func convertTraceSpanID(id TraceSpanID) (trace.TraceSpanID, error) {
	traceID, err := convertHexID(string(id.TraceID))
	if err != nil {
		return trace.TraceSpanID{}, fmt.Errorf("invalid TraceID %q: %w", id.TraceID, err)
	}
	spanID, err := convertHexID(string(id.SpanID))
	if err != nil {
		return trace.TraceSpanID{}, fmt.Errorf("invalid SpanID %q: %w", id.TraceID, err)
	}

	return trace.TraceSpanID{
		TraceID: trace.TraceID(traceID),
		SpanID:  trace.SpanID(spanID),
	}, nil
}

// See https://www.jaegertracing.io/docs/1.22/client-libraries/#value
func convertHexID(v string) (int64, error) {
	return strconv.ParseInt(v, 16, 64)
}
