package org

import (
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func countRune(s string, r rune) int {
	count := 0
	for _, c := range s {
		if c == r {
			count++
		}
	}
	return count
}

type Text struct {
	Pos     Pos
	EndPos  Pos
	Content string
	IsRaw   bool
}

// Schedule data is parsed from timestamps
// SCHEDULE or DEADLINE blocks
type Schedule struct {
	Pos    Pos
	EndPos Pos
}

type LineBreak struct {
	Pos                        Pos
	Count                      int
	BetweenMultibyteCharacters bool
}
type ExplicitLineBreak struct {
	Pos Pos
}

type StatisticToken struct {
	Pos     Pos
	EndPos  Pos
	Content string
}

type Timestamp struct {
	Pos    Pos
	EndPos Pos
	Time   *OrgDate
	/*
		Time     time.Time
		IsDate   bool
		Interval string
	*/
}

type Emphasis struct {
	Pos     Pos
	EndPos  Pos
	Kind    string
	Content []Node
}

type InlineBlock struct {
	Pos        Pos
	EndPos     Pos
	Name       string
	Parameters []string
	Children   []Node
	Keywords   []Keyword
}

type LatexFragment struct {
	Pos         Pos
	OpeningPair string
	ClosingPair string
	Content     []Node
}

type FootnoteLink struct {
	Pos        Pos
	Name       string
	Definition *FootnoteDefinition
}

type RegularLink struct {
	Pos         Pos
	EndPos      Pos
	Protocol    string
	Description []Node
	URL         string
	AutoLink    bool
}

type Macro struct {
	Pos        Pos
	Name       string
	Parameters []string
}

var validURLCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~:/?#[]@!$&'()*+,;="
var autolinkProtocols = regexp.MustCompile(`^(https?|ftp|file)$`)
var imageExtensionRegexp = regexp.MustCompile(`^[.](png|gif|jpe?g|svg|tiff?)$`)
var videoExtensionRegexp = regexp.MustCompile(`^[.](webm|mp4)$`)

var subScriptSuperScriptRegexp = regexp.MustCompile(`^([_^]){([^{}]+?)}`)
var timestampRegexp = regexp.MustCompile(`^<(\d{4}-\d{2}-\d{2})( [A-Za-z]+)?( \d{2}:\d{2})?( \+\d+[dwmy])?>`)
var footnoteRegexp = regexp.MustCompile(`^\[fn:([\w-]*?)(:(.*?))?\]`)
var statisticsTokenRegexp = regexp.MustCompile(`^\[(\d+/\d+|\d+%)\]`)
var latexFragmentRegexp = regexp.MustCompile(`(?s)^\\begin{(\w+)}(.*)\\end{(\w+)}`)
var inlineBlockRegexp = regexp.MustCompile(`src_(\w+)(\[(.*)\])?{(.*)}`)
var inlineExportBlockRegexp = regexp.MustCompile(`@@(\w+):(.*?)@@`)
var macroRegexp = regexp.MustCompile(`{{{(.*)\((.*)\)}}}`)

var timestampFormat = "2006-01-02 Mon 15:04"
var datestampFormat = "2006-01-02 Mon"

var latexFragmentPairs = map[string]string{
	`\(`: `\)`,
	`\[`: `\]`,
	`$$`: `$$`,
	`$`:  `$`,
}

func (d *Document) parseInline(input string, i int) (nodes []Node) {
	previous, current := 0, 0
	newlineOffset := 0
	//inputStart := 0
	for current < len(input) {
		rewind, consumed, node := 0, 0, (Node)(nil)
		switch input[current] {
		case '^':
			consumed, node = d.parseSubOrSuperScript(input, current, i)
		case '_':
			rewind, consumed, node = d.parseSubScriptOrEmphasisOrInlineBlock(input, current, i)
		case '@':
			consumed, node = d.parseInlineExportBlock(input, current, i)
		case '*', '/', '+':
			consumed, node = d.parseEmphasis(input, current, false, i)
		case '=', '~':
			consumed, node = d.parseEmphasis(input, current, true, i)
		case '[':
			consumed, node = d.parseOpeningBracket(input, current, i)
		case '{':
			consumed, node = d.parseMacro(input, current, i)
		case '<':
			consumed, node = d.parseTimestamp(input, current, i)
		case '\\':
			consumed, node = d.parseExplicitLineBreakOrLatexFragment(input, current, i)
		case '$':
			consumed, node = d.parseLatexFragment(input, current, 1, i)
		case '\n':
			newlineOffset += 1
			//inputStart = current
			consumed, node = d.parseLineBreak(input, current, i)
		case ':':
			rewind, consumed, node = d.parseAutoLink(input, current, i)
		}
		current -= rewind
		if consumed != 0 {
			if current > previous {
				inputContent := input[previous:current]
				inputPos := d.tokens[i].Pos()
				if newlineOffset > 0 {
					inputPos.Row += newlineOffset
					inputPos.Col = previous
				}
				nodes = append(nodes, Text{inputPos, computeTextEnd(inputPos, inputContent), inputContent, false})
			}
			if node != nil {
				nodes = append(nodes, node)
			}
			current += consumed
			previous = current
		} else {
			current++
		}
	}

	if previous < len(input) {
		start := d.tokens[i].Pos()
		content := input[previous:]
		nodes = append(nodes, Text{d.tokens[i].Pos(), computeTextEnd(start, content), content, false})
	}
	return nodes
}

func (d *Document) parseRawInline(input string, ni int) (nodes []Node) {
	previous, current := 0, 0
	for current < len(input) {
		if input[current] == '\n' {
			consumed, node := d.parseLineBreak(input, current, ni)
			if current > previous {
				content := input[previous:current]
				start := d.tokens[ni].Pos()
				nodes = append(nodes, Text{start, computeTextEnd(start, content), content, true})
			}
			nodes = append(nodes, node)
			current += consumed
			previous = current
		} else {
			current++
		}
	}
	if previous < len(input) {
		content := input[previous:]
		start := d.tokens[ni].Pos()
		nodes = append(nodes, Text{start, computeTextEnd(start, content), content, true})
	}
	return nodes
}

func (d *Document) parseLineBreak(input string, start int, ni int) (int, Node) {
	i := start
	for ; i < len(input) && input[i] == '\n'; i++ {
	}
	_, beforeLen := utf8.DecodeLastRuneInString(input[:start])
	_, afterLen := utf8.DecodeRuneInString(input[i:])
	return i - start, LineBreak{Pos{d.tokens[ni].Pos().Row, start}, i - start, beforeLen > 1 && afterLen > 1}
}

func (d *Document) parseInlineBlock(input string, start int, ni int) (int, int, Node) {
	if !(strings.HasSuffix(input[:start], "src") && (start-4 < 0 || unicode.IsSpace(rune(input[start-4])))) {
		return 0, 0, nil
	}
	if m := inlineBlockRegexp.FindStringSubmatch(input[start-3:]); m != nil {
		temp := d.lastKeywords
		d.lastKeywords = nil
		return 3, len(m[0]), InlineBlock{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, start + len(m[0])}, "src", strings.Fields(m[1] + " " + m[3]), d.parseRawInline(m[4], ni), temp}
	}
	return 0, 0, nil
}

func (d *Document) parseInlineExportBlock(input string, start int, ni int) (int, Node) {
	if m := inlineExportBlockRegexp.FindStringSubmatch(input[start:]); m != nil {
		temp := d.lastKeywords
		d.lastKeywords = nil
		return len(m[0]), InlineBlock{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, start + len(m[0])}, "export", m[1:2], d.parseRawInline(m[2], ni), temp}
	}
	return 0, nil
}

func (d *Document) parseExplicitLineBreakOrLatexFragment(input string, start int, ni int) (int, Node) {
	switch {
	case start+2 >= len(input):
	case input[start+1] == '\\' && start != 0 && input[start-1] != '\n':
		for i := start + 2; i <= len(input)-1 && unicode.IsSpace(rune(input[i])); i++ {
			if input[i] == '\n' {
				return i + 1 - start, ExplicitLineBreak{Pos{d.tokens[ni].Pos().Row, start}}
			}
		}
	case input[start+1] == '(' || input[start+1] == '[':
		return d.parseLatexFragment(input, start, 2, ni)
	case strings.Index(input[start:], `\begin{`) == 0:
		if m := latexFragmentRegexp.FindStringSubmatch(input[start:]); m != nil {
			if open, content, close := m[1], m[2], m[3]; open == close {
				openingPair, closingPair := `\begin{`+open+`}`, `\end{`+close+`}`
				i := strings.Index(input[start:], closingPair)
				return i + len(closingPair), LatexFragment{Pos{d.tokens[ni].Pos().Row, start}, openingPair, closingPair, d.parseRawInline(content, ni)}
			}
		}
	}
	return 0, nil
}

func (d *Document) parseLatexFragment(input string, start int, pairLength int, ni int) (int, Node) {
	if start+2 >= len(input) {
		return 0, nil
	}
	if pairLength == 1 && input[start:start+2] == "$$" {
		pairLength = 2
	}
	openingPair := input[start : start+pairLength]
	closingPair := latexFragmentPairs[openingPair]
	if i := strings.Index(input[start+pairLength:], closingPair); i != -1 {
		content := d.parseRawInline(input[start+pairLength:start+pairLength+i], ni)
		return i + pairLength + pairLength, LatexFragment{Pos{d.tokens[ni].Pos().Row, start}, openingPair, closingPair, content}
	}
	return 0, nil
}

func (d *Document) parseSubOrSuperScript(input string, start int, ni int) (int, Node) {
	if m := subScriptSuperScriptRegexp.FindStringSubmatch(input[start:]); m != nil {
		fullLen := len(m[2]) + 3
		startRow := d.tokens[ni].Pos().Row
		return fullLen, Emphasis{Pos{startRow, start}, Pos{startRow, start + fullLen}, m[1] + "{}", []Node{Text{Pos{startRow, start}, computeTextEnd(Pos{startRow, start}, m[2]), m[2], false}}}
	}
	return 0, nil
}

func (d *Document) parseSubScriptOrEmphasisOrInlineBlock(input string, start int, ni int) (int, int, Node) {
	if rewind, consumed, node := d.parseInlineBlock(input, start, ni); consumed != 0 {
		return rewind, consumed, node
	} else if consumed, node := d.parseSubOrSuperScript(input, start, ni); consumed != 0 {
		return 0, consumed, node
	}
	consumed, node := d.parseEmphasis(input, start, false, ni)
	return 0, consumed, node
}

func (d *Document) parseOpeningBracket(input string, start int, ni int) (int, Node) {
	if len(input[start:]) >= 2 && input[start] == '[' && input[start+1] == '[' {
		return d.parseRegularLink(input, start, ni)
	} else if footnoteRegexp.MatchString(input[start:]) {
		return d.parseFootnoteReference(input, start, ni)
	} else if statisticsTokenRegexp.MatchString(input[start:]) {
		return d.parseStatisticToken(input, start, ni)
	}
	return 0, nil
}

func (d *Document) parseMacro(input string, start int, ni int) (int, Node) {
	if m := macroRegexp.FindStringSubmatch(input[start:]); m != nil {
		return len(m[0]), Macro{Pos{d.tokens[ni].Pos().Row, start}, m[1], strings.Split(m[2], ",")}
	}
	return 0, nil
}

func (d *Document) parseFootnoteReference(input string, start int, ni int) (int, Node) {
	if m := footnoteRegexp.FindStringSubmatch(input[start:]); m != nil {
		name, definition := m[1], m[3]
		if name == "" && definition == "" {
			return 0, nil
		}
		link := FootnoteLink{Pos{d.tokens[ni].Pos().Row, start}, name, nil}
		if definition != "" {
			nodes := d.parseInline(definition, ni)
			end := d.tokens[ni].EndPos()
			if len(nodes) > 0 {
				end = nodes[len(nodes)-1].GetEnd()
			}
			link.Definition = &FootnoteDefinition{Pos{d.tokens[ni].Pos().Row, start}, name, []Node{Paragraph{Pos{d.tokens[ni].Pos().Row, start}, end, nodes}}, true}
		}
		return len(m[0]), link
	}
	return 0, nil
}

func (d *Document) parseStatisticToken(input string, start int, ni int) (int, Node) {
	if m := statisticsTokenRegexp.FindStringSubmatch(input[start:]); m != nil {
		fullLen := len(m[1]) + 2
		startRow := d.tokens[ni].Pos().Row
		return fullLen, StatisticToken{Pos{startRow, start}, Pos{startRow, start + fullLen}, m[1]}
	}
	return 0, nil
}

func (d *Document) parseAutoLink(input string, start int, ni int) (int, int, Node) {
	if !d.AutoLink || start == 0 || len(input[start:]) < 3 || input[start:start+3] != "://" {
		return 0, 0, nil
	}
	protocolStart, protocol := start-1, ""
	for ; protocolStart > 0; protocolStart-- {
		if !unicode.IsLetter(rune(input[protocolStart])) {
			protocolStart++
			break
		}
	}
	if m := autolinkProtocols.FindStringSubmatch(input[protocolStart:start]); m != nil {
		protocol = m[1]
	} else {
		return 0, 0, nil
	}
	end := start
	for ; end < len(input) && strings.ContainsRune(validURLCharacters, rune(input[end])); end++ {
	}
	path := input[start:end]
	if path == "://" {
		return 0, 0, nil
	}
	return len(protocol), len(path + protocol), RegularLink{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, end}, protocol, nil, protocol + path, true}
}

func (d *Document) parseRegularLink(input string, start int, ni int) (int, Node) {
	input = input[start:]
	if len(input) < 3 || input[:2] != "[[" || input[2] == '[' {
		return 0, nil
	}
	end := strings.Index(input, "]]")
	if end == -1 {
		return 0, nil
	}
	rawLinkParts := strings.Split(input[2:end], "][")
	description, link := ([]Node)(nil), rawLinkParts[0]
	if len(rawLinkParts) == 2 {
		link, description = rawLinkParts[0], d.parseInline(rawLinkParts[1], ni)
	}
	if strings.ContainsRune(link, '\n') {
		return 0, nil
	}
	consumed := end + 2
	protocol, linkParts := "", strings.SplitN(link, ":", 2)
	if len(linkParts) == 2 {
		protocol = linkParts[0]
	}
	return consumed, RegularLink{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, end + 2}, protocol, description, link, false}
}

func (d *Document) parseTimestamp(input string, start int, ni int) (int, Node) {
	s, _, m := ParseTimestamp(input[start:])
	if s != nil {
		startRow := d.tokens[ni].Pos().Row
		fullLen := len(m["_fullmatch"])
		timestamp := Timestamp{Pos{startRow, start}, Pos{startRow, start + fullLen}, s /*, isDate, interval*/}
		if d.Outline.last != nil && d.Outline.last.Headline != nil {
			d.Outline.last.Headline.Timestamp = &timestamp
		}
		return fullLen, timestamp
	}
	return 0, nil
}

func (self *Timestamp) IsZero() bool {
	return self == nil || self.Time == nil || self.Time.IsZero()
}

func (d *Document) parseEmphasis(input string, start int, isRaw bool, ni int) (int, Node) {
	marker, i := input[start], start
	if !hasValidPreAndBorderChars(input, i) {
		return 0, nil
	}
	for i, consumedNewLines := i+1, 0; i < len(input) && consumedNewLines <= d.MaxEmphasisNewLines; i++ {
		if input[i] == '\n' {
			consumedNewLines++
		}

		if input[i] == marker && i != start+1 && hasValidPostAndBorderChars(input, i) {
			if isRaw {
				return i + 1 - start, Emphasis{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, i}, input[start : start+1], d.parseRawInline(input[start+1:i], ni)}
			}
			return i + 1 - start, Emphasis{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, i}, input[start : start+1], d.parseInline(input[start+1:i], ni)}
		}
	}
	return 0, nil
}

// see org-emphasis-regexp-components (emacs elisp variable)

func hasValidPreAndBorderChars(input string, i int) bool {
	return (i+1 >= len(input) || isValidBorderChar(rune(input[i+1]))) && (i == 0 || isValidPreChar(rune(input[i-1])))
}

func hasValidPostAndBorderChars(input string, i int) bool {
	return (i == 0 || isValidBorderChar(rune(input[i-1]))) && (i+1 >= len(input) || isValidPostChar(rune(input[i+1])))
}

func isValidPreChar(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(`-({'"`, r)
}

func isValidPostChar(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(`-.,:!?;'")}[`, r)
}

func isValidBorderChar(r rune) bool { return !unicode.IsSpace(r) }

func (l RegularLink) Kind() string {
	description := String(l.Description)
	descProtocol, descExt := strings.SplitN(description, ":", 2)[0], path.Ext(description)
	if ok := descProtocol == "file" || descProtocol == "http" || descProtocol == "https"; ok && imageExtensionRegexp.MatchString(descExt) {
		return "image"
	} else if ok && videoExtensionRegexp.MatchString(descExt) {
		return "video"
	}

	if p := l.Protocol; l.Description != nil || (p != "" && p != "file" && p != "http" && p != "https") {
		return "regular"
	}
	if imageExtensionRegexp.MatchString(path.Ext(l.URL)) {
		return "image"
	}
	if videoExtensionRegexp.MatchString(path.Ext(l.URL)) {
		return "video"
	}
	return "regular"
}

func (n Text) String() string              { return orgWriter.WriteNodesAsString(n) }
func (n LineBreak) String() string         { return orgWriter.WriteNodesAsString(n) }
func (n ExplicitLineBreak) String() string { return orgWriter.WriteNodesAsString(n) }
func (n StatisticToken) String() string    { return orgWriter.WriteNodesAsString(n) }
func (n Emphasis) String() string          { return orgWriter.WriteNodesAsString(n) }
func (n InlineBlock) String() string       { return orgWriter.WriteNodesAsString(n) }
func (n LatexFragment) String() string     { return orgWriter.WriteNodesAsString(n) }
func (n FootnoteLink) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n RegularLink) String() string       { return orgWriter.WriteNodesAsString(n) }
func (n Macro) String() string             { return orgWriter.WriteNodesAsString(n) }
func (n Timestamp) String() string         { return orgWriter.WriteNodesAsString(n) }
func (n Text) GetPos() Pos                 { return n.Pos }
func (n LineBreak) GetPos() Pos            { return n.Pos }
func (n ExplicitLineBreak) GetPos() Pos    { return n.Pos }
func (n StatisticToken) GetPos() Pos       { return n.Pos }
func (n Emphasis) GetPos() Pos             { return n.Pos }
func (n InlineBlock) GetPos() Pos          { return n.Pos }
func (n LatexFragment) GetPos() Pos        { return n.Pos }
func (n FootnoteLink) GetPos() Pos         { return n.Pos }
func (n RegularLink) GetPos() Pos          { return n.Pos }
func (n Macro) GetPos() Pos                { return n.Pos }
func (n Timestamp) GetPos() Pos            { return n.Pos }
func computeTextEnd(pos Pos, content string) Pos {
	temp := strings.Split(strings.TrimRight(content, "\n"), "\n")
	res := Pos{Row: pos.Row + (len(temp) - 1), Col: len(temp[len(temp)-1])}
	return res
}
func (n Text) GetEnd() Pos              { return n.EndPos }
func (n LineBreak) GetEnd() Pos         { return Pos{n.Pos.Row + n.Count - 1, 0} }
func (n ExplicitLineBreak) GetEnd() Pos { return n.Pos }
func (n StatisticToken) GetEnd() Pos    { return n.EndPos }
func (n Emphasis) GetEnd() Pos {
	return n.EndPos
}
func (n InlineBlock) GetEnd() Pos {
	return n.EndPos
}
func (n LatexFragment) GetEnd() Pos {
	return n.Content[len(n.Content)-1].GetEnd()
}
func (n FootnoteLink) GetEnd() Pos { return n.Pos }
func (n RegularLink) GetEnd() Pos {
	return n.EndPos
}
func (n Macro) GetEnd() Pos {
	return n.Pos
}
func (n Timestamp) GetEnd() Pos {
	return n.EndPos
}

/////

func (n Text) GetType() NodeType              { return TextNode }
func (n LineBreak) GetType() NodeType         { return LineBreakNode }
func (n ExplicitLineBreak) GetType() NodeType { return ExplicitLineBreakNode }
func (n StatisticToken) GetType() NodeType    { return StatisticTokenNode }
func (n Emphasis) GetType() NodeType          { return EmphasisNode }
func (n InlineBlock) GetType() NodeType       { return InlineBlockNode }
func (n LatexFragment) GetType() NodeType     { return LatexFragmentNode }
func (n FootnoteLink) GetType() NodeType      { return FootnoteLinkNode }
func (n RegularLink) GetType() NodeType       { return RegularLinkNode }
func (n Macro) GetType() NodeType             { return MacroNode }
func (n Timestamp) GetType() NodeType         { return TimestampNode }

func (n Text) GetTypeName() string              { return GetNodeTypeName(n.GetType()) }
func (n LineBreak) GetTypeName() string         { return GetNodeTypeName(n.GetType()) }
func (n ExplicitLineBreak) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n StatisticToken) GetTypeName() string    { return GetNodeTypeName(n.GetType()) }
func (n Emphasis) GetTypeName() string          { return GetNodeTypeName(n.GetType()) }
func (n InlineBlock) GetTypeName() string       { return GetNodeTypeName(n.GetType()) }
func (n LatexFragment) GetTypeName() string     { return GetNodeTypeName(n.GetType()) }
func (n FootnoteLink) GetTypeName() string      { return GetNodeTypeName(n.GetType()) }
func (n RegularLink) GetTypeName() string       { return GetNodeTypeName(n.GetType()) }
func (n Macro) GetTypeName() string             { return GetNodeTypeName(n.GetType()) }
func (n Timestamp) GetTypeName() string         { return GetNodeTypeName(n.GetType()) }

func (n Text) GetChildren() []Node              { return nil }
func (n LineBreak) GetChildren() []Node         { return nil }
func (n ExplicitLineBreak) GetChildren() []Node { return nil }
func (n StatisticToken) GetChildren() []Node    { return nil }
func (n Emphasis) GetChildren() []Node          { return n.Content }
func (n InlineBlock) GetChildren() []Node       { return n.Children }
func (n LatexFragment) GetChildren() []Node     { return n.Content }
func (n FootnoteLink) GetChildren() []Node      { return nil }
func (n RegularLink) GetChildren() []Node       { return n.Description }
func (n Macro) GetChildren() []Node             { return nil }
func (n Timestamp) GetChildren() []Node         { return nil }
