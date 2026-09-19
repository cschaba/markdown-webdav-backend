package render

import (
	"strings"
	"testing"
)

// embedVault: notes by name, each reachable as [[Name]] at /Name.
func embedVault(notes map[string]string) fakeLinks {
	links := fakeLinks{"pic.png": "/img/pic.png"}
	for name, src := range notes {
		links[name] = "/" + name
		links["/"+name+".md"] = "/" + name // how an embedded note names itself
		links["src:"+name+".md"] = src
	}
	return links
}

func render(t *testing.T, links fakeLinks, src, from string) string {
	t.Helper()
	html, _, err := New(links, "/-/tag/").Render([]byte(src), from)
	if err != nil {
		t.Fatal(err)
	}
	return string(html)
}

func TestEmbedNote(t *testing.T) {
	links := embedVault(map[string]string{
		"Recipe": "---\ntitle: ignored here\ntags: [food]\n---\nMix **flour** and water.\n\n- one\n- two\n\n![[pic.png]] and [[Other]].\n",
		"Other":  "other",
	})
	html := render(t, links, "Before ![[Recipe]] after.\n", "Host.md")
	want := "<p>Before </p>\n" +
		`<div class="transclusion">` + "\n" +
		`<div class="transclusion-title"><a href="/Recipe">Recipe</a></div>` + "\n" +
		"<p>Mix <strong>flour</strong> and water.</p>\n<ul>\n<li>one</li>\n<li>two</li>\n</ul>\n" +
		`<p><img src="/img/pic.png"> and <a href="/Other">Other</a>.</p>` + "\n" +
		"</div>\n<p> after.</p>\n"
	if html != want {
		t.Errorf("got:\n%s\nwant:\n%s", html, want)
	}
	if strings.Contains(html, "<p><div") || strings.Contains(html, "ignored here") || strings.Contains(html, "food") {
		t.Error("a block inside a paragraph, or front matter in the embed")
	}
	// alone in its paragraph, it leaves no empty paragraphs behind
	if alone := render(t, links, "![[Other]]\n", "Host.md"); strings.Contains(alone, "<p></p>") || strings.Count(alone, "<p>") != 1 {
		t.Errorf("embed alone:\n%s", alone)
	}
	// in a list item, and two in one paragraph
	if list := render(t, links, "- ![[Other]]\n- plain\n", "Host.md"); !strings.Contains(strings.ReplaceAll(list, "\n", ""), `<li><div class="transclusion">`) {
		t.Errorf("embed in a list:\n%s", list)
	}
	if two := render(t, links, "![[Other]] ![[Recipe]]\n", "Host.md"); strings.Count(two, `class="transclusion"`) != 2 {
		t.Errorf("two embeds in one paragraph:\n%s", two)
	}
	// where a block cannot stand, the embed stays a link
	for _, src := range []string{"*![[Other]]*", "| a |\n|---|\n| ![[Other]] |"} {
		if html := render(t, links, src, "Host.md"); strings.Contains(html, "transclusion") || !strings.Contains(html, `<a href="/Other">Other</a>`) {
			t.Errorf("%q:\n%s", src, html)
		}
	}
	// a link is a link: only ![[...]] embeds
	if html := render(t, links, "[[Other]]", "Host.md"); strings.Contains(html, "transclusion") {
		t.Errorf("a plain link embedded the note:\n%s", html)
	}
}

func TestEmbedSection(t *testing.T) {
	links := embedVault(map[string]string{
		"Book": "intro\n\n# Überblick\n\nfirst\n\n## Details\n\nnested\n\n# Later\n\nlast\n",
	})
	html := render(t, links, "![[Book#Überblick]]\n\n![[Book#Details]]\n\n![[Book#Nope]]\n", "Host.md")
	for _, want := range []string{
		`<a href="/Book#überblick">Book &gt; Überblick</a></div>`,
		`<a href="/Book#details">Book &gt; Details</a></div>`,
		// not embedded, and the link says why
		`<a href="/Book#nope" class="missing-heading"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output lacks %q\n%s", want, html)
		}
	}
	first, rest, _ := strings.Cut(html, "Book &gt; Details")
	// the section is the heading and all below it, up to the next heading of its level
	if !strings.Contains(first, "first") || !strings.Contains(first, "nested") || strings.Contains(first, "intro") || strings.Contains(first, "last") {
		t.Errorf("section Überblick:\n%s", first)
	}
	if !strings.Contains(rest, "nested") || strings.Contains(rest, "first") || strings.Contains(rest, "last") {
		t.Errorf("section Details:\n%s", rest)
	}
	if strings.Count(html, `class="transclusion"`) != 2 {
		t.Errorf("want two embeds:\n%s", html)
	}
}

func TestEmbedKeepsThePageIntact(t *testing.T) {
	links := embedVault(map[string]string{
		"Part": "# Notes\n\nSee [[#Notes]] and [[#Missing]].\n\n```mermaid\ngraph TD; A-->B;\n```\n",
	})
	html := render(t, links, "# Notes\n\n![[Part]]\n\n[[#Notes]]\n", "Host.md")
	// The host's heading keeps its id; the embedded one of the same name has
	// none, or the page would have the id twice and the host's link a rival.
	if strings.Count(html, `id="notes"`) != 1 || !strings.Contains(html, `<a href="#notes">Notes</a>`) {
		t.Errorf("heading ids:\n%s", html)
	}
	// "This note" inside the embed is the embedded note, elsewhere.
	for _, want := range []string{`<a href="/Part#notes">Notes</a>`, `<a href="/Part#missing" class="missing-heading"`} {
		if !strings.Contains(html, want) {
			t.Errorf("output lacks %q\n%s", want, html)
		}
	}
	// The diagram needs the script: once, on the page, not inside the embed.
	script := strings.Index(html, "mermaid.min.js")
	if strings.Count(html, "mermaid.min.js") != 1 || script < strings.LastIndex(html, "</div>") {
		t.Errorf("mermaid script: %d times, at %d:\n%s", strings.Count(html, "mermaid.min.js"), script, html)
	}
	// A host with diagrams of its own still gets one script.
	both := render(t, links, "```mermaid\ngraph TD; X-->Y;\n```\n\n![[Part]]\n", "Host.md")
	if strings.Count(both, "mermaid.min.js") != 1 || strings.Count(both, `class="mermaid"`) != 2 {
		t.Errorf("host and embed both with diagrams:\n%s", both)
	}
}

func TestEmbedsGoOneLevelDeep(t *testing.T) {
	links := embedVault(map[string]string{
		"Outer": "outer text\n\n![[Inner]]\n\n![[pic.png]]\n",
		"Inner": "inner text",
		"Self":  "me\n\n![[Self]]\n",
		"A":     "a\n\n![[B]]\n",
		"B":     "b\n\n![[A]]\n",
	})
	html := render(t, links, "![[Outer]]\n", "Host.md")
	if strings.Count(html, `class="transclusion"`) != 1 || strings.Contains(html, "inner text") {
		t.Errorf("embedded more than one level:\n%s", html)
	}
	// inside the embed the note embed is a link that says so; images still show
	for _, want := range []string{
		`<a href="/Inner" class="not-embedded" title="Not embedded here: notes are embedded one level deep">Inner</a>`,
		`<img src="/img/pic.png">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output lacks %q\n%s", want, html)
		}
	}
	// On its own page, Outer does embed Inner.
	if own := render(t, links, links["src:Outer.md"], "Outer.md"); !strings.Contains(own, "inner text") {
		t.Errorf("Outer's own page:\n%s", own)
	}
	// A note embedding itself, and two embedding each other, end.
	if self := render(t, links, links["src:Self.md"], "Self.md"); strings.Contains(self, "transclusion") || !strings.Contains(self, "a note cannot embed itself") {
		t.Errorf("self embed:\n%s", self)
	}
	if loop := render(t, links, links["src:A.md"], "A.md"); strings.Count(loop, `class="transclusion"`) != 1 || strings.Count(loop, "<p>a</p>") != 1 {
		t.Errorf("A and B embedding each other:\n%s", loop)
	}
}
