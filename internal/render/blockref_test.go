package render

import (
	"strings"
	"testing"
)

func TestTrailingBlockID(t *testing.T) {
	for value, want := range map[string]string{
		"A paragraph. ^claim":      "claim",
		"^alone":                   "alone",
		"tabs\t^id":                "id",
		"trailing space ^id ":      "id",
		"digits and dashes ^a-1-b": "a-1-b",
		// not ids: no space before the caret, or something else in it
		"2^10":                        "",
		"a^b":                         "",
		"under ^bad_id":               "",
		"dot ^id.":                    "",
		"space ^ id":                  "",
		"bare ^":                      "",
		"none at all":                 "",
		"^id in the middle of a line": "",
	} {
		if got, _ := trailingBlockID(value); got != want {
			t.Errorf("trailingBlockID(%q) = %q, want %q", value, got, want)
		}
	}
	// cut says where the text before the id ends, trailing spaces already gone
	if _, cut := trailingBlockID("A paragraph.  ^claim"); cut != len("A paragraph.") {
		t.Errorf("cut = %d, want %d", cut, len("A paragraph."))
	}
}

// Every anchor Meta.Blocks claims must be an id in the rendered page. They are
// two sides of one promise: the index answers "the note has this block" from
// Meta.Blocks, and the browser looks for the id. A block whose renderer drops
// attributes - a code block - is the reason the anchor element exists at all.
func TestBlockAnchors(t *testing.T) {
	const src = "A paragraph. ^para\n\n" +
		"- an item ^item\n- another\n\n" +
		"> quoted ^quoted\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n^table\n\n" +
		"```go\nx := 1\n```\n\n^code\n\n" +
		"```mermaid\ngraph TD; A-->B;\n```\n\n^diagram\n\n" +
		"# A heading\n\n^afterheading\n\n" +
		"1. ordered ^ordered\n"
	html, meta, err := New(fakeLinks{}, "/-/tag/").Render([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"^para", "^item", "^quoted", "^table", "^code", "^diagram", "^afterheading", "^ordered"}
	if got := strings.Join(meta.Blocks, " "); got != strings.Join(want, " ") {
		t.Errorf("Meta.Blocks = %q, want %q", got, strings.Join(want, " "))
	}
	for _, anchor := range want {
		if !strings.Contains(string(html), `id="`+anchor+`"`) {
			t.Errorf("no id %q in the page, although Meta.Blocks claims it:\n%s", anchor, html)
		}
	}
	// the id is a name, not text: it is gone from what the reader sees
	for _, gone := range []string{"^para", "^item</li>", "^table", "^code"} {
		if strings.Contains(string(html), ">"+gone) {
			t.Errorf("%q is still shown as text:\n%s", gone, html)
		}
	}
	// a heading keeps the id it already has; its "^id" line anchors beside it
	if !strings.Contains(string(html), `id="a-heading"`) {
		t.Errorf("the heading lost its own id:\n%s", html)
	}
}

func TestBlockAnchorPlacement(t *testing.T) {
	for src, want := range map[string]string{
		// a named list item, in the item and not in the text block inside it
		"- one ^a\n- two": `<ul>
<li id="^a">one</li>
<li>two</li>
</ul>`,
		// "^id" alone after a table names the table
		"| a |\n|---|\n| 1 |\n\n^t": `<table id="^t">`,
		// with nothing above it, there is nothing to name: it stays text
		"^orphan\n\ntext": "<p>^orphan</p>\n<p>text</p>",
		// of two blocks named alike the first wins, as with two equal headings
		"first ^dup\n\nsecond ^dup": `<p id="^dup">first</p>` + "\n" + `<p id="^dup">second</p>`,
		// a paragraph of several lines is named by its last one
		"line one\nline two ^multi": `<p id="^multi">line one` + "\n" + "line two</p>",
	} {
		html, _, err := New(fakeLinks{}, "/-/tag/").Render([]byte(src), "")
		if got := strings.TrimSpace(string(html)); err != nil || !strings.Contains(got, want) {
			t.Errorf("%q\n got: %s\nwant it to hold: %s (err %v)", src, got, want, err)
		}
	}
}

// ![[Note#^id]] shows the named block and nothing else of the note, and the
// ids it carried stay behind: they are the other note's, not this page's.
func TestEmbeddedBlock(t *testing.T) {
	const other = "# Heading\n\nFirst. ^one\n\nSecond.\n\n- item ^two\n- other item\n"
	r := New(fakeLinks{"Other": "/Other", "src:Other.md": other}, "/-/tag/")
	for src, want := range map[string][]string{
		"![[Other#^one]]": {
			`<div class="transclusion-title"><a href="/Other#^one">Other &gt; ^one</a></div>`,
			"<p>First.</p>",
		},
		"![[Other#^two]]": {"<ul>\n<li>item</li>\n</ul>"},
	} {
		html, _, err := r.Render([]byte(src), "Host.md")
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range append(want, "") {
			if w != "" && !strings.Contains(string(html), w) {
				t.Errorf("%s lacks %q:\n%s", src, w, html)
			}
		}
		for _, unwanted := range []string{"Second.", "other item", "Heading", `id="^`} {
			if strings.Contains(string(html), unwanted) {
				t.Errorf("%s shows %q, which is not the named block:\n%s", src, unwanted, html)
			}
		}
	}
	// a block that is not there leaves the embed a link, as a missing heading does
	html, _, err := r.Render([]byte("![[Other#^none]]"), "Host.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), noBlockText) || strings.Contains(string(html), "transclusion") {
		t.Errorf("an embed of a block that is not there:\n%s", html)
	}
}
