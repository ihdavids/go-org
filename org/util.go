package org

import "strings"

func isSecondBlankLine(d *Document, i int) bool {
	if i-1 <= 0 {
		return false
	}
	t1, t2 := d.tokens[i-1], d.tokens[i]
	if t1.kind == "text" && t2.kind == "text" && len(strings.TrimSpace(t1.content)) == 0 && len(strings.TrimSpace(t2.content)) == 0 {
		return true
	}
	return false
}

func isImageOrVideoLink(n Node) bool {
	if l, ok := n.(RegularLink); ok && l.Kind() == "video" || l.Kind() == "image" {
		return true
	}
	return false
}

func Prepend[T any](slice []T, elems ...T) []T {
	return append(elems, slice...)
}
