package org

import (
	"strings"
	"testing"
)

// A range across days is one timestamp with an end. It used to be read as
// two timestamps, and the heading kept the second: the range reported its
// last day as its start and had no end.
func TestTimestampRangeIsOneTimestamp(t *testing.T) {
	src := "* Code freeze\n<2026-10-14 Wed>--<2026-10-16 Fri>\n"
	doc := New().Parse(strings.NewReader(src), "range.org")
	if doc.Error != nil {
		t.Fatal(doc.Error)
	}
	h := doc.Outline.Children[0].Headline
	if h.Timestamp == nil || h.Timestamp.Time == nil {
		t.Fatal("no timestamp on the heading")
	}
	d := h.Timestamp.Time
	if got := d.Start.Format("2006-01-02"); got != "2026-10-14" {
		t.Errorf("start %s, want 2026-10-14", got)
	}
	if got := d.End.Format("2006-01-02"); got != "2026-10-16" {
		t.Errorf("end %s, want 2026-10-16", got)
	}
	// And it writes back the way org spells it.
	if got := d.ToString(); got != "<2026-10-14 Wed>--<2026-10-16 Fri>" {
		t.Errorf("written as %q", got)
	}
}

// A time span within one day is written inside one stamp; a span across days
// as two. Both used to be written as "<start -- end>", which org cannot read.
func TestTimestampRangeSpellings(t *testing.T) {
	d, _, _ := ParseTimestamp("<2026-10-14 Wed 10:00-12:00>")
	if d == nil {
		t.Fatal("did not parse")
	}
	if got := d.ToString(); got != "<2026-10-14 Wed 10:00-12:00>" {
		t.Errorf("same day span written as %q", got)
	}
	// Mixed kinds are two timestamps, as org reads them.
	src := "* x\n<2026-10-14 Wed>--[2026-10-16 Fri]\n"
	doc := New().Parse(strings.NewReader(src), "mixed.org")
	h := doc.Outline.Children[0].Headline
	if h.Timestamp != nil && h.Timestamp.Time != nil && !h.Timestamp.Time.End.IsZero() {
		t.Errorf("an active and an inactive stamp made a range")
	}
}

// Org's two spellings of a run of days, from the manual. The second is a
// meeting at the same hour each day: its run is the first day's start to the
// last day's end, and the times inside it have to survive a rewrite - they
// used to come back as <… 10:00>--<… 10:00>, both 11:00s gone.
func TestTimestampRangeManualForms(t *testing.T) {
	src := "* Trips\n** Meeting in Amsterdam\n   <2004-08-23 Mon>--<2004-08-26 Thu>\n" +
		"** This weeks committee meetings\n   <2004-08-23 Mon 10:00-11:00>--<2004-08-26 Thu 10:00-11:00>\n"
	doc := New().Silent().Parse(strings.NewReader(src), "manual.org")
	kids := doc.Outline.Children[0].Children
	if len(kids) != 2 {
		t.Fatalf("want 2 headings, got %d", len(kids))
	}
	cases := []struct {
		start, end, written string
	}{
		{"2004-08-23 00:00", "2004-08-26 00:00", "<2004-08-23 Mon>--<2004-08-26 Thu>"},
		{"2004-08-23 10:00", "2004-08-26 11:00", "<2004-08-23 Mon 10:00-11:00>--<2004-08-26 Thu 10:00-11:00>"},
	}
	for i, c := range cases {
		ts := kids[i].Headline.Timestamp
		if ts == nil || ts.Time == nil {
			t.Fatalf("%d: no timestamp", i)
		}
		d := ts.Time
		if got := d.Start.Format("2006-01-02 15:04"); got != c.start {
			t.Errorf("%d: start %s, want %s", i, got, c.start)
		}
		if got := d.End.Format("2006-01-02 15:04"); got != c.end {
			t.Errorf("%d: end %s, want %s", i, got, c.end)
		}
		if got := d.ToString(); got != c.written {
			t.Errorf("%d: written as %q, want %q", i, got, c.written)
		}
	}
	// Writing the document back gives back what was read.
	out, err := doc.Write(NewOrgWriter())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if !strings.Contains(out, c.written) {
			t.Errorf("org writer lost %q:\n%s", c.written, out)
		}
	}
	// And html shows two stamps joined by --, not one stamp around both.
	html, err := doc.Write(NewHTMLWriter())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "&lt;2004-08-23 Mon&gt;--&lt;2004-08-26 Thu&gt;") {
		t.Errorf("html range spelled wrong:\n%s", html)
	}
}
