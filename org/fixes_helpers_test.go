package org

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Helpers shared by the tests of the parser fixes.

// parseString parses input as an org document, failing the test if the parse
// reported an error.
func parseString(t *testing.T, input string) *Document {
	t.Helper()
	return parseWith(t, New().Silent(), input)
}

func parseWith(t *testing.T, c *Configuration, input string) *Document {
	t.Helper()
	d := c.Parse(strings.NewReader(input), "./test.org")
	if d.Error != nil {
		t.Fatalf("parse error: %v\ninput:\n%s", d.Error, input)
	}
	return d
}

// writeOrg writes the document back as org.
func writeOrg(t *testing.T, d *Document) string {
	t.Helper()
	out, err := d.Write(NewOrgWriter())
	if err != nil {
		t.Fatalf("org write error: %v", err)
	}
	return out
}

// writeHTML writes the document as HTML.
func writeHTML(t *testing.T, d *Document) string {
	t.Helper()
	out, err := d.Write(NewHTMLWriter())
	if err != nil {
		t.Fatalf("html write error: %v", err)
	}
	return out
}

// assertRoundTrip checks that input is written back unchanged.
func assertRoundTrip(t *testing.T, input string) {
	t.Helper()
	if out := writeOrg(t, parseString(t, input)); out != input {
		t.Errorf("round trip changed the document\n--- input\n%s\n--- output\n%s", input, out)
	}
}

func assertContains(t *testing.T, s, want string) {
	t.Helper()
	if !strings.Contains(s, want) {
		t.Errorf("expected to find\n  %q\nin\n  %s", want, s)
	}
}

func assertNotContains(t *testing.T, s, unwanted string) {
	t.Helper()
	if strings.Contains(s, unwanted) {
		t.Errorf("did not expect to find\n  %q\nin\n  %s", unwanted, s)
	}
}

// children returns every node directly below n, including those the generic
// GetChildren leaves out (headline titles, the node of a NodeWithMeta, results).
func children(n Node) []Node {
	switch n := n.(type) {
	case *Headline:
		return append(append([]Node{}, n.Title...), n.Children...)
	case Headline:
		return append(append([]Node{}, n.Title...), n.Children...)
	case NodeWithMeta:
		return []Node{n.Node}
	case Result:
		return []Node{n.Node}
	case Block:
		if n.Result != nil {
			return append(append([]Node{}, n.Children...), n.Result)
		}
		return n.Children
	case *Block:
		if n.Result != nil {
			return append(append([]Node{}, n.Children...), n.Result)
		}
		return n.Children
	case FootnoteLink:
		if n.Definition != nil {
			return n.Definition.Children
		}
		return nil
	case nil:
		return nil
	}
	return n.GetChildren()
}

// findAll returns every node of type T in the tree, depth first.
func findAll[T any](nodes []Node) []T {
	found := []T{}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			if n == nil {
				continue
			}
			if v, ok := n.(T); ok {
				found = append(found, v)
			}
			walk(children(n))
		}
	}
	walk(nodes)
	return found
}

// findOne returns the only node of type T in the tree, failing the test if
// there is not exactly one.
func findOne[T any](t *testing.T, nodes []Node) T {
	t.Helper()
	found := findAll[T](nodes)
	if len(found) != 1 {
		var zero T
		t.Fatalf("expected exactly one %T, found %d in:\n%s", zero, len(found), dumpNodes(nodes))
		return zero
	}
	return found[0]
}

func dumpNodes(nodes []Node) string {
	out := strings.Builder{}
	var walk func([]Node, string)
	walk = func(nodes []Node, indent string) {
		for _, n := range nodes {
			if n == nil {
				continue
			}
			out.WriteString(fmt.Sprintf("%s%T %q\n", indent, n, n.String()))
			walk(children(n), indent+"  ")
		}
	}
	walk(nodes, "")
	return out.String()
}

// textOf concatenates the content of the Text nodes in nodes.
func textOf(nodes []Node) string {
	out := strings.Builder{}
	for _, t := range findAll[Text](nodes) {
		out.WriteString(t.Content)
	}
	return out.String()
}

// rawText renders the text and line breaks of raw content, such as the
// children of a src block.
func rawText(nodes []Node) string {
	out := strings.Builder{}
	for _, n := range nodes {
		switch n := n.(type) {
		case Text:
			out.WriteString(n.Content)
		case LineBreak:
			out.WriteString(strings.Repeat("\n", n.Count))
		}
	}
	return out.String()
}

// fakeFiles makes a Configuration read files from a map rather than the disk.
func fakeFiles(files map[string]string) *Configuration {
	c := New().Silent()
	c.ReadFile = func(name string) ([]byte, error) {
		if content, ok := files[name]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}
	return c
}

func headline(t *testing.T, d *Document, i int) *Headline {
	t.Helper()
	headlines := findAll[*Headline](d.Nodes)
	if i >= len(headlines) {
		t.Fatalf("expected at least %d headlines, found %d", i+1, len(headlines))
	}
	return headlines[i]
}
