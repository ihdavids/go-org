package org

import "fmt"

// Writer is the interface that is used to export a parsed document into a new format. See Document.Write().
type Writer interface {
	Before(*Document) // Before is called before any nodes are passed to the writer.
	After(*Document)  // After is called after all nodes have been passed to the writer.
	String() string   // String is called at the very end to retrieve the final output.

	WriterWithExtensions() Writer
	WriteNodesAsString(...Node) string

	WriteKeyword(Keyword)
	WriteInclude(Include)
	WriteComment(Comment)
	WriteNodeWithMeta(NodeWithMeta)
	WriteNodeWithName(NodeWithName)
	WriteHeadline(Headline)
	WriteBlock(Block)
	WriteResult(Result)
	WriteInlineBlock(InlineBlock)
	WriteExample(Example)
	WriteDrawer(Drawer)
	WritePropertyDrawer(PropertyDrawer)
	WriteList(List)
	WriteListItem(ListItem)
	WriteDescriptiveListItem(DescriptiveListItem)
	WriteTable(Table)
	WriteHorizontalRule(HorizontalRule)
	WriteParagraph(Paragraph)
	WriteText(Text)
	WriteEmphasis(Emphasis)
	WriteLatexFragment(LatexFragment)
	WriteStatisticToken(StatisticToken)
	WriteExplicitLineBreak(ExplicitLineBreak)
	WriteLineBreak(LineBreak)
	WriteRegularLink(RegularLink)
	WriteMacro(Macro)
	WriteTimestamp(Timestamp)
	WriteFootnoteLink(FootnoteLink)
	WriteFootnoteDefinition(FootnoteDefinition)
	WriteSDC(SDC)
	WriteClock(Clock)
	NodeIdx(int)
	ResetLineBreak()
}

// Node types added after the Writer interface are written through these
// optional interfaces, so that a writer written before them still compiles and
// still writes something sensible for them.

// TargetWriter writes a <<target>>. Without it a target is not written - in
// an export it is invisible anyway.
type TargetWriter interface{ WriteTarget(Target) }

// RadioTargetWriter writes a <<<radio target>>>. Without it the radio target
// is written as its text.
type RadioTargetWriter interface{ WriteRadioTarget(RadioTarget) }

// BabelCallWriter writes a #+CALL: line and its results. Without it the call
// is written as its keyword, followed by its results.
type BabelCallWriter interface{ WriteBabelCall(BabelCall) }

// TableElWriter writes a table.el table. Without it the table is written as an
// example of its lines.
type TableElWriter interface{ WriteTableEl(TableEl) }

func WriteNodes(w Writer, nodes ...Node) {
	WriteNodesLB(0, w, nodes...)
}

func WriteNodesLB(offset int, w Writer, nodes ...Node) {
	w = w.WriterWithExtensions()
	if offset <= 0 {
		w.ResetLineBreak()
	}
	for i, n := range nodes {
		w.NodeIdx(i + offset)
		switch n := n.(type) {
		case Keyword:
			w.WriteKeyword(n)
		case Include:
			w.WriteInclude(n)
		case Comment:
			w.WriteComment(n)
		case NodeWithMeta:
			w.WriteNodeWithMeta(n)
		case NodeWithName:
			w.WriteNodeWithName(n)
		case *NodeWithName:
			w.WriteNodeWithName(*n)
		case Headline:
			w.WriteHeadline(n)
		case *Headline:
			w.WriteHeadline(*n)
		case Block:
			w.WriteBlock(n)
		case *Block:
			w.WriteBlock(*n)
		case Result:
			w.WriteResult(n)
		case InlineBlock:
			w.WriteInlineBlock(n)
		case *InlineBlock:
			w.WriteInlineBlock(*n)
		case Example:
			w.WriteExample(n)
		case Drawer:
			w.WriteDrawer(n)
		case *Drawer:
			w.WriteDrawer(*n)
		case PropertyDrawer:
			w.WritePropertyDrawer(n)
		case *PropertyDrawer:
			w.WritePropertyDrawer(*n)
		case List:
			w.WriteList(n)
		case ListItem:
			w.WriteListItem(n)
		case DescriptiveListItem:
			w.WriteDescriptiveListItem(n)
		case Table:
			w.WriteTable(n)
		case *Table:
			w.WriteTable(*n)
		case HorizontalRule:
			w.WriteHorizontalRule(n)
		case Paragraph:
			w.WriteParagraph(n)
		case Text:
			w.WriteText(n)
		case Emphasis:
			w.WriteEmphasis(n)
		case LatexFragment:
			w.WriteLatexFragment(n)
		case StatisticToken:
			w.WriteStatisticToken(n)
		case ExplicitLineBreak:
			w.WriteExplicitLineBreak(n)
		case LineBreak:
			w.WriteLineBreak(n)
		case RegularLink:
			w.WriteRegularLink(n)
		case Macro:
			w.WriteMacro(n)
		case Timestamp:
			w.WriteTimestamp(n)
		case FootnoteLink:
			w.WriteFootnoteLink(n)
		case FootnoteDefinition:
			w.WriteFootnoteDefinition(n)
		case SDC:
			w.WriteSDC(n)
		case Clock:
			w.WriteClock(n)
		case IncludedContent:
			WriteNodes(w, n.Nodes...)
		case Target:
			if tw, ok := w.(TargetWriter); ok {
				tw.WriteTarget(n)
			}
		case RadioTarget:
			if rw, ok := w.(RadioTargetWriter); ok {
				rw.WriteRadioTarget(n)
			} else {
				WriteNodes(w, n.Children...)
			}
		case BabelCall:
			if bw, ok := w.(BabelCallWriter); ok {
				bw.WriteBabelCall(n)
			} else {
				w.WriteKeyword(n.Keyword)
				if n.Result != nil {
					WriteNodes(w, n.Result)
				}
			}
		case TableEl:
			if tw, ok := w.(TableElWriter); ok {
				tw.WriteTableEl(n)
			} else {
				example := Example{Pos: n.Pos}
				for _, line := range n.Lines {
					example.Children = append(example.Children, Text{Content: line, IsRaw: true})
				}
				w.WriteExample(example)
			}
		default:
			if n != nil {
				panic(fmt.Sprintf("bad node %T %#v", n, n))
			}
		}
	}
}
