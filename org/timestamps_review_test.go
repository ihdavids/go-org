package org

import "testing"

// An hourly repeater parses, and -0d keeps its zero when written back.
func TestRepeatHourAndWarnZero(t *testing.T) {
	d, _, _ := ParseTimestamp("<2026-10-10 Sat 10:00 +1h>")
	if d == nil || d.RepeatDWMY != "h" || d.RepeatString() != "+1h" {
		t.Fatalf("+1h: got %#v", d)
	}
	w, _, _ := ParseTimestamp("<2026-10-11 Sun -0d>")
	if w == nil || w.WarnNum != "0" || w.WarnString() != "-0d" {
		t.Fatalf("-0d: got %#v (%q)", w, w.WarnString())
	}
	if got := w.ToDate(); got != "<2026-10-11 Sun -0d>" {
		t.Fatalf("-0d written back as %q", got)
	}
}
