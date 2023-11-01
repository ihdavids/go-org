package org

import (
	"math"
	"regexp"
	"strings"
)

type Paragraph struct {
	Pos      Pos
	EndPos   Pos
	Children []Node
}
type HorizontalRule struct {
	Pos Pos
}

var horizontalRuleRegexp = regexp.MustCompile(`^(\s*)-{5,}\s*$`)
var plainTextRegexp = regexp.MustCompile(`^(\s*)(.*)`)

func lexText(line string, row, col int) (token, bool) {
	if m := plainTextRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		return token{"text", len(m[1]), m[2], m, pos, computeTextEnd(pos, m[1]+m[2])}, true
	}
	return nilToken, false
}

func lexHorizontalRule(line string, row, col int) (token, bool) {
	if m := horizontalRuleRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		return token{"horizontalRule", len(m[1]), "", m, pos, Pos{row, col + len(m[1])}}, true
	}
	return nilToken, false
}

func (d *Document) parseParagraph(i int, parentStop stopFn) (int, Node) {
	lines, start := []string{d.tokens[i].content}, i
	stop := func(d *Document, i int) bool {
		return parentStop(d, i) || d.tokens[i].kind != "text" || d.tokens[i].content == ""
	}
	for i += 1; !stop(d, i); i++ {
		lvl := math.Max(float64(d.tokens[i].lvl-d.baseLvl), 0)
		lines = append(lines, strings.Repeat(" ", int(lvl))+d.tokens[i].content)
	}
	consumed := i - start
	end := i - 1
	return consumed, Paragraph{d.tokens[start].Pos(), d.tokens[end].EndPos(), d.parseInline(strings.Join(lines, "\n"), start)}
}

func (d *Document) parseHorizontalRule(i int, parentStop stopFn) (int, Node) {
	return 1, HorizontalRule{d.tokens[i].Pos()}
}

func (n Paragraph) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n HorizontalRule) String() string { return orgWriter.WriteNodesAsString(n) }
func (n HorizontalRule) GetPos() Pos    { return n.Pos }
func (n Paragraph) GetPos() Pos         { return n.Pos }
func (n HorizontalRule) GetEnd() Pos    { return n.Pos }
func (n Paragraph) GetEnd() Pos {
	return n.EndPos
}
