package org

import (
	"strings"
	"testing"
)

func stampsOf(h *Headline) []string {
	var out []string
	for _, ts := range h.Timestamps {
		out = append(out, ts.Time.ToString())
	}
	return out
}

// Headline.Timestamp is one stamp; a heading with two appointments has two,
// and Timestamps keeps every active one of the heading's own, in file order.
func TestHeadlineKeepsEveryActiveTimestamp(t *testing.T) {
	src := `#+TODO: TODO | DONE
* TODO Two meetings <2026-10-05 Mon 10:00>
SCHEDULED: <2026-10-01 Thu> DEADLINE: <2026-10-09 Fri>
:PROPERTIES:
:ID: abc
:END:
:LOGBOOK:
- Note taken on <2026-09-01 Tue>
CLOCK: [2026-10-01 Thu 09:00]--[2026-10-01 Thu 10:00] =>  1:00
:END:
Then again <2026-10-12 Mon +1w>, and a record [2026-10-03 Sat].
- a list item <2026-10-20 Tue>--<2026-10-22 Thu>
** Child <2026-11-01 Sun>
Child body <2026-11-02 Mon>
* Next
`
	doc := New().Parse(strings.NewReader(src), "stamps.org")
	if doc.Error != nil {
		t.Fatal(doc.Error)
	}
	h := doc.Outline.Children[0].Headline
	got := strings.Join(stampsOf(h), " | ")
	want := "<2026-10-05 Mon 10:00> | <2026-10-12 Mon +1w> | <2026-10-20 Tue>--<2026-10-22 Thu>"
	if got != want {
		t.Errorf("parent stamps\n got %s\nwant %s", got, want)
	}
	// Timestamp is as it always was: the last active stamp read, which here
	// is the range (the drawer's stamp came before it).
	if h.Timestamp == nil || h.Timestamp.Time.ToString() != "<2026-10-20 Tue>--<2026-10-22 Thu>" {
		t.Errorf("Timestamp changed: %v", h.Timestamp)
	}
	child := doc.Outline.Children[0].Children[0].Headline
	if got := strings.Join(stampsOf(child), " | "); got != "<2026-11-01 Sun> | <2026-11-02 Mon>" {
		t.Errorf("child stamps %s", got)
	}
	if next := doc.Outline.Children[1].Headline; len(next.Timestamps) != 0 {
		t.Errorf("a heading with no stamps has %v", stampsOf(next))
	}
}

// Emacs writes drawers and planning in column zero; the stamps still belong
// to the heading above them and a drawer's stamp is still not one of them.
func TestHeadlineTimestampsInAColumnZeroFile(t *testing.T) {
	src := `* Parent
** Task
SCHEDULED: <2026-10-01 Thu>
:PROPERTIES:
:STYLE: habit
:END:
:NOTES:
<2026-09-09 Wed>
:END:
Body <2026-10-07 Wed>
** Other
<2026-10-08 Thu>
`
	doc := New().Parse(strings.NewReader(src), "col0.org")
	parent := doc.Outline.Children[0]
	if len(parent.Children) != 2 {
		t.Fatalf("outline lost its shape: %d children", len(parent.Children))
	}
	if got := strings.Join(stampsOf(parent.Children[0].Headline), " | "); got != "<2026-10-07 Wed>" {
		t.Errorf("task stamps %q", got)
	}
	if got := strings.Join(stampsOf(parent.Children[1].Headline), " | "); got != "<2026-10-08 Thu>" {
		t.Errorf("other stamps %q", got)
	}
	if len(parent.Headline.Timestamps) != 0 {
		t.Errorf("parent took a child's stamp: %v", stampsOf(parent.Headline))
	}
}

func TestOrgDateBeforeAfterInRange(t *testing.T) {
	// Only the inline parser joins <a>--<b> into one range.
	doc := New().Parse(strings.NewReader("* R <2026-10-14 Wed>--<2026-10-16 Fri>\n"), "r.org")
	d := doc.Outline.Children[0].Headline.Timestamp.Time
	p, _, _ := ParseTimestamp("<2026-10-15 Thu>")
	before, _, _ := ParseTimestamp("<2026-10-10 Sat>")
	if !d.Before(p.Start) || d.After(p.Start) {
		t.Error("the 14th is before the 15th")
	}
	if !d.After(before.Start) || d.Before(before.Start) {
		t.Error("the 14th is after the 10th")
	}
	if !d.DateTimeInRange(p.Start) || d.DateTimeInRange(before.Start) {
		t.Error("in range wrong")
	}
	late, _, _ := ParseTimestamp("<2026-10-20 Tue>")
	if d.DateTimeInRange(late.Start) {
		t.Error("a range does not contain a day after its end")
	}
	if !d.HasOverlap(p) || d.HasOverlap(late) {
		t.Error("overlap wrong")
	}
}

func TestRepeatAndWarnStrings(t *testing.T) {
	d, _, _ := ParseTimestamp("<2026-10-14 Wed .+2d/4d -3d>")
	if d.RepeatString() != ".+2d/4d" || d.WarnString() != "-3d" {
		t.Errorf("got %q %q", d.RepeatString(), d.WarnString())
	}
	if d.ToString() != "<2026-10-14 Wed .+2d/4d -3d>" {
		t.Errorf("written %q", d.ToString())
	}
	p, _, _ := ParseTimestamp("<2026-10-14 Wed>")
	if p.RepeatString() != "" || p.WarnString() != "" {
		t.Error("no cookies")
	}
}
