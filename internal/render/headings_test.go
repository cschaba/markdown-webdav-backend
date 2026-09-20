package render

import (
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	for heading, want := range map[string]string{
		"Tables":                    "tables",
		"  Some Heading  ":          "some-heading",
		"Vitamin B komplex":         "vitamin-b-komplex",
		"Überprüfung & Maße":        "überprüfung--maße", // goldmark's own ids would say "berprfung--mae"
		"Ärger mit ÖPNV":            "ärger-mit-öpnv",
		"日本語 の 見出し":                 "日本語-の-見出し",
		"snake_case and kebab-case": "snake-case-and-kebab-case",
		"C++ / C#":                  "c--c",
		"**Bold** and `code`":       "bold-and-code",
		"2026-09-18":                "2026-09-18",
		"?!":                        "heading",
		"":                          "heading",
	} {
		if got := Slug(heading); got != want {
			t.Errorf("Slug(%q) = %q, want %q", heading, got, want)
		}
	}
}

type headingLinks struct{ fakeLinks }

// "Book" has exactly these headings and named blocks; everything else has any.
func (headingLinks) HasAnchor(from, target, id string) bool {
	return (target != "Book" && target != "Book.md") ||
		strings.Contains(" überblick chapter section notes notes-1 ^abc123 ", " "+id+" ")
}

func TestLinksToHeadings(t *testing.T) {
	r := New(headingLinks{fakeLinks{"Book": "/Book", "Book.md": "/Book"}}, "/-/tag/")
	for src, want := range map[string]string{
		// the link and the heading use one slug, umlauts included
		"[[Book#Überblick]]": `<a href="/Book#überblick">Book &gt; Überblick</a>`,
		"[[Book#überblick]]": `<a href="/Book#überblick">Book &gt; überblick</a>`,
		// an alias replaces the text, with or without a heading
		"[[Book|the book]]":          `<a href="/Book">the book</a>`,
		"[[Book#Chapter|see ch. 1]]": `<a href="/Book#chapter">see ch. 1</a>`,
		// a nested reference lands on the last heading named
		"[[Book#Chapter#Section]]": `<a href="/Book#section">Book &gt; Chapter &gt; Section</a>`,
		// a block reference lands on the block it names
		"[[Book#^abc123]]": `<a href="/Book#^abc123">Book &gt; ^abc123</a>`,
		"[[Book#^ABC123]]": `<a href="/Book#^abc123">Book &gt; ^ABC123</a>`,
		// a block that is not there: still the note, but marked
		"[[Book#^gone]]": `<a href="/Book#^gone" class="missing-heading" title="The note has no block with this id">Book &gt; ^gone</a>`,
		// a heading that is not there: still the note, but marked
		"[[Book#Nope]]": `<a href="/Book#nope" class="missing-heading" title="The note has no heading of this name">Book &gt; Nope</a>`,
		// Markdown links take the same road
		// (goldmark percent-encodes what it writes; a browser decodes it before looking for the id)
		"[md](Book.md#%C3%9Cberblick)": `<a href="/Book#%C3%BCberblick">md</a>`,
		"[md](Book.md#Nope)":           `<a href="/Book#nope" title="The note has no heading of this name" class="missing-heading">md</a>`,
		// a note that does not exist stays a missing note, heading or not
		"[[Gone#Chapter]]": `<span class="missing" title="Not found in this vault: Gone">Gone#Chapter</span>`,
	} {
		html, _, err := r.Render([]byte(src), "")
		if got := strings.TrimSpace(string(html)); err != nil || got != "<p>"+want+"</p>" {
			t.Errorf("%s\n got: %s\nwant: <p>%s</p>", src, got, want)
		}
	}
}

func TestHeadingsOfTheSameNote(t *testing.T) {
	src := "[[#Überblick]] [[#Notes]] [[#Missing|alias]] [md](#%C3%9Cberblick) [[#Later]]\n\n" +
		"# Überblick\n\n## Notes\n\n## Notes\n\n### **Bold** one\n\n## Later\n"
	html, meta, err := New(fakeLinks{}, "/-/tag/").Render([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(meta.Headings, " "); got != "überblick notes notes-1 bold-one later" {
		t.Errorf("heading ids = %q", got) // a repeated heading is numbered; a link by name gets the first
	}
	for _, want := range []string{
		`<h1 id="überblick">`, `<h2 id="notes">`, `<h2 id="notes-1">`, `<h3 id="bold-one">`,
		`<a href="#überblick">Überblick</a>`, // the text is the heading, without the "#"
		`<a href="#notes">Notes</a>`,
		`<a href="#missing" class="missing-heading" title="The note has no heading of this name">alias</a>`,
		`<a href="#%C3%BCberblick">md</a>`,
		`<a href="#later">Later</a>`, // a heading further down than the link
	} {
		if !strings.Contains(string(html), want) {
			t.Errorf("output lacks %q\n%s", want, html)
		}
	}
	if len(meta.Links) != 0 {
		t.Errorf("links within a note are no links to other files: %q", meta.Links)
	}
}
