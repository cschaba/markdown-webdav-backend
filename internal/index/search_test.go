package index

import (
	"strings"
	"testing"
)

func searchVault(t *testing.T) *Index {
	t.Helper()
	root := writeVault(t, map[string]string{
		// "plan" in ever weaker places, to pin the ranking down
		"Plan.md":                    "nothing about it in the text",
		"projects/Garden plan.md":    "beds and paths",
		"projects/Planning.md":       "how to go about it",
		"tagged.md":                  "---\ntags: [plan]\n---\nbody",
		"Headed.md":                  "# The plan\n\ntext",
		"Mentions.md":                "We made a plan. Then another plan, a third plan, and plan plan plan plan.",
		"Once.md":                    "A single plan here.",
		"Inside.md":                  "Life on another planet.",
		"Code.md":                    "```sh\n# plan in a comment\n```\n",
		"Props.md":                   "---\nstatus: plan\n---\nnothing here",
		"Unrelated.md":               "bread and butter",
		"projects/Roof.md":           "---\ntags: [house/roof]\n---\nslates, and a plan for the gutter",
		"Long.md":                    strings.Repeat("filler ", 60) + "needle " + strings.Repeat("filler ", 60),
		"Sketch.excalidraw.md":       "---\nexcalidraw-plugin: parsed\n---\n## Text Elements\nplan ^abc\n",
		"attachments/floor plan.pdf": "",
		"attachments/photo.png":      "",
		".obsidian/plan.json":        "",
	})
	idx := New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	return idx
}

func paths(results []SearchResult) string {
	var out []string
	for _, r := range results {
		out = append(out, r.Path)
	}
	return strings.Join(out, " | ")
}

func TestSearchRanking(t *testing.T) {
	got := paths(searchVault(t).Search("plan"))
	want := strings.Join([]string{
		"Plan.md",                    // the title is the term
		"attachments/floor plan.pdf", // a word of a file's name counts like one of a title
		"projects/Garden plan.md",    // a word of the title
		"projects/Planning.md",       // part of a title word
		"tagged.md",                  // the tag
		"Props.md",                   // a property's value
		"Headed.md",                  // a heading
		"Mentions.md",                // the text, often (capped)
		"Code.md",                    // the text, once; "# plan" in a code block is no heading
		"Once.md",
		"projects/Roof.md",
		"Inside.md", // only inside another word
	}, " | ")
	if got != want {
		t.Errorf("order:\n got: %s\nwant: %s", got, want)
	}
	for _, absent := range []string{"Unrelated.md", "Sketch.excalidraw.md", ".obsidian"} {
		if strings.Contains(got, absent) {
			t.Errorf("%s must not be found: drawings and dot folders are not searched", absent)
		}
	}
}

func TestSearchPercent(t *testing.T) {
	idx := searchVault(t)
	percent := func(q string) map[string]int {
		out := map[string]int{}
		for _, r := range idx.Search(q) {
			out[r.Path] = r.Percent
		}
		return out
	}
	got := percent("plan")
	for p, want := range map[string]int{
		"Plan.md":                    100, // named exactly that; more than 100 points, still 100%
		"projects/Garden plan.md":    70,  // a word of the title (60) and of the file name (10)
		"projects/Planning.md":       50,
		"tagged.md":                  30,
		"Props.md":                   25, // status: plan
		"Headed.md":                  20, // heading (15), and that line is text too (2+3)
		"Mentions.md":                13, // capped: 5 hits count, however many there are
		"attachments/floor plan.pdf": 70, // a file name is scored like a title
		"Once.md":                    5,
		"Inside.md":                  2, // only inside "planet"
	} {
		if got[p] != want {
			t.Errorf("%s: %d%%, want %d%%", p, got[p], want)
		}
	}
	// The scale is per term: a note matching both words as well as it can
	// reads the same as a note matching one word as well as it can.
	if got := percent("garden plan")["projects/Garden plan.md"]; got != 70 {
		t.Errorf(`"garden plan" as two words: %d%%, want 70%%`, got)
	}
	if got := percent(`"garden plan"`)["projects/Garden plan.md"]; got != 100 {
		t.Errorf(`"garden plan" as a phrase: %d%%, want 100%%`, got)
	}
	// Signals add up to the cap, and the uncapped score still decides the order.
	writeFile(t, idx.root, "projects/Roof.md", "---\ntags: [roof]\n---\n# Roof work\n\nroof roof roof")
	idx.Update("projects/Roof.md")
	writeFile(t, idx.root, "Roof.md", "")
	idx.Update("Roof.md")
	roof := idx.Search("roof")
	if len(roof) != 2 || roof[0].Path != "projects/Roof.md" || roof[0].Percent != 100 || roof[1].Percent != 100 || roof[0].Score <= roof[1].Score {
		t.Errorf("two notes at the cap: %+v", roof)
	}

	// Filters alone do not rank, so there is nothing to put a number on.
	for p, got := range percent("path:projects") {
		if got != 0 {
			t.Errorf("%s: %d%% for a query without words", p, got)
		}
	}
}

func TestSearchProperties(t *testing.T) {
	root := writeVault(t, map[string]string{
		"Talk.md": "---\ntitle: A talk\nsource: https://example.com/watch?v=abc\nauthor:\n  - \"[[Karla Tutorials]]\"\n" +
			"created: 2026-09-04\npublished:\nrating: 5\nmeta:\n  venue: Lausanne\n" +
			"description: Martin Odersky compares language designs at length, " + strings.Repeat("and more ", 30) + "\n" +
			"tags: [clippings]\n---\nThe body says nothing about him.\n",
		"Other.md": "---\ncreated: 2025-01-01\nsource: a book\n---\nOdersky is mentioned once.\n",
		"Plain.md": "no front matter, no match",
	})
	idx := New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	// Found through a property, and ranked above a mere mention.
	r := idx.Search("odersky")
	if paths(r) != "Talk.md | Other.md" || r[0].Percent != 25 {
		t.Fatalf("got %s (%d%%)", paths(r), r[0].Percent)
	}
	if s := r[0].Snippets; len(s) != 1 || !strings.HasPrefix(s[0], "description: Martin Odersky compares") || !strings.HasSuffix(s[0], "…") {
		t.Errorf("snippet = %q; want the property line, cut to length", s)
	}
	for q, want := range map[string]string{
		"karla":         "Talk.md", // inside a list, inside a wikilink
		"lausanne":      "Talk.md", // nested
		"2026-09":       "Talk.md", // a date, as it is written in the note
		"example.com":   "Talk.md", // a URL
		"created":       "",        // keys are not searched: every note has them
		"source":        "",
		"description":   "",
		"clippings":     "Talk.md", // tags keep their own, higher score...
		"tag:clippings": "Talk.md",
		"a talk":        "Talk.md", // ...and so does the title
	} {
		if got := paths(idx.Search(q)); got != want {
			t.Errorf("Search(%q) = %q, want %q", q, got, want)
		}
	}
	if got := idx.Search("clippings")[0].Percent; got != 30 {
		t.Errorf("a tag scored %d%%: it must not also count as a property", got)
	}
	if got := idx.Search(`"a talk"`)[0].Percent; got != 100 {
		t.Errorf("the title scored %d%%: it must not also count as a property", got)
	}
	if s := idx.Search("2026-09")[0].Snippets; len(s) != 1 || s[0] != "created: 2026-09-04" {
		t.Errorf("date snippet = %q", s)
	}
	// A property that arrives through a later write is searchable too.
	writeFile(t, root, "Plain.md", "---\nauthor: Grace Hopper\n---\ntext")
	idx.Update("Plain.md")
	if got := paths(idx.Search("hopper")); got != "Plain.md" {
		t.Errorf("after an update: %q", got)
	}
}

func TestSearchSyntax(t *testing.T) {
	idx := searchVault(t)
	for q, want := range map[string]string{
		"PLAN gutter":        "projects/Roof.md", // every word must occur; case is ignored
		`"a plan for"`:       "projects/Roof.md", // a phrase
		`"plan a"`:           "",                 // ... in that order
		"plan path:projects": "projects/Garden plan.md | projects/Planning.md | projects/Roof.md",
		"path:projects":      "projects/Garden plan.md | projects/Planning.md | projects/Roof.md", // a filter alone lists, by title
		"tag:house":          "projects/Roof.md",                                                  // a nested tag counts for its parent
		"tag:#house/roof":    "projects/Roof.md",
		"tag:house bread":    "",
		"plan tag:plan":      "tagged.md",
		`path:"floor plan"`:  "", // a filter alone lists notes, not files
		`floor path:attach`:  "attachments/floor plan.pdf",
		"photo":              "attachments/photo.png",
		"":                   "",
		"   ":                "",
		"tag:":               "",
	} {
		if got := paths(idx.Search(q)); got != want {
			t.Errorf("Search(%q)\n got: %s\nwant: %s", q, got, want)
		}
	}
}

func TestSearchSnippets(t *testing.T) {
	idx := searchVault(t)
	r := idx.Search("plan")
	byPath := map[string]SearchResult{}
	for _, res := range r {
		byPath[res.Path] = res
	}
	if got := byPath["Headed.md"].Snippets; len(got) != 1 || got[0] != "# The plan" {
		t.Errorf("snippets = %q", got)
	}
	if got := byPath["Plan.md"].Snippets; len(got) != 0 {
		t.Errorf("a title match has no line to show, got %q", got)
	}
	if got := byPath["attachments/floor plan.pdf"]; !got.Attachment || got.URL != "/v/attachments/floor%20plan.pdf" {
		t.Errorf("attachment = %+v", got)
	}
	long := idx.Search("needle")[0].Snippets[0]
	if !strings.Contains(long, "needle") || !strings.HasPrefix(long, "…") || !strings.HasSuffix(long, "…") || len([]rune(long)) > snippetLen+2 {
		t.Errorf("long line not cut around the match: %q", long)
	}
}

func TestSearchSeesFilesChangedOnDisk(t *testing.T) {
	idx := searchVault(t)
	writeFile(t, idx.root, "Unrelated.md", "now it mentions a xylophone")
	if got := paths(idx.Search("xylophone")); got != "Unrelated.md" {
		t.Errorf("got %q; search must read the files, not a copy", got)
	}
}

// What a comment holds is not in the page (internal/render/comment.go), so it
// must not be findable either. The search never parses a note, so this is the
// one place where the two readings of "%%" can drift apart.
func TestStripComments(t *testing.T) {
	for text, want := range map[string]string{
		"before %%hidden%% after":         "before  after",
		"%%whole line%%":                  "",
		"a%%b%%c":                         "ac",
		"%%one%% x %%two%%":               " x ",
		"before %%never closed":           "before %%never closed",
		"stray %% sign":                   "stray %% sign",
		"`%%code%%` stays":                "`%%code%%` stays",
		"``a `%%x%%` b`` stays":           "``a `%%x%%` b`` stays",
		"```\n%%code%%\n```":              "```\n%%code%%\n```",
		"~~~\n%%code%%\n~~~":              "~~~\n%%code%%\n~~~",
		"a\n%%\nhidden\n%%\nb":            "a\n\n\n\nb", // the note keeps its lines
		"a\n%%\n```\nstill hidden\n%%\nb": "a\n\n\n\n\nb",
		"a\n%%\nto the end":               "a\n\n",
		"nothing to do here":              "nothing to do here",
	} {
		if got := stripComments(text); got != want {
			t.Errorf("stripComments(%q)\n got: %q\nwant: %q", text, got, want)
		}
	}
}
