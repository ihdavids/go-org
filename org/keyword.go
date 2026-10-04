package org

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	// The optional value of a dual keyword, `#+CAPTION[short]: long` or
	// `#+RESULTS[hash]:` - the part in brackets. Empty when there is none.
	Optional string
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
	ShortCaption    [][]Node // ShortCaption[i] is the `[short]` form of Caption[i], nil when there is none.
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

// A comment is a `#` followed by a space or by nothing at all - a lone `#` is
// an empty comment, not a paragraph.
var commentRegexp = regexp.MustCompile(`^(\s*)#(\s(.*)|$)`)

// `#+KEY[optional]:` - the key of a dual keyword with its optional value.
var dualKeywordRegexp = regexp.MustCompile(`^([^\[\]]+)\[(.*)\]$`)

// `#+CALL: name[inside header](arguments) end header`
var babelCallRegexp = regexp.MustCompile(`^([^\[\]()\s]+)(?:\[(.*?)\])?\((.*)\)\s*(.*)$`)
var attributeRegexp = regexp.MustCompile(`(?:^|\s+)(:[-\w]+)\s+(.*)$`)

func lexKeywordOrComment(line string, row, col int) (token, bool) {
	if m := keywordRegexp.FindStringSubmatch(line); m != nil {
		return token{"keyword", len(m[1]), m[0], m, Pos{row, col + len(m[1])}, Pos{row, col + len(m[0])}}, true
	} else if m := commentRegexp.FindStringSubmatch(line); m != nil {
		return token{"comment", len(m[1]), m[3], m, Pos{row, col + len(m[1])}, Pos{row, col + len(m[0])}}, true
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
	case "CALL":
		return d.parseBabelCall(k, i, stop)
	case "LINK":
		if parts := strings.SplitN(k.Value, " ", 2); len(parts) == 2 {
			d.Links[parts[0]] = strings.TrimSpace(parts[1])
		}
		return 1, k
	case "MACRO":
		// The name is the first word and the definition is everything after
		// it. Splitting on every space kept only the first word of the
		// definition, so `#+MACRO: greet Hello there $1` expanded to "Hello".
		if parts := strings.SplitN(k.Value, " ", 2); len(parts) == 2 {
			d.Macros[parts[0]] = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 && parts[0] != "" {
			d.Macros[parts[0]] = ""
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
			var short []Node
			if k.Optional != "" {
				short = d.parseInline(k.Optional, i)
			}
			meta.ShortCaption = append(meta.ShortCaption, short)
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
	k, v, optional := t.matches[2], t.matches[4], ""
	if m := dualKeywordRegexp.FindStringSubmatch(k); m != nil {
		k, optional = m[1], m[2]
	}
	p := t.pos
	return Keyword{Pos: t.pos, EndPos: Pos{Row: p.Row, Col: p.Col + len(t.content)}, Key: strings.ToUpper(k), Value: strings.TrimSpace(v), Optional: optional}
}

// IncludedContent is what an `#+INCLUDE:` of an org file resolves to: the nodes
// of the included file, parsed as org, to be written in place of the keyword.
type IncludedContent struct {
	Pos    Pos
	EndPos Pos
	Path   string
	Nodes  []Node
}

// BabelCall is a `#+CALL: name[inside header](arguments) end header` line,
// along with the `#+RESULTS:` that follows it, if any.
type BabelCall struct {
	Keyword
	Name         string
	InsideHeader string
	Arguments    string
	EndHeader    string
	Result       Node
}

type includeSpec struct {
	path     string // the file, without any ::search option
	search   string // what follows :: in the file name - `*Heading`, `#custom-id` or a #+NAME
	kind     string // src, example, export, quote... or "" for org
	lang     string // the language of a src block or the backend of an export block
	lines    string // the :lines range, "5-10", "-10" or "10-"
	minLevel int    // the :minlevel for the top headlines of an included org file
}

// splitIncludeArgs splits the value of an #+INCLUDE: keyword into words,
// keeping a quoted string together as one word, without its quotes.
func splitIncludeArgs(s string) []string {
	args, current, inQuotes, quoted := []string{}, strings.Builder{}, false, false
	flush := func() {
		if current.Len() > 0 || quoted {
			args = append(args, current.String())
		}
		current.Reset()
		quoted = false
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes, quoted = !inQuotes, true
		case unicode.IsSpace(r) && !inQuotes:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return args
}

func parseIncludeSpec(value string) (includeSpec, bool) {
	spec, args := includeSpec{}, splitIncludeArgs(value)
	if len(args) == 0 || args[0] == "" {
		return spec, false
	}
	spec.path = args[0]
	if i := strings.Index(spec.path, "::"); i >= 0 {
		spec.path, spec.search = spec.path[:i], spec.path[i+2:]
	}
	rest := args[1:]
	for j := 0; j < len(rest); j++ {
		if a := rest[j]; strings.HasPrefix(a, ":") {
			val := ""
			if j+1 < len(rest) && !strings.HasPrefix(rest[j+1], ":") {
				val, j = rest[j+1], j+1
			}
			switch strings.ToLower(a) {
			case ":lines":
				spec.lines = val
			case ":minlevel":
				spec.minLevel, _ = strconv.Atoi(val)
			}
		} else if spec.kind == "" {
			spec.kind = a
		} else if spec.lang == "" {
			spec.lang = a
		}
	}
	return spec, true
}

// selectLines applies an #+INCLUDE: :lines range. As in org, the range is one
// based and its end is excluded: "5-10" is lines 5 to 9.
func selectLines(content, spec string) string {
	parts := strings.SplitN(spec, "-", 2)
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	from, to := 1, len(lines)+1
	if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
		from = v
	}
	if len(parts) == 2 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			to = v
		}
	} else {
		to = from + 1
	}
	if from < 1 {
		from = 1
	}
	if to > len(lines)+1 {
		to = len(lines) + 1
	}
	if from >= to {
		return ""
	}
	return strings.Join(lines[from-1:to-1], "")
}

func (d *Document) resolvePath(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(filepath.Dir(d.Path), path)
	}
	return path
}

func (d *Document) parseInclude(k Keyword, ni int) (int, Node) {
	keywords := d.lastKeywords
	d.lastKeywords = nil
	resolve := func() Node {
		d.Log.Printf("Bad include %#v", k)
		return k
	}
	if spec, ok := parseIncludeSpec(k.Value); ok {
		path := d.resolvePath(spec.path)
		resolve = func() Node {
			for _, ancestor := range append(d.includeAncestors, d.Path) {
				if filepath.Clean(ancestor) == filepath.Clean(path) {
					d.Log.Printf("Bad include %#v: %s includes itself", k, path)
					return k
				}
			}
			bs, err := d.ReadFile(path)
			if err != nil {
				d.Log.Printf("Bad include %#v: %s", k, err)
				return k
			}
			content := string(bs)
			if spec.lines != "" {
				content = selectLines(content, spec.lines)
			}
			kind := strings.ToUpper(spec.kind)
			if kind != "" && isRawTextBlock(kind) {
				parameters := []string{}
				if spec.lang != "" {
					parameters = append(parameters, spec.lang)
				}
				return Block{Name: kind, Pos: k.Pos, EndPos: k.GetEnd(), Parameters: parameters, Children: d.parseRawInline(content, ni), Keywords: keywords}
			}
			included := d.parseIncluded(content, path)
			if included.Error != nil {
				d.Log.Printf("Bad include %#v: %s", k, included.Error)
				return k
			}
			if kind != "" {
				return Block{Name: kind, Pos: k.Pos, EndPos: k.GetEnd(), Children: included.Nodes, Keywords: keywords}
			}
			nodes := included.Nodes
			if spec.search != "" {
				found := findIncludeTarget(included, spec.search)
				if found == nil {
					d.Log.Printf("Bad include %#v: nothing matches ::%s in %s", k, spec.search, path)
					return k
				}
				nodes = []Node{found}
			}
			if spec.minLevel > 0 {
				shiftHeadlines(nodes, spec.minLevel)
			}
			return IncludedContent{Pos: k.Pos, EndPos: k.GetEnd(), Path: path, Nodes: nodes}
		}
	}
	return 1, Include{k.GetEnd(), k, resolve}
}

// parseIncluded parses an included org file with this document's configuration,
// remembering which files are being included so that a cycle can be refused.
func (d *Document) parseIncluded(content, path string) *Document {
	c := *d.Configuration
	ancestors := append(append([]string{}, d.includeAncestors...), d.Path)
	c.includeAncestors = ancestors
	return c.Parse(strings.NewReader(content), path)
}

// findIncludeTarget finds what the ::search part of an included file name names:
// `*Heading` for a heading by its title, `#id` for a heading by its CUSTOM_ID, and
// anything else for an element by its #+NAME.
func findIncludeTarget(d *Document, search string) Node {
	var find func(nodes []Node) Node
	find = func(nodes []Node) Node {
		for _, n := range nodes {
			if h, ok := n.(*Headline); ok {
				switch {
				case strings.HasPrefix(search, "*") && strings.TrimSpace(String(h.Title...)) == strings.TrimSpace(search[1:]):
					return h
				case strings.HasPrefix(search, "#"):
					if id, ok := h.Properties.Get("CUSTOM_ID"); ok && id == search[1:] {
						return h
					}
				}
				if found := find(h.Children); found != nil {
					return found
				}
			}
		}
		return nil
	}
	if strings.HasPrefix(search, "*") || strings.HasPrefix(search, "#") {
		return find(d.Nodes)
	}
	return d.NamedNodes[search]
}

// shiftHeadlines moves the headlines of an included file so that the top ones
// are at minLevel, keeping their relative depths.
func shiftHeadlines(nodes []Node, minLevel int) {
	top := 0
	var walk func(nodes []Node, f func(*Headline))
	walk = func(nodes []Node, f func(*Headline)) {
		for _, n := range nodes {
			if h, ok := n.(*Headline); ok {
				f(h)
				walk(h.Children, f)
			}
		}
	}
	walk(nodes, func(h *Headline) {
		if top == 0 || h.Lvl < top {
			top = h.Lvl
		}
	})
	if top == 0 {
		return
	}
	delta := minLevel - top
	walk(nodes, func(h *Headline) { h.Lvl += delta })
}

func (d *Document) parseBabelCall(k Keyword, i int, stop stopFn) (int, Node) {
	call := BabelCall{Keyword: k, Name: strings.TrimSpace(k.Value)}
	if m := babelCallRegexp.FindStringSubmatch(k.Value); m != nil {
		call.Name, call.InsideHeader, call.Arguments, call.EndHeader = m[1], m[2], m[3], strings.TrimSpace(m[4])
	}
	consumed, result := d.parseSrcBlockResult(i+1, stop)
	call.Result = result
	return consumed + 1, call
}

func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// HTTPReadURL fetches a remote file over http(s). Set Configuration.ReadURL to
// it to allow `#+SETUPFILE:` to load setup files from the web.
func HTTPReadURL(url string) ([]byte, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (d *Document) loadSetupFile(k Keyword) (int, Node) {
	path := strings.Trim(k.Value, `"`)
	var bs []byte
	var err error
	if isURL(path) {
		if d.ReadURL == nil {
			d.Log.Printf("Bad setup file: %#v: remote setup files are disabled (set Configuration.ReadURL)", k)
			return 1, k
		}
		bs, err = d.ReadURL(path)
	} else {
		path = d.resolvePath(path)
		bs, err = d.ReadFile(path)
	}
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
	// Macros and link abbreviations are kept apart from the other settings,
	// and a setup file is as much the place for them as for any other.
	for k, v := range setupDocument.Macros {
		d.Macros[k] = v
	}
	for k, v := range setupDocument.Links {
		d.Links[k] = v
	}
	return 1, k
}

func (n Comment) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n Keyword) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n NodeWithMeta) String() string { return orgWriter.WriteNodesAsString(n) }
func (n NodeWithName) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Include) String() string      { return orgWriter.WriteNodesAsString(n) }

func (n IncludedContent) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n IncludedContent) GetPos() Pos         { return n.Pos }
func (n IncludedContent) GetEnd() Pos         { return n.EndPos }
func (n IncludedContent) GetType() NodeType   { return IncludedContentNode }
func (n IncludedContent) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n IncludedContent) GetChildren() []Node { return n.Nodes }

func (n BabelCall) String() string { return orgWriter.WriteNodesAsString(n) }
func (n BabelCall) GetPos() Pos    { return n.Keyword.GetPos() }
func (n BabelCall) GetEnd() Pos {
	if n.Result != nil {
		return n.Result.GetEnd()
	}
	return n.Keyword.GetEnd()
}
func (n BabelCall) GetType() NodeType   { return BabelCallNode }
func (n BabelCall) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n BabelCall) GetChildren() []Node {
	if n.Result != nil {
		return []Node{n.Result}
	}
	return nil
}

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
