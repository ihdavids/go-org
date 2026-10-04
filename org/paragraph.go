package org

import (
	"math"
	"regexp"
	"strings"
	"unicode"
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
	// A LaTeX environment, \begin{name} at the start of a line, runs to its
	// \end{name} whatever is in between - blank lines included, and lines that
	// would otherwise read as lists, tables or keywords.
	environmentEnd := d.latexEnvironmentEnd(i, parentStop)
	stop := func(d *Document, i int) bool {
		if i <= environmentEnd {
			return parentStop(d, i)
		}
		return parentStop(d, i) || d.tokens[i].kind != "text" || d.tokens[i].content == "" || environmentEnd >= 0
	}
	for i += 1; !stop(d, i); i++ {
		content := d.tokens[i].content
		if i <= environmentEnd {
			content = strings.TrimLeftFunc(d.tokens[i].matches[0], unicode.IsSpace)
		}
		lvl := math.Max(float64(d.tokens[i].lvl-d.baseLvl), 0)
		if content == "" {
			lvl = 0
		}
		lines = append(lines, strings.Repeat(" ", int(lvl))+content)
	}
	consumed := i - start
	end := i - 1
	return consumed, Paragraph{d.tokens[start].Pos(), d.tokens[end].EndPos(), d.parseInline(strings.Join(lines, "\n"), start)}
}

// latexEnvironmentEnd returns the index of the token closing the LaTeX
// environment that the token at i opens, or -1 if it opens none.
func (d *Document) latexEnvironmentEnd(i int, parentStop stopFn) int {
	m := latexEnvironmentBeginRegexp.FindStringSubmatch(d.tokens[i].content)
	if m == nil {
		return -1
	}
	closing := `\end{` + m[1] + `}`
	if strings.Contains(d.tokens[i].content, closing) {
		return -1
	}
	for j := i + 1; j < len(d.tokens) && !parentStop(d, j); j++ {
		if d.tokens[j].kind == "headline" {
			return -1
		}
		if strings.Contains(d.tokens[j].matches[0], closing) {
			return j
		}
	}
	return -1
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
func (n Paragraph) GetType() NodeType        { return ParagraphNode }
func (n Paragraph) GetTypeName() string      { return GetNodeTypeName(n.GetType()) }
func (n HorizontalRule) GetType() NodeType   { return HorizontalRuleNode }
func (n HorizontalRule) GetTypeName() string { return GetNodeTypeName(n.GetType()) }

func (n Paragraph) GetChildren() []Node      { return n.Children }
func (n HorizontalRule) GetChildren() []Node { return nil }
