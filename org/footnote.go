package org

import (
	"regexp"
)

type FootnoteDefinition struct {
	Pos      Pos
	Name     string
	Children []Node
	Inline   bool
}

var footnoteDefinitionRegexp = regexp.MustCompile(`^\[fn:([\w-]+)\](\s+(.+)|\s*$)`)

func lexFootnoteDefinition(line string, row, col int) (token, bool) {
	if m := footnoteDefinitionRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		return token{"footnoteDefinition", 0, m[1], m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func (d *Document) parseFootnoteDefinition(i int, parentStop stopFn) (int, Node) {
	start, name := i, d.tokens[i].content
	d.tokens[i] = tokenize(d.tokens[i].matches[2], d.tokens[i].Pos().Row)
	stop := func(d *Document, i int) bool {
		return parentStop(d, i) ||
			(isSecondBlankLine(d, i) && i > start+1) ||
			d.tokens[i].kind == "headline" || d.tokens[i].kind == "footnoteDefinition"
	}
	consumed, nodes := d.parseMany(i, stop)
	definition := FootnoteDefinition{d.tokens[i].Pos(), name, nodes, false}
	return consumed, definition
}

func (n FootnoteDefinition) String() string { return orgWriter.WriteNodesAsString(n) }
func (n FootnoteDefinition) GetPos() Pos    { return n.Pos }
func (n FootnoteDefinition) GetEnd() Pos {
	if len(n.Children) > 0 {
		return n.Children[len(n.Children)-1].GetEnd()
	}
	return n.GetPos()
}
