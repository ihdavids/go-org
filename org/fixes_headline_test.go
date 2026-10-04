package org

import (
	"reflect"
	"strings"
	"testing"
)

// A line that merely contains SCHEDULED: was read as a planning line, and
// everything else on it was lost.
func TestPlanningOnlyOnPlanningLines(t *testing.T) {
	t.Run("keyword inside prose is prose", func(t *testing.T) {
		input := "We were SCHEDULED: <2026-10-01 Thu> back then\n"
		d := parseString(t, "* H\n"+input)
		if h := headline(t, d, 0); h.Scheduled != nil {
			t.Errorf("prose should not schedule the heading")
		}
		if n := len(findAll[SDC](d.Nodes)); n != 0 {
			t.Errorf("expected no planning, found %d", n)
		}
		assertContains(t, writeOrg(t, d), "We were SCHEDULED: <2026-10-01 Thu> back then")
		assertRoundTrip(t, input)
	})

	t.Run("text after the timestamp is not planning", func(t *testing.T) {
		input := "SCHEDULED: <2026-10-01 Thu> and then some\n"
		d := parseString(t, input)
		if n := len(findAll[SDC](d.Nodes)); n != 0 {
			t.Errorf("expected no planning, found %d", n)
		}
		assertRoundTrip(t, input)
	})

	t.Run("planning line still schedules the heading", func(t *testing.T) {
		d := parseString(t, "* H\n  SCHEDULED: <2026-10-01 Thu>\n")
		h := headline(t, d, 0)
		if h.Scheduled == nil || h.Scheduled.Date.Start.Day() != 1 {
			t.Errorf("expected the heading to be scheduled, got %+v", h.Scheduled)
		}
	})

	t.Run("several keywords on one line", func(t *testing.T) {
		d := parseString(t, "* DONE H\nCLOSED: [2026-10-02 Fri 10:00] DEADLINE: <2026-10-03 Sat> SCHEDULED: <2026-10-01 Thu>\n")
		h := headline(t, d, 0)
		if h.Closed == nil || h.Deadline == nil || h.Scheduled == nil {
			t.Errorf("expected closed, deadline and scheduled, got %v %v %v", h.Closed, h.Deadline, h.Scheduled)
		}
	})

	t.Run("planning with a range", func(t *testing.T) {
		d := parseString(t, "* H\nSCHEDULED: <2026-10-01 Thu 10:00-11:00>\n")
		if headline(t, d, 0).Scheduled == nil {
			t.Errorf("expected the heading to be scheduled")
		}
	})
}

// A DEADLINE line had its column added to its row.
func TestDeadlinePosition(t *testing.T) {
	d := parseString(t, "* H\n  DEADLINE: <2026-01-01 Thu>\n")
	sdc := findOne[SDC](t, d.Nodes)
	if sdc.GetPos() != (Pos{1, 2}) {
		t.Errorf("expected the deadline at row 1 column 2, got %v", sdc.GetPos())
	}
	d = parseString(t, "* H\n  SCHEDULED: <2026-01-01 Thu>\n")
	if sdc := findOne[SDC](t, d.Nodes); sdc.GetPos() != (Pos{1, 2}) {
		t.Errorf("expected the schedule at row 1 column 2, got %v", sdc.GetPos())
	}
}

// Progress cookies were never read: [1/3] gave nil, [33%] gave [0/0], and the
// writer then added [0/0] to the title.
func TestHeadlineProgressCookie(t *testing.T) {
	cases := []struct {
		input string
		want  *CheckStatus
	}{
		{"* TODO Task [1/3]\n", &CheckStatus{Num: 1, Den: 3, Type: "/"}},
		{"* Task [33%]\n", &CheckStatus{Num: 33, Type: "%"}},
		{"* Task [/]\n", &CheckStatus{Type: "/"}},
		{"* Task [%]\n", &CheckStatus{Type: "%"}},
		{"* Task [2/5] with more title\n", &CheckStatus{Num: 2, Den: 5, Type: "/"}},
		{"* Task [1/2] then [50%]\n", &CheckStatus{Num: 50, Type: "%"}},
		{"* Task with no cookie\n", nil},
		{"* Task [x/y]\n", nil},
	}
	for _, c := range cases {
		t.Run(strings.TrimSpace(c.input), func(t *testing.T) {
			h := headline(t, parseString(t, c.input), 0)
			if !reflect.DeepEqual(h.CheckStatus, c.want) {
				t.Errorf("expected %+v, got %+v", c.want, h.CheckStatus)
			}
			assertRoundTrip(t, c.input)
		})
	}

	t.Run("cookie set on a heading without one is written", func(t *testing.T) {
		d := parseString(t, "* Task\n")
		headline(t, d, 0).CheckStatus = &CheckStatus{Num: 1, Den: 2, Type: "/"}
		assertContains(t, writeOrg(t, d), "* Task [1/2]")
	})
}

// #+SEQ_TODO and #+TYP_TODO were ignored.
func TestTodoKeywordSequences(t *testing.T) {
	cases := []struct {
		name, input string
		status      string
		todo, done  []string
	}{
		{"default", "* TODO x\n", "TODO", []string{"TODO"}, []string{"DONE"}},
		{"seq_todo", "#+SEQ_TODO: WAIT | GONE\n* WAIT x\n", "WAIT", []string{"WAIT"}, []string{"GONE"}},
		{"typ_todo", "#+TYP_TODO: Fred Sara | FIXED\n* Sara x\n", "Sara", []string{"Fred", "Sara"}, []string{"FIXED"}},
		{"several lines", "#+TODO: A | B\n#+TODO: C | D\n* C x\n", "C", []string{"A", "C"}, []string{"B", "D"}},
		{"todo and seq_todo together", "#+TODO: A | B\n#+SEQ_TODO: C | D\n* D x\n", "D", []string{"A", "C"}, []string{"B", "D"}},
		{"fast access keys", "#+TODO: NEXT(n!) WAIT(w@/!) | DONE(d)\n* WAIT x\n", "WAIT", []string{"NEXT", "WAIT"}, []string{"DONE"}},
		{"no bar means the last is done", "#+TODO: OPEN REVIEW CLOSED\n* CLOSED x\n", "CLOSED", []string{"OPEN", "REVIEW"}, []string{"CLOSED"}},
		{"buffer keywords replace the defaults", "#+SEQ_TODO: OPEN | SHUT\n* TODO x\n", "", []string{"OPEN"}, []string{"SHUT"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := parseString(t, c.input)
			todo, done := d.TodoKeywords()
			if !reflect.DeepEqual(todo, c.todo) || !reflect.DeepEqual(done, c.done) {
				t.Errorf("expected %v | %v, got %v | %v", c.todo, c.done, todo, done)
			}
			if h := headline(t, d, 0); h.Status != c.status {
				t.Errorf("expected status %q, got %q (title %q)", c.status, h.Status, String(h.Title...))
			}
		})
	}
}

// Done and not done keywords were indistinguishable.
func TestTodoDoneDistinction(t *testing.T) {
	d := parseString(t, "#+TODO: TODO NEXT | DONE CANCELLED\n* TODO a\n* NEXT b\n* DONE c\n* CANCELLED d\n* plain e\n")
	want := []struct{ todo, done bool }{{true, false}, {true, false}, {false, true}, {false, true}, {false, false}}
	for i, w := range want {
		h := headline(t, d, i)
		if h.IsTodo() != w.todo || h.IsDone() != w.done {
			t.Errorf("%q: expected todo=%v done=%v, got todo=%v done=%v", h.Status, w.todo, w.done, h.IsTodo(), h.IsDone())
		}
	}
}

// `* TODO` with no title was not a TODO.
func TestBareTodoKeyword(t *testing.T) {
	h := headline(t, parseString(t, "* TODO\n"), 0)
	if h.Status != "TODO" || len(h.Title) != 0 {
		t.Errorf("expected status TODO with no title, got %q %q", h.Status, String(h.Title...))
	}
	assertRoundTrip(t, "* TODO\n")
	assertRoundTrip(t, "* DONE [#A]\n")

	t.Run("keyword is a whole word", func(t *testing.T) {
		h := headline(t, parseString(t, "* TODOS are fun\n"), 0)
		if h.Status != "" {
			t.Errorf("TODOS is not the TODO keyword")
		}
	})
}

// Priorities were limited to A, B and C.
func TestPriorities(t *testing.T) {
	cases := map[string]string{
		"* TODO [#D] x\n": "D",
		"* [#1] x\n":      "1",
		"* [#10] x\n":     "10",
		"* [#A] x\n":      "A",
		"* [#a] x\n":      "",
		"* [#] x\n":       "",
		"* [#A]\n":        "A",
	}
	for input, want := range cases {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			h := headline(t, parseString(t, input), 0)
			if h.Priority != want {
				t.Errorf("expected priority %q, got %q", want, h.Priority)
			}
			if want != "" {
				assertRoundTrip(t, input)
			}
		})
	}
	assertContains(t, writeHTML(t, parseString(t, "* [#7] x\n")), `<span class="priority">[7]</span>`)
}

// Tags were limited to ASCII.
func TestUnicodeTags(t *testing.T) {
	h := headline(t, parseString(t, "* Head :café:日本:tag_1@x:\n"), 0)
	if want := []string{"café", "日本", "tag_1@x"}; !reflect.DeepEqual(h.Tags, want) {
		t.Errorf("expected tags %v, got %v", want, h.Tags)
	}
	if title := String(h.Title...); title != "Head" {
		t.Errorf("tags should not be in the title, got %q", title)
	}

	t.Run("not tags", func(t *testing.T) {
		h := headline(t, parseString(t, "* Time 10:30:45\n"), 0)
		if len(h.Tags) != 0 {
			t.Errorf("expected no tags, got %v", h.Tags)
		}
	})
}

// COMMENT headings were not recognised.
func TestCommentHeadline(t *testing.T) {
	input := "* TODO [#A] COMMENT Draft :tag:\nsecret\n** Child\nmore secret\n* Public\nvisible\n"
	d := parseString(t, input)
	h := headline(t, d, 0)
	if !h.IsComment || h.Status != "TODO" || h.Priority != "A" || String(h.Title...) != "Draft" {
		t.Errorf("unexpected heading: comment=%v status=%q priority=%q title=%q", h.IsComment, h.Status, h.Priority, String(h.Title...))
	}
	html := writeHTML(t, d)
	assertNotContains(t, html, "secret")
	assertNotContains(t, html, "Draft")
	assertNotContains(t, html, "Child")
	assertContains(t, html, "visible")
	assertContains(t, writeOrg(t, d), "* TODO [#A] COMMENT Draft")

	t.Run("bare COMMENT heading", func(t *testing.T) {
		h := headline(t, parseString(t, "* COMMENT\n"), 0)
		if !h.IsComment || len(h.Title) != 0 {
			t.Errorf("expected an untitled comment heading")
		}
		assertRoundTrip(t, "* COMMENT\n")
	})

	t.Run("COMMENTARY is a title", func(t *testing.T) {
		if headline(t, parseString(t, "* COMMENTARY\n"), 0).IsComment {
			t.Errorf("COMMENTARY is not the COMMENT keyword")
		}
	})
}

// The ARCHIVE tag had no effect on export.
func TestArchivedHeadline(t *testing.T) {
	input := "* Old :ARCHIVE:\nold body\n** Old child\n* New\nnew body\n"
	h := headline(t, parseString(t, input), 0)
	if !h.IsArchived() {
		t.Fatalf("expected an archived heading")
	}

	t.Run("exported as its heading alone by default", func(t *testing.T) {
		html := writeHTML(t, parseString(t, input))
		assertContains(t, html, "Old")
		assertNotContains(t, html, "old body")
		assertNotContains(t, html, "Old child")
		assertContains(t, html, "new body")
	})

	t.Run("arch:t exports it all", func(t *testing.T) {
		html := writeHTML(t, parseString(t, "#+OPTIONS: arch:t\n"+input))
		assertContains(t, html, "old body")
		assertContains(t, html, "Old child")
	})

	t.Run("arch:nil exports none of it", func(t *testing.T) {
		html := writeHTML(t, parseString(t, "#+OPTIONS: arch:nil\n"+input))
		assertNotContains(t, html, "Old")
		assertContains(t, html, "new body")
	})
}

// Only the first drawer after the property drawer was recorded.
func TestAllDrawersRecorded(t *testing.T) {
	d := parseString(t, "* H\n:PROPERTIES:\n:A: 1\n:END:\n:LOGBOOK:\nx\n:END:\n:NOTES:\ny\n:END:\nbody\n:LATER:\nz\n:END:\n")
	h := headline(t, d, 0)
	names := []string{}
	for _, drawer := range h.Drawers {
		names = append(names, drawer.Name)
	}
	if want := []string{"LOGBOOK", "NOTES", "LATER"}; !reflect.DeepEqual(names, want) {
		t.Errorf("expected drawers %v, got %v", want, names)
	}
	if h.FindDrawer("NOTES") == nil {
		t.Errorf("FindDrawer should find the second drawer")
	}
	if v, _ := h.Properties.Get("A"); v != "1" {
		t.Errorf("the property drawer should still be read, got %q", v)
	}
	if len(findAll[*Drawer](d.Nodes)) != 3 {
		t.Errorf("drawers should stay in the heading's children")
	}
}

// A `:word:` line opened a drawer that swallowed the rest of the section.
func TestUnclosedDrawerIsText(t *testing.T) {
	t.Run("lone :word: line", func(t *testing.T) {
		input := "Para\n:smile:\nmore text\n"
		d := parseString(t, input)
		if n := len(findAll[*Drawer](d.Nodes)); n != 0 {
			t.Errorf("expected no drawer, found %d", n)
		}
		assertContains(t, textOf(d.Nodes), ":smile:")
		assertContains(t, writeHTML(t, d), "more text")
		assertRoundTrip(t, input)
	})

	t.Run("end after the next heading does not count", func(t *testing.T) {
		d := parseString(t, "* A\n:smile:\ntext\n* B\n:END:\n")
		if n := len(findAll[*Drawer](d.Nodes)); n != 0 {
			t.Errorf("expected no drawer, found %d", n)
		}
		if h := headline(t, d, 1); String(h.Title...) != "B" {
			t.Errorf("the second heading should be intact")
		}
	})

	t.Run("closed drawer is a drawer", func(t *testing.T) {
		d := parseString(t, ":NOTE:\ninside\n:END:\nafter\n")
		drawer := findOne[*Drawer](t, d.Nodes)
		if drawer.Name != "NOTE" || !strings.Contains(textOf(drawer.Children), "inside") {
			t.Errorf("unexpected drawer %#v", drawer)
		}
	})
}

// :VAR+: was a separate property rather than adding to VAR; Set duplicated an
// existing property and Append did too.
func TestPropertyDrawer(t *testing.T) {
	input := "* H\n:PROPERTIES:\n:VAR: a\n:VAR+: b\n:VAR+: c\n:ONLY+: x\n:END:\n"

	t.Run("plus appends to the value", func(t *testing.T) {
		props := headline(t, parseString(t, input), 0).Properties
		if v, ok := props.Get("VAR"); !ok || v != "a b c" {
			t.Errorf("expected \"a b c\", got %q %v", v, ok)
		}
		if v, ok := props.Get("ONLY"); !ok || v != "x" {
			t.Errorf("a property only ever added to is still a property, got %q %v", v, ok)
		}
		if _, ok := props.Get("MISSING"); ok {
			t.Errorf("missing property found")
		}
	})

	t.Run("written back as written", func(t *testing.T) {
		out := writeOrg(t, parseString(t, input))
		assertContains(t, out, ":VAR+: b")
		assertContains(t, out, ":VAR+: c")
	})

	t.Run("set replaces without duplicating", func(t *testing.T) {
		props := headline(t, parseString(t, input), 0).Properties
		props.Set("VAR", "z")
		if v, _ := props.Get("VAR"); v != "z" {
			t.Errorf("expected z, got %q", v)
		}
		count := 0
		for _, kv := range props.Properties {
			if kv[0] == "VAR" || kv[0] == "VAR+" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected one VAR line, got %d: %v", count, props.Properties)
		}
		props.Set("NEW", "n")
		if v, _ := props.Get("NEW"); v != "n" {
			t.Errorf("new property not set")
		}
	})

	t.Run("append adds without duplicating", func(t *testing.T) {
		props := &PropertyDrawer{Properties: [][]string{{"A", "1"}}}
		props.Append("A", "2")
		if len(props.Properties) != 1 || props.Properties[0][1] != "12" {
			t.Errorf("unexpected properties %v", props.Properties)
		}
		props.Append("B", "x")
		if len(props.Properties) != 2 {
			t.Errorf("a new property should be added, got %v", props.Properties)
		}
	})
}
