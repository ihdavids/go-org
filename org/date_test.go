package org

import (
	"testing"
)

func TestDateParse(t *testing.T) {
	sched, _ := OrgDateScheduled.Parse("SCHEDULED: <2004-12-25 Sat>")
	badsched, _ := OrgDateScheduled.Parse("DEADLINE: <2004-12-25 Sat>")
	deadl, _ := OrgDateDeadline.Parse("DEADLINE: <2004-02-29 Sun>")
	if sched.ToDate() != "<2004-12-25 Sat>" {
		t.Fatalf("Scheduled did not pass: %s got %q %q", "2004-12-25", sched.Start.String(), sched.End.String())
	}
	if badsched != nil {
		t.Fatalf("Bad Schedule parse parsed something erroneously")
	}
	if deadl.ToDate() != "<2004-02-29 Sun>" && !deadl.End.IsZero() {
		t.Fatalf("Deadline parse did not pass: %s got %q %q", "2004-02-29", deadl.Start.String(), sched.End.String())
	} else {
		t.Logf(deadl.ToDate())
	}
}

func TestParseSDC(t *testing.T) {
	sched, sdt := ParseSDC("SCHEDULED: <2004-12-25 Sat>")
	badsched, bdt := ParseSDC("DEADLINE: <2004-12-25 Sat>")
	deadl, ddt := ParseSDC("DEADLINE: <2004-02-29 Sun>")
	if sdt == Scheduled && sched.ToDate() != "<2004-12-25 Sat>" {
		t.Fatalf("Scheduled did not pass: %s got %q %q", "2004-12-25", sched.Start.String(), sched.End.String())
	}
	if bdt == NilDate && badsched.ToDate() != "<2004-12-25 Sat>" {
		t.Fatalf("Bad Schedule parse parsed something erroneously")
	}
	if ddt == Deadline && deadl.ToDate() != "<2004-02-29 Sun>" && !deadl.End.IsZero() {
		t.Fatalf("Deadline parse did not pass: %s got %q %q", "2004-02-29", deadl.Start.String(), sched.End.String())
	} else {
		t.Logf(deadl.ToDate())
	}
}

func TestTimestamps(t *testing.T) {
	// NOTE the single digit
	s, dt, _ := ParseTimestamp("<2004-1-25 Sun>")
	if s == nil || dt != ActiveTimeStamp || s.ToDate() != "<2004-01-25 Sun>" || s.HasTime() || s.HasEnd() || s.RepeatRule != nil {
		strTime := ""
		if s != nil {
			strTime = s.ToDate()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 Sun> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25>")
	if s == nil || dt != ActiveTimeStamp || s.ToDate() != "<2004-01-25 Sun>" || s.HasTime() || s.HasEnd() || s.RepeatRule != nil {
		strTime := ""
		if s != nil {
			strTime = s.ToDate()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 5:45>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45>" || !s.HasTime() || s.HasEnd() || s.RepeatRule != nil {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 5:45> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 Sun 5:45>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45>" || !s.HasTime() || s.HasEnd() || s.RepeatRule != nil {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 Sun 5:45> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 Sun 5:45 +1d>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45 +1d>" || !s.HasTime() || s.HasEnd() || s.RepeatRule == nil {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 Sun 5:45 +1d> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 5:45 +1d>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45 +1d>" || !s.HasTime() || s.HasEnd() || s.RepeatRule == nil {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 5:45 +1d> got: %s", strTime)
	}
}

// A planning line may carry a repeater and a warning period, and both have to
// survive being written back out.
//
// CompileSDCRe used to build its regex with nocookie, which wanted the closing
// bracket immediately after the day - so `SCHEDULED: <2026-09-28 Mon .+2d>` did
// not match the planning-line pattern *at all*. The consequences were quiet and
// wide: the date never reached Headline.Scheduled, so the agenda did not see it,
// a habit had no schedule, and the line was re-parsed as body text whose inline
// timestamp landed on Headline.Timestamp instead.
//
// ToDate then dropped the cookie on the way out, so even a date that had been
// parsed lost its repeat the next time the file was written.
func TestSDCKeepsItsCookie(t *testing.T) {
	for _, c := range []struct {
		line string
		want string
		kind DateType
	}{
		{"SCHEDULED: <2026-09-28 Mon>", "<2026-09-28 Mon>", Scheduled},
		// A habit: repeat from when it was last done.
		{"SCHEDULED: <2026-09-28 Mon .+2d>", "<2026-09-28 Mon .+2d>", Scheduled},
		{"SCHEDULED: <2026-09-28 Mon ++1w>", "<2026-09-28 Mon ++1w>", Scheduled},
		{"DEADLINE: <2026-09-28 Mon +1w>", "<2026-09-28 Mon +1w>", Deadline},
		// A repeat and a warning period together. The warning's interval used
		// to be written into the repeat's number, so this came out as
		// "+1w1w" with the -3d missing.
		{"DEADLINE: <2026-09-28 Mon +1w -3d>", "<2026-09-28 Mon +1w -3d>", Deadline},
		// With a time on it, which is the path that always worked.
		{"SCHEDULED: <2026-09-28 Mon 09:30 +1d>", "<2026-09-28 Mon 09:30 +1d>", Scheduled},
		{"CLOSED: [2026-09-28 Mon 09:30]", "[2026-09-28 Mon 09:30]", Closed},
	} {
		d, dt := ParseSDC(c.line)
		if d == nil {
			t.Errorf("ParseSDC(%q) did not parse", c.line)
			continue
		}
		if dt != c.kind {
			t.Errorf("ParseSDC(%q) kind = %v, want %v", c.line, dt, c.kind)
		}
		if got := d.ToString(); got != c.want {
			t.Errorf("ParseSDC(%q).ToString() = %q, want %q", c.line, got, c.want)
		}
	}
}
