package org

import (
	"regexp"
	"strings"
)

type Drawer struct {
	Pos      Pos
	Name     string
	Children []Node
}

type PropertyDrawer struct {
	Pos        Pos
	Properties [][]string
}

var beginDrawerRegexp = regexp.MustCompile(`^(\s*):(\S+):\s*$`)
var endDrawerRegexp = regexp.MustCompile(`(?i)^(\s*):END:\s*$`)
var propertyRegexp = regexp.MustCompile(`^(\s*):(\S+):(\s+(.*)$|$)`)

func lexDrawer(line string, row, col int) (token, bool) {
	if m := endDrawerRegexp.FindStringSubmatch(line); m != nil {
		return token{"endDrawer", len(m[1]), "", m, Pos{row, col}}, true
	} else if m := beginDrawerRegexp.FindStringSubmatch(line); m != nil {
		return token{"beginDrawer", len(m[1]), strings.ToUpper(m[2]), m, Pos{row, col}}, true
	}
	return nilToken, false
}

func (d *Document) parseDrawer(i int, parentStop stopFn) (int, Node) {
	name := strings.ToUpper(d.tokens[i].content)
	if name == "PROPERTIES" {
		return d.parsePropertyDrawer(i, parentStop)
	}
	drawer, start := Drawer{Pos: d.tokens[i].Pos(), Name: name}, i
	i++
	stop := func(d *Document, i int) bool {
		if parentStop(d, i) {
			return true
		}
		kind := d.tokens[i].kind
		return kind == "beginDrawer" || kind == "endDrawer" || kind == "headline"
	}
	for {
		consumed, nodes := d.parseMany(i, stop)
		i += consumed
		drawer.Children = append(drawer.Children, nodes...)
		if i < len(d.tokens) && d.tokens[i].kind == "beginDrawer" {
			p := Paragraph{d.tokens[i].Pos(), []Node{Text{d.tokens[i].Pos(), ":" + d.tokens[i].content + ":", false}}}
			drawer.Children = append(drawer.Children, p)
			i++
		} else {
			break
		}
	}
	if i < len(d.tokens) && d.tokens[i].kind == "endDrawer" {
		i++
	}
	return i - start, drawer
}

func (d *Document) parsePropertyDrawer(i int, parentStop stopFn) (int, Node) {
	drawer, start := PropertyDrawer{Pos: d.tokens[i].Pos()}, i
	i++
	stop := func(d *Document, i int) bool {
		return parentStop(d, i) || (d.tokens[i].kind != "text" && d.tokens[i].kind != "beginDrawer")
	}
	for ; !stop(d, i); i++ {
		m := propertyRegexp.FindStringSubmatch(d.tokens[i].matches[0])
		if m == nil {
			return 0, nil
		}
		k, v := strings.ToUpper(m[2]), strings.TrimSpace(m[4])
		drawer.Properties = append(drawer.Properties, []string{k, v})
	}
	if i < len(d.tokens) && d.tokens[i].kind == "endDrawer" {
		i++
	} else {
		return 0, nil
	}
	return i - start, drawer
}

func (d *PropertyDrawer) Get(key string) (string, bool) {
	if d == nil {
		return "", false
	}
	for _, kvPair := range d.Properties {
		if kvPair[0] == key {
			return kvPair[1], true
		}
	}
	return "", false
}

func (n Drawer) String() string         { return orgWriter.WriteNodesAsString(n) }
func (n PropertyDrawer) String() string { return orgWriter.WriteNodesAsString(n) }
func (n PropertyDrawer) GetPos() Pos    { return n.Pos }
func (n PropertyDrawer) GetEnd() Pos {
	if len(n.Properties) > 0 {
		cnt := len(n.Properties)
		pos := n.GetPos()
		pos.Row += cnt
		return pos
	}
	return n.GetPos()
}
func (n Drawer) GetPos() Pos { return n.Pos }
func (n Drawer) GetEnd() Pos {
	if len(n.Children) > 0 {
		return n.Children[len(n.Children)-1].GetEnd()
	}
	return n.GetPos()
}
