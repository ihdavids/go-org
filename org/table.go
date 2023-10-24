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

var tableSeparatorRegexp = regexp.MustCompile(`^(\s*)(\|[+-|]*)\s*$`)
var tableRowRegexp = regexp.MustCompile(`^(\s*)(\|.*)`)

var columnAlignAndLengthRegexp = regexp.MustCompile(`^<(l|c|r)?(\d+)?>$`)

func lexTable(line string, row, col int) (token, bool) {
	if m := tableSeparatorRegexp.FindStringSubmatch(line); m != nil {
		return token{"tableSeparator", len(m[1]), m[2], m, Pos{row, col}}, true
	} else if m := tableRowRegexp.FindStringSubmatch(line); m != nil {
		return token{"tableRow", len(m[1]), m[2], m, Pos{row, col}}, true
	}
	return nilToken, false
}

func (d *Document) parseTable(i int, parentStop stopFn) (int, Node) {
	rawRows, separatorIndices, start := [][]string{}, []int{}, i
	startPoss := [][]Pos{}
	endPoss := [][]Pos{}
	for ; !parentStop(d, i); i++ {
		if t := d.tokens[i]; t.kind == "tableRow" {
			rawRow := strings.FieldsFunc(d.tokens[i].content, func(r rune) bool { return r == '|' })
			startPos := d.tokens[i].pos
			endPos := Pos{Row: startPos.Row, Col: startPos.Col}
			curStartPos := []Pos{}
			curEndPos := []Pos{}
			for i := range rawRow {
				startPos = endPos
				endPos = Pos{Row: startPos.Row, Col: startPos.Col + len(rawRow[i])}
				rawRow[i] = strings.TrimSpace(rawRow[i])
				curStartPos = append(curStartPos, startPos)
				curEndPos = append(curEndPos, endPos)
			}
			rawRows = append(rawRows, rawRow)
			startPoss = append(startPoss, curStartPos)
			endPoss = append(endPoss, curEndPos)
		} else if t.kind == "tableSeparator" {
			separatorIndices = append(separatorIndices, i-start)
			rawRows = append(rawRows, nil)
		} else {
			break
		}
	}

	table := Table{nil, getColumnInfos(rawRows), separatorIndices, d.tokens[start].Pos()}
	for r, rawColumns := range rawRows {
		row := Row{nil, isSpecialRow(rawColumns)}
		starts := startPoss[r]
		ends := endPoss[r]
		if len(rawColumns) != 0 {
			for i := range table.ColumnInfos {
				column := Column{starts[i], ends[i], nil, &table.ColumnInfos[i]}
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
	return self.Columns[len(self.Columns)-1].GetEnd()
}
func (self Row) GetPos() Pos {
	return self.Columns[0].GetPos()
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
	return self.Rows[len(self.Rows)-1].GetEnd()
}
