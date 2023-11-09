package org

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// OrgWriter export an org document into pretty printed org document.
type OrgWriter struct {
	ExtendingWriter Writer
	TagsColumn      int

	strings.Builder
	Indent string
}

var exampleBlockUnescapeRegexp = regexp.MustCompile(`(^|\n)([ \t]*)(\*|,\*|#\+|,#\+)`)

var emphasisOrgBorders = map[string][]string{
	"_":   []string{"_", "_"},
	"*":   []string{"*", "*"},
	"/":   []string{"/", "/"},
	"+":   []string{"+", "+"},
	"~":   []string{"~", "~"},
	"=":   []string{"=", "="},
	"_{}": []string{"_{", "}"},
	"^{}": []string{"^{", "}"},
}

func NewOrgWriter() *OrgWriter {
	return &OrgWriter{
		TagsColumn: 77,
	}
}

func (w *OrgWriter) WriterWithExtensions() Writer {
	if w.ExtendingWriter != nil {
		return w.ExtendingWriter
	}
	return w
}

func (w *OrgWriter) Before(d *Document) {}
func (w *OrgWriter) After(d *Document)  {}

func (w *OrgWriter) WriteNodesAsString(nodes ...Node) string {
	builder := w.Builder
	w.Builder = strings.Builder{}
	WriteNodes(w, nodes...)
	out := w.String()
	w.Builder = builder
	return out
}

func (w *OrgWriter) WriteHeadline(h Headline) {
	start := w.Len()
	w.WriteString(strings.Repeat("*", h.Lvl))
	if h.Status != "" {
		w.WriteString(" " + h.Status)
	}
	if h.Priority != "" {
		w.WriteString(" [#" + h.Priority + "]")
	}
	w.WriteString(" ")
	WriteNodes(w, h.Title...)
	if h.CheckStatus != nil {
		w.WriteString(h.CheckStatus.String())
	}
	if len(h.Tags) != 0 {
		tString := ":" + strings.Join(h.Tags, ":") + ":"
		if n := w.TagsColumn - len(tString) - (w.Len() - start); n > 0 {
			w.WriteString(strings.Repeat(" ", n) + tString)
		} else {
			w.WriteString(" " + tString)
		}
	}
	w.WriteString("\n")
	originalIndent := w.Indent
	w.Indent = strings.Repeat(" ", h.Lvl+1)
	//if len(h.Children) != 0 {
	//	w.WriteString(w.Indent)
	//}
	if h.Properties != nil {
		WriteNodes(w, *h.Properties)
	}
	WriteNodes(w, h.Children...)
	w.Indent = originalIndent
}

func (w *OrgWriter) WriteBlock(b Block) {
	w.WriteString(w.Indent + "#+BEGIN_" + b.Name)
	if len(b.Parameters) != 0 {
		w.WriteString(" " + strings.Join(b.Parameters, " "))
	}
	w.WriteString("\n")
	if isRawTextBlock(b.Name) {
		w.WriteString(w.Indent)
	}
	content := w.WriteNodesAsString(b.Children...)
	if b.Name == "EXAMPLE" || (b.Name == "SRC" && len(b.Parameters) >= 1 && b.Parameters[0] == "org") {
		content = exampleBlockUnescapeRegexp.ReplaceAllString(content, "$1$2,$3")
	}
	w.WriteString(content)
	if !isRawTextBlock(b.Name) {
		w.WriteString(w.Indent)
	}
	w.WriteString("#+END_" + b.Name + "\n")

	if b.Result != nil {
		w.WriteString("\n")
		WriteNodes(w, b.Result)
	}
}

func (w *OrgWriter) WriteResult(r Result) {
	w.WriteString(w.Indent + "#+RESULTS:\n")
	WriteNodes(w, r.Node)
}

func (w *OrgWriter) WriteInlineBlock(b InlineBlock) {
	switch b.Name {
	case "src":
		w.WriteString(b.Name + "_" + b.Parameters[0])
		if len(b.Parameters) > 1 {
			w.WriteString("[" + strings.Join(b.Parameters[1:], " ") + "]")
		}
		w.WriteString("{")
		WriteNodes(w, b.Children...)
		w.WriteString("}")
	case "export":
		w.WriteString("@@" + b.Parameters[0] + ":")
		WriteNodes(w, b.Children...)
		w.WriteString("@@")
	}
}

func (w *OrgWriter) WriteDrawer(d Drawer) {
	w.WriteString(w.Indent + ":" + d.Name + ":\n")
	WriteNodes(w, d.Children...)
	w.WriteString(w.Indent + ":END:\n")
}

func (w *OrgWriter) WritePropertyDrawer(d PropertyDrawer) {
	w.WriteString(w.Indent + ":PROPERTIES:\n")
	for _, kvPair := range d.Properties {
		k, v := kvPair[0], kvPair[1]
		if v != "" {
			v = " " + v
		}
		w.WriteString(fmt.Sprintf(w.Indent+"  :%s:%s\n", k, v))
	}
	w.WriteString(w.Indent + ":END:\n")
}

func (w *OrgWriter) WriteFootnoteDefinition(f FootnoteDefinition) {
	if f.Inline {
		w.WriteString(fmt.Sprintf("[fn:%s]", f.Name))
	} else {
		w.WriteString(fmt.Sprintf(w.Indent+"[fn:%s]", f.Name))
	}
	content := w.WriteNodesAsString(f.Children...)
	if content != "" && !unicode.IsSpace(rune(content[0])) {
		w.WriteString(" ")
	}
	w.WriteString(content)
}

func (w *OrgWriter) WriteSDC(s SDC) {
	name := ""
	switch s.DateType {
	case Scheduled:
		name = "SCHEDULED"
		break
	case Deadline:
		name = "DEADLINE"
		break
	case Closed:
		name = "CLOSED"
		break
	}
	w.WriteString(fmt.Sprintf("%s: %s\n", name, s.Date.ToString()))
}

func (w *OrgWriter) WriteParagraph(p Paragraph) {
	content := w.WriteNodesAsString(p.Children...)
	temp := strings.TrimPrefix(content, w.Indent)
	if len(content) > 0 && content[0] != '\n' && len(temp) == len(content) {
		w.WriteString(w.Indent)
	}
	w.WriteString(content + "\n")
}

func (w *OrgWriter) WriteExample(e Example) {
	for _, n := range e.Children {
		w.WriteString(w.Indent + ":")
		if content := w.WriteNodesAsString(n); content != "" {
			w.WriteString(" " + content)
		}
		w.WriteString("\n")
	}
}

func (w *OrgWriter) WriteKeyword(k Keyword) {
	w.WriteString(w.Indent + "#+" + k.Key + ":")
	if k.Value != "" {
		w.WriteString(" " + k.Value)
	}
	w.WriteString("\n")
}

func (w *OrgWriter) WriteInclude(i Include) {
	w.WriteKeyword(i.Keyword)
}

func (w *OrgWriter) WriteNodeWithMeta(n NodeWithMeta) {
	for _, ns := range n.Meta.Caption {
		w.WriteString("#+CAPTION: ")
		WriteNodes(w, ns...)
		w.WriteString("\n")
	}
	for _, attributes := range n.Meta.HTMLAttributes {
		w.WriteString("#+ATTR_HTML: ")
		w.WriteString(strings.Join(attributes, " ") + "\n")
	}
	WriteNodes(w, n.Node)
}

func (w *OrgWriter) WriteNodeWithName(n NodeWithName) {
	w.WriteString(fmt.Sprintf("#+NAME: %s\n", n.Name))
	WriteNodes(w, n.Node)
}

func (w *OrgWriter) WriteComment(c Comment) {
	w.WriteString(w.Indent + "# " + c.Content + "\n")
}

func (w *OrgWriter) WriteList(l List) { WriteNodes(w, l.Items...) }

func (w *OrgWriter) WriteListItem(li ListItem) {
	originalBuilder, originalIndent := w.Builder, w.Indent
	w.Builder, w.Indent = strings.Builder{}, w.Indent+strings.Repeat(" ", len(li.Bullet)+1)
	WriteNodes(w, li.Children...)
	content := strings.TrimPrefix(w.String(), w.Indent)
	w.Builder, w.Indent = originalBuilder, originalIndent
	w.WriteString(w.Indent + li.Bullet)
	if li.Value != "" {
		w.WriteString(fmt.Sprintf(" [@%s]", li.Value))
	}
	if li.Status != "" {
		w.WriteString(fmt.Sprintf(" [%s]", li.Status))
	}
	if len(content) > 0 && content[0] == '\n' {
		w.WriteString(content)
	} else {
		w.WriteString(" " + content)
	}
}

func (w *OrgWriter) WriteDescriptiveListItem(di DescriptiveListItem) {
	indent := w.Indent + strings.Repeat(" ", len(di.Bullet)+1)
	w.WriteString(w.Indent + di.Bullet)
	if di.Status != "" {
		w.WriteString(fmt.Sprintf(" [%s]", di.Status))
		indent = indent + strings.Repeat(" ", len(di.Status)+3)
	}
	if len(di.Term) != 0 {
		term := w.WriteNodesAsString(di.Term...)
		w.WriteString(" " + term + " ::")
		indent = indent + strings.Repeat(" ", len(term)+4)
	}
	originalBuilder, originalIndent := w.Builder, w.Indent
	w.Builder, w.Indent = strings.Builder{}, indent
	WriteNodes(w, di.Details...)
	details := strings.TrimPrefix(w.String(), w.Indent)
	w.Builder, w.Indent = originalBuilder, originalIndent
	if len(details) > 0 && details[0] == '\n' {
		w.WriteString(details)
	} else {
		w.WriteString(" " + details)
	}
}

func (w *OrgWriter) WriteTable(t Table) {
	for _, row := range t.Rows {
		w.WriteString(w.Indent)
		if len(row.Columns) == 0 {
			w.WriteString(`|`)
			for i := 0; i < len(t.ColumnInfos); i++ {
				w.WriteString(strings.Repeat("-", t.ColumnInfos[i].Len+2))
				if i < len(t.ColumnInfos)-1 {
					w.WriteString("+")
				}
			}
			w.WriteString(`|`)

		} else {
			w.WriteString(`|`)
			for _, column := range row.Columns {
				w.WriteString(` `)
				content := w.WriteNodesAsString(column.Children...)
				if content == "" {
					content = " "
				}
				n := column.Len - utf8.RuneCountInString(content)
				if n < 0 {
					n = 0
				}
				if column.Align == "center" {
					if n%2 != 0 {
						w.WriteString(" ")
					}
					w.WriteString(strings.Repeat(" ", n/2) + content + strings.Repeat(" ", n/2))
				} else if column.Align == "right" {
					w.WriteString(strings.Repeat(" ", n) + content)
				} else {
					w.WriteString(content + strings.Repeat(" ", n))
				}
				w.WriteString(` |`)
			}
		}
		w.WriteString("\n")
	}
}

func (w *OrgWriter) WriteHorizontalRule(hr HorizontalRule) {
	w.WriteString(w.Indent + "-----\n")
}

func (w *OrgWriter) WriteText(t Text) { w.WriteString(t.Content) }

func (w *OrgWriter) WriteEmphasis(e Emphasis) {
	borders, ok := emphasisOrgBorders[e.Kind]
	if !ok {
		panic(fmt.Sprintf("bad emphasis %#v", e))
	}
	w.WriteString(borders[0])
	WriteNodes(w, e.Content...)
	w.WriteString(borders[1])
}

func (w *OrgWriter) WriteLatexFragment(l LatexFragment) {
	w.WriteString(l.OpeningPair)
	WriteNodes(w, l.Content...)
	w.WriteString(l.ClosingPair)
}

func (w *OrgWriter) WriteStatisticToken(s StatisticToken) {
	w.WriteString(fmt.Sprintf("[%s]", s.Content))
}

func (w *OrgWriter) WriteLineBreak(l LineBreak) {
	w.WriteString(strings.Repeat("\n"+w.Indent, l.Count))
}

func (w *OrgWriter) WriteExplicitLineBreak(l ExplicitLineBreak) {
	w.WriteString(`\\` + "\n" + w.Indent)
}

func (w *OrgWriter) WriteTimestamp(t Timestamp) {
	w.WriteString(t.Time.ToString())
}

func (w *OrgWriter) WriteFootnoteLink(l FootnoteLink) {
	w.WriteString("[fn:" + l.Name)
	if l.Definition != nil {
		w.WriteString(":")
		WriteNodes(w, l.Definition.Children[0].(Paragraph).Children...)
	}
	w.WriteString("]")
}

func (w *OrgWriter) WriteRegularLink(l RegularLink) {
	if l.AutoLink {
		w.WriteString(l.URL)
	} else if l.Description == nil {
		w.WriteString(fmt.Sprintf("[[%s]]", l.URL))
	} else {
		w.WriteString(fmt.Sprintf("[[%s][%s]]", l.URL, w.WriteNodesAsString(l.Description...)))
	}
}

func (w *OrgWriter) WriteMacro(m Macro) {
	w.WriteString(fmt.Sprintf("{{{%s(%s)}}}", m.Name, strings.Join(m.Parameters, ",")))
}
