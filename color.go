package main

import (
	"fmt"
	"image/color"
	"math"
	"time"
)

var tickIntervals = []time.Duration{
	time.Nanosecond,
	5 * time.Nanosecond,
	10 * time.Nanosecond,
	50 * time.Nanosecond,
	100 * time.Nanosecond,
	500 * time.Nanosecond,
	time.Microsecond,
	5 * time.Microsecond,
	10 * time.Microsecond,
	50 * time.Microsecond,
	100 * time.Microsecond,
	500 * time.Microsecond,
	time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	time.Minute,
	5 * time.Minute,
	10 * time.Minute,
}

func spanColor(spanID, traceID int64) color.NRGBA {
	p := spanID ^ traceID
	hue := float64(uint16(p)) / 0xFFFF * 360.0
	return hslColor(hue, 0.4, 0.3)
}

func hslColor(h, s, l float64) color.NRGBA {
	c := (1 - math.Abs(2*l-1)) * s
	h2 := h / 60.0
	x := c * (1 - math.Abs(math.Mod(h2, 2)-1))
	var r, g, b float64
	switch {
	case h2 < 1:
		r, g, b = c, x, 0
	case h2 < 2:
		r, g, b = x, c, 0
	case h2 < 3:
		r, g, b = 0, c, x
	case h2 < 4:
		r, g, b = 0, x, c
	case h2 < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return color.NRGBA{
		R: byte((r + m) * 255),
		G: byte((g + m) * 255),
		B: byte((b + m) * 255),
		A: 0xFF,
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		us := float64(d.Nanoseconds()) / 1e3
		if us == float64(int64(us)) {
			return fmt.Sprintf("%d\u00B5s", int64(us))
		}
		return fmt.Sprintf("%.1f\u00B5s", us)
	}
	if d < time.Second {
		ms := float64(d.Nanoseconds()) / 1e6
		if ms == float64(int64(ms)) {
			return fmt.Sprintf("%dms", int64(ms))
		}
		return fmt.Sprintf("%.1fms", ms)
	}
	s := d.Seconds()
	if s == float64(int64(s)) {
		return fmt.Sprintf("%ds", int64(s))
	}
	return fmt.Sprintf("%.1fs", s)
}
