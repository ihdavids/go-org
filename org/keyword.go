package org

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
)

type Comment struct {
	Pos     Pos
	EndPos  Pos
	Content string
}

type Keyword struct {
	Pos    Pos
	EndPos Pos
	Key    string
	Value  string
}

type NodeWithName struct {
	Name string
	Node Node
}

type NodeWithMeta struct {
	Pos  Pos
	Node Node
	Meta Metadata
}

type Metadata struct {
	Caption         [][]Node
	HTMLAttributes  [][]string
	LatexAttributes [][]string
	LatexEnv        string
}

type Include struct {
	EndPos Pos
	Keyword
	Resolve func() Node
}

var keywordRegexp = regexp.MustCompile(`^(\s*)#\+([a-zA-Z][^:]*):(\s*(.*)|$)`)
var commentRegexp = regexp.MustCompile(`^(\s*)#\s(.*)`)

var includeFileRegexp = regexp.MustCompile(`(?i)^"([^"]+)" (src|example|export) (\w+)$`)
var attributeRegexp = regexp.MustCompile(`(?:^|\s+)(:[-\w]+)\s+(.*)$`)

func lexKeywordOrComment(line string, row, col int) (token, bool) {
	if m := keywordRegexp.FindStringSubmatch(line); m != nil {
		return token{"keyword", len(m[1]), m[0], m, Pos{row, col + len(m[1])}, Pos{row, col + len(m[0])}}, true
	} else if m := commentRegexp.FindStringSubmatch(line); m != nil {
		return token{"comment", len(m[1]), m[2], m, Pos{row, col + len(m[1])}, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func (d *Document) parseComment(i int, stop stopFn) (int, Node) {
	p := d.tokens[i].Pos()
	return 1, Comment{p, Pos{Row: p.Row, Col: p.Col + len(d.tokens[i].content)}, d.tokens[i].content}
}

func (d *Document) parseKeyword(i int, stop stopFn) (int, Node) {
	k := parseKeyword(d.tokens[i], i)
	switch k.Key {
	case "NAME":
		return d.parseNodeWithName(k, i, stop)
	case "SETUPFILE":
		return d.loadSetupFile(k)
	case "INCLUDE":
		return d.parseInclude(k, i)
	case "LINK":
		if parts := strings.SplitN(k.Value, " ", 2); len(parts) == 2 {
			d.Links[parts[0]] = parts[1]
		}
		return 1, k
	case "MACRO":
		if parts := strings.Split(k.Value, " "); len(parts) >= 2 {
			d.Macros[parts[0]] = parts[1]
		}
		return 1, k
	case "CAPTION", "ATTR_HTML", "ENV", "ATTR_LATEX":
		consumed, node := d.parseAffiliated(i, stop)
		if consumed != 0 {
			return consumed, node
		}
		fallthrough
	case "TBLFM":
		return d.parseTableFormat(k)
	default:
		if _, ok := d.BufferSettings[k.Key]; ok {
			d.BufferSettings[k.Key] = strings.Join([]string{d.BufferSettings[k.Key], k.Value}, "\n")
		} else {
			d.BufferSettings[k.Key] = k.Value
		}
		// Keep a record of all generic keywords that did not have a direct impact on a node.
		d.lastKeywords = append(d.lastKeywords, k)
		return 1, k
	}
}

func Last[E any](s []E) (E, bool) {
	if len(s) == 0 {
		var zero E
		return zero, false
	}
	return s[len(s)-1], true
}

func (d *Document) parseTableFormat(k Keyword) (int, Node) {
	ch := d.currentHeadline.Get()
	if ch != nil && ch.Tables != nil && len(ch.Tables) > 0 {
		// Modern org mode allows for multiple TBLFM statements one after another.
		if ch.Tables[len(ch.Tables)-1].Formulas == nil {
			ch.Tables[len(ch.Tables)-1].Formulas = &Formulas{Keywords: []*Keyword{&k}}
		} else {
			ch.Tables[len(ch.Tables)-1].Formulas.AppendKeyword(&k)
		}
	}
	return 1, k
}

func (d *Document) parseNodeWithName(k Keyword, i int, stop stopFn) (int, Node) {
	if stop(d, i+1) {
		return 0, nil
	}
	consumed, node := d.parseOne(i+1, stop)
	if consumed == 0 || node == nil {
		return 0, nil
	}
	d.NamedNodes[k.Value] = node
	return consumed + 1, &NodeWithName{k.Value, node}
}

func (d *Document) parseAffiliated(i int, stop stopFn) (int, Node) {
	start, meta := i, Metadata{}
	startPos := d.tokens[i].pos
	for ; !stop(d, i) && d.tokens[i].kind == "keyword"; i++ {
		switch k := parseKeyword(d.tokens[i], i); k.Key {
		case "CAPTION":
			meta.Caption = append(meta.Caption, d.parseInline(k.Value, i))
		case "ATTR_HTML":
			attributes, rest := []string{}, k.Value
			for {
				if k, m := "", attributeRegexp.FindStringSubmatch(rest); m != nil {
					k, rest = m[1], m[2]
					attributes = append(attributes, k)
					if v, m := "", attributeRegexp.FindStringSubmatchIndex(rest); m != nil {
						v, rest = rest[:m[0]], rest[m[0]:]
						attributes = append(attributes, v)
					} else {
						attributes = append(attributes, strings.TrimSpace(rest))
						break
					}
				} else {
					break
				}
			}
			meta.HTMLAttributes = append(meta.HTMLAttributes, attributes)
		case "ATTR_LATEX":
			attributes, rest := []string{}, k.Value
			for {
				if k, m := "", attributeRegexp.FindStringSubmatch(rest); m != nil {
					k, rest = m[1], m[2]
					attributes = append(attributes, k)
					if v, m := "", attributeRegexp.FindStringSubmatchIndex(rest); m != nil {
						v, rest = rest[:m[0]], rest[m[0]:]
						attributes = append(attributes, v)
					} else {
						attributes = append(attributes, strings.TrimSpace(rest))
						break
					}
				} else {
					break
				}
			}
			meta.LatexAttributes = append(meta.LatexAttributes, attributes)
		case "ENV":
			meta.LatexEnv = strings.TrimSpace(k.Value)
		default:
			return 0, nil
		}
	}
	if stop(d, i) {
		return 0, nil
	}
	consumed, node := d.parseOne(i, stop)
	if consumed == 0 || node == nil {
		return 0, nil
	}
	i += consumed
	return i - start, NodeWithMeta{startPos, node, meta}
}

func parseKeyword(t token, ni int) Keyword {
	k, v := t.matches[2], t.matches[4]
	p := t.pos
	return Keyword{t.pos, Pos{Row: p.Row, Col: p.Col + len(t.content)}, strings.ToUpper(k), strings.TrimSpace(v)}
}

// TODO: This will break the token stream positions in the file!
func (d *Document) parseInclude(k Keyword, ni int) (int, Node) {
	resolve := func() Node {
		d.Log.Printf("Bad include %#v", k)
		return k
	}
	var m []string
	if m = includeFileRegexp.FindStringSubmatch(k.Value); m != nil {
		path, kind, lang := m[1], m[2], m[3]
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(d.Path), path)
		}
		resolve = func() Node {
			bs, err := d.ReadFile(path)
			if err != nil {
				d.Log.Printf("Bad include %#v: %s", k, err)
				return k
			}
			b := Block{strings.ToUpper(kind), k.Pos, k.GetEnd(), []string{lang}, d.parseRawInline(string(bs), ni), nil, d.lastKeywords}
			d.lastKeywords = nil
			return b
		}
	}
	p := k.GetEnd()
	return 1, Include{Pos{p.Row, p.Col + len(m[0])}, k, resolve}
}

func (d *Document) loadSetupFile(k Keyword) (int, Node) {
	path := k.Value
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(d.Path), path)
	}
	bs, err := d.ReadFile(path)
	if err != nil {
		d.Log.Printf("Bad setup file: %#v: %s", k, err)
		return 1, k
	}
	setupDocument := d.Configuration.Parse(bytes.NewReader(bs), path)
	if err := setupDocument.Error; err != nil {
		d.Log.Printf("Bad setup file: %#v: %s", k, err)
		return 1, k
	}
	for k, v := range setupDocument.BufferSettings {
		d.BufferSettings[k] = v
	}
	return 1, k
}

func (n Comment) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n Keyword) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n NodeWithMeta) String() string { return orgWriter.WriteNodesAsString(n) }
func (n NodeWithName) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Include) String() string      { return orgWriter.WriteNodesAsString(n) }

func (n Include) GetPos() Pos      { return n.Keyword.GetPos() }
func (n NodeWithName) GetPos() Pos { return n.Node.GetPos() }
func (n NodeWithMeta) GetPos() Pos { return n.Pos }
func (n Comment) GetPos() Pos      { return n.Pos }
func (n Keyword) GetPos() Pos      { return n.Pos }
func (n Include) GetEnd() Pos      { return n.EndPos }
func (n NodeWithName) GetEnd() Pos { return n.Node.GetEnd() }
func (n NodeWithMeta) GetEnd() Pos { return n.Node.GetEnd() } // Metadata precedes the node, so ignore
func (n Comment) GetEnd() Pos      { return n.EndPos }
func (n Keyword) GetEnd() Pos      { return n.EndPos }

func (n Comment) GetType() NodeType      { return CommentNode }
func (n Keyword) GetType() NodeType      { return KeywordNode }
func (n NodeWithMeta) GetType() NodeType { return NodeWithMetaNode }
func (n NodeWithName) GetType() NodeType { return NodeWithNameNode }
func (n Include) GetType() NodeType      { return IncludeNode }

func (n Comment) GetTypeName() string      { return GetNodeTypeName(n.GetType()) }
func (n Keyword) GetTypeName() string      { return GetNodeTypeName(n.GetType()) }
func (n NodeWithMeta) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n NodeWithName) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n Include) GetTypeName() string      { return GetNodeTypeName(n.GetType()) }

func (n Comment) GetChildren() []Node      { return nil }
func (n Keyword) GetChildren() []Node      { return nil }
func (n NodeWithMeta) GetChildren() []Node { return nil }
func (n NodeWithName) GetChildren() []Node { return []Node{n.Node} }
func (n Include) GetChildren() []Node      { return nil }

func (n NodeWithName) IsTable() bool { return n.GetType() == TableNode }
