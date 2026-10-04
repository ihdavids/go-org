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
	// A drawer is only a drawer if it is closed by an :END: before the next
	// heading. Otherwise `:smile:` on a line of its own is just text - and
	// reading it as a drawer swallowed the rest of the section, and wrote an
	// :END: into the file that had never been there.
	if !d.hasDrawerEnd(i+1, parentStop) {
		return 0, nil
	}
	drawer, start := &Drawer{Pos: d.tokens[i].Pos(), Name: name}, i
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
	drawer, start := &PropertyDrawer{Pos: d.tokens[i].Pos()}, i
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

func (d *Document) hasDrawerEnd(i int, parentStop stopFn) bool {
	for ; i < len(d.tokens) && !parentStop(d, i); i++ {
		switch d.tokens[i].kind {
		case "endDrawer":
			return true
		case "headline":
			return false
		}
	}
	return false
}

// Get returns the value of a property. A `:KEY+:` line adds to the value of
// KEY rather than being a property of its own - `:VAR: a` then `:VAR+: b` is
// VAR with the value "a b" - as org reads it.
func (d *PropertyDrawer) Get(key string) (string, bool) {
	if d == nil {
		return "", false
	}
	value, found := "", false
	for _, kvPair := range d.Properties {
		switch kvPair[0] {
		case key:
			value, found = kvPair[1], true
		case key + "+":
			if found && value != "" {
				value += " " + kvPair[1]
			} else {
				value = kvPair[1]
			}
			found = true
		}
	}
	return value, found
}

func (d *PropertyDrawer) Set(key string, val string) {
	if d == nil {
		return
	}
	// Setting a property replaces it, including anything `:KEY+:` lines
	// added to it. This used to update the property and then add it a
	// second time as well.
	didSet := false
	properties := d.Properties[:0]
	for _, kvPair := range d.Properties {
		switch kvPair[0] {
		case key:
			if didSet {
				continue
			}
			kvPair[1], didSet = val, true
		case key + "+":
			continue
		}
		properties = append(properties, kvPair)
	}
	d.Properties = properties
	if !didSet {
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
			didAdd = true
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

func (n Drawer) GetChildren() []Node         { return n.Children }
func (n PropertyDrawer) GetChildren() []Node { return nil }
