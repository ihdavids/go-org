package org

import (
	"strings"
	"testing"
)

// Org keeps all of a heading's planning on one line - `DEADLINE: <...>
// SCHEDULED: <...>`, with `CLOSED: [...]` joining them when the heading is
// marked done. The parse read whichever keyword the lexer had claimed the line
// for and dropped the rest, so a heading with a deadline and a schedule had no
// deadline and nothing anywhere said so.
func TestEveryPlanningKeywordOnOneLineIsRead(t *testing.T) {
	for _, c := range []struct {
		name                    string
		src                     string
		sched, deadline, closed bool
	}{
		{"deadline then scheduled", "* TODO a\nDEADLINE: <2026-10-01 Thu> SCHEDULED: <2026-09-28 Mon>\n", true, true, false},
		{"scheduled then deadline", "* TODO a\nSCHEDULED: <2026-09-28 Mon> DEADLINE: <2026-10-01 Thu>\n", true, true, false},
		{"closed and deadline", "* DONE a\nCLOSED: [2026-09-28 Mon 14:32] DEADLINE: <2026-10-01 Thu>\n", false, true, true},
		{"all three", "* DONE a\nCLOSED: [2026-09-28 Mon 14:32] DEADLINE: <2026-10-01 Thu> SCHEDULED: <2026-09-28 Mon>\n", true, true, true},
		{"separate lines still work", "* DONE a\nCLOSED: [2026-09-28 Mon 14:32]\nSCHEDULED: <2026-09-28 Mon>\n", true, false, true},
		{"one on its own", "* TODO a\nSCHEDULED: <2026-09-28 Mon>\n", true, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := firstHeadline(t, c.src)
			if got := h.Scheduled != nil; got != c.sched {
				t.Errorf("scheduled: got %v want %v", got, c.sched)
			}
			if got := h.Deadline != nil; got != c.deadline {
				t.Errorf("deadline: got %v want %v", got, c.deadline)
			}
			if got := h.Closed != nil; got != c.closed {
				t.Errorf("closed: got %v want %v", got, c.closed)
			}
		})
	}
}

// A repeater has to survive the line it shares with other planning keywords -
// which is the whole point of reading them all, since a repeating task that is
// marked done is exactly the case where CLOSED lands on the same line.
func TestARepeaterSurvivesASharedPlanningLine(t *testing.T) {
	h := firstHeadline(t, "* DONE a\nCLOSED: [2026-09-28 Mon 14:32] SCHEDULED: <2026-09-28 Mon .+2d>\n")
	if h.Scheduled == nil {
		t.Fatal("no scheduled date")
	}
	d := h.Scheduled.Date
	if d.RepeatPre != ".+" || d.RepeatDWMY != "d" {
		t.Fatalf("repeater lost: pre=%q dwmy=%q", d.RepeatPre, d.RepeatDWMY)
	}
	if d.RepeatRule == nil || d.RepeatRule.Options.Interval != 2 {
		t.Fatalf("interval lost: %v", d.RepeatRule)
	}
}

// One line in has to be one line out, or the first thing to rewrite a file
// reformats every done heading in it.
func TestAPlanningLineIsWrittenBackAsOneLine(t *testing.T) {
	src := "* DONE a\nCLOSED: [2026-09-28 Mon 14:32] DEADLINE: <2026-10-01 Thu> SCHEDULED: <2026-09-28 Mon .+2d>\n"
	d := New().Parse(strings.NewReader(src), "t.org")
	out, err := d.Write(NewOrgWriter())
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, want := range []string{"CLOSED: [2026-09-28 Mon 14:32]", "DEADLINE: <2026-10-01 Thu>", "SCHEDULED: <2026-09-28 Mon .+2d>"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from:\n%s", want, out)
		}
	}
	planning := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "CLOSED:") || strings.Contains(line, "DEADLINE:") || strings.Contains(line, "SCHEDULED:") {
			planning++
		}
	}
	if planning != 1 {
		t.Errorf("expected one planning line, got %d:\n%s", planning, out)
	}
}

func firstHeadline(t *testing.T, src string) *Headline {
	t.Helper()
	d := New().Parse(strings.NewReader(src), "t.org")
	for _, n := range d.Nodes {
		if h, ok := n.(*Headline); ok {
			return h
		}
		if h, ok := n.(Headline); ok {
			return &h
		}
	}
	t.Fatalf("no headline parsed from %q", src)
	return nil
}

// org-habit writes `.+2d/4d`: every two days, and definitely overdue after four.
// Without the slack half in the pattern the whole timestamp failed to match, so
// a habit written the way org-habit documents had no scheduled date at all - the
// planning line was re-read as body text and the heading came back with
// Scheduled nil, which is the same silence `CompileSDCRe` used to produce for a
// plain repeater.
func TestAHabitsSlackRepeaterParses(t *testing.T) {
	for _, c := range []struct {
		line    string
		pre     string
		unit    string
		n       int
		maxNum  string
		maxDWMY string
	}{
		{"SCHEDULED: <2026-09-26 Sat .+2d/4d>", ".+", "d", 2, "4", "d"},
		{"SCHEDULED: <2026-09-26 Sat ++1w/2w>", "++", "w", 1, "2", "w"},
		{"SCHEDULED: <2026-09-26 Sat .+2d>", ".+", "d", 2, "", ""},
	} {
		h := firstHeadline(t, "* TODO a\n"+c.line+"\n")
		if h.Scheduled == nil {
			t.Fatalf("%q did not parse at all", c.line)
		}
		d := h.Scheduled.Date
		if strings.TrimSpace(d.RepeatPre) != c.pre || strings.TrimSpace(d.RepeatDWMY) != c.unit {
			t.Errorf("%q: pre=%q dwmy=%q", c.line, d.RepeatPre, d.RepeatDWMY)
		}
		if d.RepeatRule == nil || d.RepeatRule.Options.Interval != c.n {
			t.Errorf("%q: interval %v want %d", c.line, d.RepeatRule, c.n)
		}
		if d.RepeatMaxNum != c.maxNum || d.RepeatMaxDWMY != c.maxDWMY {
			t.Errorf("%q: max %q%q want %q%q", c.line, d.RepeatMaxNum, d.RepeatMaxDWMY, c.maxNum, c.maxDWMY)
		}
	}
}

// A repeater that parses and then loses half of itself on the next write is
// worse than one that never parsed: the file quietly stops saying what it said.
func TestASlackRepeaterIsWrittenBack(t *testing.T) {
	src := "* TODO a\nSCHEDULED: <2026-09-26 Sat .+2d/4d>\n"
	d := New().Parse(strings.NewReader(src), "t.org")
	out, err := d.Write(NewOrgWriter())
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(out, ".+2d/4d") {
		t.Errorf("the slack half was lost:\n%s", out)
	}
}

// Org's logging notation puts more than an access key in a keyword's cookie:
// `NEXT(n!)` logs a timestamp on entering NEXT, `WAITING(w@/!)` keeps a note
// going in and a timestamp coming out. Only a three-character `(x)` was trimmed,
// so the keyword stayed `NEXT(n!)` - and because keywords are matched on a
// headline by literal prefix, `* NEXT Write the thing` came out as a heading with
// no keyword at all. It was not a task, could not be found by its keyword, and
// never reached an agenda.
func TestKeywordsWithLoggingCookiesAreRecognised(t *testing.T) {
	src := "#+TODO: TODO(t) NEXT(n!) WAITING(w@/!) HOLD(h@/@) | DONE(d) CANCELLED(c@)\n\n" +
		"* NEXT Write the thing\n* WAITING On a reply\n* HOLD Parked\n* TODO Plain\n* DONE Fin\n* CANCELLED Dropped\n"
	d := New().Parse(strings.NewReader(src), "t.org")
	want := []struct{ status, title string }{
		{"NEXT", "Write the thing"},
		{"WAITING", "On a reply"},
		{"HOLD", "Parked"},
		{"TODO", "Plain"},
		{"DONE", "Fin"},
		{"CANCELLED", "Dropped"},
	}
	i := 0
	for _, n := range d.Nodes {
		h, ok := n.(*Headline)
		if !ok {
			continue
		}
		if i >= len(want) {
			t.Fatalf("more headlines than expected")
		}
		if h.Status != want[i].status || String(h.Title...) != want[i].title {
			t.Errorf("headline %d: got status %q title %q, want %q / %q",
				i, h.Status, String(h.Title...), want[i].status, want[i].title)
		}
		i++
	}
	if i != len(want) {
		t.Errorf("parsed %d headlines, wanted %d", i, len(want))
	}
}

// A word that is nothing but a cookie must not be trimmed to nothing, or every
// headline would match the empty keyword.
func TestATrimmedKeywordIsNeverEmpty(t *testing.T) {
	for _, got := range trimFastTags([]string{"(t)", "TODO(t)", "NEXT(n!)", "DONE"}) {
		if got == "" {
			t.Error("a keyword was trimmed away to nothing")
		}
	}
}
