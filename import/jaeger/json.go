package jaeger

import "time"

type File struct {
	Data []Trace `json:"data"`
}

type TraceID string
type SpanID string
type ProcessID string

type TraceSpanID struct {
	TraceID TraceID `json:"traceID"`
	SpanID  SpanID  `json:"spanID"`
}

type Duration int64 // in microseconds

func (a Duration) Min(b Duration) Duration {
	if a < b {
		return a
	}
	return b
}

func (a Duration) Max(b Duration) Duration {
	if a > b {
		return a
	}
	return b
}

func (d Duration) Std() time.Duration {
	return time.Duration(d) * time.Microsecond
}

type Flags int32

const (
	SampledFlag = Flags(0b01)
	DebugFlag   = Flags(0b10)
)

type Trace struct {
	TraceID   TraceID               `json:"traceID"`
	Spans     []Span                `json:"spans"`
	Processes map[ProcessID]Process `json:"processes"`
}

type Span struct {
	TraceSpanID
	Flags         Flags     `json:"flags"`
	OperationName string    `json:"operationName"`
	References    []SpanRef `json:"references"`
	StartTime     Duration  `json:"startTime"`
	Duration      Duration  `json:"duration"`
	Tags          []Tag     `json:"tags"`
	Logs          []Log     `json:"logs"`
	ProcessID     ProcessID `json:"processID"`
	Warnings      []string  `json:"warnings,omitempty"`
}

type Log struct {
	Timestamp Duration
	Fields    []Tag
}

type SpanRef struct {
	RefType SpanRefType `json:"refType"`
	TraceSpanID
}

type SpanRefType string

const (
	ChildOf     = SpanRefType("CHILD_OF")
	FollowsFrom = SpanRefType("FOLLOWS_FROM")
)

type Process struct {
	ServiceName string `json:"serviceName"`
	Tags        []Tag  `json:"tags"`
}

type Tag struct {
	Type  TagType     `json:"type"`
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type TagType string

const (
	StringTag = TagType("string")
)
