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
	starts := ""
	ends := ""
	startPos := []Pos{Pos{2, 5}, Pos{2, 14}, Pos{2, 25}, Pos{3, 5}, Pos{3, 14}, Pos{3, 25}, Pos{4, 5}, Pos{4, 14}, Pos{4, 25}}
	endPos := []Pos{Pos{2, 13}, Pos{2, 24}, Pos{2, 33}, Pos{3, 13}, Pos{3, 24}, Pos{3, 33}, Pos{4, 13}, Pos{4, 24}, Pos{4, 33}}
	tableStart := Pos{2, 4}
	tableEnd := Pos{4, 34}
	for _, c := range h.Headline.Children {
		if c.GetType() == TableNode {
			if c.GetPos() != tableStart {
				t.Errorf("Table start does not match up! %v vs %v", c.GetPos(), tableStart)
			}
			if c.GetEnd() != tableEnd {
				t.Errorf("Table end does not match up! %v vs %v", c.GetEnd(), tableEnd)
			}
			//fmt.Printf("%v : %v \n%v\n", c.GetPos(), c.GetEnd(), c.String())
			tbl := c.(Table)
			for ridx, r := range tbl.Rows {
				for cidx, col := range r.Columns {
					starts += fmt.Sprintf("%v,", col.GetPos())
					ends += fmt.Sprintf("%v,", col.GetEnd())
					s := col.GetPos()
					e := col.GetEnd()
					tsp := startPos[cidx]
					tsp.Row = 2 + ridx
					tep := endPos[cidx]
					tep.Row = 2 + ridx
					if tsp != s {
						t.Errorf("START CELL %d,%d does not match up %v vs %v\n", ridx, cidx, s, tsp)
					}
					if tep != e {
						t.Errorf("END CELL %d,%d does not match up %v vs %v\n", ridx, cidx, e, tep)
					}
				}
			}
			//fmt.Printf("%v\n", starts)
			//fmt.Printf("%v\n", ends)
		}
	}
	//fmt.Printf("%v vs %v\n", table.GetPos(), table.GetEnd())
}
