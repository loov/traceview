package tef

// This package implements
// https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU/preview?tab=t.0#heading=h.yr4qxyxotyw

/*
{
  "traceEvents": [
    {"name": "Asub", "cat": "PERF", "ph": "B", "pid": 22630, "tid": 22630, "ts": 829},
    {"name": "Asub", "cat": "PERF", "ph": "E", "pid": 22630, "tid": 22630, "ts": 833}
  ],
  "displayTimeUnit": "ns",
  "systemTraceEvents": "SystemTraceData",
  "otherData": {
    "version": "My Application v1.0"
  },
  "stackFrames": {...}
  "samples": [...],
}
*/

type File struct {
	TraceEvents []Event `json:"traceEvents"`
	// If provided displayTimeUnit is a string that specifies in which unit timestamps should be displayed.
	// This supports values of “ms” or “ns”. By default this is value is “ms”.
	DisplayTimeUnit string `json:"displayTimeUnit"`
	// If provided systemTraceEvents is a string of Linux ftrace data or Windows ETW trace data.
	// This data must start with # tracer: and adhere to the Linux ftrace format or adhere to Windows ETW format.
	SystemTraceEvents string `json:"systemTraceEvents"`
	// If provided, the stackFrames field is a dictionary of stack frames, their ids,
	// and their parents that allows compact representation of stack traces throughout
	// the rest of the trace file. It is optional but sometimes very useful in shrinking file sizes.
	StackFrames map[string]StackFrame `json:"stackFrames"`
	// The samples array is used to store sampling profiler data from a OS level profiler.
	// It stores samples that are different from trace event samples, and is meant to augment the
	// traceEvent data with lower level information. It is OK to have a trace event file with just
	// sample data, but in that case  traceEvents must still be provided and set to [].
	// For more information on sample data, refer to the global samples section.
	Samples []Sample `json:"samples"`
	// Any other properties seen in the object, in this case otherData are assumed to be metadata for the trace.
	// They will be collected and stored in an array in the trace model. This metadata is accessible through the Metadata button in Trace Viewer.
	OtherData map[string]any `json:"otherData"`
}

/*
{
  "name": "myName",
  "cat": "category,list",
  "ph": "B",
  "ts": 12345,
  "pid": 123,
  "tid": 456,
  "args": {
    "someArg": 1,
    "anotherArg": {
      "value": "my value"
    }
  }
}
*/

type Event struct {
	// ID is a unique identifier for async events.
	ID string `json:"id,omitempty"`
	// The name of the event, as displayed in Trace Viewer
	Name string `json:"name"`
	// The event categories. This is a comma separated list of categories for the event.
	// The categories can be used to hide events in the Trace Viewer UI.
	Category string `json:"cat"`
	// The event type. This is a single character which changes depending on the type of
	// event being output. The valid values are listed in the table below. We will discuss each phase type below.
	Phase string `json:"ph"`
	// The tracing clock timestamp of the event. The timestamps are provided at microsecond granularity.
	Timestamp int64 `json:"ts"`
	// Optional. The thread clock timestamp of the event. The timestamps are provided at microsecond granularity.
	ThreadTimestamp int64 `json:"ts"`
	// The process ID for the process that output this event.
	ProcessID int64 `json:"pid"`
	// The thread ID for the thread that output this event.
	ThreadID int64 `json:"tid"`

	// StackFrame is a reference to StackFrames map.
	StackFrame string `json:"sf"`
	// Stack can be used instead of StackFrame to provide raw frames.
	Stack []string `json:"stack,omitzero"`

	// Any arguments provided for the event. Some of the event types have required argument fields,
	// otherwise, you can put any information you wish in here. The arguments are displayed in
	// Trace Viewer when you view an event in the analysis section.
	Args map[string]any `json:"args"`

	// Duration specifies the duration for Complete events.
	Duration int64 `json:"dur,omitzero"`
	// EndStackFrame is a reference to StackFrames map. Only relevant to Complete event.
	EndStackFrame string `json:"esf,omitempty"`
	// EndStack can be used instead of StackFrame to provide raw frames. Only relevant to Complete event.
	EndStack []string `json:"estack,omitzero"`
}

type Phase string

const (
	DurationBegin Phase = "B"
	DurationEnd   Phase = "E"
	Complete      Phase = "X"
	Instant       Phase = "i"
	Counter       Phase = "C"

	AsyncStart   Phase = "b"
	AsyncInstant Phase = "n"
	AsyncEnd     Phase = "e"

	DeprecatedAsyncStart    Phase = "S"
	DeprecatedAsyncStepInto Phase = "T"
	DeprecatedAsyncPast     Phase = "p"
	DeprecatedAsyncEnd      Phase = "F"

	FlowStart Phase = "s"
	FlowStep  Phase = "t"
	FlowEnd   Phase = "f"

	ObjectCreated   Phase = "N"
	ObjectSnapshot  Phase = "O"
	ObjectDestroyed Phase = "D"

	Metadata Phase = "M"

	MemoryDumpGlobal  Phase = "V"
	MemoryDumpProcess Phase = "v"

	Mark Phase = "R"

	ClockSync Phase = "c"
	Context   Phase = ","
)

type StackFrame struct {
	Parent   string `json:"parent"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

/*
 {
   'cpu': 0, 'tid': 1, 'ts': 1000.0,
   'name': 'cycles:HG', 'sf': 3, 'weight': 1
 }
*/

type Sample struct {
	CPU        int64  `json:"cpu"`
	ThreadID   int64  `json:"tid"`
	Timestamp  int64  `json:"ts"`
	Name       string `json:"name"`
	StackFrame string `json:"sf"`
	Weight     int64  `json:"weight"`
}
