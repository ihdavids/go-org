package org

import (
	"reflect"
	"strings"
	"testing"
)

// Several macros on a line were read as one, and {{{name}}} was not a macro.
func TestMacroReferences(t *testing.T) {
	t.Run("two on a line", func(t *testing.T) {
		macros := findAll[Macro](parseString(t, "{{{a(1)}}} and {{{b(2)}}}\n").Nodes)
		if len(macros) != 2 || macros[0].Name != "a" || macros[1].Name != "b" {
			t.Fatalf("expected macros a and b, got %#v", macros)
		}
		if !reflect.DeepEqual(macros[0].Parameters, []string{"1"}) || !reflect.DeepEqual(macros[1].Parameters, []string{"2"}) {
			t.Errorf("unexpected parameters %v %v", macros[0].Parameters, macros[1].Parameters)
		}
		assertRoundTrip(t, "{{{a(1)}}} and {{{b(2)}}}\n")
	})

	t.Run("without arguments", func(t *testing.T) {
		m := findOne[Macro](t, parseString(t, "Today is {{{date}}}.\n").Nodes)
		if m.Name != "date" || m.Parameters != nil {
			t.Errorf("unexpected macro %#v", m)
		}
		assertRoundTrip(t, "Today is {{{date}}}.\n")
	})

	t.Run("empty argument list", func(t *testing.T) {
		m := findOne[Macro](t, parseString(t, "{{{m()}}}\n").Nodes)
		if !reflect.DeepEqual(m.Parameters, []string{""}) {
			t.Errorf("unexpected parameters %#v", m.Parameters)
		}
		assertRoundTrip(t, "{{{m()}}}\n")
	})

	t.Run("arguments with parentheses and escaped commas", func(t *testing.T) {
		m := findOne[Macro](t, parseString(t, "{{{m(f(x),a\\, b)}}}\n").Nodes)
		if want := []string{"f(x)", "a, b"}; !reflect.DeepEqual(m.Parameters, want) {
			t.Errorf("expected %q, got %q", want, m.Parameters)
		}
		assertRoundTrip(t, "{{{m(f(x),a\\, b)}}}\n")
	})

	t.Run("expanded in html", func(t *testing.T) {
		d := parseString(t, "#+MACRO: pair $1 and $2\n{{{pair(x,y)}}} {{{pair(1,2)}}}\n")
		html := writeHTML(t, d)
		assertContains(t, html, "x and y")
		assertContains(t, html, "1 and 2")
	})

	t.Run("built in macros", func(t *testing.T) {
		d := parseString(t, "#+TITLE: The Title\n#+AUTHOR: Someone\n#+FOO: bar\n{{{title}}} by {{{author}}}, {{{keyword(FOO)}}}; {{{results(=42=)}}}\n")
		html := writeHTML(t, d)
		assertContains(t, html, "<p>The Title by Someone, bar; ")
		assertContains(t, html, `<code class="verbatim">42</code>`)
	})

	t.Run("a defined macro wins over a built in", func(t *testing.T) {
		d := parseString(t, "#+TITLE: Ignored\n#+MACRO: title Mine\n{{{title}}}\n")
		assertContains(t, writeHTML(t, d), "Mine")
	})
}

// Inactive timestamps in text were not timestamps.
func TestInactiveTimestampInline(t *testing.T) {
	t.Run("parsed as a timestamp", func(t *testing.T) {
		input := "Met on [2026-10-01 Thu] and <2026-10-02 Fri>.\n"
		d := parseString(t, input)
		stamps := findAll[Timestamp](d.Nodes)
		if len(stamps) != 2 {
			t.Fatalf("expected two timestamps, got %s", dumpNodes(d.Nodes))
		}
		if stamps[0].Time.TimestampType != Inactive || stamps[0].Time.Start.Day() != 1 {
			t.Errorf("expected an inactive timestamp on the 1st, got %+v", stamps[0].Time)
		}
		if stamps[1].Time.TimestampType != Active || stamps[1].Time.Start.Day() != 2 {
			t.Errorf("expected an active timestamp on the 2nd, got %+v", stamps[1].Time)
		}
		assertRoundTrip(t, input)
		assertContains(t, writeHTML(t, d), `<span class="timestamp">&lsqb;2026-10-01 Thu&rsqb;</span>`)
	})

	t.Run("ends at its own bracket", func(t *testing.T) {
		d := parseString(t, "[2026-10-01 Thu] and [x]\n")
		stamp := findOne[Timestamp](t, d.Nodes)
		if stamp.EndPos.Col != len("[2026-10-01 Thu]") {
			t.Errorf("timestamp should end at its bracket, ends at %d", stamp.EndPos.Col)
		}
		assertContains(t, textOf(d.Nodes), " and [x]")
	})

	t.Run("with time and range", func(t *testing.T) {
		input := "[2026-10-01 Thu 10:00]--[2026-10-03 Sat 12:00]\n"
		stamp := findOne[Timestamp](t, parseString(t, input).Nodes)
		if stamp.Time.End.Day() != 3 {
			t.Errorf("expected a range ending on the 3rd, got %+v", stamp.Time)
		}
		assertRoundTrip(t, input)
	})

	t.Run("does not become the heading's timestamp", func(t *testing.T) {
		h := headline(t, parseString(t, "* H\nnoted [2026-10-01 Thu]\n"), 0)
		if h.Timestamp != nil {
			t.Errorf("an inactive timestamp is not the heading's, got %v", h.Timestamp)
		}
		h = headline(t, parseString(t, "* H\nmeet <2026-10-05 Mon> noted [2026-10-01 Thu]\n"), 0)
		if h.Timestamp == nil || h.Timestamp.Time.Start.Day() != 5 {
			t.Errorf("the active timestamp should be the heading's, got %v", h.Timestamp)
		}
	})

	t.Run("a stray bracket or angle does not reach a later timestamp", func(t *testing.T) {
		for _, input := range []string{"a < b and <2026-10-02 Fri>\n", "see [1] or [2026-10-02 Fri]\n"} {
			d := parseString(t, input)
			stamp := findOne[Timestamp](t, d.Nodes)
			if stamp.Time.Start.Day() != 2 || stamp.Pos.Col != strings.Index(input, "2026")-1 {
				t.Errorf("%q: timestamp parsed at the wrong place: %+v", input, stamp)
			}
			assertRoundTrip(t, input)
		}
	})

	t.Run("statistic cookie is not a timestamp", func(t *testing.T) {
		d := parseString(t, "- [2/3] done\n")
		if len(findAll[Timestamp](d.Nodes)) != 0 || len(findAll[StatisticToken](d.Nodes)) != 1 {
			t.Errorf("expected a statistic token, got %s", dumpNodes(d.Nodes))
		}
	})
}

// a_b and a^b were never subscript and superscript; they are now with ^:t.
func TestUnbracedSubSuperscripts(t *testing.T) {
	withT := "#+OPTIONS: ^:t\n"

	t.Run("off by default, so prose stays as written", func(t *testing.T) {
		d := parseString(t, "snake_case and my_file.txt and x^2\n")
		if n := len(findAll[Emphasis](d.Nodes)); n != 0 {
			t.Errorf("expected no sub or superscripts by default, got %s", dumpNodes(d.Nodes))
		}
		braced := findOne[Emphasis](t, parseString(t, "a_{b}\n").Nodes)
		if braced.Kind != "_{}" || braced.Unbraced {
			t.Errorf("braced subscripts are still read by default, got %#v", braced)
		}
	})

	cases := []struct {
		input, kind, content string
	}{
		{"H_2O", "_{}", "2O"},
		{"x^2", "^{}", "2"},
		{"e^-1", "^{}", "-1"},
		{"a_*", "_{}", "*"},
		{"v_1.5", "_{}", "1.5"},
		{"word_é", "_{}", "é"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			input := withT + c.input + "\n"
			e := findOne[Emphasis](t, parseString(t, input).Nodes)
			if e.Kind != c.kind || !e.Unbraced || textOf(e.Content) != c.content {
				t.Errorf("expected %s %q, got %s %q unbraced=%v", c.kind, c.content, e.Kind, textOf(e.Content), e.Unbraced)
			}
			assertRoundTrip(t, input)
		})
	}

	t.Run("trailing punctuation is not part of it", func(t *testing.T) {
		e := findOne[Emphasis](t, parseString(t, withT+"x_1, then\n").Nodes)
		if textOf(e.Content) != "1" {
			t.Errorf("expected 1, got %q", textOf(e.Content))
		}
	})

	t.Run("needs a character before it", func(t *testing.T) {
		d := parseString(t, withT+"a _b and ^c\n")
		if n := len(findAll[Emphasis](d.Nodes)); n != 0 {
			t.Errorf("expected nothing, got %s", dumpNodes(d.Nodes))
		}
	})

	t.Run("underline still wins after a space", func(t *testing.T) {
		e := findOne[Emphasis](t, parseString(t, withT+"an _underlined_ word\n").Nodes)
		if e.Kind != "_" {
			t.Errorf("expected underline, got %q", e.Kind)
		}
	})

	t.Run("html", func(t *testing.T) {
		assertContains(t, writeHTML(t, parseString(t, withT+"H_2O\n")), "H<sub>2O</sub>")
	})

	t.Run("^:nil turns off the braced form too", func(t *testing.T) {
		d := parseString(t, "#+OPTIONS: ^:nil\na_{b}\n")
		if n := len(findAll[Emphasis](d.Nodes)); n != 0 {
			t.Errorf("expected nothing with ^:nil, got %s", dumpNodes(d.Nodes))
		}
	})

	t.Run("DefaultSettings can turn it on", func(t *testing.T) {
		c := New().Silent()
		c.DefaultSettings["OPTIONS"] += " ^:t"
		e := findOne[Emphasis](t, parseWith(t, c, "x^2\n").Nodes)
		if !e.Unbraced {
			t.Errorf("expected an unbraced superscript")
		}
	})
}

// src_lang{...} took everything up to the last } on the line.
func TestInlineSrcBlock(t *testing.T) {
	t.Run("ends at its matching brace", func(t *testing.T) {
		input := "run src_python{1+1} and {x}\n"
		d := parseString(t, input)
		b := findOne[InlineBlock](t, d.Nodes)
		if b.Name != "src" || b.Parameters[0] != "python" || rawText(b.Children) != "1+1" {
			t.Errorf("unexpected block %#v", b)
		}
		assertContains(t, textOf(d.Nodes), " and {x}")
		assertRoundTrip(t, input)
	})

	t.Run("nested braces and bracketed headers", func(t *testing.T) {
		input := "src_sh[:var x=a[1]]{echo {a}} done\n"
		b := findOne[InlineBlock](t, parseString(t, input).Nodes)
		if rawText(b.Children) != "echo {a}" {
			t.Errorf("unexpected body %q", rawText(b.Children))
		}
		if want := []string{"sh", ":var", "x=a[1]"}; !reflect.DeepEqual(b.Parameters, want) {
			t.Errorf("expected parameters %v, got %v", want, b.Parameters)
		}
		assertRoundTrip(t, input)
	})

	t.Run("unclosed is text", func(t *testing.T) {
		d := parseString(t, "src_python{1+1\n")
		if len(findAll[InlineBlock](d.Nodes)) != 0 {
			t.Errorf("expected no inline block")
		}
	})
}

// call_name(args) was not parsed.
func TestInlineBabelCall(t *testing.T) {
	input := "Result: call_double[:session s](n=4)[:results raw] here\n"
	d := parseString(t, input)
	b := findOne[InlineBlock](t, d.Nodes)
	if want := []string{"double", ":session s", "n=4", ":results raw"}; b.Name != "call" || !reflect.DeepEqual(b.Parameters, want) {
		t.Errorf("expected call %v, got %s %v", want, b.Name, b.Parameters)
	}
	assertRoundTrip(t, input)
	html := writeHTML(t, d)
	assertNotContains(t, html, "call_")
	assertContains(t, html, "here")

	t.Run("simple", func(t *testing.T) {
		b := findOne[InlineBlock](t, parseString(t, "call_f()\n").Nodes)
		if want := []string{"f", "", "", ""}; !reflect.DeepEqual(b.Parameters, want) {
			t.Errorf("expected %v, got %v", want, b.Parameters)
		}
		assertRoundTrip(t, "call_f()\n")
	})

	t.Run("not inside a word", func(t *testing.T) {
		if len(findAll[InlineBlock](parseString(t, "recall_f(1)\n").Nodes)) != 0 {
			t.Errorf("recall_f is not a call")
		}
	})

	t.Run("needs arguments", func(t *testing.T) {
		if len(findAll[InlineBlock](parseString(t, "call_f and more\n").Nodes)) != 0 {
			t.Errorf("call_f without parentheses is not a call")
		}
	})
}

// An inline footnote ended at its first ], cutting off any link inside it.
func TestInlineFootnoteWithBrackets(t *testing.T) {
	input := "Text[fn::see [[https://x.org][x]] here] more\n"
	d := parseString(t, input)
	link := findOne[FootnoteLink](t, d.Nodes)
	if link.Definition == nil {
		t.Fatalf("expected an inline definition")
	}
	if l := findOne[RegularLink](t, link.Definition.Children); l.URL != "https://x.org" {
		t.Errorf("expected the link inside the footnote, got %q", l.URL)
	}
	if got := textOf(link.Definition.Children); got != "see x here" {
		t.Errorf("unexpected definition %q", got)
	}
	assertContains(t, textOf(d.Nodes), " more")
	assertRoundTrip(t, input)

	t.Run("named with definition", func(t *testing.T) {
		link := findOne[FootnoteLink](t, parseString(t, "a[fn:note:with [brackets]] b\n").Nodes)
		if link.Name != "note" || textOf(link.Definition.Children) != "with [brackets]" {
			t.Errorf("unexpected footnote %#v", link)
		}
	})

	t.Run("reference", func(t *testing.T) {
		link := findOne[FootnoteLink](t, parseString(t, "a[fn:1] b\n").Nodes)
		if link.Name != "1" || link.Definition != nil {
			t.Errorf("unexpected footnote %#v", link)
		}
	})

	t.Run("unclosed is text", func(t *testing.T) {
		if len(findAll[FootnoteLink](parseString(t, "a[fn::open [x] b\n").Nodes)) != 0 {
			t.Errorf("an unclosed footnote is not a footnote")
		}
	})
}

// <<targets>> and <<<radio targets>>> were not parsed.
func TestTargets(t *testing.T) {
	input := "See <<here>> and <<<radio>>> then [[here]].\n"
	d := parseString(t, input)
	target := findOne[Target](t, d.Nodes)
	radio := findOne[RadioTarget](t, d.Nodes)
	if target.Name != "here" || radio.Name != "radio" || textOf(radio.Children) != "radio" {
		t.Errorf("unexpected targets %#v %#v", target, radio)
	}
	if !d.Targets["here"] || !d.Targets["radio"] {
		t.Errorf("targets not recorded: %v", d.Targets)
	}
	assertRoundTrip(t, input)
	html := writeHTML(t, d)
	assertContains(t, html, `<a id="target-here"></a>`)
	assertContains(t, html, `<a id="target-radio">radio</a>`)
	assertContains(t, html, `<a href="#target-here">here</a>`)
	assertNotContains(t, html, "&lt;&lt;")

	t.Run("not targets", func(t *testing.T) {
		for _, input := range []string{"<< x>>\n", "<<x >>\n", "<<>>\n", "a << b >> c\n"} {
			d := parseString(t, input)
			if len(findAll[Target](d.Nodes))+len(findAll[RadioTarget](d.Nodes)) != 0 {
				t.Errorf("%q should not be a target", input)
			}
		}
	})

	t.Run("writer without the optional interfaces", func(t *testing.T) {
		out, err := parseString(t, input).Write(newPlainHTMLWriter())
		if err != nil {
			t.Fatal(err)
		}
		assertContains(t, out, "See  and radio then")
	})
}

// <protocol:path> was not a link.
func TestAngleLinks(t *testing.T) {
	input := "Visit <https://orgmode.org/manual/> or <mailto:a@b.org>.\n"
	d := parseString(t, input)
	links := findAll[RegularLink](d.Nodes)
	if len(links) != 2 || links[0].URL != "https://orgmode.org/manual/" || !links[0].AngleLink || links[1].Protocol != "mailto" {
		t.Fatalf("unexpected links %#v", links)
	}
	assertRoundTrip(t, input)
	html := writeHTML(t, d)
	assertContains(t, html, `<a href="https://orgmode.org/manual/">https://orgmode.org/manual/</a>`)
	assertNotContains(t, html, "&lt;https")

	t.Run("unknown protocol is text", func(t *testing.T) {
		if len(findAll[RegularLink](parseString(t, "<foo:bar>\n").Nodes)) != 0 {
			t.Errorf("<foo:bar> is not a link")
		}
	})

	t.Run("timestamp is still a timestamp", func(t *testing.T) {
		if len(findAll[Timestamp](parseString(t, "<2026-10-02 Fri>\n").Nodes)) != 1 {
			t.Errorf("expected a timestamp")
		}
	})
}

// Plain links were only http, https, ftp and file, and kept trailing punctuation.
func TestPlainLinks(t *testing.T) {
	cases := []struct{ input, url string }{
		{"write to mailto:someone@example.org today", "mailto:someone@example.org"},
		{"see doi:10.1000/182 for details", "doi:10.1000/182"},
		{"see https://orgmode.org.", "https://orgmode.org"},
		{"(see https://orgmode.org)", "https://orgmode.org"},
		{"https://en.wikipedia.org/wiki/Go_(game) is", "https://en.wikipedia.org/wiki/Go_(game)"},
		{"is it https://x.org/a?b=1, or not", "https://x.org/a?b=1"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			d := parseString(t, c.input+"\n")
			link := findOne[RegularLink](t, d.Nodes)
			if link.URL != c.url || !link.AutoLink {
				t.Errorf("expected %q, got %q", c.url, link.URL)
			}
			assertRoundTrip(t, c.input+"\n")
		})
	}

	t.Run("not links", func(t *testing.T) {
		for _, input := range []string{"note: this\n", "mailto: nobody\n", "xhttps://x.org\n"} {
			if n := len(findAll[RegularLink](parseString(t, input).Nodes)); n != 0 {
				t.Errorf("%q should have no link, found %d", input, n)
			}
		}
	})
}

// Escaped brackets in a link were not understood.
func TestEscapedLinkBrackets(t *testing.T) {
	input := "[[file:a\\]b.org][desc]]\n"
	d := parseString(t, input)
	link := findOne[RegularLink](t, d.Nodes)
	if link.URL != "file:a]b.org" || textOf(link.Description) != "desc" {
		t.Errorf("unexpected link %q %q", link.URL, textOf(link.Description))
	}
	assertRoundTrip(t, input)

	t.Run("balanced brackets need no escape", func(t *testing.T) {
		input := "[[https://x.org/?a[1]=2]]\n"
		link := findOne[RegularLink](t, parseString(t, input).Nodes)
		if link.URL != "https://x.org/?a[1]=2" {
			t.Errorf("unexpected url %q", link.URL)
		}
		assertRoundTrip(t, input)
	})

	t.Run("image as description", func(t *testing.T) {
		input := "[[https://x.org][[[file:img.png]]]]\n"
		links := findAll[RegularLink](parseString(t, input).Nodes)
		if len(links) != 2 || links[0].URL != "https://x.org" || links[1].URL != "file:img.png" {
			t.Errorf("expected the link and the image inside it, got %#v", links)
		}
		assertRoundTrip(t, input)
	})
}

// [/] and [%] were not statistic cookies.
func TestEmptyStatisticCookies(t *testing.T) {
	d := parseString(t, "- foo [/] and [%]\n")
	tokens := findAll[StatisticToken](d.Nodes)
	if len(tokens) != 2 || tokens[0].Content != "/" || tokens[1].Content != "%" {
		t.Errorf("expected two empty cookies, got %#v", tokens)
	}
	assertRoundTrip(t, "- foo [/] and [%]\n")
	assertContains(t, writeHTML(t, d), `<code class="statistic">[/]</code>`)
}
