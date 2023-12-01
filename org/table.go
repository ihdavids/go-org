package org

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Formulas struct {
	Keywords []*Keyword
	Formulas []*Formula
}
type Table struct {
	Rows             []*Row
	ColumnInfos      []ColumnInfo
	SeparatorIndices []int
	Pos              Pos
	Formulas         *Formulas
}

type Row struct {
	Columns   []*Column
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

	table := &Table{nil, getColumnInfos(rawRows), separatorIndices, d.tokens[start].Pos(), nil}
	var starts []Pos
	var ends []Pos
	for r, rawColumns := range rawRows {
		row := &Row{nil, isSpecialRow(rawColumns)}
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
				column := &Column{s, e, nil, &table.ColumnInfos[i]}
				if i < len(rawColumns) {
					column.Children = d.parseInline(rawColumns[i], start) // TODO: This is off by the row index
				}
				row.Columns = append(row.Columns, column)
			}
		}
		table.Rows = append(table.Rows, row)
	}
	ch := d.currentHeadline.Get()
	if ch != nil {
		ch.Tables = append(ch.Tables, table)
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

func (s *Table) GetRealRowCol(row, col int) (int, int) {
	specialCount := 0
	for i, r := range s.Rows {
		fmt.Printf("BB: %d==%d\n", (i + 1), (row + specialCount))
		if r.IsSpecial == true {
			specialCount += 1
		}
		fmt.Printf("VV: %d==%d\n", (i + 1), (row + specialCount))
		if (i + 1) == (row + specialCount) {
			return i, (col - 1)
		}
	}
	fmt.Printf("NO MATCH\n")
	return -1, -1
}

func (s *Table) SetVal(row, col int, val string) {
	if s == nil {
		return
	}
	row, col = s.GetRealRowCol(row, col)
	if col >= 0 && row >= 0 && row < len(s.Rows) {
		if col < len(s.Rows[row].Columns) {
			s.Rows[row].Columns[col].Children = []Node{Text{Content: val}}
		}
	}
}

func (s *Table) SetValRef(r *RowColRef, v string) {
	if s == nil {
		return
	}
	s.SetVal(r.Row, r.Col, v)
}

func (s *Table) GetVal(row, col int) string {
	if s == nil {
		return ""
	}
	row, col = s.GetRealRowCol(row, col)
	if col >= 0 && row >= 0 && row < len(s.Rows) {
		if col < len(s.Rows[row].Columns) {
			w := OrgWriter{}
			return w.WriteNodesAsString(s.Rows[row].Columns[col].Children...)
		}
	}
	return ""
}

//////////////////// FORMULA MANAGEMENT //////////////////////////////////////////////

type Formula struct {
	Keyword         *Keyword
	FormulaStr      string
	SubKeywordIndex int
	Target          *FormulaTarget
	Expr            string
	Valid           bool
}

type RowColRef struct {
	Row int
	Col int
}

type FormulaTarget struct {
	Raw   string
	Start RowColRef
	End   RowColRef
}

func (s *Formulas) AppendKeyword(k *Keyword) {
	if k != nil {
		s.Keywords = append(s.Keywords, k)
	}
}

// Tricky nesting here, watch out for indexs
var tableTargetRe = regexp.MustCompile(`\s*(([@](?P<rowonly>[-]?[0-9><]+))|([$](?P<colonly>[-]?[0-9><]+))|([@](?P<row>[-]?[0-9><]+)[$](?P<col>[-]?[0-9><]+)))\s*$`)

func MakeRowColDef(s string) RowColRef {
	r := RowColRef{}
	if m := tableTargetRe.FindStringSubmatch(s); m != nil {
		if m[3] != "" {
			r.Row, _ = strconv.Atoi(m[3])
			r.Col = -1
		} else if m[5] != "" {
			r.Row = -1
			r.Col, _ = strconv.Atoi(m[5])
		} else {
			r.Row, _ = strconv.Atoi(m[7])
			r.Col, _ = strconv.Atoi(m[8])
		}
	}
	return r
}

func (s *FormulaTarget) Process() {
	if s.Raw != "" {
		temp := strings.Split(s.Raw, "..")
		if len(temp) == 1 {
			s.Start = MakeRowColDef(temp[0])
			s.End = s.Start
		} else {
			s.Start = MakeRowColDef(temp[0])
			s.End = MakeRowColDef(temp[1])
		}
	}
}

func (s *FormulaTarget) IsRowRange() bool {
	return s.IsEntireRow() || (s.Start.Row != s.End.Row && s.Start.Col == s.End.Col)
}

func (s *FormulaTarget) IsColRange() bool {
	return s.IsEntireCol() || (s.Start.Row == s.End.Row && s.Start.Col != s.End.Col)
}

func (s *FormulaTarget) IsEntireRow() bool {
	return s.Start.Col == -1
}

func (s *FormulaTarget) IsEntireCol() bool {
	return s.Start.Row == -1
}

func (s *FormulaTarget) IsPositiveRange() bool {
	if s.IsColRange() && !s.IsEntireCol() {
		return s.Start.Col <= s.End.Col
	}
	if s.IsRowRange() && !s.IsEntireRow() {
		return s.Start.Row <= s.End.Row
	}
	return true
}

func CreatePosIterator(rs, re int) func() int {
	strt := rs - 1
	end := re
	return func() int {
		if strt >= end {
			return -1
		}
		strt += 1
		return strt
	}
}

func CreateNegIterator(rs, re int) func() int {
	strt := rs + 1
	end := re
	return func() int {
		if strt <= end {
			return -1
		}
		strt -= 1
		return strt
	}
}

func ClampToMinMax(sr int, max int) int {
	if sr <= 0 {
		return 1
	}
	if sr > max {
		return max
	}
	return sr
}

// This bugbear returns an iterator that can iterate over a range given a table.
func (s *FormulaTarget) CreateIterator(tbl *Table) func() *RowColRef {
	if tbl == nil {
		return func() *RowColRef {
			return nil
		}
	}
	cur := &RowColRef{Row: s.Start.Row, Col: s.End.Col}
	maxRows := len(tbl.Rows)
	maxCols := 0
	if maxRows > 0 {
		maxCols = len(tbl.Rows[0].Columns)
	}
	if s.IsColRange() {
		if s.IsPositiveRange() {
			sv := ClampToMinMax(s.Start.Row, maxRows)
			if !s.IsEntireCol() {
				maxRows = ClampToMinMax(s.End.Row, maxRows)
			}
			it := CreatePosIterator(sv, maxRows)
			return func() *RowColRef {
				cur.Row = it()
				if cur.Row == -1 {
					return nil
				}
				return cur
			}
		} else {
			minRows := 1
			sv := ClampToMinMax(s.Start.Row, maxRows)
			if !s.IsEntireCol() {
				minRows = ClampToMinMax(s.End.Row, maxRows)
			}
			it := CreateNegIterator(sv, minRows)
			return func() *RowColRef {
				cur.Row = it()
				if cur.Row == -1 {
					return nil
				}
				return cur
			}
		}
	} else if s.IsRowRange() {
		if s.IsPositiveRange() {
			sv := ClampToMinMax(s.Start.Col, maxCols)
			if !s.IsEntireRow() {
				maxCols = ClampToMinMax(s.End.Col, maxCols)
			}
			it := CreatePosIterator(sv, maxCols)
			return func() *RowColRef {
				cur.Col = it()
				if cur.Col == -1 {
					return nil
				}
				return cur
			}
		} else {
			minCols := 1
			sv := ClampToMinMax(s.Start.Col, maxCols)
			if !s.IsEntireRow() {
				minCols = ClampToMinMax(s.End.Col, maxCols)
			}
			it := CreateNegIterator(sv, minCols)
			return func() *RowColRef {
				cur.Col = it()
				if cur.Col == -1 {
					return nil
				}
				return cur
			}
		}
	} else {
		if s.IsPositiveRange() {
			sr := ClampToMinMax(s.Start.Row, maxRows)
			sc := ClampToMinMax(s.Start.Col, maxCols)
			er := ClampToMinMax(s.End.Row, maxRows)
			ec := ClampToMinMax(s.End.Col, maxCols)
			rit := CreatePosIterator(sr, er)
			cit := CreatePosIterator(sc, ec)
			cur.Col = cit()
			return func() *RowColRef {
				cur.Row = rit()
				if cur.Row == -1 {
					cur.Col = cit()
					if cur.Col == -1 {
						return nil
					}
					rit = CreatePosIterator(sr, er)
					cur.Row = rit()
				}
				return cur
			}
		} else {
			sr := ClampToMinMax(s.Start.Row, maxRows)
			sc := ClampToMinMax(s.Start.Col, maxCols)
			er := ClampToMinMax(s.End.Row, maxRows)
			ec := ClampToMinMax(s.End.Col, maxCols)
			rit := CreateNegIterator(sr, er)
			cit := CreateNegIterator(sc, ec)
			return func() *RowColRef {
				cur.Row = rit()
				if cur.Row == -1 {
					cur.Col = cit()
					if cur.Col == -1 {
						return nil
					}
					rit = CreateNegIterator(sr, er)
					cur.Row = rit()
				}
				return cur
			}
		}
	}
}

func (s *Formula) Process() {
	tempStr := strings.Split(s.FormulaStr, "=")
	if len(tempStr) == 2 {
		s.Valid = true
		s.Target = &FormulaTarget{Raw: strings.TrimSpace(tempStr[0])}
		s.Target.Process()
		s.Expr = tempStr[1]
	}
}

func (s *Formulas) Process() {
	if len(s.Formulas) > 0 {
		return
	}
	// First build a giant list of all our known formulas
	frms := []*Formula{}
	for _, k := range s.Keywords {
		tempFormulas := strings.Split(k.Value, "::")
		for i, f := range tempFormulas {
			f = strings.TrimSpace(f)
			if f != "" {
				frm := &Formula{FormulaStr: f, Keyword: k, SubKeywordIndex: i}
				frm.Process()
				frms = append(frms, frm)
			}
		}
	}
	s.Formulas = frms
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

func (n Table) GetChildren() []Node { return nil }

// These are not actually nodes
func (n Row) GetChildren() []*Column { return n.Columns }
func (n Column) GetChildren() []Node { return n.Children }
