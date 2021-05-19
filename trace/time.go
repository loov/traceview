package trace

import "time"

// Time in nanoseconds
type Time int64

func NewTime(t time.Duration) Time { return Time(t.Nanoseconds()) }

func (t Time) Std() time.Duration {
	return time.Duration(int64(t) * int64(time.Nanosecond))
}

func (t Time) Min(b Time) Time {
	if t < b {
		return t
	}
	return b
}

func (t Time) Max(b Time) Time {
	if t > b {
		return t
	}
	return b
}
