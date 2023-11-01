package org

import (
	b64 "encoding/base64"
	"fmt"
	"hash"
	"regexp"
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
	Pos        Pos
	EndPos     Pos
	Index      int
	Lvl        int
	Status     string
	Priority   string
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
	// Schedules  []Schedule
}

var headlineRegexp = regexp.MustCompile(`^([*]+)\s+(.*)`)
var tagRegexp = regexp.MustCompile(`(.*?)\s+(:[A-Za-z0-9_@#%:]+:\s*$)`)
var pdoneRegexp = regexp.MustCompile(`(.*?)\s\[\s*((?P<percent>\d+)%)|((?P<a>\d+)/(?P<b>\d+))\s*\]\s*$`)

func lexHeadline(line string, row, col int) (token, bool) {
	if m := headlineRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		return token{"headline", 0, m[2], m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func reMatchParams(re *regexp.Regexp, m []string) (paramsMap map[string]string) {

	paramsMap = make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i > 0 && i <= len(m) {
			paramsMap[name] = m[i]
		}
	}
	return paramsMap
}

func (d *Document) parseHeadline(i int, parentStop stopFn) (int, Node) {
	pos := d.tokens[i].Pos()
	t, headline := d.tokens[i], Headline{Pos: pos, EndPos: d.tokens[i].EndPos()}
	headline.Lvl = len(t.matches[1])

	headline.Index = d.addHeadline(&headline)

	text := t.content
	todoKeywords := trimFastTags(
		strings.FieldsFunc(d.Get("TODO"), func(r rune) bool { return unicode.IsSpace(r) || r == '|' }),
	)
	for _, k := range todoKeywords {
		if strings.HasPrefix(text, k) && len(text) > len(k) && unicode.IsSpace(rune(text[len(k)])) {
			headline.Status = k
			text = text[len(k)+1:]
			break
		}
	}

	if len(text) >= 4 && text[0:2] == "[#" && strings.Contains("ABC", text[2:3]) && text[3] == ']' {
		headline.Priority = text[2:3]
		text = strings.TrimSpace(text[4:])
	}

	if m := tagRegexp.FindStringSubmatch(text); m != nil {
		text = m[1]
		headline.Tags = strings.FieldsFunc(m[2], func(r rune) bool { return r == ':' })
	}

	if m := pdoneRegexp.FindStringSubmatch(text); m != nil {
		info := reMatchParams(pdoneRegexp, m)
		headline.CheckStatus = new(CheckStatus)
		if p, ok := info["precent"]; ok {
			per, err := strconv.Atoi(p)
			if err != nil {
				headline.CheckStatus.Type = "%"
				headline.CheckStatus.Num = per
			} else {
				headline.CheckStatus = nil
			}
		} else {
			a, err := strconv.Atoi(info["a"])
			b, err2 := strconv.Atoi(info["b"])
			if err != nil && err2 != nil {
				headline.CheckStatus.Type = "/"
				headline.CheckStatus.Num = a
				headline.CheckStatus.Den = b
			} else {
				headline.CheckStatus = nil
			}
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

	stop := func(d *Document, i int) bool {
		return parentStop(d, i) || d.tokens[i].kind == "headline" && len(d.tokens[i].matches[1]) <= headline.Lvl
	}
	consumed, nodes := d.parseMany(i+1, stop)
	if len(nodes) > 0 {
		if d, ok := nodes[0].(PropertyDrawer); ok {
			headline.Properties = &d
			nodes = nodes[1:]
		}
	}
	headline.Children = nodes
	d.currentHeadline = &headline
	d.Outline.lastHash.Pop()
	return consumed + 1, headline
}

type SDC struct {
	Pos      Pos
	Date     *OrgDate
	DateType DateType
}

func (self *SDC) IsZero() bool {
	return self == nil || self.Date == nil || self.Date.IsZero()
}

func (d *Document) parseScheduled(i int, parentStop stopFn) (int, Node) {
	s, dt := ParseSDC(d.tokens[i].content)
	sdc := SDC{d.tokens[i].Pos(), s, dt}
	if d.Outline.last != nil && d.Outline.last.Headline != nil {
		d.Outline.last.Headline.Scheduled = &sdc
	}
	return 1, sdc
}
func (d *Document) parseDeadline(i int, parentStop stopFn) (int, Node) {
	s, dt := ParseSDC(d.tokens[i].content)
	sdc := SDC{d.tokens[i].Pos(), s, dt}
	if d.Outline.last != nil && d.Outline.last.Headline != nil {
		d.Outline.last.Headline.Deadline = &sdc
	}
	return 1, sdc
}
func (d *Document) parseClosed(i int, parentStop stopFn) (int, Node) {
	s, dt := ParseSDC(d.tokens[i].content)
	sdc := SDC{d.tokens[i].Pos(), s, dt}
	if d.Outline.last != nil && d.Outline.last.Headline != nil {
		d.Outline.last.Headline.Closed = &sdc
	}
	return 1, sdc
}

func trimFastTags(tags []string) []string {
	trimmedTags := make([]string, len(tags))
	for i, t := range tags {
		lParen := strings.LastIndex(t, "(")
		rParen := strings.LastIndex(t, ")")
		end := len(t) - 1
		if lParen == end-2 && rParen == end {
			trimmedTags[i] = t[:end-2]
		} else {
			trimmedTags[i] = t
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

func (parent *Section) add(current *Section) {
	if parent.Headline == nil || parent.Headline.Lvl < current.Headline.Lvl {
		parent.Children = append(parent.Children, current)
		current.Parent = parent
	} else {
		parent.Parent.add(current)
	}
}

func (n Headline) String() string   { return orgWriter.WriteNodesAsString(n) }
func (n Headline) GetPos() Pos      { return n.Pos }
func (n Headline) GetTokenEnd() Pos { return n.EndPos }
func (n Headline) GetEnd() Pos {
	if len(n.Children) > 0 {
		return n.Children[len(n.Children)-1].GetEnd()
	}
	return n.GetTokenEnd()
}
func (n SDC) String() string { return orgWriter.WriteNodesAsString(n) }
func (n SDC) GetPos() Pos    { return n.Pos }
func (n SDC) GetEnd() Pos    { return n.Pos }
