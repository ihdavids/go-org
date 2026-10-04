package org

import (
	"regexp"
	"strings"
	"unicode"
)

type Block struct {
	Name       string
	Pos        Pos
	EndPos     Pos
	Parameters []string // the language (of a src block) or backend (of an export block), then header arguments as key, value pairs
	// The switches of the block, `-n -r -l "(ref:%s)"` - written between the
	// language and the header arguments, as separate words.
	Switches []string
	Children []Node
	Result   Node
	Keywords []Keyword
}

type Result struct {
	Pos   Pos
	Node  Node
	Hash  string // the hash of `#+RESULTS[hash]:`, set when org caches the result
	Value string // what follows the colon, `#+RESULTS: name`
}

type Example struct {
	Pos      Pos
	Children []Node
}

var exampleLineRegexp = regexp.MustCompile(`^(\s*):(\s(.*)|\s*$)`)

// A block name is anything up to the first space: `#+BEGIN_my-block` is
// the block my-block, ended by `#+END_my-block`.
var beginBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+BEGIN_(\S+)(.*)`)
var endBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+END_(\S+)`)
var beginDynBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+BEGIN\b(.*)`)
var endDynBlockRegexp = regexp.MustCompile(`(?i)^(\s*)#\+END\b`)
var resultRegexp = regexp.MustCompile(`(?i)^(\s*)#\+RESULTS(?:\[([^\]]*)\])?:\s*(.*)$`)
var exampleBlockEscapeRegexp = regexp.MustCompile(`(^|\n)([ \t]*),([ \t]*)(\*|,\*|#\+|,#\+)`)

func lexBlock(line string, row, col int) (token, bool) {
	if m := beginBlockRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"beginBlock", len(m[1]), strings.ToUpper(m[2]), m, pos, Pos{row, col + len(m[0])}}, true
	} else if m := endBlockRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"endBlock", len(m[1]), strings.ToUpper(m[2]), m, pos, Pos{row, col + len(m[0])}}, true
	} else if m := beginDynBlockRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"beginBlock", len(m[1]), strings.ToUpper("DYN"), m, pos, Pos{row, col + len(m[0])}}, true
	} else if m := endDynBlockRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"endBlock", len(m[1]), strings.ToUpper("DYN"), m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func lexResult(line string, row, col int) (token, bool) {
	if m := resultRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"result", len(m[1]), "", m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func lexExample(line string, row, col int) (token, bool) {
	if m := exampleLineRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"example", len(m[1]), m[3], m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

/*
	func isRawTextBlock(name string) bool {
		return name == "SRC" || name == "EXAMPLE" || name == "EXPORT" || name == "VERSE" || name == "QUOTE" || name == "CUSTOM"
	}
*/
// The contents of a raw block are text, not org: a `* line` in a COMMENT
// block is not a heading.
func isRawTextBlock(name string) bool {
	return name == "SRC" || name == "EXAMPLE" || name == "EXPORT" || name == "COMMENT"
}

func (d *Document) parseBlock(i int, parentStop stopFn) (int, Node) {
	t, start := d.tokens[i], i
	name := t.content
	var parameters, switches []string
	if name == "DYN" {
		parameters, switches = splitParametersAndSwitches(t.matches[2])
	} else {
		parameters, switches = splitParametersAndSwitches(t.matches[3])
	}
	trim := trimIndentUpTo(d.tokens[i].lvl)
	stop := func(d *Document, i int) bool {
		return i >= len(d.tokens) || (d.tokens[i].kind == "endBlock" && d.tokens[i].content == name)
	}
	block, i := &Block{Name: name, Pos: d.tokens[start].Pos(), EndPos: d.tokens[start].EndPos(), Parameters: parameters, Switches: switches, Keywords: d.lastKeywords}, i+1
	d.lastKeywords = nil
	if isRawTextBlock(name) {
		rawText := ""
		for ; !stop(d, i); i++ {
			rawText += trim(d.tokens[i].matches[0]) + "\n"
		}
		if name == "EXAMPLE" || (name == "SRC" && len(parameters) >= 1 && parameters[0] == "org") {
			rawText = exampleBlockEscapeRegexp.ReplaceAllString(rawText, "$1$2$3$4")
		}
		block.Children = d.parseRawInline(rawText, i)
	} else {
		consumed, nodes := d.parseMany(i, stop)
		block.Children = nodes
		i += consumed
	}
	if i >= len(d.tokens) || d.tokens[i].kind != "endBlock" || d.tokens[i].content != name {
		return 0, nil
	}
	block.EndPos = d.tokens[i].EndPos()
	if name == "SRC" {
		consumed, result := d.parseSrcBlockResult(i+1, parentStop)
		block.Result = result
		i += consumed
	}
	return i + 1 - start, block
}

func (d *Document) parseSrcBlockResult(i int, parentStop stopFn) (int, Node) {
	start := i
	for ; !parentStop(d, i) && d.tokens[i].kind == "text" && d.tokens[i].content == ""; i++ {
	}
	if parentStop(d, i) || d.tokens[i].kind != "result" {
		return 0, nil
	}
	consumed, result := d.parseResult(i, parentStop)
	return (i - start) + consumed, result
}

func (d *Document) parseExample(i int, parentStop stopFn) (int, Node) {
	example, start := Example{}, i
	example.Pos = d.tokens[i].Pos()
	for ; !parentStop(d, i) && d.tokens[i].kind == "example"; i++ {
		p := d.tokens[i].Pos()
		content := d.tokens[i].content
		example.Children = append(example.Children, Text{p, computeTextEnd(p, content), content, true})
	}
	return i - start, example
}

func (d *Document) parseResult(i int, parentStop stopFn) (int, Node) {
	if i+1 >= len(d.tokens) {
		return 0, nil
	}
	m := d.tokens[i].matches
	consumed, node := d.parseOne(i+1, parentStop)
	return consumed + 1, Result{Pos: d.tokens[i].Pos(), Node: node, Hash: m[2], Value: strings.TrimSpace(m[3])}
}

func trimIndentUpTo(max int) func(string) string {
	return func(line string) string {
		i := 0
		for ; i < len(line) && i < max && unicode.IsSpace(rune(line[i])); i++ {
		}
		return line[i:]
	}
}

// splitParametersAndSwitches splits what follows #+BEGIN_NAME into the
// parameters - the language or backend, then the header arguments - and the
// switches, the words starting with - or + that come before the first header
// argument. `python -n :results output` is the language python, the switch -n
// and the argument :results output; read as one, the language was "python -n".
func splitParametersAndSwitches(s string) ([]string, []string) {
	parameters := splitParameters(s)
	if len(parameters) == 0 || strings.HasPrefix(parameters[0], ":") {
		return parameters, nil
	}
	words := splitSwitchWords(parameters[0])
	lang, switches := "", []string{}
	for i, w := range words {
		if i == 0 && !strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "+") {
			lang = w
			continue
		}
		switches = append(switches, w)
	}
	if len(switches) == 0 {
		switches = nil
	}
	if lang == "" {
		return parameters[1:], switches
	}
	parameters[0] = lang
	return parameters, switches
}

// splitSwitchWords splits on spaces, keeping a quoted string - the argument of
// -l "(ref:%s)" - as one word with its quotes.
func splitSwitchWords(s string) []string {
	words, current, inQuotes := []string{}, strings.Builder{}, false
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case unicode.IsSpace(r) && !inQuotes:
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

func splitParameters(s string) []string {
	parameters, parts := []string{}, strings.Split(s, " :")
	lang, rest := strings.TrimSpace(parts[0]), parts[1:]
	if lang != "" {
		parameters = append(parameters, lang)
	}
	for _, p := range rest {
		kv := strings.SplitN(p+" ", " ", 2)
		parameters = append(parameters, ":"+kv[0], strings.TrimSpace(kv[1]))
	}
	return parameters
}

func (b Block) ParameterMap() map[string]string {
	if len(b.Parameters) == 0 {
		return nil
	}
	m := map[string]string{":lang": b.Parameters[0]}
	for i := 1; i+1 < len(b.Parameters); i += 2 {
		m[b.Parameters[i]] = b.Parameters[i+1]
	}
	return m
}

func (n Example) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Block) String() string   { return orgWriter.WriteNodesAsString(n) }
func (n Result) String() string  { return orgWriter.WriteNodesAsString(n) }
func (self Example) GetPos() Pos { return self.Pos }
func (self Example) GetEnd() Pos {
	if len(self.Children) > 0 {
		return self.Children[len(self.Children)-1].GetEnd()
	}
	return self.GetPos()
}
func (self Block) GetPos() Pos { return self.Pos }
func (self Block) GetEnd() Pos {
	/*
		if len(self.Children) > 0 {
			return self.Children[len(self.Children)-1].GetEnd()
		}
		return self.GetPos()
	*/
	return self.EndPos
}
func (self Result) GetPos() Pos { return self.Pos }
func (self Result) GetEnd() Pos {
	return self.Node.GetEnd()
}

// Magnet fidget pen
// resticable tool holder
// Open any size jar

func (n Example) GetType() NodeType { return ExampleNode }
func (n Block) GetType() NodeType   { return BlockNode }
func (n Result) GetType() NodeType  { return ResultNode }

func (n Example) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n Block) GetTypeName() string   { return GetNodeTypeName(n.GetType()) }
func (n Result) GetTypeName() string  { return GetNodeTypeName(n.GetType()) }

func (n Example) GetChildren() []Node { return n.Children }
func (n Block) GetChildren() []Node   { return n.Children }
func (n Result) GetChildren() []Node  { return nil }
