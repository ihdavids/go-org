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
			tbl := c.(*Table)
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

func TestOrgRangesList(t *testing.T) {
	path := "./testrange/list.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	objStart := Pos{3, 4}
	//objEnd := Pos{7, 30} // This may not be right might need to be 30??!
	objEnd := Pos{8, 3} // This is goofy list is treated as non blank line after list
	pList := [][]Pos{[]Pos{Pos{3, 4}, {3, 22}}, []Pos{{4, 4}, {4, 23}}, []Pos{{5, 4}, {8, 3}}}
	for _, c := range h.Headline.Children {
		//fmt.Printf("%v : %v \n%v\n", c.GetPos(), c.GetEnd(), c.String())
		if c.GetType() == ListNode {
			if c.GetPos() != objStart {
				t.Errorf("List start does not match up! %v vs %v\n%v", c.GetPos(), objStart, c.String())
			}
			if c.GetEnd() != objEnd {
				t.Errorf("List end does not match up! %v vs %v\n%v", c.GetEnd(), objEnd, c.String())
			}
			lst := c.(List)
			for idx, li := range lst.Items {
				res := pList[idx]
				if res[0] != li.GetPos() {
					t.Errorf("%d: List item does not match range %v vs %v\n%v", idx, li.GetPos(), res[0], li.String())
				}
				if res[1] != li.GetEnd() {
					t.Errorf("%d: End List item does not match range %v vs %v\n%v", idx, li.GetEnd(), res[1], li.String())
				}
				//fmt.Printf("%d: [%v %v]\n", idx, li.GetPos(), li.GetEnd())
			}
			break
		} else if c.GetType() != ParagraphNode {
			t.Errorf("Invalid node type: %v\n", c.GetTypeName())
			//fmt.Printf("%v\n", c.GetTypeName())
		}
	}
	// Numbered list
	h = d.Outline.Children[1]
	objStart = Pos{12, 4}
	objEnd = Pos{16, 31} // This may not be right might need to be 30??!
	pList = [][]Pos{[]Pos{Pos{12, 4}, {12, 23}}, []Pos{{13, 4}, {13, 24}}, []Pos{{14, 4}, {16, 31}}}
	for _, c := range h.Headline.Children {
		//fmt.Printf("%v : %v \n%v\n", c.GetPos(), c.GetEnd(), c.String())
		if c.GetType() == ListNode {
			if c.GetPos() != objStart {
				t.Errorf("List start does not match up! %v vs %v\n%v", c.GetPos(), objStart, c.String())
			}
			if c.GetEnd() != objEnd {
				t.Errorf("List end does not match up! %v vs %v\n%v", c.GetEnd(), objEnd, c.String())
			}
			lst := c.(List)
			for idx, li := range lst.Items {
				res := pList[idx]
				//fmt.Printf("%d: [%v %v]\n", idx, li.GetPos(), li.GetEnd())
				if res[0] != li.GetPos() {
					t.Errorf("%d: List item does not match range %v vs %v\n%v", idx, li.GetPos(), res[0], li.String())
				}
				if res[1] != li.GetEnd() {
					t.Errorf("%d: End List item does not match range %v vs %v\n%v", idx, li.GetEnd(), res[1], li.String())
				}
			}
			break
		} else if c.GetType() != ParagraphNode {
			t.Errorf("Invalid node type: %v\n", c.GetTypeName())
			//fmt.Printf("%v\n", c.GetTypeName())
		}
	}
}

func TestOrgRangesDrawer(t *testing.T) {
	path := "./testrange/drawer.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	objStart := Pos{1, 2}
	objEnd := Pos{4, 7}
	c := h.Headline.Properties
	if c.GetPos() != objStart {
		t.Errorf("Drawer start does not match up! %v vs %v", c.GetPos(), objStart)
	}
	if c.GetEnd() != objEnd {
		t.Errorf("Drawer end does not match up! %v vs %v", c.GetEnd(), objEnd)
	}
	if len(c.Properties) != 2 {
		t.Errorf("Properties count does not match up! %d vs %d", len(c.Properties), 2)
	}
	//fmt.Printf("%v : %v - %v\n%v\n", c.GetPos(), c.GetEnd(), c.GetTypeName(), c.String())
}

func TestOrgRangesSDC(t *testing.T) {
	path := "./testrange/sdc.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	objStart := Pos{1, 4}
	objEnd := Pos{1, 37}
	c := h.Headline.Scheduled
	if c.GetPos() != objStart {
		t.Errorf("Scheduled start does not match up! %v vs %v", c.GetPos(), objStart)
	}
	if c.GetEnd() != objEnd {
		t.Errorf("Scheduled end does not match up! %v vs %v", c.GetEnd(), objEnd)
	}
	//fmt.Printf("%v : %v - %v\n%v\n", c.GetPos(), c.GetEnd(), c.GetTypeName(), c.String())
}

func TestOrgRangesText(t *testing.T) {
	path := "./testrange/text.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	objStart := Pos{1, 0}
	objEnd := Pos{4, 8}
	c := h.Headline.Children[0]
	if c.GetPos() != objStart {
		t.Errorf("Text start does not match up! %v vs %v", c.GetPos(), objStart)
	}
	if c.GetEnd() != objEnd {
		t.Errorf("Text end does not match up! %v vs %v", c.GetEnd(), objEnd)
	}
	//fmt.Printf("%v : %v - %v\n%v\n", c.GetPos(), c.GetEnd(), c.GetTypeName(), c.String())
}

func TestOrgRangesBlock(t *testing.T) {
	path := "./testrange/block.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	objStart := Pos{2, 4}
	objEnd := Pos{7, 13}
	for _, c := range h.Headline.Children {
		if c.GetType() == BlockNode {
			if c.GetPos() != objStart {
				t.Errorf("Block start does not match up! %v vs %v", c.GetPos(), objStart)
			}
			if c.GetEnd() != objEnd {
				t.Errorf("Block end does not match up! %v vs %v", c.GetEnd(), objEnd)
			}
			//fmt.Printf("%v : %v - %v\n%v\n", c.GetPos(), c.GetEnd(), c.GetTypeName(), c.String())
		}
	}
	h = d.Outline.Children[1]
	objStart = Pos{11, 4}
	objEnd = Pos{13, 13}
	for _, c := range h.Headline.Children {
		if c.GetType() == BlockNode {
			if c.GetPos() != objStart {
				t.Errorf("Block2 start does not match up! %v vs %v", c.GetPos(), objStart)
			}
			if c.GetEnd() != objEnd {
				t.Errorf("Block2 end does not match up! %v vs %v", c.GetEnd(), objEnd)
			}
			//fmt.Printf("%v : %v - %v\n%v\n", c.GetPos(), c.GetEnd(), c.GetTypeName(), c.String())
		}
	}
}

func TestOrgRangesResult(t *testing.T) {
	path := "./testrange/result.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	h := d.Outline.Children[0]
	objStart := Pos{7, 4}
	objEnd := Pos{8, 15}
	for _, c := range h.Headline.Children {
		//fmt.Printf("%v : %v - %v\n%v\n", c.GetPos(), c.GetEnd(), c.GetTypeName(), c.String())
		if c.GetType() == BlockNode {
			b := c.(*Block)
			r := b.Result
			if r.GetPos() != objStart {
				t.Errorf("Result start does not match up! %v vs %v", r.GetPos(), objStart)
			}
			if r.GetEnd() != objEnd {
				t.Errorf("Result end does not match up! %v vs %v", r.GetEnd(), objEnd)
			}
		}
	}
}
