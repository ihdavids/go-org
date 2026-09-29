package org

import (
	"strings"
	"testing"
)

// A descriptive list - `- term :: definition` - used to panic the parser.
//
// parseListItem parsed the term with `d.parseInline(dterm, i)`, and by that
// point `i` had been walked to the end of the item by the loop above it, which
// for a list at the end of a file is past the end of the token slice.
// parseInline indexes d.tokens[ni] for the positions it hangs on its nodes, so
// it went out of range - and Parse recovers a panic into d.Error, which meant
// the whole file came back with no nodes at all.
//
// The cost of that was not limited to descriptive lists: one anywhere in a file
// made the *file* unparseable, so every exporter - html, latex, markdown - got
// an empty document. It is pinned here because the failure is silent from the
// outside: an export simply comes back empty.
func TestDescriptiveListDoesNotPanicTheParser(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"at the end of a file", "- term :: what it means\n"},
		{"several items", "- a :: one\n- b :: two\n"},
		{"with no definition", "- term ::\n"},
		{"under a headline", "* H\n\n- term :: what it means\n"},
		{"followed by prose", "- term :: meaning\n\nand then a paragraph\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := New().Silent().Parse(strings.NewReader(c.in), "x.org")
			if d.Error != nil {
				t.Fatalf("parse failed: %v", d.Error)
			}
			if len(d.Nodes) == 0 {
				t.Fatalf("no nodes came back for %q", c.in)
			}
		})
	}
}

// The term keeps its own inline markup, and lands on the line it was written on
// rather than on whatever line the item happened to end at.
func TestDescriptiveListTermIsParsedInPlace(t *testing.T) {
	d := New().Silent().Parse(strings.NewReader("some prose\n\n- /term/ :: what it means\n"), "x.org")
	if d.Error != nil {
		t.Fatalf("parse failed: %v", d.Error)
	}
	var item *DescriptiveListItem
	var walk func(ns []Node)
	walk = func(ns []Node) {
		for _, n := range ns {
			switch v := n.(type) {
			case DescriptiveListItem:
				item = &v
			case List:
				walk(v.Items)
			}
		}
	}
	walk(d.Nodes)
	if item == nil {
		t.Fatal("no descriptive list item was parsed")
	}
	if len(item.Term) == 0 {
		t.Fatal("the term came back empty")
	}
	if _, ok := item.Term[0].(Emphasis); !ok {
		t.Errorf("expected the term's emphasis to be parsed, got %T", item.Term[0])
	}
	// The term is on the third line of the file, which is row 2.
	if row := item.Term[0].GetPos().Row; row != 2 {
		t.Errorf("the term should sit on the line it was written on, got row %d", row)
	}
}
