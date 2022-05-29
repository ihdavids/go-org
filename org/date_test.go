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
	if s == nil || dt != ActiveTimeStamp || s.ToDate() != "<2004-01-25 Sun>" {
		strTime := ""
		if s != nil {
			strTime = s.ToDate()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 Sun> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25>")
	if s == nil || dt != ActiveTimeStamp || s.ToDate() != "<2004-01-25 Sun>" {
		strTime := ""
		if s != nil {
			strTime = s.ToDate()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 5:45>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45>" {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 5:45> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 Sun 5:45>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45>" {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 Sun 5:45> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 Sun 5:45 +1d>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45 +1d>" {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 Sun 5:45 +1d> got: %s", strTime)
	}
	s, dt, _ = ParseTimestamp("<2004-1-25 5:45 +1d>")
	if s == nil || dt != ActiveTimeStamp || s.ToString() != "<2004-01-25 Sun 05:45 +1d>" {
		strTime := ""
		if s != nil {
			strTime = s.ToString()
		}
		t.Fatalf("Timestamp did not match: <2004-1-25 5:45 +1d> got: %s", strTime)
	}
}
