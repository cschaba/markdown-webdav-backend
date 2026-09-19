package render

import (
	"regexp"
	"strings"
	"testing"
)

type fakeLinks map[string]string

func (f fakeLinks) ResolveLink(target string) (string, bool) {
	url, ok := f[target]
	return url, ok
}

func (f fakeLinks) ResolveEmbed(target string) Embed {
	switch {
	case IsImage(target):
		return Embed{Image: f[target]}
	case target == "Sketch.excalidraw":
		return Embed{Drawing: true, Image: "/Sketch.excalidraw.light.svg", DarkImage: "/Sketch.excalidraw.dark.svg"}
	case target == "Unexported.excalidraw":
		return Embed{Drawing: true}
	}
	return Embed{}
}

func TestRender(t *testing.T) {
	r := New(fakeLinks{"My Note": "/folder/My%20Note", "photo.png": "/img/photo.png"}, "/-/tag/")
	src := "---\ntitle: T\n---\n" +
		"[[My Note]] [[My Note#Some Heading|label]] [[Missing]] ![[photo.png]] #tag/sub\n\n" +
		"<script>alert(1)</script>\n\n" +
		"```go\nfunc main() {}\n```\n\n" +
		"```mermaid\ngraph TD; A-->B;\n```\n"
	html, meta, err := r.Render([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{
		`<a href="/folder/My%20Note">My Note</a>`,
		`<a href="/folder/My%20Note#some-heading">label</a>`,
		`<img src="/img/photo.png">`,
		`href="/-/tag/tag/sub"`,
		`<pre class="mermaid">`,
		`class="chroma"`,
		"Missing",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q\n%s", want, out)
		}
	}
	if want := `<span class="missing" title="Not found in this vault: Missing">Missing</span>`; !strings.Contains(out, want) {
		t.Errorf("output lacks %q\n%s", want, out)
	}
	for _, reject := range []string{"<script>alert", "title: T", `Missing</a>`} {
		if strings.Contains(out, reject) {
			t.Errorf("output contains %q\n%s", reject, out)
		}
	}
	if meta.Title != "T" || strings.Join(meta.Tags, ",") != "tag/sub" {
		t.Errorf("meta = %+v", meta)
	}
	if got := strings.Join(meta.Links, ","); got != "My Note,My Note,Missing,photo.png" {
		t.Errorf("links = %q", got)
	}
}

func TestBrokenFrontmatterStillRenders(t *testing.T) {
	html, _, err := New(fakeLinks{}, "/-/tag/").Render([]byte("---\ntags: [unclosed\n---\nBody text"))
	if err != nil || !strings.Contains(string(html), "Body text") {
		t.Errorf("html = %q, err = %v", html, err)
	}
}

func TestEveryKindOfLinkIsRecorded(t *testing.T) {
	r := New(fakeLinks{"Other note.md": "/deep/Other%20note", "img/a.png": "/img/a.png"}, "/-/tag/")
	src := "---\nrelated:\n  - \"[[Prop Note|label]]\"\nsource: \"see [[Second#Part]] too\"\n---\n" +
		"[md](Other%20note.md#Some%20Heading) ![pic](../img/a.png) [ext](https://example.com/x.md) " +
		"[frag](#local) [search](-/search?q=x) [[Wiki]] `[[code]]`\n"
	html, meta, err := r.Render([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(meta.Links, "|"); got != "Prop Note|Second|Other note.md|img/a.png|Wiki" {
		t.Errorf("links = %q", got)
	}
	for _, want := range []string{
		`href="/deep/Other%20note#some-heading"`, // found by name, not relative to this note
		`src="/img/a.png"`,
		`href="https://example.com/x.md"`,
		`href="#local"`,
		`<a href="-/search?q=x">search</a>`, // the server's own pages are not vault files
	} {
		if !strings.Contains(string(html), want) {
			t.Errorf("output lacks %q\n%s", want, html)
		}
	}
}

func TestHeadingsFold(t *testing.T) {
	src := "intro\n\n# One\n\na\n\n## One-A\n\nb[^n]\n\n### Deep\n\nc\n\n## One-B\n\nd\n\n# Two\n\n" +
		"> ## quoted heading\n\n```mermaid\ngraph TD; A-->B;\n```\n\n[^n]: note\n"
	html, _, err := New(fakeLinks{}, "/-/tag/").Render([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// Reduce the page to its structure: D( = section opens, ) = closes,
	// hN = heading, words = content.
	out := string(html)
	for old, new := range map[string]string{
		"<details class=\"fold\" open>": " D( ", "</details>": " ) ",
	} {
		out = strings.ReplaceAll(out, old, new)
	}
	var shape []string
	for _, field := range strings.Fields(regexp.MustCompile(`<h(\d)[^>]*>`).ReplaceAllString(out, " h$1 ")) {
		switch {
		case field == "D(" || field == ")" || len(field) == 2 && field[0] == 'h':
			shape = append(shape, field)
		case strings.HasPrefix(field, "<p>") && len(field) > 3:
			shape = append(shape, field[3:4])
		case strings.Contains(field, "mermaid.min.js"):
			shape = append(shape, "SCRIPT")
		case strings.Contains(field, `class="footnotes"`):
			shape = append(shape, "FOOTNOTES")
		}
	}
	// "h2" inside the block quote stays where it is and opens no section.
	// Script and footnotes end up outside every section.
	want := "i D( h1 a D( h2 b D( h3 c ) ) D( h2 d ) ) D( h1 h2 ) SCRIPT FOOTNOTES n"
	if got := strings.Join(shape, " "); got != want {
		t.Errorf("structure\n got: %s\nwant: %s\n%s", got, want, html)
	}
	for _, want := range []string{
		`<summary><h1 id="one">One</h1>`, // heading keeps its anchor
		"<p>intro</p>\n<details",         // text before the first heading is not folded
	} {
		if !strings.Contains(string(html), want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// A missing target still counts as a link: the index must know about it, so
// the backlink appears as soon as the file does.
func TestMissingTargetsAreStillLinks(t *testing.T) {
	r := New(fakeLinks{}, "/-/tag/")
	src := []byte("![[gone.png|200]] ![alt](img/gone.png) [[Gone Note]] <b>&\"</b> [[a\"b]]")
	html, meta, err := r.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(meta.Links, "|"); got != `gone.png|img/gone.png|Gone Note|a"b` {
		t.Errorf("links = %q", got)
	}
	if got := strings.Join(r.Meta(src).Links, "|"); got != strings.Join(meta.Links, "|") {
		t.Errorf("Meta sees other links than Render: %q", got)
	}
	// The target ends up in an attribute; it must not be able to leave it.
	if want := `title="Not found in this vault: a&quot;b"`; !strings.Contains(string(html), want) {
		t.Errorf("output lacks %q\n%s", want, html)
	}
}

func TestEmbeds(t *testing.T) {
	r := New(fakeLinks{
		"photo.png": "/img/photo.png", "Note": "/Note",
		"Sketch.excalidraw": "/Sketch.excalidraw", "Unexported.excalidraw": "/Unexported.excalidraw",
	}, "/-/tag/")
	for src, want := range map[string]string{
		"![[photo.png]] after": `<p><img src="/img/photo.png"> after</p>`, // no stray closing tag
		"![[Note]]":            `<p><a href="/Note">Note</a></p>`,
		"![[Sketch.excalidraw|300]]": `<p><a class="drawing" href="/Sketch.excalidraw"><picture>` +
			`<source media="(prefers-color-scheme: dark)" srcset="/Sketch.excalidraw.dark.svg">` +
			`<img src="/Sketch.excalidraw.light.svg" width="300"></picture></a></p>`,
		"[[Sketch.excalidraw]]": `<p><a href="/Sketch.excalidraw">Sketch.excalidraw</a></p>`, // a link stays a link
		"![[Unexported.excalidraw]]": `<p><a class="drawing-unexported" href="/Unexported.excalidraw" title="` + DrawingHelp +
			`">Unexported.excalidraw</a></p>`,
	} {
		html, _, err := r.Render([]byte(src))
		if got := strings.TrimSpace(string(html)); err != nil || got != want {
			t.Errorf("%s\n got: %s\nwant: %s (err %v)", src, got, want, err)
		}
	}
}
