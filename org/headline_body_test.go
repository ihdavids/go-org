package org

import (
	"strings"
	"testing"
)

// A heading's body runs to the next heading of the same level or above,
// whatever is written in column zero in between. Emacs writes drawers and
// planning lines there since org-adapt-indentation went nil (org 9.5), and for
// a while each of them ended the heading: the drawer, everything after it and
// the child headings were hoisted to the top of the document, and the heading
// had no Properties.
func TestColumnZeroLinesStayInTheHeading(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
	}{
		{"property drawer", "* TODO Parent :ETA:\n:PROPERTIES:\n:PARENT: ETA-5555\n:END:\nBody\n** TODO Child\n"},
		{"planning then drawer", "* TODO Parent :ETA:\nSCHEDULED: <2026-10-02 Fri>\n:PROPERTIES:\n:PARENT: ETA-5555\n:END:\n** TODO Child\n"},
		{"indented, under a deep heading", "*** TODO Parent :ETA:\n  :PROPERTIES:\n  :PARENT: ETA-5555\n  :END:\n**** TODO Child\n"},
		{"logbook and block", "* TODO Parent :ETA:\n:PROPERTIES:\n:PARENT: ETA-5555\n:END:\n:LOGBOOK:\nCLOCK: [2026-10-01 Thu 10:00]--[2026-10-01 Thu 11:00] =>  1:00\n:END:\n#+begin_src go\nx\n#+end_src\n** TODO Child\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := New().Parse(strings.NewReader(c.src), "x.org")
			if len(d.Nodes) != 1 {
				t.Fatalf("want one top-level node (the parent), got %d: %#v", len(d.Nodes), d.Nodes)
			}
			h, ok := d.Nodes[0].(*Headline)
			if !ok {
				t.Fatalf("top node is %T", d.Nodes[0])
			}
			if v, ok := h.Properties.Get("PARENT"); !ok || v != "ETA-5555" {
				t.Errorf("PARENT = %q, %v; want ETA-5555", v, ok)
			}
			var child *Headline
			for _, n := range h.Children {
				if ch, ok := n.(*Headline); ok {
					child = ch
				}
			}
			if child == nil || child.Lvl != h.Lvl+1 {
				t.Fatalf("the child heading is not under its parent: %#v", h.Children)
			}
		})
	}
}

// What the column-zero case must not change: a heading still ends at the next
// one of its level, and a sibling is not swallowed as a child.
func TestHeadingStillEndsAtItsSibling(t *testing.T) {
	d := New().Parse(strings.NewReader("* A\n:PROPERTIES:\n:X: 1\n:END:\n* B\n:PROPERTIES:\n:X: 2\n:END:\n"), "x.org")
	if len(d.Nodes) != 2 {
		t.Fatalf("want two top-level headings, got %d", len(d.Nodes))
	}
	for k, want := range []string{"1", "2"} {
		h := d.Nodes[k].(*Headline)
		if v, _ := h.Properties.Get("X"); v != want {
			t.Errorf("heading %d: X = %q, want %q", k, v, want)
		}
	}
}
