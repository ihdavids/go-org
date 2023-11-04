package org

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type List struct {
	Kind  string
	Pos   Pos
	Items []Node
}

type ListItem struct {
	Bullet   string
	Status   string
	Value    string
	Pos      Pos
	Children []Node
}

type DescriptiveListItem struct {
	Bullet  string
	Status  string
	Pos     Pos
	Term    []Node
	Details []Node
}

var unorderedListRegexp = regexp.MustCompile(`^(\s*)([+*-])(\s+(.*)|$)`)
var orderedListRegexp = regexp.MustCompile(`^(\s*)(([0-9]+|[a-zA-Z])[.)])(\s+(.*)|$)`)
var descriptiveListItemRegexp = regexp.MustCompile(`\s::(\s|$)`)
var listItemValueRegexp = regexp.MustCompile(`\[@(\d+)\]\s`)
var listItemStatusRegexp = regexp.MustCompile(`\[( |X|-)\]\s`)

func lexList(line string, row, col int) (token, bool) {
	if m := unorderedListRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		pos.Col += len(m[1])
		return token{"unorderedList", len(m[1]), m[4], m, pos, Pos{row, col + len(m[0])}}, true
	} else if m := orderedListRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		pos.Col += len(m[1])
		return token{"orderedList", len(m[1]), m[5], m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func isListToken(t token) bool {
	return t.kind == "unorderedList" || t.kind == "orderedList"
}

func listKind(t token) (string, string) {
	kind := ""
	switch bullet := t.matches[2]; {
	case bullet == "*" || bullet == "+" || bullet == "-":
		kind = "unordered"
	case unicode.IsLetter(rune(bullet[0])), unicode.IsDigit(rune(bullet[0])):
		kind = "ordered"
	default:
		panic(fmt.Sprintf("bad list bullet '%s': %#v", bullet, t))
	}
	if descriptiveListItemRegexp.MatchString(t.content) {
		return kind, "descriptive"
	}
	return kind, kind
}

func (d *Document) parseList(i int, parentStop stopFn) (int, Node) {
	start, lvl := i, d.tokens[i].lvl
	listMainKind, kind := listKind(d.tokens[i])
	list := List{Kind: kind, Pos: (d.tokens[start].Pos())}
	stop := func(*Document, int) bool {
		if parentStop(d, i) || d.tokens[i].lvl != lvl || !isListToken(d.tokens[i]) {
			return true
		}
		itemMainKind, _ := listKind(d.tokens[i])
		return itemMainKind != listMainKind
	}
	for !stop(d, i) {
		consumed, node := d.parseListItem(list, i, parentStop)
		i += consumed
		list.Items = append(list.Items, node)
	}
	return i - start, list
}

func (d *Document) parseListItem(l List, i int, parentStop stopFn) (int, Node) {
	start, nodes, bullet := i, []Node{}, d.tokens[i].matches[2]
	minIndent, dterm, content, status, value := d.tokens[i].lvl+len(bullet), "", d.tokens[i].content, "", ""
	originalBaseLvl := d.baseLvl
	d.baseLvl = minIndent + 1
	if m := listItemValueRegexp.FindStringSubmatch(content); m != nil && l.Kind == "ordered" {
		value, content = m[1], content[len("[@] ")+len(m[1]):]
	}
	if m := listItemStatusRegexp.FindStringSubmatch(content); m != nil {
		status, content = m[1], content[len("[ ] "):]
	}
	if l.Kind == "descriptive" {
		if m := descriptiveListItemRegexp.FindStringIndex(content); m != nil {
			dterm, content = content[:m[0]], content[m[1]:]
			d.baseLvl = strings.Index(d.tokens[i].matches[0], " ::") + 4
		}
	}

	pos := d.tokens[start].Pos() // This is not right!
	pos = Pos{Row: pos.Row, Col: pos.Col}
	if d.tokens[start].kind == "text" {
		if tok, ok := lexList(d.tokens[start].content, pos.Row, pos.Col); ok {
			pos = tok.pos
		}
	}
	d.tokens[i] = tokenize(strings.Repeat(" ", minIndent)+content, d.tokens[i].Pos().Row)
	d.tokens[i].pos.Col += 1
	d.tokens[i].endPos.Col += 1
	stop := func(d *Document, i int) bool {
		if parentStop(d, i) {
			return true
		}
		t := d.tokens[i]
		return t.lvl < minIndent && !(t.kind == "text" && t.content == "")
	}
	for !stop(d, i) && (i <= start+1 || !isSecondBlankLine(d, i)) {
		consumed, node := d.parseOne(i, stop)
		i += consumed
		nodes = append(nodes, node)
	}
	d.baseLvl = originalBaseLvl
	if l.Kind == "descriptive" {
		return i - start, DescriptiveListItem{bullet, status, pos, d.parseInline(dterm, i), nodes}
	}
	return i - start, ListItem{bullet, status, value, pos, nodes}
}

func (n List) String() string                { return orgWriter.WriteNodesAsString(n) }
func (n ListItem) String() string            { return orgWriter.WriteNodesAsString(n) }
func (n DescriptiveListItem) String() string { return orgWriter.WriteNodesAsString(n) }
func (self DescriptiveListItem) GetPos() Pos { return self.Pos }
func (self ListItem) GetPos() Pos            { return self.Pos }
func (self List) GetPos() Pos                { return self.Pos }
func (self DescriptiveListItem) GetEnd() Pos {
	if len(self.Details) > 0 {
		return self.Details[len(self.Details)-1].GetEnd()
	}
	return self.GetPos()
}
func (self ListItem) GetEnd() Pos {
	if len(self.Children) > 0 {
		return self.Children[len(self.Children)-1].GetEnd()
	}
	return self.GetPos()
}
func (self List) GetEnd() Pos {
	if len(self.Items) > 0 {
		return self.Items[len(self.Items)-1].GetEnd()
	}
	return self.GetPos()
}

func (n List) GetType() NodeType                { return ListNode }
func (n ListItem) GetType() NodeType            { return ListItemNode }
func (n DescriptiveListItem) GetType() NodeType { return DescriptiveListItemNode }

func (n List) GetTypeName() string                { return GetNodeTypeName(n.GetType()) }
func (n ListItem) GetTypeName() string            { return GetNodeTypeName(n.GetType()) }
func (n DescriptiveListItem) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
