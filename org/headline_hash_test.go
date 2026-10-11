package org

import (
	"strings"
	"testing"
)

// An untitled heading must not share a hash with the heading before it: orgs
// looks a heading's parent up by hash, and a heading that is its own parent
// sends the walk up the outline round for ever.
func TestUntitledHeadingHasItsOwnHash(t *testing.T) {
	src := "* Tasks\n** TODO\n** Other\n"
	doc := New().Parse(strings.NewReader(src), "plan.org")
	var hashes []string
	var walk func(ns []*Section)
	walk = func(ns []*Section) {
		for _, n := range ns {
			hashes = append(hashes, n.Hash)
			walk(n.Children)
		}
	}
	walk(doc.Outline.Children)
	if len(hashes) != 3 {
		t.Fatalf("want 3 headings, got %d", len(hashes))
	}
	seen := map[string]bool{}
	for _, h := range hashes {
		if seen[h] {
			t.Fatalf("two headings share hash %s: %v", h, hashes)
		}
		seen[h] = true
	}
}
