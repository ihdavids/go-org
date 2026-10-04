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
	// Unbraced marks a subscript or superscript written without braces, H_2O
	// rather than H_{2}O. Its Kind is still "_{}" or "^{}".
	Unbraced bool
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
	AngleLink   bool // written <protocol:path>
}

// Target is a dedicated target, <<name>>, which [[name]] links to.
type Target struct {
	Pos    Pos
	EndPos Pos
	Name   string
}

// RadioTarget is a radio target, <<<name>>>.
type RadioTarget struct {
	Pos      Pos
	EndPos   Pos
	Name     string
	Children []Node
}

type Macro struct {
	Pos        Pos
	Name       string
	Parameters []string // nil for a macro written without parentheses, {{{name}}}
}

var validURLCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~:/?#[]@!$&'()*+,;="

// The link types recognised in plain text, `see https://... or mailto:...`.
var autolinkProtocols = regexp.MustCompile(`^(https?|ftps?|file|mailto|news|doi|irc|ssh|sftp|tel)$`)

// The link types recognised in an angle link, <protocol:path>.
var angleLinkProtocols = regexp.MustCompile(`^(https?|ftps?|file|mailto|news|doi|irc|ssh|sftp|tel|id|attachment|info|help|man|shell|elisp|bibtex|docview|gnus|rmail|mhe|bbdb)$`)
var angleLinkRegexp = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9+.-]*):([^\s<>\[\]](?:[^<>\[\]\n]*[^\s<>\[\]])?)>`)
var targetRegexp = regexp.MustCompile(`^<<([^<>\s](?:[^<>\n]*[^<>\s])?)>>`)
var radioTargetRegexp = regexp.MustCompile(`^<<<([^<>\s](?:[^<>\n]*[^<>\s])?)>>>`)
var imageExtensionRegexp = regexp.MustCompile(`^[.](png|gif|jpe?g|svg|tiff?)$`)
var videoExtensionRegexp = regexp.MustCompile(`^[.](webm|mp4)$`)

var subScriptSuperScriptRegexp = regexp.MustCompile(`^([_^]){([^{}]+?)}`)
var timestampRegexp = regexp.MustCompile(`^<(\d{4}-\d{2}-\d{2})( [A-Za-z]+)?( \d{2}:\d{2})?( \+\d+[dwmy])?>`)
var footnoteRegexp = regexp.MustCompile(`^\[fn:([\w-]*)[:\]]`)

// [1/3], [33%], and the empty cookies [/] and [%] that org fills in.
var statisticsTokenRegexp = regexp.MustCompile(`^\[(\d*/\d*|\d*%)\]`)
var latexEnvironmentBeginRegexp = regexp.MustCompile(`^\\begin\{([A-Za-z0-9*]+)\}`)
var inlineExportBlockRegexp = regexp.MustCompile(`@@(\w+):(.*?)@@`)

// {{{name}}} or {{{name(arguments)}}} - the arguments end at the first )}}},
// so that two macros on one line are two macros.
var macroRegexp = regexp.MustCompile(`^\{\{\{([a-zA-Z][-\w]*)(?:\(((?s:.*?))\))?\}\}\}`)

// An unbraced subscript or superscript: a_b, x^2, H_2O, e^-1, a_*.
var unbracedSubScriptSuperScriptRegexp = regexp.MustCompile(`^([_^])(\*|[+-]?[\pL\pN.,\\]*[\pL\pN])`)

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
			consumed, node = d.parseOpeningAngle(input, current, i)
		case 'c':
			consumed, node = d.parseInlineBabelCall(input, current, i)
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
	// src_LANG[HEADERS]{BODY}: the headers and the body each end at their
	// matching bracket on the same line, not at the last one on the line.
	rest, i := input[start+1:], 0
	for i < len(rest) && !unicode.IsSpace(rune(rest[i])) && !strings.ContainsRune("[]{}", rune(rest[i])) {
		i++
	}
	lang := rest[:i]
	if lang == "" {
		return 0, 0, nil
	}
	headers := ""
	if i < len(rest) && rest[i] == '[' {
		end := matchingBracket(rest, i, '[', ']')
		if end < 0 {
			return 0, 0, nil
		}
		headers, i = rest[i+1:end], end+1
	}
	if i >= len(rest) || rest[i] != '{' {
		return 0, 0, nil
	}
	end := matchingBracket(rest, i, '{', '}')
	if end < 0 {
		return 0, 0, nil
	}
	body := rest[i+1 : end]
	length := 3 + 1 + end + 1 // src, _, through the closing brace
	temp := d.lastKeywords
	d.lastKeywords = nil
	row := d.tokens[ni].Pos().Row
	return 3, length, InlineBlock{Pos{row, start - 3}, Pos{row, start - 3 + length}, "src", strings.Fields(lang + " " + headers), d.parseRawInline(body, ni), temp}
}

// matchingBracket returns the index of the bracket closing the one at
// s[open], counting nested pairs and stopping at the end of the line; -1 if
// there is none.
func matchingBracket(s string, open int, opening, closing byte) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '\n':
			return -1
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseInlineBabelCall reads call_NAME[INSIDE](ARGUMENTS)[END] - an inline
// babel call - into an InlineBlock named "call", with the parameters name,
// inside header, arguments and end header.
func (d *Document) parseInlineBabelCall(input string, start int, ni int) (int, Node) {
	if !strings.HasPrefix(input[start:], "call_") {
		return 0, nil
	}
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(input[:start]); unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return 0, nil
		}
	}
	rest, i := input[start+len("call_"):], 0
	for i < len(rest) && !unicode.IsSpace(rune(rest[i])) && !strings.ContainsRune("[]()", rune(rest[i])) {
		i++
	}
	name := rest[:i]
	if name == "" {
		return 0, nil
	}
	inside, end := "", ""
	if i < len(rest) && rest[i] == '[' {
		e := matchingBracket(rest, i, '[', ']')
		if e < 0 {
			return 0, nil
		}
		inside, i = rest[i+1:e], e+1
	}
	if i >= len(rest) || rest[i] != '(' {
		return 0, nil
	}
	e := matchingBracket(rest, i, '(', ')')
	if e < 0 {
		return 0, nil
	}
	arguments := rest[i+1 : e]
	i = e + 1
	if i < len(rest) && rest[i] == '[' {
		e := matchingBracket(rest, i, '[', ']')
		if e < 0 {
			return 0, nil
		}
		end, i = rest[i+1:e], e+1
	}
	length := len("call_") + i
	row := d.tokens[ni].Pos().Row
	return length, InlineBlock{Pos: Pos{row, start}, EndPos: Pos{row, start + length}, Name: "call", Parameters: []string{name, inside, arguments, end}}
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
		// \begin{name} ... \end{name}, closed by the first \end of the same
		// name. Names such as align* are names too.
		if m := latexEnvironmentBeginRegexp.FindStringSubmatch(input[start:]); m != nil {
			openingPair, closingPair := m[0], `\end{`+m[1]+`}`
			if i := strings.Index(input[start+len(openingPair):], closingPair); i >= 0 {
				content := input[start+len(openingPair) : start+len(openingPair)+i]
				return len(openingPair) + i + len(closingPair), LatexFragment{Pos{d.tokens[ni].Pos().Row, start}, openingPair, closingPair, d.parseRawInline(content, ni)}
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

// subSuperscriptOption is the ^ export option: t reads a_b and a^b as
// subscript and superscript, {} only the braced a_{b} and a^{b}, and nil
// neither. The default is {} - unlike Emacs, where it is t - so that the
// snake_case names and file_names.txt common in prose are left alone unless a
// document asks for #+OPTIONS: ^:t.
func (d *Document) subSuperscriptOption() string {
	for _, settings := range []map[string]string{d.BufferSettings, d.DefaultSettings} {
		value := ""
		for _, field := range strings.Fields(settings["OPTIONS"]) {
			if strings.HasPrefix(field, "^:") {
				value = field[2:]
			}
		}
		if value != "" {
			return value
		}
	}
	return "{}"
}

func (d *Document) parseSubOrSuperScript(input string, start int, ni int) (int, Node) {
	consumed, node := d.parseBracedSubOrSuperScript(input, start, ni)
	if consumed == 0 {
		consumed, node = d.parseUnbracedSubOrSuperScript(input, start, ni)
	}
	return consumed, node
}

func (d *Document) parseBracedSubOrSuperScript(input string, start int, ni int) (int, Node) {
	if d.subSuperscriptOption() == "nil" {
		return 0, nil
	}
	if m := subScriptSuperScriptRegexp.FindStringSubmatch(input[start:]); m != nil {
		fullLen := len(m[2]) + 3
		startRow := d.tokens[ni].Pos().Row
		return fullLen, Emphasis{Pos{startRow, start}, Pos{startRow, start + fullLen}, m[1] + "{}", []Node{Text{Pos{startRow, start}, computeTextEnd(Pos{startRow, start}, m[2]), m[2], false}}, false}
	}
	return 0, nil
}

// An unbraced subscript or superscript follows a character that is not a space:
// H_2O, x^2. It is the same Emphasis as the braced form, marked Unbraced so that
// it is written back the way it was written.
func (d *Document) parseUnbracedSubOrSuperScript(input string, start int, ni int) (int, Node) {
	if start == 0 || d.subSuperscriptOption() != "t" {
		return 0, nil
	}
	if r, _ := utf8.DecodeLastRuneInString(input[:start]); unicode.IsSpace(r) {
		return 0, nil
	}
	if m := unbracedSubScriptSuperScriptRegexp.FindStringSubmatch(input[start:]); m != nil {
		startRow := d.tokens[ni].Pos().Row
		fullLen := len(m[0])
		return fullLen, Emphasis{Pos{startRow, start}, Pos{startRow, start + fullLen}, m[1] + "{}", []Node{Text{Pos{startRow, start + 1}, computeTextEnd(Pos{startRow, start + 1}, m[2]), m[2], false}}, true}
	}
	return 0, nil
}

func (d *Document) parseSubScriptOrEmphasisOrInlineBlock(input string, start int, ni int) (int, int, Node) {
	if rewind, consumed, node := d.parseInlineBlock(input, start, ni); consumed != 0 {
		return rewind, consumed, node
	} else if consumed, node := d.parseBracedSubOrSuperScript(input, start, ni); consumed != 0 {
		return 0, consumed, node
	} else if consumed, node := d.parseEmphasis(input, start, false, ni); consumed != 0 {
		return 0, consumed, node
	}
	consumed, node := d.parseUnbracedSubOrSuperScript(input, start, ni)
	return 0, consumed, node
}

func (d *Document) parseOpeningBracket(input string, start int, ni int) (int, Node) {
	if len(input[start:]) >= 2 && input[start] == '[' && input[start+1] == '[' {
		return d.parseRegularLink(input, start, ni)
	} else if footnoteRegexp.MatchString(input[start:]) {
		return d.parseFootnoteReference(input, start, ni)
	} else if len(input[start:]) > 1 && input[start+1] >= '0' && input[start+1] <= '9' {
		// An inactive timestamp, [2026-10-01 Thu], is as much a timestamp as
		// an active one - it is just not one the agenda shows.
		if consumed, node := d.parseTimestamp(input, start, ni); consumed != 0 {
			return consumed, node
		}
	}
	if statisticsTokenRegexp.MatchString(input[start:]) {
		return d.parseStatisticToken(input, start, ni)
	}
	return 0, nil
}

func (d *Document) parseMacro(input string, start int, ni int) (int, Node) {
	if m := macroRegexp.FindStringSubmatchIndex(input[start:]); m != nil {
		name := input[start+m[2] : start+m[3]]
		var parameters []string
		if m[4] >= 0 {
			parameters = splitMacroArguments(input[start+m[4] : start+m[5]])
		}
		return m[1], Macro{Pos{d.tokens[ni].Pos().Row, start}, name, parameters}
	}
	return 0, nil
}

// splitMacroArguments splits the arguments of a macro on commas. A comma
// escaped with a backslash, \,, is part of an argument.
func splitMacroArguments(s string) []string {
	arguments, current := []string{}, strings.Builder{}
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == ',':
			current.WriteByte(',')
			i++
		case s[i] == ',':
			arguments = append(arguments, current.String())
			current.Reset()
		default:
			current.WriteByte(s[i])
		}
	}
	return append(arguments, current.String())
}

// parseFootnoteReference reads [fn:name], [fn:name:definition] and
// [fn::definition]. An inline definition ends at its matching bracket, so it
// may hold links and other bracketed markup.
func (d *Document) parseFootnoteReference(input string, start int, ni int) (int, Node) {
	m := footnoteRegexp.FindStringSubmatch(input[start:])
	if m == nil {
		return 0, nil
	}
	name, length := m[1], len(m[0])
	definition := ""
	if strings.HasSuffix(m[0], ":") {
		end := matchingBracket(input[start:], 0, '[', ']')
		if end < 0 {
			return 0, nil
		}
		definition, length = input[start+len(m[0]):start+end], end+1
		if name == "" && definition == "" {
			return 0, nil
		}
	} else if name == "" {
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
	return length, link
}

func (d *Document) parseStatisticToken(input string, start int, ni int) (int, Node) {
	if m := statisticsTokenRegexp.FindStringSubmatch(input[start:]); m != nil {
		fullLen := len(m[0])
		startRow := d.tokens[ni].Pos().Row
		return fullLen, StatisticToken{Pos{startRow, start}, Pos{startRow, start + fullLen}, m[1]}
	}
	return 0, nil
}

func (d *Document) parseAutoLink(input string, start int, ni int) (int, int, Node) {
	if !d.AutoLink || start == 0 || start+1 >= len(input) {
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
	end := start + 1
	for ; end < len(input) && strings.ContainsRune(plainLinkCharacters, rune(input[end])); end++ {
	}
	// Punctuation at the end belongs to the sentence, not the link: "see
	// https://example.org." - and so does a closing parenthesis that has
	// no opening one in the link, "(see https://example.org)".
	for end > start+1 {
		c := input[end-1]
		if strings.ContainsRune(".,;:!?'\"*=~", rune(c)) ||
			(c == ')' && strings.Count(input[start:end], "(") < strings.Count(input[start:end], ")")) {
			end--
			continue
		}
		break
	}
	path := input[start:end]
	if path == ":" || path == "://" {
		return 0, 0, nil
	}
	return len(protocol), len(path + protocol), RegularLink{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, end}, protocol, nil, protocol + path, true, false}
}

// The characters of a plain link: those of a URL, less the brackets that would
// end it inside a bracket link.
var plainLinkCharacters = strings.NewReplacer("[", "", "]", "").Replace(validURLCharacters)

// parseRegularLink reads [[link]] and [[link][description]]. Brackets in the
// link are escaped with a backslash, [[file:a\]b.org]], or balanced.
func (d *Document) parseRegularLink(input string, start int, ni int) (int, Node) {
	input = input[start:]
	if len(input) < 3 || input[:2] != "[[" || input[2] == '[' {
		return 0, nil
	}
	linkEnd, depth := -1, 0
	for i := 2; i < len(input) && linkEnd < 0; i++ {
		switch input[i] {
		case '\\':
			if i+1 < len(input) && (input[i+1] == '[' || input[i+1] == ']') {
				i++
			}
		case '\n':
			return 0, nil
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			} else {
				linkEnd = i
			}
		}
	}
	if linkEnd < 0 || linkEnd+1 >= len(input) {
		return 0, nil
	}
	link, description, end := unescapeLinkPath(input[2:linkEnd]), ([]Node)(nil), -1
	switch input[linkEnd+1] {
	case ']':
		end = linkEnd + 2
	case '[':
		i := strings.Index(input[linkEnd+2:], "]]")
		if i < 0 {
			return 0, nil
		}
		// A description that is itself a link, an image [[img.png]], ends
		// a pair of brackets later.
		if strings.HasPrefix(input[linkEnd+2:], "[[") {
			if j := strings.Index(input[linkEnd+2+i+2:], "]]"); j >= 0 && !strings.Contains(input[linkEnd+2:linkEnd+2+i+2+j], "\n") {
				i += j + 2
			}
		}
		description = d.parseInline(input[linkEnd+2:linkEnd+2+i], ni)
		end = linkEnd + 2 + i + 2
	default:
		return 0, nil
	}
	if strings.ContainsRune(link, '\n') {
		return 0, nil
	}
	protocol, linkParts := "", strings.SplitN(link, ":", 2)
	if len(linkParts) == 2 {
		protocol = linkParts[0]
	}
	return end, RegularLink{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, start + end}, protocol, description, link, false, false}
}

func unescapeLinkPath(s string) string {
	return strings.NewReplacer(`\[`, "[", `\]`, "]").Replace(s)
}

// escapeLinkPath escapes the brackets of a link for writing it back into
// [[...]], unless they pair up, in which case they are read back as they are.
func escapeLinkPath(s string) string {
	depth := 0
	for _, r := range s {
		if r == '[' {
			depth++
		} else if r == ']' {
			if depth--; depth < 0 {
				break
			}
		}
	}
	if depth == 0 {
		return s
	}
	return strings.NewReplacer("[", `\[`, "]", `\]`).Replace(s)
}

// parseOpeningAngle reads what may start with <: a radio target <<<name>>>, a
// target <<name>>, an angle link <https://...> or a timestamp.
func (d *Document) parseOpeningAngle(input string, start int, ni int) (int, Node) {
	row := d.tokens[ni].Pos().Row
	rest := input[start:]
	if m := radioTargetRegexp.FindStringSubmatch(rest); m != nil {
		d.addTarget(m[1])
		return len(m[0]), RadioTarget{Pos{row, start}, Pos{row, start + len(m[0])}, m[1], d.parseInline(m[1], ni)}
	}
	if m := targetRegexp.FindStringSubmatch(rest); m != nil && !strings.HasPrefix(rest, "<<<") {
		d.addTarget(m[1])
		return len(m[0]), Target{Pos{row, start}, Pos{row, start + len(m[0])}, m[1]}
	}
	if m := angleLinkRegexp.FindStringSubmatch(rest); m != nil && angleLinkProtocols.MatchString(m[1]) {
		return len(m[0]), RegularLink{Pos{row, start}, Pos{row, start + len(m[0])}, m[1], nil, m[1] + ":" + m[2], false, true}
	}
	return d.parseTimestamp(input, start, ni)
}

func (d *Document) addTarget(name string) {
	if d.Targets == nil {
		d.Targets = map[string]bool{}
	}
	d.Targets[name] = true
}

func (d *Document) parseTimestamp(input string, start int, ni int) (int, Node) {
	s, _, m := ParseTimestampPrefix(input[start:])
	if s != nil {
		startRow := d.tokens[ni].Pos().Row
		fullLen := len(m["_fullmatch"])
		// A range across days, <2026-10-14 Wed>--<2026-10-16 Fri>, is one
		// timestamp with an end, not two timestamps with "--" between them.
		// Read as two, the heading kept the second as its timestamp, so a
		// range reported its last day as its start and had no end at all.
		// Both halves must be the same kind (active or inactive), as org
		// requires.
		if rest := input[start+fullLen:]; strings.HasPrefix(rest, "--") && len(rest) > 2 && (rest[2] == '<' || rest[2] == '[') {
			if e, _, em := ParseTimestampPrefix(rest[2:]); e != nil && e.TimestampType == s.TimestampType && !e.Start.Before(s.Start) {
				// Either stamp may itself be a span of the day (10:00-11:00).
				// The run is first start to last end; the inner two times are
				// kept so the range writes back exactly as it was written.
				if s.HasEnd() {
					s.FirstEnd = s.End
				}
				if e.HasEnd() {
					s.LastStart = e.Start
					s.End = e.End
				} else {
					s.End = e.Start
				}
				s.HaveTime = s.HaveTime || e.HaveTime
				fullLen += 2 + len(em["_fullmatch"])
			}
		}
		timestamp := Timestamp{Pos{startRow, start}, Pos{startRow, start + fullLen}, s /*, isDate, interval*/}
		// Only an active timestamp is the heading's: an inactive one is a
		// record of when something happened, not an appointment.
		if s.TimestampType == Active && d.Outline.last != nil && d.Outline.last.Headline != nil {
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
				return i + 1 - start, Emphasis{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, i}, input[start : start+1], d.parseRawInline(input[start+1:i], ni), false}
			}
			return i + 1 - start, Emphasis{Pos{d.tokens[ni].Pos().Row, start}, Pos{d.tokens[ni].Pos().Row, i}, input[start : start+1], d.parseInline(input[start+1:i], ni), false}
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
	description := String(l.Description...)
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
func (n Target) String() string            { return orgWriter.WriteNodesAsString(n) }
func (n RadioTarget) String() string       { return orgWriter.WriteNodesAsString(n) }
func (n Target) GetPos() Pos               { return n.Pos }
func (n RadioTarget) GetPos() Pos          { return n.Pos }
func (n Target) GetEnd() Pos               { return n.EndPos }
func (n RadioTarget) GetEnd() Pos          { return n.EndPos }
func (n Target) GetType() NodeType         { return TargetNode }
func (n RadioTarget) GetType() NodeType    { return RadioTargetNode }
func (n Target) GetTypeName() string       { return GetNodeTypeName(n.GetType()) }
func (n RadioTarget) GetTypeName() string  { return GetNodeTypeName(n.GetType()) }
func (n Target) GetChildren() []Node       { return nil }
func (n RadioTarget) GetChildren() []Node  { return n.Children }
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
