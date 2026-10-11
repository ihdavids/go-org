package org

import (
	"strings"
	"sync"
	"testing"
)

// Fixes ported from upstream niklasfasching/go-org.

// Emphasis next to a multibyte character was not recognized: the pre and
// post chars were read as single bytes (upstream a520664).
func TestEmphasisMultibyteBorder(t *testing.T) {
	d := parseString(t, "*emphasis ending with a multibyte character 习* and 习*not emphasis*\n")
	html := writeHTML(t, d)
	assertContains(t, html, "<strong>emphasis ending with a multibyte character 习</strong>")
	assertNotContains(t, html, "<strong>not emphasis</strong>")
	assertRoundTrip(t, "*emphasis ending with a multibyte character 习*\n")
}

// Emphasis directly followed by an explicit line break `\\` was not
// recognized (upstream f27340e).
func TestEmphasisBeforeExplicitLineBreak(t *testing.T) {
	input := "emphasis just before =explicit line break=\\\\\n=plus more emphasis=\n"
	html := writeHTML(t, parseString(t, input))
	assertContains(t, html, `<code class="verbatim">explicit line break</code><br>`)
	assertContains(t, html, `<code class="verbatim">plus more emphasis</code>`)
	assertRoundTrip(t, input)
}

// String could be called from several goroutines at once and raced on the
// shared org writer (upstream 5464ab3). Run with -race to see the original bug.
func TestStringIsSafeForConcurrentUse(t *testing.T) {
	d := parseString(t, "* Headline\nsome *text* with a [[https://example.com][link]]\n")
	want := String(d.Nodes...)
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := String(d.Nodes...); got != want {
				errs <- got
			}
			_ = d.Nodes[0].String()
		}()
	}
	wg.Wait()
	close(errs)
	for got := range errs {
		t.Errorf("concurrent String() = %q, want %q", got, want)
	}
}

// A footnote linked only from another footnote's definition was dropped
// (upstream 79fd96a).
func TestFootnoteLinkedFromFootnoteDefinition(t *testing.T) {
	input := "text[fn:1]\n\n[fn:1] first, see also[fn:2].\n\n[fn:2] Footnotes can be linked from another footnote's definition.\n"
	html := writeHTML(t, parseString(t, input))
	assertContains(t, html, `<sup id="footnote-2"><a href="#footnote-reference-2">2</a></sup>`)
	assertContains(t, html, "Footnotes can be linked from another footnote&#39;s definition.")
}

// Image and video links were only recognized by lower case extensions, and
// several image types emacs knows were missing (upstream 939b187, d4f684b).
func TestMediaExtensionsCaseInsensitive(t *testing.T) {
	for _, tc := range []struct{ link, kind string }{
		{"[[file:image.PNG]]", "image"},
		{"[[file:image.Jpeg]]", "image"},
		{"[[file:image.webp]]", "image"},
		{"[[file:image.xpm]]", "image"},
		{"[[file:image.pgm]]", "image"},
		{"[[file:clip.MP4]]", "video"},
		{"[[file:notes.txt]]", "regular"},
	} {
		l := findOne[RegularLink](t, parseString(t, tc.link+"\n").Nodes)
		if got := l.Kind(); got != tc.kind {
			t.Errorf("%s: kind %q, want %q", tc.link, got, tc.kind)
		}
	}
	assertContains(t, writeHTML(t, parseString(t, "[[file:image.PNG]]\n")), `<img src="image.PNG"`)
}

// Several inline src blocks on one line were merged into one (upstream
// f537c0b). Our parser already matches brackets, this guards it.
func TestMultipleInlineBlocksInParagraph(t *testing.T) {
	d := parseString(t, "src_schema[:exports code]{hello} and this src_schema[:exports code]{world}\n")
	blocks := findAll[InlineBlock](d.Nodes)
	if len(blocks) != 2 {
		t.Fatalf("want 2 inline blocks, got %s", dumpNodes(d.Nodes))
	}
	if rawText(blocks[0].Children) != "hello" || rawText(blocks[1].Children) != "world" {
		t.Errorf("unexpected inline blocks %s", dumpNodes(d.Nodes))
	}
}

// A source block starting with a blank line kept it in the html (upstream 0de50d6).
func TestBlockLeadingNewlineTrimmedInHTML(t *testing.T) {
	input := "#+BEGIN_SRC text\n\na source block with leading newline\n  with leading space\n\n#+END_SRC\n"
	html := writeHTML(t, parseString(t, input))
	assertContains(t, html, "<pre>\na source block with leading newline\n  with leading space\n</pre>")
}

// A plain title was wrapped in a paragraph inside the h1 (upstream 81cb0c3).
func TestSimpleTitleNotWrappedInParagraph(t *testing.T) {
	html := writeHTML(t, parseString(t, "#+TITLE: Title *here*\n"))
	assertContains(t, html, `<h1 class="title">Title <strong>here</strong></h1>`)
}

// :noweb strip-export removes noweb references from the exported code
// (upstream 5f599b9).
func TestNowebStripExport(t *testing.T) {
	input := "#+BEGIN_SRC raku :results output :noweb strip-export :exports both\n<<defn>>describe <a b c>;\n#+END_SRC\n"
	d := parseString(t, input)
	html := writeHTML(t, d)
	assertContains(t, html, "describe &lt;a b c&gt;;")
	assertNotContains(t, html, "defn")

	t.Run("kept without strip-export", func(t *testing.T) {
		html := writeHTML(t, parseString(t, strings.Replace(input, ":noweb strip-export ", "", 1)))
		assertContains(t, html, "defn")
	})
}
