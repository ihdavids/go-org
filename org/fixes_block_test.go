package org

import (
	"reflect"
	"strings"
	"testing"
)

// A block name with a hyphen was cut at the hyphen.
func TestBlockNamesWithHyphen(t *testing.T) {
	input := "#+BEGIN_MY-BLOCK param\n  some *text*\n#+END_MY-BLOCK\n"
	d := parseString(t, input)
	b := findOne[*Block](t, d.Nodes)
	if b.Name != "MY-BLOCK" || !reflect.DeepEqual(b.Parameters, []string{"param"}) {
		t.Errorf("unexpected block %q %v", b.Name, b.Parameters)
	}
	if len(findAll[Emphasis](b.Children)) != 1 {
		t.Errorf("a special block's content is org, got %s", dumpNodes(b.Children))
	}
	assertRoundTrip(t, input)
	assertContains(t, writeHTML(t, d), `<div class="my-block-block">`)

	t.Run("must be closed by the same name", func(t *testing.T) {
		d := parseString(t, "#+BEGIN_MY-BLOCK\nx\n#+END_MY\n")
		if len(findAll[*Block](d.Nodes)) != 0 {
			t.Errorf("#+END_MY does not close MY-BLOCK")
		}
	})
}

// The content of a COMMENT block was parsed as org, and exported.
func TestCommentBlockIsRaw(t *testing.T) {
	input := "#+BEGIN_COMMENT\n  * Not a headline\n  - not a list\n#+END_COMMENT\nafter\n"
	d := parseString(t, input)
	b := findOne[*Block](t, d.Nodes)
	if len(findAll[*Headline](d.Nodes)) != 0 || len(findAll[List](d.Nodes)) != 0 {
		t.Errorf("the content of a comment block is not org: %s", dumpNodes(d.Nodes))
	}
	if got := rawText(b.Children); got != "  * Not a headline\n  - not a list\n" {
		t.Errorf("unexpected content %q", got)
	}
	html := writeHTML(t, d)
	assertNotContains(t, html, "Not a headline")
	assertContains(t, html, "after")
	assertRoundTrip(t, input)
}

// #+RESULTS[hash]: was not a result.
func TestResultsWithHash(t *testing.T) {
	input := "#+BEGIN_SRC sh\n  echo hi\n#+END_SRC\n\n#+RESULTS[a1b2c3]:\n: hi\n"
	d := parseString(t, input)
	b := findOne[*Block](t, d.Nodes)
	result, ok := b.Result.(Result)
	if !ok {
		t.Fatalf("expected the result to be attached to the block, got %s", dumpNodes(d.Nodes))
	}
	if result.Hash != "a1b2c3" || textOf([]Node{result.Node}) != "hi" {
		t.Errorf("unexpected result %#v", result)
	}
	assertRoundTrip(t, input)
	assertNotContains(t, writeHTML(t, d), "RESULTS")

	t.Run("named result", func(t *testing.T) {
		input := "#+RESULTS: answer\n: 42\n"
		result := findOne[Result](t, parseString(t, input).Nodes)
		if result.Value != "answer" || result.Hash != "" {
			t.Errorf("unexpected result %#v", result)
		}
		assertRoundTrip(t, input)
	})
}

// Switches were stuck to the language: the language of
// `#+BEGIN_SRC python -n :results output` was "python -n".
func TestSrcBlockSwitches(t *testing.T) {
	cases := []struct {
		input      string
		parameters []string
		switches   []string
	}{
		{"#+BEGIN_SRC python -n :results output\n  x\n#+END_SRC\n", []string{"python", ":results", "output"}, []string{"-n"}},
		{"#+BEGIN_SRC emacs-lisp -n 10 -r -l \"(ref:%s)\"\n  x\n#+END_SRC\n", []string{"emacs-lisp"}, []string{"-n", "10", "-r", "-l", "\"(ref:%s)\""}},
		{"#+BEGIN_SRC python :results output\n  x\n#+END_SRC\n", []string{"python", ":results", "output"}, nil},
		{"#+BEGIN_SRC python\n  x\n#+END_SRC\n", []string{"python"}, nil},
		{"#+BEGIN_EXAMPLE -n\n  x\n#+END_EXAMPLE\n", []string{}, []string{"-n"}},
		{"#+BEGIN_SRC python +n :var a=1\n  x\n#+END_SRC\n", []string{"python", ":var", "a=1"}, []string{"+n"}},
	}
	for _, c := range cases {
		t.Run(strings.SplitN(c.input, "\n", 2)[0], func(t *testing.T) {
			b := findOne[*Block](t, parseString(t, c.input).Nodes)
			if !reflect.DeepEqual(b.Parameters, c.parameters) || !reflect.DeepEqual(b.Switches, c.switches) {
				t.Errorf("expected parameters %q switches %q, got %q %q", c.parameters, c.switches, b.Parameters, b.Switches)
			}
			assertRoundTrip(t, c.input)
		})
	}

	t.Run("html uses the bare language", func(t *testing.T) {
		html := writeHTML(t, parseString(t, cases[0].input))
		assertContains(t, html, `<div class="src src-python">`)
	})

	t.Run("parameter map", func(t *testing.T) {
		b := findOne[*Block](t, parseString(t, cases[0].input).Nodes)
		if m := b.ParameterMap(); m[":lang"] != "python" || m[":results"] != "output" {
			t.Errorf("unexpected parameter map %v", m)
		}
	})
}

// Empty cells were dropped, moving every cell after them a column left.
func TestTableEmptyCells(t *testing.T) {
	input := "| a || c |\n| 1 | 2 | 3 |\n"
	d := parseString(t, input)
	table := findOne[*Table](t, d.Nodes)
	if len(table.ColumnInfos) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.ColumnInfos))
	}
	first := table.Rows[0].Columns
	if textOf(first[0].Children) != "a" || len(first[1].Children) != 0 || textOf(first[2].Children) != "c" {
		t.Errorf("expected a, empty, c - got %q %q %q", textOf(first[0].Children), textOf(first[1].Children), textOf(first[2].Children))
	}
	if table.GetVal(1, 3) != "c" || table.GetVal(1, 2) != "" {
		t.Errorf("cells are in the wrong columns: %q %q", table.GetVal(1, 2), table.GetVal(1, 3))
	}
	html := writeHTML(t, parseString(t, "| a || c |\n| d | e | f |\n"))
	assertContains(t, html, "<td>a</td>\n<td></td>\n<td>c</td>")

	t.Run("empty last cell", func(t *testing.T) {
		table := findOne[*Table](t, parseString(t, "| a ||\n").Nodes)
		if len(table.ColumnInfos) != 2 {
			t.Errorf("expected 2 columns, got %d", len(table.ColumnInfos))
		}
	})

	t.Run("space after the last bar is not a column", func(t *testing.T) {
		table := findOne[*Table](t, parseString(t, "| a | b |   \n").Nodes)
		if len(table.ColumnInfos) != 2 {
			t.Errorf("expected 2 columns, got %d", len(table.ColumnInfos))
		}
	})

	t.Run("cell positions", func(t *testing.T) {
		table := findOne[*Table](t, parseString(t, "| a || c |\n").Nodes)
		cols := table.Rows[0].Columns
		if cols[0].Pos.Col != 1 || cols[1].Pos.Col != 5 || cols[2].Pos.Col != 6 {
			t.Errorf("unexpected cell positions %v %v %v", cols[0].Pos, cols[1].Pos, cols[2].Pos)
		}
	})

	t.Run("bar inside verbatim splits the cell, as in org", func(t *testing.T) {
		table := findOne[*Table](t, parseString(t, "| =a|b= | c |\n").Nodes)
		if len(table.ColumnInfos) != 3 {
			t.Errorf("org splits on every bar - write \\vert for one in a cell; got %d columns", len(table.ColumnInfos))
		}
		assertContains(t, writeHTML(t, parseString(t, "| a\\vert{}b |\n")), "<td>a|b</td>")
	})
}

// table.el tables were not supported, and their borders read as strikethrough.
func TestTableEl(t *testing.T) {
	input := "+-----+-----+\n| a   | b   |\n+=====+=====+\n| 1   | 2   |\n| one | two |\n+-----+-----+\n"

	t.Run("parsed as one table", func(t *testing.T) {
		d := parseString(t, "before\n\n"+input+"\nafter\n")
		table := findOne[TableEl](t, d.Nodes)
		if len(table.Lines) != 6 {
			t.Errorf("expected 6 lines, got %d", len(table.Lines))
		}
		if len(findAll[Emphasis](d.Nodes)) != 0 || len(findAll[*Table](d.Nodes)) != 0 {
			t.Errorf("a table.el table is neither strikethrough nor an org table: %s", dumpNodes(d.Nodes))
		}
	})

	t.Run("cells", func(t *testing.T) {
		rows, header, ok := findOne[TableEl](t, parseString(t, input).Nodes).Cells()
		want := [][]string{{"a", "b"}, {"1 one", "2 two"}}
		if !ok || header != 1 || !reflect.DeepEqual(rows, want) {
			t.Errorf("expected %v with 1 header row, got %v %d %v", want, rows, header, ok)
		}
	})

	t.Run("html table", func(t *testing.T) {
		html := writeHTML(t, parseString(t, input))
		assertContains(t, html, `<table class="table-el">`)
		assertContains(t, html, "<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>")
		assertContains(t, html, "<td>1 one</td>")
	})

	t.Run("irregular table is shown as drawn", func(t *testing.T) {
		irregular := "+-----+-----+\n| spans     |\n+-----+-----+\n"
		html := writeHTML(t, parseString(t, irregular))
		assertContains(t, html, `<pre class="table-el">`)
		assertContains(t, html, "| spans     |")
	})

	t.Run("round trip", func(t *testing.T) {
		assertRoundTrip(t, input)
		assertRoundTrip(t, "* H\n  +---+\n  | x |\n  +---+\n")
	})

	t.Run("indented under a heading", func(t *testing.T) {
		d := parseString(t, "* H\n  +---+\n  | x |\n  +---+\n")
		table := findOne[TableEl](t, d.Nodes)
		if table.Lines[1] != "| x |" {
			t.Errorf("lines should not keep the table's indentation, got %q", table.Lines[1])
		}
	})

	t.Run("writer without TableElWriter shows it as an example", func(t *testing.T) {
		out, err := parseString(t, input).Write(newPlainHTMLWriter())
		if err != nil {
			t.Fatal(err)
		}
		assertContains(t, out, "<pre class=\"example\">\n+-----+-----+")
	})
}

// A LaTeX environment was cut at its first blank line.
func TestLatexEnvironmentAcrossBlankLines(t *testing.T) {
	input := "\\begin{align*}\na &= 1 \\\\\n\nb &= 2\n- c\n\\end{align*}\n"
	d := parseString(t, input)
	fragment := findOne[LatexFragment](t, d.Nodes)
	if fragment.OpeningPair != "\\begin{align*}" || fragment.ClosingPair != "\\end{align*}" {
		t.Errorf("unexpected pairs %q %q", fragment.OpeningPair, fragment.ClosingPair)
	}
	if got := rawText(fragment.Content); got != "\na &= 1 \\\\\n\nb &= 2\n- c\n" {
		t.Errorf("unexpected content %q", got)
	}
	if len(findAll[List](d.Nodes)) != 0 {
		t.Errorf("a line inside the environment is not a list")
	}
	assertRoundTrip(t, input)
	assertContains(t, writeHTML(t, d), "\\begin{align*}\na &amp;= 1 \\\\\n\nb &amp;= 2\n- c\n\\end{align*}")

	t.Run("text after the environment is its own paragraph", func(t *testing.T) {
		d := parseString(t, "\\begin{equation}\nx\n\n\\end{equation}\nafter\n")
		if n := len(findAll[Paragraph](d.Nodes)); n != 2 {
			t.Errorf("expected 2 paragraphs, got %d: %s", n, dumpNodes(d.Nodes))
		}
	})

	t.Run("unclosed environment is ordinary text", func(t *testing.T) {
		d := parseString(t, "\\begin{equation}\nx\n\ny\n")
		if len(findAll[LatexFragment](d.Nodes)) != 0 {
			t.Errorf("expected no fragment")
		}
		if n := len(findAll[Paragraph](d.Nodes)); n < 2 {
			t.Errorf("expected separate paragraphs, got %d", n)
		}
	})

	t.Run("closed by its own name", func(t *testing.T) {
		d := parseString(t, "\\begin{a}\n\\begin{b}\n\n\\end{b}\n\\end{a}\n")
		fragment := findOne[LatexFragment](t, d.Nodes)
		if fragment.ClosingPair != "\\end{a}" {
			t.Errorf("expected the environment to close at \\end{a}, got %q", fragment.ClosingPair)
		}
	})

	t.Run("does not run past a heading", func(t *testing.T) {
		d := parseString(t, "* A\n\\begin{x}\n\n* B\n\\end{x}\n")
		if len(findAll[*Headline](d.Nodes)) != 2 {
			t.Errorf("both headings should survive: %s", dumpNodes(d.Nodes))
		}
	})
}

// A line over 64KB made the whole parse fail.
func TestLongLines(t *testing.T) {
	long := strings.Repeat("a", 200*1024)
	d := parseString(t, "before\n\n"+long+"\n\nafter\n")
	if got := textOf(d.Nodes); !strings.Contains(got, long) || !strings.Contains(got, "after") {
		t.Errorf("the long line was not read in full")
	}
}
