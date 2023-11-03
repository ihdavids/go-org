// Package org is an Org mode syntax processor.
//
// It parses plain text into an AST and can export it as HTML or pretty printed Org mode syntax.
// Further export formats can be defined using the Writer interface.
//
// You probably want to start with something like this:
//
//	input := strings.NewReader("Your Org mode input")
//	html, err := org.New().Parse(input, "./").Write(org.NewHTMLWriter())
//	if err != nil {
//	    log.Fatalf("Something went wrong: %s", err)
//	}
//	log.Print(html)
package org

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime/debug"
	"strings"
)

type Configuration struct {
	MaxEmphasisNewLines int                                   // Maximum number of newlines inside an emphasis. See org-emphasis-regexp-components newline.
	AutoLink            bool                                  // Try to convert text passages that look like hyperlinks into hyperlinks.
	DefaultSettings     map[string]string                     // Default values for settings that are overriden by setting the same key in BufferSettings.
	Log                 *log.Logger                           // Log is used to print warnings during parsing.
	ReadFile            func(filename string) ([]byte, error) // ReadFile is used to read e.g. #+INCLUDE files.
}

// Document contains the parsing results and a pointer to the Configuration.
type Document struct {
	*Configuration
	Path            string // Path of the file containing the parse input - used to resolve relative paths during parsing (e.g. INCLUDE).
	tokens          []token
	baseLvl         int
	Macros          map[string]string
	Links           map[string]string
	Nodes           []Node
	NamedNodes      map[string]Node
	Outline         Outline           // Outline is a Table Of Contents for the document and contains all sections (headline + content).
	BufferSettings  map[string]string // Settings contains all settings that were parsed from keywords.
	Error           error
	currentHeadline *Headline
}

type NodeType int

const (
	// since iota starts with 0, the first value
	// defined here will be the default
	UndefinedNode NodeType = iota
	HealineNode
	DateTimeNode
	TableNode
	TableRowNode
	TableColNode
	ParagraphNode
	HorizontalRuleNode
	DrawerNode
	PropertyDrawerNode
	TextNode
	LineBreakNode
	ExplicitLineBreakNode
	StatisticTokenNode
	EmphasisNode
	InlineBlockNode
	LatexFragmentNode
	FootnoteLinkNode
	RegularLinkNode
	MacroNode
	TimestampNode
	FootnoteDefinitionNode
)

func GetNodeTypeName(t NodeType) string {
	switch t {
	case UndefinedNode:
		return "undefined"
	case HealineNode:
		return "headline"
	case DateTimeNode:
		return "datetime"
	case TableNode:
		return "table"
	case TableRowNode:
		return "tablerow"
	case TableColNode:
		return "tablecol"
	case ParagraphNode:
		return "paragraph"
	case HorizontalRuleNode:
		return "horizontalrule"
	case DrawerNode:
		return "drawer"
	case PropertyDrawerNode:
		return "propeprtydrawer"
	case TextNode:
		return "text"
	case LineBreakNode:
		return "linebreak"
	case ExplicitLineBreakNode:
		return "explicitlinebreak"
	case StatisticTokenNode:
		return "statistictoken"
	case EmphasisNode:
		return "emphasis"
	case InlineBlockNode:
		return "inlineblock"
	case LatexFragmentNode:
		return "latex"
	case FootnoteLinkNode:
		return "footnotelink"
	case RegularLinkNode:
		return "link"
	case MacroNode:
		return "macro"
	case TimestampNode:
		return "timestamp"
	case FootnoteDefinitionNode:
		return "footnotedef"
	}
	return "undefined"
}

// Node represents a parsed node of the document.
type Node interface {
	String() string      // String returns the pretty printed Org mode string for the node (see OrgWriter).
	GetPos() Pos         // Position in the file of the token
	GetEnd() Pos         // Position of next token in stream
	GetType() NodeType   // What type of node are we?
	GetTypeName() string // Descriptive name of node type
}

type lexFn = func(line string, row, col int) (t token, ok bool)
type parseFn = func(*Document, int, stopFn) (int, Node)
type stopFn = func(*Document, int) bool

type Pos struct {
	Row int
	Col int
}
type token struct {
	kind    string
	lvl     int
	content string
	matches []string
	pos     Pos
	endPos  Pos
}

func (self token) Pos() Pos {
	return self.pos
}

func (self token) EndPos() Pos {
	return self.endPos
}

var lexFns = []lexFn{
	lexHeadline,
	lexDrawer,
	lexBlock,
	lexResult,
	lexList,
	lexTable,
	lexHorizontalRule,
	lexKeywordOrComment,
	lexFootnoteDefinition,
	lexExample,
	lexText,
	lexScheduled,
	lexDeadline,
	lexClosed,
}

var nilToken = token{"nil", -1, "", nil, Pos{0, 0}, Pos{0, 0}}
var orgWriter = NewOrgWriter()

// New returns a new Configuration with (hopefully) sane defaults.
func New() *Configuration {
	return &Configuration{
		AutoLink:            true,
		MaxEmphasisNewLines: 1,
		DefaultSettings: map[string]string{
			"TODO":         "TODO | DONE",
			"EXCLUDE_TAGS": "noexport",
			"OPTIONS":      "toc:t <:t e:t f:t pri:t todo:t tags:t title:t ealb:nil",
		},
		Log:      log.New(os.Stderr, "go-org: ", 0),
		ReadFile: ioutil.ReadFile,
	}
}

// String returns the pretty printed Org mode string for the given nodes (see OrgWriter).
func String(nodes []Node) string { return orgWriter.WriteNodesAsString(nodes...) }

// Write is called after with an instance of the Writer interface to export a parsed Document into another format.
func (d *Document) Write(w Writer) (out string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("could not write output: %s", recovered)
		}
	}()
	if d.Error != nil {
		return "", d.Error
	} else if d.Nodes == nil {
		return "", fmt.Errorf("could not write output: parse was not called")
	}
	w.Before(d)
	WriteNodes(w, d.Nodes...)
	w.After(d)
	return w.String(), err
}

// Parse parses the input into an AST (and some other helpful fields like Outline).
// To allow method chaining, errors are stored in document.Error rather than being returned.
func (c *Configuration) Parse(input io.Reader, path string) (d *Document) {
	outlineSection := &Section{}
	d = &Document{
		Configuration:  c,
		Outline:        Outline{outlineSection, outlineSection, 0, []hash.Hash{}},
		BufferSettings: map[string]string{},
		NamedNodes:     map[string]Node{},
		Links:          map[string]string{},
		Macros:         map[string]string{},
		Path:           path,
	}
	d.Outline.lastHash.Push(sha1.New())
	d.Outline.lastHash.Peek().Write([]byte(path))
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
			d.Error = fmt.Errorf("could not parse input: %v", recovered)
		}
	}()
	if d.tokens != nil {
		d.Error = fmt.Errorf("parse was called multiple times")
	}
	d.tokenize(input)
	_, nodes := d.parseMany(0, func(d *Document, i int) bool { return i >= len(d.tokens) })
	d.Nodes = nodes
	return d
}

// Silent disables all logging of warnings during parsing.
func (c *Configuration) Silent() *Configuration {
	c.Log = log.New(ioutil.Discard, "", 0)
	return c
}

func (d *Document) tokenize(input io.Reader) {
	d.tokens = []token{}
	scanner := bufio.NewScanner(input)
	lineNum := 0
	for scanner.Scan() {
		d.tokens = append(d.tokens, tokenize(scanner.Text(), lineNum))
		lineNum += 1
	}
	if err := scanner.Err(); err != nil {
		d.Error = fmt.Errorf("could not tokenize input: %s", err)
	}
}

// Get returns the value for key in BufferSettings or DefaultSettings if key does not exist in the former
func (d *Document) Get(key string) string {
	if v, ok := d.BufferSettings[key]; ok {
		return v
	}
	if v, ok := d.DefaultSettings[key]; ok {
		return v
	}
	return ""
}

// GetOption returns the value associated to the export option key
// Currently supported options:
// - < (export timestamps)
// - e (export org entities)
// - f (export footnotes)
// - title (export title)
// - toc (export table of content. an int limits the included org headline lvl)
// - todo (export headline todo status)
// - pri (export headline priority)
// - tags (export headline tags)
// - ealb (non-standard) (export with east asian line breaks / ignore line breaks between multi-byte characters)
// see https://orgmode.org/manual/Export-settings.html for more information
func (d *Document) GetOption(key string) string {
	get := func(settings map[string]string) string {
		for _, field := range strings.Fields(settings["OPTIONS"]) {
			if strings.HasPrefix(field, key+":") {
				return field[len(key)+1:]
			}
		}
		return ""
	}
	value := get(d.BufferSettings)
	if value == "" {
		value = get(d.DefaultSettings)
	}
	if value == "" {
		value = "nil"
		d.Log.Printf("Missing value for export option %s", key)
	}
	return value
}

func (d *Document) parseOne(i int, stop stopFn) (consumed int, node Node) {
	switch d.tokens[i].kind {
	case "unorderedList", "orderedList":
		consumed, node = d.parseList(i, stop)
	case "tableRow", "tableSeparator":
		consumed, node = d.parseTable(i, stop)
	case "beginBlock":
		consumed, node = d.parseBlock(i, stop)
	case "result":
		consumed, node = d.parseResult(i, stop)
	case "beginDrawer":
		consumed, node = d.parseDrawer(i, stop)
	case "text":
		consumed, node = d.parseParagraph(i, stop)
	case "example":
		consumed, node = d.parseExample(i, stop)
	case "horizontalRule":
		consumed, node = d.parseHorizontalRule(i, stop)
	case "comment":
		consumed, node = d.parseComment(i, stop)
	case "keyword":
		consumed, node = d.parseKeyword(i, stop)
	case "headline":
		consumed, node = d.parseHeadline(i, stop)
	case "deadline":
		consumed, node = d.parseDeadline(i, stop)
	case "scheduled":
		consumed, node = d.parseScheduled(i, stop)
	case "closed":
		consumed, node = d.parseClosed(i, stop)
	case "footnoteDefinition":
		consumed, node = d.parseFootnoteDefinition(i, stop)
	}

	if consumed != 0 {
		return consumed, node
	}
	d.Log.Printf("Could not parse token %#v: Falling back to treating it as plain text.", d.tokens[i])
	m := plainTextRegexp.FindStringSubmatch(d.tokens[i].matches[0])
	d.tokens[i] = token{"text", len(m[1]), m[2], m, d.tokens[i].pos, computeTextEnd(d.tokens[i].pos, m[2])}
	return d.parseOne(i, stop)
}

func (d *Document) parseMany(i int, stop stopFn) (int, []Node) {
	start, nodes := i, []Node{}
	for i < len(d.tokens) && !stop(d, i) {
		consumed, node := d.parseOne(i, stop)
		i += consumed
		nodes = append(nodes, node)
	}
	return i - start, nodes
}

func (d *Document) addHeadline(headline *Headline) int {
	current := &Section{Headline: headline}
	d.Outline.last.add(current)
	d.Outline.count++
	d.Outline.last = current
	return d.Outline.count
}

func tokenize(line string, row int) token {
	col := 0
	for _, lexFn := range lexFns {
		if token, ok := lexFn(line, row, col); ok {
			return token
		}
	}
	panic(fmt.Sprintf("could not lex line: %s", line))
}
