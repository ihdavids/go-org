package org

import (
	"fmt"
	"strings"
	"testing"
)

func posValidate(t *testing.T, n Node, start Pos, end Pos) {
	if n.GetPos() != start {
		t.Errorf("Start pos is not valid for node: %v [%v] vs [%v]", n.String(), start, n.GetPos())
	}
	if n.GetEnd() != end {
		t.Errorf("End pos is not valid for node: %v [%v] vs [%v]", n.String(), end, n.GetEnd())
	}
}

func TestOrgRanges(t *testing.T) {
	path := "./testdata/headlines.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	// Validate top level headings in object
	starts := []Pos{Pos{2, 0}, Pos{8, 0}, Pos{9, 0}, Pos{16, 0}, Pos{22, 0}, Pos{26, 0}, Pos{30, 0}, Pos{34, 0}}
	ends := []Pos{Pos{7, 33}, Pos{8, 48}, Pos{15, 90}, Pos{21, 24}, Pos{25, 31}, Pos{29, 0}, Pos{33, 5}, Pos{40, 19}}
	for idx, n := range d.Outline.Children {
		//fmt.Printf("%v : %v : %v\n", n.Headline.GetPos(), n.Headline.GetEnd(), n.Headline.Title)
		posValidate(t, n.Headline, starts[idx], ends[idx])
		//for _, c := range n.Children {
		//	fmt.Printf("%v : %v : %v\n", c.Headline.GetPos(), c.Headline.GetEnd(), c.Headline.Title)
		//}
		//for _, c := range n.Headline.Children {
		//	fmt.Printf("%v : %v : %v\n", c.GetPos(), c.GetEnd(), c.String())
		//}
	}
}

func TestOrgRangesTable(t *testing.T) {
	path := "./testrange/table.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	// table := h.Headline.Children[0]
	for _, c := range h.Headline.Children {
		fmt.Printf("%v : %v : %v\n", c.GetPos(), c.GetEnd(), c.String())
	}
	//fmt.Printf("%v vs %v\n", table.GetPos(), table.GetEnd())
}
