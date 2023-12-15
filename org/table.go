package org

import (
	"bytes"
	"errors"
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
	Cur              RowColRef
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

	table := &Table{nil, getColumnInfos(rawRows), separatorIndices, d.tokens[start].Pos(), nil, RowColRef{1, 1, false, false}}
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

func (s *Table) IsSeparatorRow(row int) bool {
	for _, v := range s.SeparatorIndices {
		if v == row {
			return true
		}
	}
	return false
}

func (s *Table) GetRealRowCol(row, col int) (int, int) {
	specialCount := 0
	for i, _ := range s.Rows {
		if s.IsSeparatorRow(i) {
			specialCount += 1
		}
		if (i + 1) == (row + specialCount) {
			return i, (col - 1)
		}
	}
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

func (s *Table) GetValRef(r *RowColRef) string {
	row := r.Row
	col := r.Col
	if r.RelativeRow {
		row = s.Cur.Row - row
	}
	if r.RelativeCol {
		col = s.Cur.Col - col
	}
	return s.GetVal(row, col)
}

func (s *Table) GetWidth() int {
	h := s.GetHeight()
	if h > 0 {
		return len(s.Rows[0].Columns)
	}
	return 0
}

func (s *Table) GetHeight() int {
	if s == nil {
		return 0
	}
	return len(s.Rows)
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

func (s *Table) ClampRow(r int) int {
	return ClampToMinMax(r, s.GetHeight())
}

func (s *Table) ClampCol(r int) int {
	return ClampToMinMax(r, s.GetWidth())
}

func (s *Table) CurrentRow() int {
	return s.Cur.Row
}

func (s *Table) CurrentCol() int {
	return s.Cur.Col
}

func (s *Table) RecomputeColumnInfos() {
	w := OrgWriter{}
	for r := 0; r < len(s.Rows); r++ {
		row := s.Rows[r]
		for c := 0; c < len(row.Columns); c++ {
			col := row.Columns[c]
			v := w.WriteNodesAsString(col.Children...)
			l := utf8.RuneCountInString(v)
			if l > s.ColumnInfos[c].Len {
				s.ColumnInfos[c].Len = l
			}
		}
	}
}

//////////////////// FORMULA MANAGEMENT //////////////////////////////////////////////

type Formula struct {
	Keyword         *Keyword
	FormulaStr      string
	SubKeywordIndex int
	Target          *FormulaTarget
	Expr            string
	Format          string
	Valid           bool
}

type RowColRef struct {
	Row         int
	Col         int
	RelativeCol bool
	RelativeRow bool
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

// numeral describes the value and symbol of a single roman numeral
type numeral struct {
	val int
	sym []byte
}

var (
	// InvalidRomanNumeral - error for when a roman numeral string provided is not a valid roman numeral
	InvalidRomanNumeral = errors.New("invalid roman numeral")
	// IntegerOutOfBounds - error for when the integer provided is invalid and unable to be converted to a roman numeral
	IntegerOutOfBounds = errors.New("integer must be between 1 and 3999")

	// all unique numerals ordered from largest to smallest
	nums = []numeral{
		{1000, []byte("M")},
		{900, []byte("CM")},
		{500, []byte("D")},
		{400, []byte("CD")},
		{100, []byte("C")},
		{90, []byte("XC")},
		{50, []byte("L")},
		{40, []byte("XL")},
		{10, []byte("X")},
		{9, []byte("IX")},
		{5, []byte("V")},
		{4, []byte("IV")},
		{1, []byte("I")},
	}

	// lookup arrays used for converting from an int to a roman numeral extremely quickly.
	// method from here: https://rosettacode.org/wiki/Roman_numerals/Encode#Go
	r0 = []string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX"}
	r1 = []string{"", "X", "XX", "XXX", "XL", "L", "LX", "LXX", "LXXX", "XC"}
	r2 = []string{"", "C", "CC", "CCC", "CD", "D", "DC", "DCC", "DCCC", "CM"}
	r3 = []string{"", "M", "MM", "MMM"}
)

// IntToString converts an integer value to a roman numeral string. An error is
// returned if the integer is not between 1 and 3999.
func RomanIntToString(input int) (string, error) {
	if romanOutOfBounds(input) {
		return "", IntegerOutOfBounds
	}
	return romanIntToRoman(input), nil
}

// IntToBytes converts an integer value to a roman numeral byte array. An error is
// returned if the integer is not between 1 and 3999.
func RomanIntToBytes(input int) ([]byte, error) {
	str, err := RomanIntToString(input)
	return []byte(str), err
}

// outOfBounds checks to ensure an input value is valid for roman numerals without the need of
// vinculum (used for values of 4,000 and greater)
func romanOutOfBounds(input int) bool {
	return input < 1 || input > 3999
}

func romanIntToRoman(n int) string {
	// This is efficient in Go. The 4 operands are evaluated,
	// then a single allocation is made of the exact size needed for the result.
	return r3[n%1e4/1e3] + r2[n%1e3/1e2] + r1[n%100/10] + r0[n%10]
}

// StringToInt converts a roman numeral string to an integer. Roman numerals for numbers
// outside of the range 1 to 3,999 will return an error. Empty strings will return 0
// with no error thrown.
func RomanStringToInt(input string) (int, error) {
	return RomanBytesToInt([]byte(input))
}

// BytesToInt converts a roman numeral byte array to an integer. Roman numerals for numbers
// outside of the range 1 to 3,999 will return an error. Nil or empty []byte will return 0
// with no error thrown.
func RomanBytesToInt(input []byte) (int, error) {
	if input == nil || len(input) == 0 {
		return 0, nil
	}
	if output, ok := romanToInt(input); ok {
		return output, nil
	}
	return 0, InvalidRomanNumeral
}

func romanToInt(input []byte) (int, bool) {
	var output int
	for _, n := range nums {
		for bytes.HasPrefix(input, n.sym) {
			output += n.val
			input = input[len(n.sym):]
		}
	}
	// if we are still left with input string values then the
	// input was invalid and the bool is returned as false
	return output, len(input) == 0
}

func ConvertRomanNumeral(v string) int {
	if r, e := RomanStringToInt(v); e == nil {
		return r
	}
	return 1
}

func GetRow(v string, tbl *Table) (int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 1, false
	} else if v[0] == '<' {
		return tbl.ClampRow(len(v)), false
	} else if v[0] == '>' {
		return tbl.ClampRow(tbl.GetHeight() - len(tbl.SeparatorIndices) - (len(v) - 1)), false
	} else if v[0] == 'I' || v[0] == 'V' || v[0] == 'X' {
		r := ConvertRomanNumeral(v)
		if r >= 1 && r <= len(tbl.SeparatorIndices) {
			r -= 1
			return tbl.ClampRow(tbl.SeparatorIndices[r]), false
		}
	}
	if r, err := strconv.Atoi(v); err == nil {
		rel := false
		if r <= 0 {
			rel = true
			r = -r
		}
		return r, rel
	}
	return 1, false
}

func GetCol(v string, tbl *Table) (int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 1, false
	}
	if v[0] == '<' {
		return tbl.ClampCol(len(v)), false
	}
	if v[0] == '>' {
		return tbl.ClampCol(tbl.GetHeight() - (len(v) - 1)), false
	}
	if r, err := strconv.Atoi(v); err == nil {
		rel := false
		if r <= 0 {
			rel = true
			r = -r
		}
		return r, rel
	}
	return 1, false
}

func MakeRowColDef(s string, tbl *Table) RowColRef {
	r := RowColRef{}
	rel := false
	if m := tableTargetRe.FindStringSubmatch(s); m != nil {
		if m[3] != "" {
			r.Row, rel = GetRow(m[3], tbl)
			r.Col = -1
			r.RelativeRow = rel
		} else if m[5] != "" {
			r.Row = -1
			r.Col, rel = GetCol(m[5], tbl)
			r.RelativeCol = rel
		} else {
			relc := false
			r.Row, rel = GetRow(m[7], tbl)
			r.Col, relc = GetCol(m[8], tbl)
			r.RelativeCol = relc
			r.RelativeRow = rel
		}
	}
	return r
}

func (s *FormulaTarget) Process(tbl *Table) {
	if s.Raw != "" {
		temp := strings.Split(s.Raw, "..")
		if len(temp) == 1 {
			s.Start = MakeRowColDef(temp[0], tbl)
			s.End = s.Start
		} else {
			s.Start = MakeRowColDef(temp[0], tbl)
			s.End = MakeRowColDef(temp[1], tbl)
		}
	}
}

func MakeRowColDefParsed(rin, cin string, tbl *Table) RowColRef {
	r := RowColRef{}
	rel := false
	if rin != "" && cin == "" {
		r.Row, rel = GetRow(rin, tbl)
		r.Col = -1
		r.RelativeRow = rel
	} else if cin != "" && rin == "" {
		r.Row = -1
		r.Col, rel = GetCol(cin, tbl)
		r.RelativeCol = rel
	} else {
		relc := false
		r.Row, rel = GetRow(rin, tbl)
		r.Col, relc = GetCol(cin, tbl)
		r.RelativeRow = rel
		r.RelativeCol = relc
	}
	return r
}

func MakeFormulaTarget(r1, c1, r2, c2 string, tbl *Table) *FormulaTarget {
	s := &FormulaTarget{}
	if r2 == "" && c2 == "" {
		s.Start = MakeRowColDefParsed(r1, c1, tbl)
		s.End = s.Start
	} else {
		s.Start = MakeRowColDefParsed(r1, c1, tbl)
		s.End = MakeRowColDefParsed(r2, c2, tbl)
	}
	return s
}

// Change in row
func IsRowRange(s, e *RowColRef) bool {
	return s.IsEntireCol() || (s.Row != e.Row && s.Col == e.Col)
}

// Change in col
func IsColRange(s, e *RowColRef) bool {
	return s.IsEntireRow() || (s.Row == e.Row && s.Col != e.Col)
}

func (s *RowColRef) IsEntireRow() bool {
	return s.Col == -1
}

func (s *RowColRef) IsEntireCol() bool {
	return s.Row == -1
}

func IsPositiveRange(s, e *RowColRef) bool {
	if IsColRange(s, e) && !s.IsEntireCol() {
		return s.Col <= e.Col
	}
	if IsRowRange(s, e) && !s.IsEntireRow() {
		return s.Row <= e.Row
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

type ColRefIterator func() *RowColRef

func FixupPos(tbl *Table, p *RowColRef) *RowColRef {
	r := RowColRef{Row: p.Row, Col: p.Col, RelativeCol: p.RelativeCol, RelativeRow: p.RelativeRow}
	if r.RelativeCol {
		r.Col = ClampToMinMax(tbl.Cur.Col-r.Col, tbl.GetWidth())
	}
	if r.RelativeRow {
		r.Row = ClampToMinMax(tbl.Cur.Row-r.Row, tbl.GetHeight())
	}
	return &r
}

// This bugbear returns an iterator that can iterate over a range given a table.
func (s *FormulaTarget) CreateIterator(tbl *Table) ColRefIterator {
	if tbl == nil {
		return func() *RowColRef {
			return nil
		}
	}
	cur := &RowColRef{Row: s.Start.Row, Col: s.End.Col}
	start := FixupPos(tbl, &s.Start)
	end := FixupPos(tbl, &s.End)

	maxRows := tbl.GetHeight()
	maxCols := tbl.GetWidth()
	// Change in col (IE along a row)
	if IsColRange(start, end) {
		if IsPositiveRange(start, end) {
			sv := ClampToMinMax(start.Col, maxCols)
			if !start.IsEntireRow() {
				maxCols = ClampToMinMax(end.Col, maxCols)
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
			sv := ClampToMinMax(start.Col, maxCols)
			if !start.IsEntireRow() {
				minCols = ClampToMinMax(end.Col, maxCols)
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
	} else if IsRowRange(start, end) {
		if IsPositiveRange(start, end) {
			sv := ClampToMinMax(start.Row, maxRows)
			if !start.IsEntireCol() {
				maxRows = ClampToMinMax(end.Row, maxRows)
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
			sv := ClampToMinMax(start.Row, maxRows)
			if !start.IsEntireCol() {
				minRows = ClampToMinMax(end.Row, maxRows)
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
	} else {
		if IsPositiveRange(start, end) {
			sr := ClampToMinMax(start.Row, maxRows)
			sc := ClampToMinMax(start.Col, maxCols)
			er := ClampToMinMax(end.Row, maxRows)
			ec := ClampToMinMax(end.Col, maxCols)
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
			sr := ClampToMinMax(start.Row, maxRows)
			sc := ClampToMinMax(start.Col, maxCols)
			er := ClampToMinMax(end.Row, maxRows)
			ec := ClampToMinMax(end.Col, maxCols)
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

func (s *Formula) Process(tbl *Table) {
	// Do we need to validate the formula?
	if tgt, expr, found := strings.Cut(s.FormulaStr, "="); found {
		s.Valid = true
		s.Target = &FormulaTarget{Raw: strings.TrimSpace(tgt)}
		s.Target.Process(tbl)
		// This might not be good enough! May need to be smarter than this.
		if frmt := strings.LastIndex(expr, ";"); frmt != -1 {
			s.Expr = expr[:frmt]
			s.Format = expr[frmt+1:]
		} else {
			s.Expr = expr
			s.Format = ""
		}
	}
}

func (s *Formulas) Process(tbl *Table) {
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
				frm.Process(tbl)
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
