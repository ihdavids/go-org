package org

import (
	"html"
	"strings"
)

// TableEl is a table.el table - the kind drawn with + and - borders rather than
// an org table. Org does not edit these itself, so they are kept as they were
// written, line for line.
//
//	+-----+-----+
//	| a   | b   |
//	+=====+=====+
//	| 1   | 2   |
//	+-----+-----+
type TableEl struct {
	Pos    Pos
	EndPos Pos
	Lines  []string // the lines of the table, without the indentation of the table itself
}

func (d *Document) parseTableEl(i int, parentStop stopFn) (int, Node) {
	start, lvl := i, d.tokens[i].lvl
	table := TableEl{Pos: d.tokens[i].Pos()}
	for ; i < len(d.tokens) && !parentStop(d, i); i++ {
		line := d.tokens[i].matches[0]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || (trimmed[0] != '|' && trimmed[0] != '+') {
			break
		}
		table.Lines = append(table.Lines, trimIndentUpTo(lvl)(strings.TrimRight(line, " \t")))
		table.EndPos = d.tokens[i].EndPos()
	}
	return i - start, table
}

// Columns returns the positions of the column borders of the table, taken from
// its first line, or nil if the table is irregular (spans cells across columns
// or rows), in which case it can only be shown as drawn.
func (t TableEl) Columns() []int {
	if len(t.Lines) < 2 {
		return nil
	}
	borders := []int{}
	for i, r := range t.Lines[0] {
		if r == '+' {
			borders = append(borders, i)
		}
	}
	if len(borders) < 2 {
		return nil
	}
	for _, line := range t.Lines {
		if len(line) != borders[len(borders)-1]+1 {
			return nil
		}
		for _, b := range borders {
			if c := line[b]; c != '+' && c != '|' {
				return nil
			}
		}
		if line[0] == '+' {
			for j := 0; j+1 < len(borders); j++ {
				if seg := strings.Trim(line[borders[j]+1:borders[j+1]], "-="); seg != "" {
					return nil
				}
			}
		}
	}
	return borders
}

// Cells returns the rows of the table, each a list of cell texts, and how many
// of them are the header - the rows above a `+===+` border. A cell that spans
// several lines has them joined by a space. ok is false for an irregular table.
func (t TableEl) Cells() (rows [][]string, headerRows int, ok bool) {
	borders := t.Columns()
	if borders == nil {
		return nil, 0, false
	}
	var current [][]string
	flush := func() {
		if current == nil {
			return
		}
		row := make([]string, len(borders)-1)
		for c := range row {
			parts := []string{}
			for _, line := range current {
				if p := strings.TrimSpace(line[c]); p != "" {
					parts = append(parts, p)
				}
			}
			row[c] = strings.Join(parts, " ")
		}
		rows = append(rows, row)
		current = nil
	}
	for _, line := range t.Lines {
		if line[0] == '+' {
			flush()
			if strings.Contains(line, "=") && headerRows == 0 {
				headerRows = len(rows)
			}
			continue
		}
		cells := []string{}
		for j := 0; j+1 < len(borders); j++ {
			cells = append(cells, line[borders[j]+1:borders[j+1]])
		}
		current = append(current, cells)
	}
	flush()
	return rows, headerRows, true
}

func (t TableEl) html() string {
	rows, headerRows, ok := t.Cells()
	if !ok {
		return `<pre class="table-el">` + "\n" + html.EscapeString(strings.Join(t.Lines, "\n")) + "\n</pre>\n"
	}
	out := strings.Builder{}
	out.WriteString(`<table class="table-el">` + "\n")
	for i, row := range rows {
		tag := "td"
		if i < headerRows {
			tag = "th"
		}
		if i == 0 && headerRows > 0 {
			out.WriteString("<thead>\n")
		} else if i == headerRows {
			out.WriteString("<tbody>\n")
		}
		out.WriteString("<tr>\n")
		for _, cell := range row {
			out.WriteString("<" + tag + ">" + html.EscapeString(cell) + "</" + tag + ">\n")
		}
		out.WriteString("</tr>\n")
		if i == headerRows-1 {
			out.WriteString("</thead>\n")
		}
	}
	if len(rows) > headerRows {
		out.WriteString("</tbody>\n")
	}
	out.WriteString("</table>\n")
	return out.String()
}

func (n TableEl) String() string      { return orgWriter.WriteNodesAsString(n) }
func (n TableEl) GetPos() Pos         { return n.Pos }
func (n TableEl) GetEnd() Pos         { return n.EndPos }
func (n TableEl) GetType() NodeType   { return TableElNode }
func (n TableEl) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
func (n TableEl) GetChildren() []Node { return nil }
