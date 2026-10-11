package org

import (
	"strings"
	"testing"
)

// Writing a document back must leave what org reads where org reads it: a
// dynamic block as `#+BEGIN: name params` ... `#+END:`, and planning on the
// line under the heading, before the property drawer. Both used to come out
// wrong from a reformat (`#+BEGIN :`, planning after the drawer), and org
// then saw neither the block nor the dates.
func TestRewriteKeepsDynamicBlocksAndPlanning(t *testing.T) {
	src := `* Clocked
#+BEGIN: clocktable :scope file :maxlevel 2
| Headline | Time |
#+END:

* TODO Planned
  SCHEDULED: <2026-10-12 Mon> DEADLINE: <2026-10-20 Tue>
  :PROPERTIES:
  :EFFORT: 2d
  :END:
  Body.

* DONE Clocked task
  :LOGBOOK:
  CLOCK: [2026-09-30 Wed 09:00]--[2026-09-30 Wed 10:00] =>  1:00
  :END:
`
	out, err := New().Parse(strings.NewReader(src), "x.org").Write(NewOrgWriter())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#+BEGIN: clocktable :scope file :maxlevel 2\n", "#+END:\n", "=>  1:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#+BEGIN :") {
		t.Errorf("a space before the colon:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.HasSuffix(l, "TODO Planned") {
			if !strings.Contains(lines[i+1], "SCHEDULED: <2026-10-12 Mon> DEADLINE: <2026-10-20 Tue>") {
				t.Errorf("planning is not the line under its heading:\n%s", out)
			}
		}
	}

	// And it reads back the same: the block is a block, the planning planning.
	again := New().Parse(strings.NewReader(out), "x.org")
	var planned, dyn bool
	var walk func(ns []Node)
	walk = func(ns []Node) {
		for _, n := range ns {
			switch v := n.(type) {
			case SDC:
				if v.DateType == Scheduled || v.DateType == Deadline {
					planned = true
				}
			case *Block:
				if v.Name == "DYN" && len(v.Parameters) > 0 && v.Parameters[0] == "clocktable" {
					dyn = true
				}
			}
		}
	}
	var sections func(ss []*Section)
	sections = func(ss []*Section) {
		for _, s := range ss {
			walk(s.Headline.Children)
			sections(s.Children)
		}
	}
	sections(again.Outline.Children)
	if !planned || !dyn {
		t.Errorf("read back: planning %v, dynamic block %v", planned, dyn)
	}
}
