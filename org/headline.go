package org

import (
	b64 "encoding/base64"
	"fmt"
	"hash"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type ShaStack []hash.Hash

func (self *ShaStack) Push(val hash.Hash) {
	*self = append(*self, val)
}

func (self *ShaStack) Pop() hash.Hash {
	n := len(*self) - 1
	r := (*self)[n]
	(*self)[n] = nil
	*self = (*self)[:n]
	return r
}

func (self *ShaStack) Peek() hash.Hash {
	return (*self)[len(*self)-1]
}

type Outline struct {
	*Section
	last     *Section
	count    int
	lastHash ShaStack
}

type Section struct {
	Hash     string
	Headline *Headline
	Parent   *Section
	Children []*Section
}
type CheckStatus struct {
	Num  int
	Den  int
	Type string
}

func (s *CheckStatus) String() string {
	if s.Type == "%" {
		return fmt.Sprintf(" [%d%s]", s.Num, "%")
	} else if s.Type == "/" {
		return fmt.Sprintf(" [%d/%d]", s.Num, s.Den)
	}
	return ""
}

type Headline struct {
	Pos      Pos
	EndPos   Pos
	Index    int
	Lvl      int
	Status   string
	Priority string
	// IsComment is set for a `* COMMENT Heading` - a commented subtree, which
	// is not exported. The COMMENT keyword is not part of the Title.
	IsComment  bool
	Properties *PropertyDrawer
	// Scheduling timestamps
	Scheduled   *SDC
	Closed      *SDC
	Deadline    *SDC
	Timestamp   *Timestamp
	Title       []Node
	Tags        []string
	Children    []Node
	CheckStatus *CheckStatus
	Drawers     []*Drawer
	Tables      []*Table
	Clocks      []*Clock
	Blocks      []*Block
	Doc         *Document
	// Schedules  []Schedule
}

var headlineRegexp = regexp.MustCompile(`^([*]+)\s+(.*)`)

// Tags are made of letters and digits of any script, `_`, `@`, `#` and `%`.
var tagRegexp = regexp.MustCompile(`(.*?)\s+(:[\p{L}\p{N}_@#%:]+:\s*$)`)

// A priority cookie is `[#A]`: an uppercase letter or a number, whatever range
// #+PRIORITIES: sets.
var priorityRegexp = regexp.MustCompile(`^\[#([A-Z]|[0-9]+)\](\s+|$)`)

// A statistics cookie anywhere in the title: [1/3], [33%], or the empty [/] and [%].
var pdoneRegexp = regexp.MustCompile(`\[(?:(\d*)%|(\d*)/(\d*))\]`)

func lexHeadline(line string, row, col int) (token, bool) {
	if m := headlineRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		return token{"headline", 0, m[2], m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func (d *Document) parseHeadline(i int, parentStop stopFn) (int, Node) {
	d.lastKeywords = nil
	pos := d.tokens[i].Pos()
	t, headline := d.tokens[i], &Headline{Pos: pos, EndPos: d.tokens[i].EndPos()}
	headline.Lvl = len(t.matches[1])
	headline.Doc = d
	headline.Index = d.addHeadline(headline)
	d.currentHeadline.Push(headline)

	text := t.content
	todo, done := d.TodoKeywords()
	for _, k := range append(todo, done...) {
		// A keyword is a whole word: `* TODO` alone is a task with no title.
		if strings.HasPrefix(text, k) && (len(text) == len(k) || unicode.IsSpace(rune(text[len(k)]))) {
			headline.Status = k
			text = strings.TrimLeftFunc(text[len(k):], unicode.IsSpace)
			break
		}
	}

	if m := priorityRegexp.FindStringSubmatch(text); m != nil {
		headline.Priority = m[1]
		text = text[len(m[0]):]
	}

	if text == "COMMENT" || strings.HasPrefix(text, "COMMENT ") {
		headline.IsComment = true
		text = strings.TrimLeftFunc(text[len("COMMENT"):], unicode.IsSpace)
	}

	if m := tagRegexp.FindStringSubmatch(text); m != nil {
		text = m[1]
		headline.Tags = strings.FieldsFunc(m[2], func(r rune) bool { return r == ':' || unicode.IsSpace(r) })
	} else if m := tagRegexp.FindStringSubmatch(" " + text); m != nil && m[1] == "" {
		// A heading that is nothing but tags, `* :tag:`.
		text = ""
		headline.Tags = strings.FieldsFunc(m[2], func(r rune) bool { return r == ':' || unicode.IsSpace(r) })
	}

	// The cookie stays in the title, where it was written, as a statistic
	// token; CheckStatus is what it says. The last cookie in the title counts.
	if ms := pdoneRegexp.FindAllStringSubmatch(text, -1); ms != nil {
		m := ms[len(ms)-1]
		if strings.HasSuffix(m[0], "%]") {
			n, _ := strconv.Atoi(m[1])
			headline.CheckStatus = &CheckStatus{Num: n, Type: "%"}
		} else {
			a, _ := strconv.Atoi(m[2])
			b, _ := strconv.Atoi(m[3])
			headline.CheckStatus = &CheckStatus{Num: a, Den: b, Type: "/"}
		}
	}

	headline.Title = d.parseInline(text, i)

	// Compute a unique ID for this node based on the document name and headlines in sequence
	// This ID shouldn't change as long as the structure of the file doesn't change.
	tHash := d.Outline.lastHash.Peek()
	var title string
	for _, n := range headline.Title {
		title += n.String()
	}
	tHash.Write([]byte(title))
	d.Outline.last.Hash = b64.StdEncoding.EncodeToString(tHash.Sum(nil))
	d.Outline.lastHash.Push(tHash)

	// A heading's body runs to the next heading of the same level or above, and
	// nothing else ends it - which is how org reads a file whatever the
	// indentation.
	//
	// This briefly stopped on planning lines, drawers and blocks too
	// (isHeadlineNode), comparing their matches[1] against the level. For a
	// headline matches[1] is the stars; for those tokens it is the indentation.
	// So a drawer, a block or a SCHEDULED line written in column zero - which
	// is what Emacs writes since org-adapt-indentation went nil in 9.5 - ended
	// the heading, and everything after it, child headings included, was
	// hoisted to the top of the document with no Properties on the heading.
	// A two-space drawer under a level-3 heading did the same.
	stop := func(d *Document, i int) bool {
		return parentStop(d, i) || d.tokens[i].kind == "headline" && len(d.tokens[i].matches[1]) <= headline.Lvl
	}
	consumed, nodes := d.parseMany(i+1, stop)
	// Scan the first few nodes for the PropertyDrawer.
	// In org-mode, planning lines (SCHEDULED/DEADLINE/CLOSED) come before
	// the property drawer, so it may not be nodes[0].
	// We also skip empty paragraphs (blank lines) that may appear between
	// planning lines and the property drawer.
	for j := 0; j < len(nodes); j++ {
		switch nd := nodes[j].(type) {
		case SDC:
			// Skip planning lines — they precede the property drawer
			continue
		case Paragraph:
			// Skip empty paragraphs (blank lines / whitespace) between
			// planning lines and the property drawer.
			if len(nd.Children) == 0 {
				continue
			}
		case *PropertyDrawer:
			headline.Properties = nd
			nodes = append(nodes[:j], nodes[j+1:]...)
			j -= 1
			continue
		case *Drawer:
			// Collected below, with the rest of the heading's drawers.
			continue
		case *Block:
			headline.Blocks = append(headline.Blocks, nd)
		}
		break
	}
	// Every drawer of the heading's own section, not only the first: a
	// :LOGBOOK: is followed by :NOTES: or any other drawer just as often.
	for _, n := range nodes {
		if drawer, ok := n.(*Drawer); ok {
			headline.Drawers = append(headline.Drawers, drawer)
		}
	}
	headline.Children = nodes
	d.Outline.lastHash.Pop()
	d.currentHeadline.Pop()
	return consumed + 1, headline
}

type SDC struct {
	Pos      Pos
	EndPos   Pos
	Date     *OrgDate
	DateType DateType

	// The other planning keywords written on the same line, in the order they
	// appear there. Org puts all of a heading's planning on one line, so this is
	// the usual case rather than an oddity; keeping them on the node is what
	// lets one line be written back as one line.
	Others []SDC
}

type Clock struct {
	Pos    Pos
	EndPos Pos
	Date   *OrgDateClock
}

func (self *SDC) IsZero() bool {
	return self == nil || self.Date == nil || self.Date.IsZero()
}

func (d *Document) parseClock(i int, parentStop stopFn) (int, Node) {
	clk := ParseClock(d.tokens[i].content)
	tclk := Clock{d.tokens[i].Pos(), d.tokens[i].EndPos(), clk}
	if d.Outline.last != nil && d.Outline.last.Headline != nil {
		d.Outline.last.Headline.Clocks = append(d.Outline.last.Headline.Clocks, &tclk)
	}
	return 1, tclk
}

// A planning line may carry more than one keyword, and every one of them has to
// be read.
//
// Org writes `DEADLINE: <...> SCHEDULED: <...>` on one line whenever a heading
// has both, and adds `CLOSED: [...]` to the front of it when the heading is
// marked done. The lexer claims such a line for whichever keyword it recognised
// first and the parse then read that one and threw the line away - so a heading
// with a deadline and a schedule had no deadline, and a done heading with a
// deadline had no closing time. Nothing said so: the date simply never reached
// the headline.
//
// So all three are looked for in the line, ordered as they were written, and the
// first becomes the node with the rest hanging off it - which is what lets the
// writer put the line back as one line rather than splitting it.
func (d *Document) parsePlanning(i int, parentStop stopFn) (int, Node) {
	content := d.tokens[i].content
	pos, endPos := d.tokens[i].Pos(), d.tokens[i].EndPos()
	type found struct {
		at  int
		sdc SDC
	}
	var parts []found
	for _, p := range []struct {
		name   string
		parser *DateParser
		dt     DateType
	}{
		{"SCHEDULED", OrgDateScheduled, Scheduled},
		{"DEADLINE", OrgDateDeadline, Deadline},
		{"CLOSED", OrgDateClosed, Closed},
	} {
		at := strings.Index(content, p.name+":")
		if at < 0 {
			continue
		}
		date, _ := p.parser.Parse(content)
		if date == nil {
			continue
		}
		parts = append(parts, found{at, SDC{pos, endPos, date, p.dt, nil}})
	}
	if len(parts) == 0 {
		// Nothing parsed. Keep the old shape - an SDC with a nil date - rather
		// than handing the line to the paragraph parser, which would be a
		// different answer to the one this has always given.
		s, dt := ParseSDC(content)
		return 1, SDC{pos, endPos, s, dt, nil}
	}
	sort.SliceStable(parts, func(a, b int) bool { return parts[a].at < parts[b].at })

	sdcs := make([]SDC, len(parts))
	for k := range parts {
		sdcs[k] = parts[k].sdc
	}
	if d.Outline.last != nil && d.Outline.last.Headline != nil {
		h := d.Outline.last.Headline
		for k := range sdcs {
			switch sdcs[k].DateType {
			case Scheduled:
				h.Scheduled = &sdcs[k]
			case Deadline:
				h.Deadline = &sdcs[k]
			case Closed:
				h.Closed = &sdcs[k]
			}
		}
	}
	node := sdcs[0]
	node.Others = sdcs[1:]
	return 1, node
}

// Take the cookie off each keyword of a `#+TODO:` line, leaving the keyword.
//
// A cookie is not always one character. `TODO(t)` is a keyword with an access
// key, and this used to handle that and only that - it required the cookie to be
// exactly three characters long, `(x)`. But org's logging notation puts more in
// there: `NEXT(n!)` logs a timestamp on entering NEXT, `WAITING(w@/!)` keeps a
// note going in and a timestamp coming out. Neither was trimmed, so the keyword
// stayed `NEXT(n!)` - and since keywords are recognised on a headline by literal
// prefix, `* NEXT Write the thing` matched nothing and came out as a heading with
// no keyword at all, titled "NEXT Write the thing". It was not a task, could not
// be found by its keyword and never reached an agenda. Every file using the
// notation the org manual documents was affected.
func trimFastTags(tags []string) []string {
	trimmedTags := make([]string, len(tags))
	for i, t := range tags {
		trimmedTags[i] = t
		if !strings.HasSuffix(t, ")") {
			continue
		}
		// `> 0` rather than `>= 0`: a word that is nothing but a cookie has no
		// keyword to be left with, and emptying it would make every headline
		// match it.
		if lParen := strings.LastIndex(t, "("); lParen > 0 {
			trimmedTags[i] = t[:lParen]
		}
	}
	return trimmedTags
}

func (h Headline) ID() string {
	if customID, ok := h.Properties.Get("CUSTOM_ID"); ok {
		return customID
	}
	return fmt.Sprintf("headline-%d", h.Index)
}

func (h Headline) HasDeadline() bool {
	return h.Deadline != nil
}

func (h Headline) HasScheduled() bool {
	return h.Scheduled != nil
}

func (h Headline) HasClosed() bool {
	return h.Closed != nil
}

func (h Headline) HasTimestamp() bool {
	return h.Timestamp != nil
}

func (h Headline) FindDrawer(name string) *Drawer {
	for _, d := range h.Drawers {
		if d.Name == name {
			return d
		}
	}
	return nil
}

func (h Headline) IsExcluded(d *Document) bool {
	for _, excludedTag := range strings.Fields(d.Get("EXCLUDE_TAGS")) {
		for _, tag := range h.Tags {
			if tag == excludedTag {
				return true
			}
		}
	}
	return false
}

// IsArchived reports whether the heading carries the ARCHIVE tag.
func (h Headline) IsArchived() bool {
	for _, tag := range h.Tags {
		if tag == "ARCHIVE" {
			return true
		}
	}
	return false
}

// IsDone reports whether the heading's keyword is one of the done states of
// its document - those after the `|` of a #+TODO: line.
func (h Headline) IsDone() bool {
	if h.Status == "" || h.Doc == nil {
		return false
	}
	_, done := h.Doc.TodoKeywords()
	for _, k := range done {
		if k == h.Status {
			return true
		}
	}
	return false
}

// IsTodo reports whether the heading's keyword is one of the not yet done
// states of its document.
func (h Headline) IsTodo() bool {
	return h.Status != "" && !h.IsDone()
}

// TodoKeywords returns the TODO keywords of the document: those still to do and
// those that are done. They come from every #+TODO:, #+SEQ_TODO: and
// #+TYP_TODO: line - each line is its own sequence, with the done states after
// the `|`, or if there is no `|` the last word alone. Fast access keys and
// logging notes, `TODO(t)` or `WAIT(w@/!)`, are not part of the keyword. With
// no such line the defaults apply, TODO and DONE.
func (d *Document) TodoKeywords() (todo []string, done []string) {
	lines := []string{}
	for _, key := range []string{"TODO", "SEQ_TODO", "TYP_TODO"} {
		if v, ok := d.BufferSettings[key]; ok {
			lines = append(lines, strings.Split(v, "\n")...)
		}
	}
	if len(lines) == 0 {
		lines = strings.Split(d.Get("TODO"), "\n")
	}
	for _, line := range lines {
		words := strings.Fields(line)
		bar := -1
		for i, w := range words {
			if w == "|" {
				bar = i
				break
			}
		}
		var t, dn []string
		if bar >= 0 {
			t, dn = words[:bar], words[bar+1:]
		} else if len(words) > 0 {
			t, dn = words[:len(words)-1], words[len(words)-1:]
		}
		todo = append(todo, trimFastTags(t)...)
		done = append(done, trimFastTags(dn)...)
	}
	return todo, done
}

func (parent *Section) add(current *Section) {
	if parent.Headline == nil || parent.Headline.Lvl < current.Headline.Lvl {
		parent.Children = append(parent.Children, current)
		current.Parent = parent
	} else {
		parent.Parent.add(current)
	}
}

func (h *Headline) AddDrawer(drawer *Drawer) {
	var newNode Node = Node(drawer)
	h.Drawers = append(h.Drawers, drawer)
	if h.Children == nil {
		h.Children = append(h.Children, drawer)
	} else {
		h.Children = Prepend(h.Children, newNode)
	}
	h.Doc.InsertNodeAfter(newNode, h)
}

func (n Headline) String() string   { return orgWriter.WriteNodesAsString(n) }
func (n Headline) GetPos() Pos      { return n.Pos }
func (n Headline) GetTokenEnd() Pos { return n.EndPos }
func (n Headline) GetEnd() Pos {
	if len(n.Children) > 0 {
		return n.Children[len(n.Children)-1].GetEnd()
	} else {
		end := n.GetTokenEnd()
		if n.Properties != nil {
			pend := n.Properties.GetEnd()
			if pend.Row > end.Row {
				end = pend
			}
		}
		if n.Drawers != nil {
			for _, d := range n.Drawers {
				pend := d.GetEnd()
				if pend.Row > end.Row {
					end = pend
				}
			}
		}
		return end
	}
}
func (n SDC) String() string { return orgWriter.WriteNodesAsString(n) }
func (n SDC) GetPos() Pos    { return n.Pos }
func (n SDC) GetEnd() Pos    { return n.EndPos }

func (n SDC) GetTypeName() string      { return GetNodeTypeName(n.GetType()) }
func (n SDC) GetType() NodeType        { return SDCNode }
func (n Headline) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n Headline) GetType() NodeType   { return HeadlineNode }

func (n Clock) String() string { return orgWriter.WriteNodesAsString(n) }
func (n Clock) GetPos() Pos    { return n.Pos }
func (n Clock) GetEnd() Pos    { return n.EndPos }

func (n Clock) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n Clock) GetType() NodeType   { return ClockNode }

func (n SDC) GetChildren() []Node      { return nil }
func (n Headline) GetChildren() []Node { return n.Children }
func (n Clock) GetChildren() []Node    { return nil }
