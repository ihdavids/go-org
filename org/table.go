package org

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Table struct {
	Rows             []Row
	ColumnInfos      []ColumnInfo
	SeparatorIndices []int
	Pos              Pos
}

type Row struct {
	Columns   []Column
	IsSpecial bool
}

type Column struct {
	Pos      Pos
	EndPos   Pos
	Children []Node
	*ColumnInfo
}

type ColumnInfo struct {
	Align      string
	Len        int
	DisplayLen int
}

var tableSeparatorRegexp = regexp.MustCompile(`^(\s*)(\|[+|-]+)\s*$`)
var tableRowRegexp = regexp.MustCompile(`^(\s*)(\|.*)`)

var columnAlignAndLengthRegexp = regexp.MustCompile(`^<(l|c|r)?(\d+)?>$`)

func lexTable(line string, row, col int) (token, bool) {
	if m := tableSeparatorRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col}
		return token{"tableSeparator", len(m[1]), m[2], m, pos, Pos{row, col + len(m[0])}}, true
	} else if m := tableRowRegexp.FindStringSubmatch(line); m != nil {
		pos := Pos{row, col + len(m[1])}
		return token{"tableRow", len(m[1]), m[2], m, pos, Pos{row, col + len(m[0])}}, true
	}
	return nilToken, false
}

func (d *Document) parseTable(i int, parentStop stopFn) (int, Node) {
	rawRows, separatorIndices, start := [][]string{}, []int{}, i
	rowStartPositions := [][]Pos{}
	rowEndPositions := [][]Pos{}
	for ; !parentStop(d, i); i++ {
		if t := d.tokens[i]; t.kind == "tableRow" {
			rawRow := strings.FieldsFunc(d.tokens[i].content, func(r rune) bool { return r == '|' })
			startPos := d.tokens[i].pos
			// We do not need to do this because we are doing it below!
			//startPos.Col += 1 // increment past separator (this is first cell)
			endPos := Pos{Row: startPos.Row, Col: startPos.Col}
			curStartPos := []Pos{}
			curEndPos := []Pos{}
			for i := range rawRow {
				startPos = endPos
				startPos.Col += 1 // increment past separator (for this cell)
				endPos = Pos{Row: startPos.Row, Col: startPos.Col + len(rawRow[i])}
				rawRow[i] = strings.TrimSpace(rawRow[i])
				curStartPos = append(curStartPos, startPos)
				curEndPos = append(curEndPos, endPos)
			}
			rawRows = append(rawRows, rawRow)
			rowStartPositions = append(rowStartPositions, curStartPos)
			rowEndPositions = append(rowEndPositions, curEndPos)
		} else if t.kind == "tableSeparator" {
			separatorIndices = append(separatorIndices, i-start)
			rawRows = append(rawRows, nil)
			rowStartPositions = append(rowStartPositions, nil)
			rowEndPositions = append(rowEndPositions, nil)
		} else {
			break
		}
	}

	table := &Table{nil, getColumnInfos(rawRows), separatorIndices, d.tokens[start].Pos()}
	var starts []Pos
	var ends []Pos
	for r, rawColumns := range rawRows {
		row := Row{nil, isSpecialRow(rawColumns)}
		if rowStartPositions[r] != nil {
			starts = rowStartPositions[r]
			ends = rowEndPositions[r]
		}
		if len(rawColumns) != 0 {
			for i := range table.ColumnInfos {
				var s Pos = Pos{0, 0}
				var e Pos = Pos{0, 0}
				if starts != nil {
					if i < len(starts) {
						s = starts[i]
					} else if i == len(starts) {
						s = ends[i-1]
						s = Pos{Row: s.Row, Col: s.Col + 1}
						e = s
					}
				}
				if ends != nil && i < len(ends) {
					e = ends[i]
				}
				column := Column{s, e, nil, &table.ColumnInfos[i]}
				if i < len(rawColumns) {
					column.Children = d.parseInline(rawColumns[i], start) // TODO: This is off by the row index
				}
				row.Columns = append(row.Columns, column)
			}
		}
		table.Rows = append(table.Rows, row)
	}
	return i - start, table
}

func getColumnInfos(rows [][]string) []ColumnInfo {
	columnCount := 0
	for _, columns := range rows {
		if n := len(columns); n > columnCount {
			columnCount = n
		}
	}

	columnInfos := make([]ColumnInfo, columnCount)
	for i := 0; i < columnCount; i++ {
		countNumeric, countNonNumeric := 0, 0
		for _, columns := range rows {
			if i >= len(columns) {
				continue
			}

			if n := utf8.RuneCountInString(columns[i]); n > columnInfos[i].Len {
				columnInfos[i].Len = n
			}

			if m := columnAlignAndLengthRegexp.FindStringSubmatch(columns[i]); m != nil && isSpecialRow(columns) {
				switch m[1] {
				case "l":
					columnInfos[i].Align = "left"
				case "c":
					columnInfos[i].Align = "center"
				case "r":
					columnInfos[i].Align = "right"
				}
				if m[2] != "" {
					l, _ := strconv.Atoi(m[2])
					columnInfos[i].DisplayLen = l
				}
			} else if _, err := strconv.ParseFloat(columns[i], 32); err == nil {
				countNumeric++
			} else if strings.TrimSpace(columns[i]) != "" {
				countNonNumeric++
			}
		}

		if columnInfos[i].Align == "" && countNumeric >= countNonNumeric {
			columnInfos[i].Align = "right"
		}
	}
	return columnInfos
}

func isSpecialRow(rawColumns []string) bool {
	isAlignRow := true
	for _, rawColumn := range rawColumns {
		if !columnAlignAndLengthRegexp.MatchString(rawColumn) && rawColumn != "" {
			isAlignRow = false
		}
	}
	return isAlignRow
}

func (self Row) GetEnd() Pos {
	if self.Columns != nil && len(self.Columns) > 0 {
		p := self.Columns[len(self.Columns)-1].GetEnd()
		p.Col += 1 // include end marker that is skipped
		return p
	}
	return Pos{0, 0}
}
func (self Row) GetPos() Pos {
	if len(self.Columns) > 0 {
		return self.Columns[0].GetPos()
	}
	return Pos{0, 0}
}
func (self Column) GetEnd() Pos {
	return self.EndPos
}
func (self Column) GetPos() Pos {
	return self.Pos
}
func (n Table) String() string { return orgWriter.WriteNodesAsString(n) }
func (self Table) GetPos() Pos { return self.Pos }
func (self Table) GetEnd() Pos {
	if self.Rows != nil && len(self.Rows) > 0 {
		return self.Rows[len(self.Rows)-1].GetEnd()
	}
	return self.Pos
}

// lazy suzan
// cupholder coasters
// Mess free clipper

func (n Table) GetType() NodeType  { return TableNode }
func (n Row) GetType() NodeType    { return TableRowNode }
func (n Column) GetType() NodeType { return TableColNode }

func (n Table) GetTypeName() string  { return GetNodeTypeName(n.GetType()) }
func (n Row) GetTypeName() string    { return GetNodeTypeName(n.GetType()) }
func (n Column) GetTypeName() string { return GetNodeTypeName(n.GetType()) }
