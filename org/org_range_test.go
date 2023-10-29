package org

import (
	"fmt"
	"strings"
	"testing"
)

func TestOrgRanges(t *testing.T) {
	path := "./testdata/headlines.org"
	reader := strings.NewReader(fileString(path))
	d := New().Silent().Parse(reader, path)
	for _, n := range d.Outline.Children {
		fmt.Printf("%v : %v : %v\n", n.Headline.GetPos(), n.Headline.GetEnd(), n.Headline.Title)
	}
}
