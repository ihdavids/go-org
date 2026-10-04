package org

import (
	"errors"
	"strings"
	"testing"
)

// #+INCLUDE: of an org file - the commonest form - panicked the whole parse,
// and the :lines, :minlevel and quote forms were not supported at all.
func TestIncludeOrgFile(t *testing.T) {
	files := map[string]string{
		"chapter.org": "* Chapter\nchapter text\n** Section\nsection text\n",
		"lines.txt":   "one\ntwo\nthree\nfour\nfive\n",
		"quote.org":   "a /quoted/ line\n",
	}

	t.Run("does not fail the parse", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"chapter.org\"\n")
		include := findOne[Include](t, d.Nodes)
		if include.GetEnd() != (Pos{0, len(`#+INCLUDE: "chapter.org"`)}) {
			t.Errorf("include should end at the end of its line, got %v", include.GetEnd())
		}
	})

	t.Run("resolves to the parsed org content", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"chapter.org\"\n")
		content, ok := findOne[Include](t, d.Nodes).Resolve().(IncludedContent)
		if !ok {
			t.Fatalf("expected IncludedContent")
		}
		headlines := findAll[*Headline](content.Nodes)
		if len(headlines) != 2 || headlines[0].Lvl != 1 || headlines[1].Lvl != 2 {
			t.Fatalf("expected the two included headlines, got %s", dumpNodes(content.Nodes))
		}
		html := writeHTML(t, d)
		assertContains(t, html, "chapter text")
		assertContains(t, html, "section text")
	})

	t.Run("unquoted file name", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: chapter.org\n")
		assertContains(t, writeHTML(t, d), "chapter text")
	})

	t.Run("minlevel shifts headlines keeping their depths", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"chapter.org\" :minlevel 3\n")
		content := findOne[Include](t, d.Nodes).Resolve().(IncludedContent)
		headlines := findAll[*Headline](content.Nodes)
		if headlines[0].Lvl != 3 || headlines[1].Lvl != 4 {
			t.Errorf("expected levels 3 and 4, got %d and %d", headlines[0].Lvl, headlines[1].Lvl)
		}
	})

	t.Run("lines range excludes its end, as in org", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"lines.txt\" example :lines \"2-4\"\n")
		block, ok := findOne[Include](t, d.Nodes).Resolve().(Block)
		if !ok {
			t.Fatalf("expected a Block")
		}
		if got := rawText(block.Children); got != "two\nthree\n" {
			t.Errorf("expected lines 2 and 3, got %q", got)
		}
	})

	t.Run("open ended line ranges", func(t *testing.T) {
		cases := map[string]string{"-3": "onetwo", "4-": "fourfive", "5": "five"}
		for spec, want := range cases {
			got := strings.ReplaceAll(selectLines(files["lines.txt"], spec), "\n", "")
			if got != want {
				t.Errorf(":lines %q: expected %q, got %q", spec, want, got)
			}
		}
	})

	t.Run("src include keeps its language", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"lines.txt\" src python\n")
		block := findOne[Include](t, d.Nodes).Resolve().(Block)
		if block.Name != "SRC" || len(block.Parameters) != 1 || block.Parameters[0] != "python" {
			t.Errorf("expected a python src block, got %s %v", block.Name, block.Parameters)
		}
	})

	t.Run("quote include is parsed as org inside a quote", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"quote.org\" quote\n")
		html := writeHTML(t, d)
		assertContains(t, html, "<blockquote>")
		assertContains(t, html, "<em>quoted</em>")
	})

	t.Run("search option includes one heading", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"chapter.org::*Section\"\n")
		html := writeHTML(t, d)
		assertContains(t, html, "section text")
		assertNotContains(t, html, "chapter text")
	})

	t.Run("include cycle is refused rather than looping", func(t *testing.T) {
		cyclic := map[string]string{"test.org": "#+INCLUDE: \"test.org\"\nbody\n"}
		d := parseWith(t, fakeFiles(cyclic), "#+INCLUDE: \"test.org\"\n")
		if _, ok := findOne[Include](t, d.Nodes).Resolve().(Keyword); !ok {
			t.Errorf("a file including itself should resolve to its keyword")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		d := parseWith(t, fakeFiles(files), "#+INCLUDE: \"nope.org\"\n")
		if _, ok := findOne[Include](t, d.Nodes).Resolve().(Keyword); !ok {
			t.Errorf("a missing file should resolve to its keyword")
		}
		writeHTML(t, d)
	})

	t.Run("round trip", func(t *testing.T) {
		assertRoundTrip(t, "#+INCLUDE: \"chapter.org\" :minlevel 2\n")
	})
}

// #+MACRO: kept only the first word of its definition.
func TestMacroDefinitionKeepsItsWholeBody(t *testing.T) {
	d := parseString(t, "#+MACRO: greet Hello there, $1!\n{{{greet(Bob)}}}\n")
	if got := d.Macros["greet"]; got != "Hello there, $1!" {
		t.Errorf("expected the whole definition, got %q", got)
	}
	assertContains(t, writeHTML(t, d), "Hello there, Bob!")

	d = parseString(t, "#+MACRO: empty\n")
	if got, ok := d.Macros["empty"]; !ok || got != "" {
		t.Errorf("a macro with no body should be defined as empty, got %q %v", got, ok)
	}
}

// #+CAPTION[short]: long was not recognised as a caption.
func TestShortCaption(t *testing.T) {
	input := "#+CAPTION[Short]: The long caption\n| a |\n"
	d := parseString(t, input)
	meta := findOne[NodeWithMeta](t, d.Nodes)
	if len(meta.Meta.Caption) != 1 || textOf(meta.Meta.Caption[0]) != "The long caption" {
		t.Errorf("expected the long caption, got %v", meta.Meta.Caption)
	}
	if len(meta.Meta.ShortCaption) != 1 || textOf(meta.Meta.ShortCaption[0]) != "Short" {
		t.Errorf("expected the short caption, got %v", meta.Meta.ShortCaption)
	}
	if _, ok := meta.Node.(*Table); !ok {
		t.Errorf("the caption should be attached to the table, got %T", meta.Node)
	}
	assertContains(t, writeHTML(t, d), "<figcaption>\nThe long caption\n</figcaption>")
	assertRoundTrip(t, input)

	t.Run("plain caption has no short form", func(t *testing.T) {
		meta := findOne[NodeWithMeta](t, parseString(t, "#+CAPTION: long\n| a |\n").Nodes)
		if len(meta.Meta.ShortCaption) != 1 || meta.Meta.ShortCaption[0] != nil {
			t.Errorf("expected no short caption, got %v", meta.Meta.ShortCaption)
		}
	})

	t.Run("keyword keeps its optional value", func(t *testing.T) {
		k := findOne[Keyword](t, parseString(t, "#+FOO[bar]: baz\n").Nodes)
		if k.Key != "FOO" || k.Optional != "bar" || k.Value != "baz" {
			t.Errorf("unexpected keyword %#v", k)
		}
		assertRoundTrip(t, "#+FOO[bar]: baz\n")
	})
}

// #+CALL: lines were generic keywords with nothing parsed from them.
func TestBabelCall(t *testing.T) {
	t.Run("parts of the call", func(t *testing.T) {
		d := parseString(t, "#+CALL: double[:session s](n=4) :results raw\n")
		call := findOne[BabelCall](t, d.Nodes)
		if call.Name != "double" || call.InsideHeader != ":session s" || call.Arguments != "n=4" || call.EndHeader != ":results raw" {
			t.Errorf("unexpected call %#v", call)
		}
		if call.Result != nil {
			t.Errorf("expected no result")
		}
	})

	t.Run("arguments with parentheses", func(t *testing.T) {
		call := findOne[BabelCall](t, parseString(t, "#+CALL: f(x=(1 2))\n").Nodes)
		if call.Name != "f" || call.Arguments != "x=(1 2)" {
			t.Errorf("unexpected call %#v", call)
		}
	})

	t.Run("name alone", func(t *testing.T) {
		call := findOne[BabelCall](t, parseString(t, "#+CALL: refresh\n").Nodes)
		if call.Name != "refresh" {
			t.Errorf("unexpected call %#v", call)
		}
	})

	t.Run("results are attached and exported", func(t *testing.T) {
		input := "#+CALL: double(n=4)\n\n#+RESULTS:\n: 8\n"
		d := parseString(t, input)
		call := findOne[BabelCall](t, d.Nodes)
		if result, ok := call.Result.(Result); !ok || textOf([]Node{result.Node}) != "8" {
			t.Fatalf("expected the result to be attached, got %#v", call.Result)
		}
		html := writeHTML(t, d)
		assertContains(t, html, "<pre class=\"example\">\n8\n</pre>")
		assertRoundTrip(t, input)
	})

	t.Run("exports none hides the results", func(t *testing.T) {
		d := parseString(t, "#+CALL: double(n=4) :exports none\n\n#+RESULTS:\n: 8\n")
		assertNotContains(t, writeHTML(t, d), "8")
	})

	t.Run("writer without BabelCallWriter falls back to keyword and result", func(t *testing.T) {
		d := parseString(t, "#+CALL: double(n=4)\n\n#+RESULTS:\n: 8\n")
		out, err := d.Write(newPlainHTMLWriter())
		if err != nil {
			t.Fatal(err)
		}
		assertContains(t, out, "8")
	})
}

// A lone # was not a comment.
func TestEmptyComment(t *testing.T) {
	d := parseString(t, "#\ntext\n")
	comment := findOne[Comment](t, d.Nodes)
	if comment.Content != "" {
		t.Errorf("expected an empty comment, got %q", comment.Content)
	}
	assertNotContains(t, writeHTML(t, d), "#")
	assertRoundTrip(t, "#\ntext\n")
	assertRoundTrip(t, "# a comment\n")

	t.Run("hash followed by a word is text", func(t *testing.T) {
		d := parseString(t, "#hashtag\n")
		if len(findAll[Comment](d.Nodes)) != 0 {
			t.Errorf("#hashtag is not a comment")
		}
	})
}

// #+SETUPFILE: only brought in keywords - not macros or link abbreviations - and
// could not be given as a URL.
func TestSetupFile(t *testing.T) {
	setup := "#+TODO: NEXT | FINISHED\n#+MACRO: greet Hello $1\n#+LINK: gh https://github.com/%s\n"

	t.Run("local file", func(t *testing.T) {
		d := parseWith(t, fakeFiles(map[string]string{"setup.org": setup}), "#+SETUPFILE: setup.org\n* NEXT thing\n{{{greet(you)}}} [[gh:org]]\n")
		if d.Macros["greet"] != "Hello $1" {
			t.Errorf("macro from the setup file missing: %v", d.Macros)
		}
		if d.Links["gh"] != "https://github.com/%s" {
			t.Errorf("link abbreviation from the setup file missing: %v", d.Links)
		}
		if h := headline(t, d, 0); h.Status != "NEXT" {
			t.Errorf("TODO keywords from the setup file should apply, got %q", h.Status)
		}
		html := writeHTML(t, d)
		assertContains(t, html, "Hello you")
		assertContains(t, html, `href="https://github.com/org"`)
	})

	t.Run("quoted file name", func(t *testing.T) {
		d := parseWith(t, fakeFiles(map[string]string{"setup.org": setup}), "#+SETUPFILE: \"setup.org\"\n")
		if d.Macros["greet"] == "" {
			t.Errorf("quoted setup file not read")
		}
	})

	t.Run("url is not fetched unless allowed", func(t *testing.T) {
		d := parseWith(t, New().Silent(), "#+SETUPFILE: https://example.org/setup.org\n")
		if len(d.Macros) != 0 {
			t.Errorf("nothing should be loaded without ReadURL")
		}
	})

	t.Run("url read through ReadURL", func(t *testing.T) {
		c := New().Silent()
		requested := ""
		c.ReadURL = func(url string) ([]byte, error) {
			requested = url
			return []byte(setup), nil
		}
		d := parseWith(t, c, "#+SETUPFILE: https://example.org/setup.org\n")
		if requested != "https://example.org/setup.org" {
			t.Errorf("expected the url to be requested, got %q", requested)
		}
		if d.Macros["greet"] != "Hello $1" {
			t.Errorf("macro from the remote setup file missing")
		}
	})

	t.Run("url that fails", func(t *testing.T) {
		c := New().Silent()
		c.ReadURL = func(string) ([]byte, error) { return nil, errors.New("offline") }
		d := parseWith(t, c, "#+SETUPFILE: https://example.org/setup.org\ntext\n")
		assertContains(t, writeHTML(t, d), "text")
	})
}

// plainWriterOnly hides every method of the wrapped writer that is not part of
// the Writer interface.
type plainWriterOnly struct{ Writer }

// newPlainHTMLWriter is an HTML writer that implements only the Writer
// interface, as a writer written before the optional writer interfaces would -
// for nested nodes as much as for the top level ones.
func newPlainHTMLWriter() Writer {
	h := NewHTMLWriter()
	p := plainWriterOnly{h}
	h.ExtendingWriter = p
	return p
}
