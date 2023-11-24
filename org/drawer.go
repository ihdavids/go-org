package org

import (
	"regexp"
	"strings"
)

type Drawer struct {
	Pos      Pos
	EndPos   Pos
	Name     string
	Children []Node
}

type PropertyDrawer struct {
	Pos        Pos
	EndPos     Pos
	Properties [][]string
}

var beginDrawerRegexp = regexp.MustCompile(`^(\s*):(\S+):\s*$`)
var endDrawerRegexp = regexp.MustCompile(`(?i)^(\s*):END:\s*$`)
var propertyRegexp = regexp.MustCompile(`^(\s*):(\S+):(\s+(.*)$|$)`)

func lexDrawer(line string, row, col int) (token, bool) {
	if m := endDrawerRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"endDrawer", len(m[1]), "", m, pos, Pos{row, col + len(m[0])}}, true
	} else if m := beginDrawerRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"beginDrawer", len(m[1]), strings.ToUpper(m[2]), m, pos, Pos{row, col + len(m[0])}}, true
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
		if kind == "endDrawer" {
			drawer.EndPos = d.tokens[i].EndPos()
			return true
		}
		if kind == "beginDrawer" || kind == "headline" {
			drawer.EndPos = d.tokens[i].Pos()
			return true
		}
		return false
	}
	for {
		consumed, nodes := d.parseMany(i, stop)
		i += consumed
		drawer.Children = append(drawer.Children, nodes...)
		if i < len(d.tokens) && d.tokens[i].kind == "beginDrawer" {
			startPos := d.tokens[i].Pos()
			content := d.tokens[i].content
			nodes := []Node{Text{startPos, computeTextEnd(startPos, content), ":" + content + ":", false}}
			end := d.tokens[i].EndPos()
			if len(nodes) > 0 {
				end = nodes[len(nodes)-1].GetEnd()
			}
			p := Paragraph{d.tokens[i].Pos(), end, nodes}
			drawer.Children = append(drawer.Children, p)
			i++
		} else {
			break
		}
	}
	if i < len(d.tokens) && d.tokens[i].kind == "endDrawer" {
		drawer.EndPos = d.tokens[i].EndPos()
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
		drawer.EndPos = d.tokens[i].EndPos()
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

func (d *PropertyDrawer) Set(key string, val string) {
	if d == nil {
		return
	}
	didAdd := false
	for i, kvPair := range d.Properties {
		if kvPair[0] == key {
			d.Properties[i][1] = val
		}
	}
	if !didAdd {
		d.Properties = append(d.Properties, []string{key, val})
	}
}

func (d *PropertyDrawer) Has(key string) bool {
	if d == nil {
		return false
	}
	for _, kvPair := range d.Properties {
		if kvPair[0] == key {
			return true
		}
	}
	return false
}

func (d *PropertyDrawer) Append(key string, val string) {
	if d == nil {
		return
	}
	didAdd := false
	for i, kvPair := range d.Properties {
		if kvPair[0] == key {
			d.Properties[i][1] += val
		}
	}
	if !didAdd {
		d.Properties = append(d.Properties, []string{key, val})
	}
}

func (n *Drawer) Append(h *Headline, node Node) {
	var after Node = n
	if len(n.Children) > 0 {
		after = n.Children[len(n.Children)-1]
	}
	// This is greate but it still has to be added to our nodes list
	n.Children = append(n.Children, node)
	h.Doc.InsertNodeAfter(node, after)
}

func (n Drawer) String() string         { return orgWriter.WriteNodesAsString(n) }
func (n PropertyDrawer) String() string { return orgWriter.WriteNodesAsString(n) }
func (n PropertyDrawer) GetPos() Pos    { return n.Pos }
func (n PropertyDrawer) GetEnd() Pos {
	return n.EndPos
}
func (n Drawer) GetPos() Pos { return n.Pos }
func (n Drawer) GetEnd() Pos {
	return n.EndPos
}

func (n Drawer) GetType() NodeType           { return DrawerNode }
func (n Drawer) GetTypeName() string         { return GetNodeTypeName(n.GetType()) }
func (n PropertyDrawer) GetType() NodeType   { return PropertyDrawerNode }
func (n PropertyDrawer) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
