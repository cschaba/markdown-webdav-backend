package web

import (
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"markdown-webdav-backend/internal/index"
)

// The test vault in testdata/vault holds one page per feature. This test is
// the checklist a person would otherwise click through; serve the same vault
// with -vault test=./testdata/vault,nogit to look at it in a browser.
func TestFixtureVault(t *testing.T) {
	get := serve(t, "test", "../../testdata/vault")
	body := func(p string) string {
		t.Helper()
		res := get(p)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %d", p, res.StatusCode)
		}
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}
	expect := func(page, html string, want, reject []string) {
		t.Helper()
		for _, w := range want {
			if !strings.Contains(html, w) {
				t.Errorf("%s lacks %q", page, w)
			}
		}
		for _, r := range reject {
			if strings.Contains(html, r) {
				t.Errorf("%s contains %q", page, r)
			}
		}
	}
	aside := func(html string) string {
		i := strings.Index(html, "<aside>")
		if i < 0 {
			t.Fatal("no backlinks section")
		}
		return html[i:]
	}

	index := body("/test/00%20Index")
	expect("00 Index", index, []string{
		"<title>Feature Test Index</title>",
		`<tr><th>status</th><td>draft</td></tr>`,
		`<a href="/test/-/tag/test/index">#test/index</a>`,
		// every link carries the vault prefix and finds its note by name
		`By name: <a href="/test/01%20Formatting">`,
		`Case-insensitive: <a href="/test/01%20Formatting">`,
		`<a href="/test/02%20Code%20and%20Diagrams">code &amp; diagrams</a>`,
		`<a href="/test/sub/03%20Nested">sub/03 Nested</a>`,
		`<a href="/test/01%20Formatting#tables">`,
		`<a href="/test/v1.2%20plan">v1.2 plan</a>`,
		`<span class="missing" title="Not found in this vault: Does Not Exist">Does Not Exist</span>`,
		`<a href="/test/sub/03%20Nested">nested note</a>`, // Markdown link from another folder
		`<a href="https://obsidian.md">Obsidian</a>`,
		`href="/test/-/tag/test/inline"`,
		// header: breadcrumb into this vault, switcher to the other
		`<a href="/test/">test</a>`, `<strong>test</strong>`, `<a href="/other/">other</a>`,
		`<a href="/test/-/tags">Tags</a>`,
	}, []string{`Does Not Exist</a>`})
	expect("00 Index backlinks", aside(index), []string{
		`<a href="/test/01%20Formatting">01 Formatting</a>`,
		`<a href="/test/sub/03%20Nested">03 Nested</a>`,
	}, []string{"02 Code"})

	formatting := body("/test/01%20Formatting")
	expect("01 Formatting", formatting, []string{
		`<h2 id="tables">Tables</h2>`, "<table>", "<del>strike</del>", `type="checkbox"`,
		`class="footnotes"`, "<blockquote>",
	}, []string{"<script>alert", "alert("})
	if open, closed := strings.Count(formatting, `<details class="fold" open>`), strings.Count(formatting, "</details>"); open != 6 || closed != 6 {
		t.Errorf("01 Formatting: %d sections opened, %d closed; want 6 headings, 6 sections", open, closed)
	}
	// reached only through a property: related: "[[01 Formatting]]"
	expect("01 Formatting backlinks", aside(formatting), []string{`<a href="/test/00%20Index">Feature Test Index</a>`}, nil)

	code := body("/test/02%20Code%20and%20Diagrams")
	if n := strings.Count(code, `<pre class="chroma"`); n != 2 {
		t.Errorf("02 Code: %d highlighted blocks, want 2", n)
	}
	if n := strings.Count(code, `<pre class="mermaid"`); n != 2 {
		t.Errorf("02 Code: %d mermaid blocks, want 2", n)
	}
	if n := strings.Count(code, "mermaid.min.js"); n != 1 {
		t.Errorf("02 Code: mermaid script included %d times", n)
	}

	expect("03 Nested", body("/test/sub/03%20Nested"), []string{
		`<img src="/test/attachments/pixel.png">`,
		`<img src="/test/attachments/pixel.png" alt="alt text">`,
		`<a href="/test/attachments/sample.pdf">sample.pdf</a>`,
	}, nil)
	for p, contentType := range map[string]string{
		"/test/attachments/pixel.png":  "image/png",
		"/test/attachments/sample.pdf": "application/pdf",
	} {
		if got := get(p).Header.Get("Content-Type"); got != contentType {
			t.Errorf("%s served as %q", p, got)
		}
	}

	missing := body("/test/04%20Missing%20attachments")
	_, article, _ := strings.Cut(missing, "<details") // above it: header and tag list, with real links
	gone, present, _ := strings.Cut(article, "Present attachments")
	present, _, _ = strings.Cut(present, "<aside>")
	const embed, inline = `<span class="missing missing-embed" title="Not found in this vault: `, `<span class="missing" title="Not found in this vault: `
	expect("04 Missing, the broken half", gone, []string{
		`Image embed: ` + embed + `gone.png">gone.png</span>`,
		`with a size: ` + embed + `gone.png">gone.png</span>`, // the size is not shown as a label
		`with alt text: ` + embed + `gone.png">a lost diagram</span>`,
		`Markdown image: ` + embed + `attachments/gone.png">a lost chart</span>`,
		`without alt text: ` + embed + `attachments/gone.png">attachments/gone.png</span>`,
		`File link: ` + inline + `gone.pdf">gone.pdf</span>`,
		`with label: ` + inline + `gone.pdf">the lost report</span>`,
		`Markdown file link: ` + inline + `attachments/gone.pdf">the lost report</span>`,
		`Note embed: ` + embed + `Gone Note">Gone Note</span>`,
	}, []string{"<img", "<a href", ">200<"}) // nothing to click, no broken-image icon
	expect("04 Missing, the present half", present, []string{
		`Full size: <img src="/test/attachments/pixel.png">`,
		`Width only: <img src="/test/attachments/pixel.png" width="64">`,
		`Width and height: <img src="/test/attachments/pixel.png" width="64" height="16">`,
		`With alt text: <img src="/test/attachments/pixel.png" alt="a checkerboard">`,
		`<img src="https://example.com/logo.png" alt="logo">`,
	}, []string{"missing", `alt="64`})
	// Asking for the file itself is a plain 404, in the web view and over a
	// stale link alike; it must not fall through to a listing or a note.
	for _, p := range []string{"/test/attachments/gone.png", "/test/gone.pdf", "/test/Gone%20Note"} {
		if code := get(p).StatusCode; code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", p, code)
		}
	}

	const sketch = `<a class="drawing" href="/test/drawings/Sketch.excalidraw"><picture>` +
		`<source media="(prefers-color-scheme: dark)" srcset="/test/drawings/Sketch.excalidraw.dark.svg">` +
		`<img src="/test/drawings/Sketch.excalidraw.light.svg"`
	expect("05 Excalidraw", body("/test/05%20Excalidraw"), []string{
		sketch + `></picture></a>`,
		sketch + ` width="300"></picture></a>`,
		`As a link rather than an embed: <a href="/test/drawings/Sketch.excalidraw">Sketch.excalidraw</a>`,
		// a legacy file, and an export named without ".excalidraw"; one theme only
		`<a class="drawing" href="/test/drawings/Legacy.excalidraw"><img src="/test/drawings/Legacy.svg"></a>`,
		`<a class="drawing-unexported" href="/test/drawings/Unexported.excalidraw" title="This drawing has no exported image.`,
		`<span class="missing missing-embed" title="Not found in this vault: Nowhere.excalidraw">`,
	}, []string{"compressed-json", "EXCALIDRAW VIEW"})

	// A drawing's own page shows the picture, never the scene wrapped in Markdown.
	drawing := body("/test/drawings/Sketch.excalidraw")
	expect("Sketch page", drawing, []string{
		"<title>Sketch</title>",
		`srcset="/test/drawings/Sketch.excalidraw.dark.svg"`,
		`<img src="/test/drawings/Sketch.excalidraw.light.svg" alt="Sketch">`,
	}, []string{"EXCALIDRAW VIEW", "Text Elements", "versionNonce", "drawing-unexported"})
	expect("Sketch backlinks", aside(drawing), []string{`<a href="/test/05%20Excalidraw">05 Excalidraw</a>`}, nil)
	expect("Sketch page by its file name", body("/test/drawings/Sketch.excalidraw.md"), []string{`alt="Sketch"`}, []string{"versionNonce"})
	expect("Legacy page", body("/test/drawings/Legacy.excalidraw"), []string{`<img src="/test/drawings/Legacy.svg" alt="Legacy">`}, []string{"versionNonce"})
	expect("Legacy scene on request", body("/test/drawings/Legacy.excalidraw?raw"), []string{`"type": "excalidraw"`}, nil)
	expect("Unexported page", body("/test/drawings/Unexported.excalidraw"),
		[]string{`<p class="drawing-unexported">This drawing has no exported image.`, "Auto-export SVG"}, []string{"<img", "versionNonce"})
	expect("drawings listing", body("/test/drawings"), []string{`<a href="/test/drawings/Sketch.excalidraw">Sketch</a>`}, nil)
	if got := get("/test/drawings/Sketch.excalidraw.light.svg"); got.Header.Get("Content-Type") != "image/svg+xml" || got.Header.Get("Content-Security-Policy") != "sandbox" {
		t.Errorf("exported SVG: type %q, CSP %q", got.Header.Get("Content-Type"), got.Header.Get("Content-Security-Policy"))
	}

	expect("graph page", body("/test/-/graph"), []string{
		`<body class="wide">`, `data-source="graph.json"`, `src="/-/graph.js"`, `data-filter="orphans"`,
		`<a href="/test/-/graph">Graph</a>`, "force-graph@", // a pinned version, not "latest"
	}, nil)
	graph := body("/test/-/graph.json")
	expect("graph data", graph, []string{
		`{"id":"00 Index.md","title":"Feature Test Index","url":"/test/00%20Index","kind":"note","tags":[`,
		`{"source":"00 Index.md","target":"04 Missing attachments.md"}`,
		`{"source":"05 Excalidraw.md","target":"drawings/Sketch.excalidraw.md"}`,
		`{"id":"missing:does not exist","title":"Does Not Exist","kind":"missing"}`, // no url: nowhere to go
		`"id":"attachments/pixel.png"`, `"kind":"attachment"`,
		`{"id":"tag:test/excalidraw","title":"#test/excalidraw","url":"/test/-/tag/test/excalidraw","kind":"tag"}`,
	}, []string{".git", "versionNonce"})
	if res := get("/test/-/graph.json"); !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Errorf("graph.json served as %q", res.Header.Get("Content-Type"))
	}

	expect("v1.2 plan", body("/test/v1.2%20plan"), []string{"this body text must still render"}, nil)

	tags := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(body("/test/-/tags"), "")
	expect("tags", tags, []string{"#test 6", "#test/missing 1", "#test/excalidraw 1", "#test/code 1", "#test/nested 1", "#überprüfung 1"}, []string{"notatag"})
	expect("tag page", body("/test/-/tag/test"), []string{"01 Formatting.md", "02 Code and Diagrams.md", "sub/03 Nested.md", "00 Index.md"}, []string{"v1.2"})
	expect("unicode tag", body("/test/-/tag/%C3%BCberpr%C3%BCfung"), []string{"00 Index.md"}, nil)

	expect("listing", body("/test/"), []string{
		`<a href="/test/attachments">attachments/</a>`,
		`<a href="/test/00%20Index">Feature Test Index</a>`,
	}, nil)
}

// The index page is how a person finds the test pages; a page missing from it
// is a feature nobody looks at.
func TestFixtureIndexListsEveryPage(t *testing.T) {
	const root, indexPage = "../../testdata/vault", "00 Index.md"
	idx := index.New(root, "/test")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	pages := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".md" {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		// Drawings are reached through the page that embeds them.
		if rel == indexPage || idx.IsDrawing(rel) {
			return nil
		}
		pages++
		for _, note := range idx.Backlinks(rel) {
			if note.Path == indexPage {
				return nil
			}
		}
		t.Errorf("%q is not linked from %q: add a line for it to the Pages table", rel, indexPage)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages == 0 {
		t.Fatal("found no pages; is the path to the test vault right?")
	}
}

func TestHome(t *testing.T) {
	two := []Vault{{"notes", "/notes/"}, {"test", "/test/"}}
	rec := httptestGet(Home(two), "/")
	if b, _ := io.ReadAll(rec.Body); rec.StatusCode != 200 || !strings.Contains(string(b), `<a href="/test/">test</a>`) {
		t.Errorf("two vaults: status %d, body %s", rec.StatusCode, b)
	}
	rec = httptestGet(Home(two[:1]), "/")
	if rec.StatusCode != http.StatusFound || rec.Header.Get("Location") != "/notes/" {
		t.Errorf("one vault: status %d, location %q", rec.StatusCode, rec.Header.Get("Location"))
	}
}
