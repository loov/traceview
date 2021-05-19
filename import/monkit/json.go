package monkit

import "time"

type File []Span

type Span struct {
	ID          SpanID       `json:"id"`
	ParentID    *SpanID      `json:"parent_id,omitempty"`
	Func        Func         `json:"func"`
	Trace       Trace        `json:"trace"`
	Start       UnixNano     `json:"start"`
	Finish      UnixNano     `json:"finish"`
	Orphaned    bool         `json:"orphaned"`
	Err         string       `json:"err"`
	Panicked    bool         `json:"panicked"`
	Args        []string     `json:"args"`
	Annotations []Annotation `json:"annotations"`
}

type Trace struct {
	ID TraceID `json:"id"`
}

type SpanID int64
type TraceID int64

type UnixNano int64

func (n UnixNano) Duration() time.Duration { return time.Duration(n) }

type Func struct {
	Package string `json:"package"`
	Name    string `json:"name"`
}

type Annotation [2]string
