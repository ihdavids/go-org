package org

import (
	"testing"
)

func TestDateParse(t *testing.T) {
	sched := OrgDateScheduled.Parse("SCHEDULED: <2004-12-25 Sat>")
	badsched := OrgDateScheduled.Parse("DEADLINE: <2004-12-25 Sat>")
	deadl := OrgDateDeadline.Parse("DEADLINE: <2004-02-29 Sun>")
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
