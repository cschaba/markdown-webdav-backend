package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
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

	// A note's statistics: in the page, hidden until asked for, and a button
	// at the foot that opens them without the key.
	expect("11 Keyboard", body("/test/11%20Keyboard"), []string{
		`<section id="note-stats" class="note-stats" aria-label="Statistics of this note" hidden>`,
		`<div><dt>Backlinks</dt><dd>1</dd></div>`,
		`<button type="button" id="stats-button" aria-controls="note-stats" aria-expanded="false" hidden>Statistics <kbd>i</kbd></button>`,
	}, []string{"<dt>Tasks</dt>"})
	// counted as the task search counts them
	expect("06 Search", body("/test/06%20Search"), []string{`<div><dt>Tasks</dt><dd>1 of 3 done</dd></div>`}, nil)
	expect("a folder", body("/test/"), nil, []string{`id="note-stats"`, `id="stats-button"`})

	// The About page counts the vault; the numbers follow the vault's pages,
	// so only that it counts is checked here, the counting in index.TestStats.
	expect("About", body("/test/-/about"), []string{
		`<a href="https://github.com/cschaba/markdown-webdav-backend" rel="noreferrer">`,
		"<tr><th>Notes</th>", "<tr><th>Drawings</th>", "<tr><th>Attachments</th>",
		"of them to a file that is missing", "<th>Last change</th>",
	}, []string{"<tr><th>Notes</th><td>0</td>"})
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
		`<source media="screen and (prefers-color-scheme: dark)" srcset="/test/drawings/Sketch.excalidraw.dark.svg">` +
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

	// Where a link leads depends on where it stands: seen from links/Here.md.
	here := body("/test/links/Here")
	for written, want := range map[string]string{
		// a name: next to the page first, then the vault
		"`[[Twin]]` —":     `<a href="/test/links/Twin">Twin</a>`,
		"`[[00 Index]]` —": `<a href="/test/00%20Index">00 Index</a>`,
		"`![[dot.png]]` —": `<img src="/test/links/dot.png">`,
		// a path: relative, then from the root
		"`[[deep/Twin]]` —":     `<a href="/test/links/deep/Twin">deep/Twin</a>`,
		"`[[sub/03 Nested]]` —": `<a href="/test/sub/03%20Nested">sub/03 Nested</a>`,
		"`![[deep/dot.png]]` —": `<img src="/test/links/deep/dot.png">`,
		// ./ and ../ are followed strictly
		"`[[./Twin]]`:":             `<a href="/test/links/Twin">./Twin</a>`,
		"`[[../Twin]]` —":           `<a href="/test/Twin">../Twin</a>`,
		"`![[./deep/dot.png|16]]`,": `<img src="/test/links/deep/dot.png" width="16">`,
		"`[[./00 Index]]` —":        `<span class="missing" title="Not found in this vault: ./00 Index">./00 Index</span>`,
		"`[[../../Twin]]` —":        `<span class="missing" title="Not found in this vault: ../../Twin">`,
		// from the vault root, strictly
		"`[[/Twin]]`:":                 `<a href="/test/Twin">/Twin</a>`,
		"`[[/links/deep/Twin]]`:":      `<a href="/test/links/deep/Twin">/links/deep/Twin</a>`,
		"`![[/links/deep/dot.png]]` —": `<img src="/test/links/deep/dot.png">`,
		"`[[/deep/Twin]]` —":           `<span class="missing" title="Not found in this vault: /deep/Twin">`,
		// Markdown links and images go the same way
		"`[relative](deep/Twin.md)`:":        `<a href="/test/links/deep/Twin">relative</a>`,
		"`[up](../Twin.md)`:":                `<a href="/test/Twin">up</a>`,
		"`[from the root](/links/Twin.md)`:": `<a href="/test/links/Twin">from the root</a>`,
		"`[by name](03%20Nested.md)` —":      `<a href="/test/sub/03%20Nested">by name</a>`,
		"`![blue](deep/dot.png)`:":           `<img src="/test/links/deep/dot.png" alt="blue">`,
		"`![gone](../dot.png)` —":            `<span class="missing missing-embed" title="Not found in this vault: ../dot.png">gone</span>`,
	} {
		// each case is one list item: what was written, in code, then what it became
		code := "<code>" + strings.NewReplacer("`", "", "&", "&amp;").Replace(strings.TrimRight(written, " —:,")) + "</code>"
		_, item, found := strings.Cut(here, "<li>"+code)
		item, _, _ = strings.Cut(item, "</li>")
		if !found || !strings.Contains(item, want) {
			t.Errorf("links/Here, %s\n got: %s\nwant: %s", written, item, want)
		}
	}
	// ...and from the root, the same words mean other files.
	expect("07 Links", body("/test/07%20Links"), []string{
		`next to this page: <a href="/test/Twin">Twin</a>`,
		`follows the path: <a href="/test/links/Twin">links/Twin</a>`,
		`red: <img src="/test/links/dot.png">`,
	}, nil)
	// Each Twin is linked from somewhere else, and the backlinks know it.
	// (the Twin in links/deep/ says [[../Twin]], which is the one in links/)
	expect("backlinks of the Twin in links/", aside(body("/test/links/Twin")), []string{"links/Here.md", "07 Links.md", "links/deep/Twin.md"}, nil)
	expect("backlinks of the Twin in the root", aside(body("/test/Twin")), []string{"links/Here.md", "07 Links.md", "links/deep/Twin.md"}, nil)
	expect("backlinks of the Twin in links/deep/", aside(body("/test/links/deep/Twin")), []string{"links/Here.md"}, []string{"07 Links.md"})

	// Aliases and headings. The target note's ids first: they are what links must hit.
	chapters := body("/test/links/Chapters")
	expect("heading ids", chapters, []string{
		`<h1 id="überblick">`, `<h2 id="maße--gewichte">`, `<h2 id="notes">`, `<h2 id="notes-1">`, `<h3 id="a-section">`, `<h2 id="bold-and-code">`,
	}, nil)
	aliases := body("/test/08%20Aliases%20and%20headings")
	const chaptersURL, noHeading = `<a href="/test/links/Chapters`, ` class="missing-heading" title="The note has no heading of this name"`
	expect("08 Aliases and headings", aliases, []string{
		chaptersURL + `">the chapters</a>`,
		chaptersURL + `#notes">alias and heading</a>`,
		`<img src="/test/links/dot.png" alt="the red dot">`,
		chaptersURL + `#notes">links/Chapters &gt; Notes</a>`,
		chaptersURL + `#maße--gewichte">links/Chapters &gt; Maße &amp; Gewichte</a>`,
		chaptersURL + `#überblick">links/Chapters &gt; überblick</a>`,
		chaptersURL + `#bold-and-code">`,
		chaptersURL + `#a-section">links/Chapters &gt; Notes &gt; A section</a>`,
		chaptersURL + `">links/Chapters &gt; ^para1</a>`,
		chaptersURL + `#no-such-heading"` + noHeading + `>`,
		`<span class="missing" title="Not found in this vault: Nowhere">`,
		`<a href="#alias">Alias</a>`, `<a href="#markdown-links">further down</a>`, `<a href="#nothing"` + noHeading + `>Nothing</a>`,
		chaptersURL + `#notes">text</a>`, chaptersURL + `#ma%C3%9Fe--gewichte">umlauts</a>`, `<a href="#alias">in this note</a>`,
	}, nil)
	if n := strings.Count(aliases, "missing-heading"); n != 2 {
		t.Errorf("08: %d links marked as pointing at a missing heading, want 2", n)
	}
	// every anchor the page links to exists in the note it points at
	for _, m := range regexp.MustCompile(`href="/test/links/Chapters#([^"]+)"( class="missing-heading")?`).FindAllStringSubmatch(aliases, -1) {
		id, _ := url.PathUnescape(m[1])
		if exists := strings.Contains(chapters, `id="`+id+`"`); exists == (m[2] != "") {
			t.Errorf("anchor %q: exists in the note = %v, marked missing = %v", id, exists, m[2] != "")
		}
	}

	// Embedded notes.
	embeds := body("/test/09%20Embedded%20notes")
	_, embeds, _ = strings.Cut(embeds, "<article>")
	embeds, _, _ = strings.Cut(embeds, "<aside>")
	expect("09 Embedded notes", embeds, []string{
		// the whole note, under its title, without its front matter
		`<div class="transclusion-title"><a href="/test/embeds/Recipe">Pancakes</a></div>`,
		`<p>Whisk <strong>250 g flour</strong>`, `<li>Fry in butter.</li>`,
		// its links and images are resolved from where it lives, the host's from the root
		`in <code>embeds/</code> — <a href="/test/embeds/Twin">Twin</a>`, `<img src="/test/links/dot.png" width="24">`,
		`is another note: <a href="/test/Twin">Twin</a>`,
		// one section: the first "Notes" with its subsection, not the second, not the rest
		`<a href="/test/links/Chapters#notes">Chapters &gt; Notes</a></div>`, "The first of two headings", "Below the first",
		// text around an embed, and an embed in a list
		"<p>Text before </p>", "<p> and text after: the paragraph is cut in two around\nthe embed.</p>", "<li>In a list item: ",
		// one level: Menu is embedded, the Recipe inside it is a link saying why
		`<a href="/test/embeds/Menu">Menu</a></div>`,
		`<a href="/test/embeds/Recipe" class="not-embedded" title="Not embedded here: notes are embedded one level deep">Recipe</a>`,
		// what stays a link
		`<a href="/test/links/Chapters#no-such-heading" class="missing-heading"`,
		`class="not-embedded" title="Not embedded: a note cannot embed itself">09 Embedded notes</a>`,
		`<span class="missing missing-embed" title="Not found in this vault: No such note">`,
		`<em><a href="/test/embeds/Twin">embeds/Twin</a></em>`,
		`as ever: <a href="/test/embeds/Recipe">embeds/Recipe</a>`,
	}, []string{"kitchen", "The second one", "Markup in a heading", "<p><div", "<p></p>"})
	if n := strings.Count(embeds, `<div class="transclusion">`); n != 6 {
		t.Errorf("09: %d embeds, want 6 (Recipe, Chapters#Notes, Twin twice, Flow, Menu)", n)
	}
	if n := strings.Count(embeds, "Whisk"); n != 1 {
		t.Errorf("09: the recipe is shown %d times; inside Menu it must be a link", n)
	}
	if strings.Count(embeds, "mermaid.min.js") != 1 || strings.Count(embeds, `<pre class="mermaid">`) != 1 {
		t.Error("09: the embedded diagram needs the script, once")
	}
	// the page's own heading ids are its own: nothing embedded brought an id along
	if ids := regexp.MustCompile(` id="([^"]*)"`).FindAllStringSubmatch(strings.Replace(embeds, ` id="note-stats"`, "", 1), -1); len(ids) != 7 {
		t.Errorf("09: %d ids on the page, want its own 7 headings: %v", len(ids), ids)
	}
	// On its own page Menu does embed the recipe, and an embed counts as a link.
	expect("Menu's own page", body("/test/embeds/Menu"), []string{`<div class="transclusion">`, "Whisk"}, nil)
	expect("backlinks of the recipe", aside(body("/test/embeds/Recipe")), []string{"09 Embedded notes.md", "embeds/Menu.md", "embeds/Twin.md"}, nil)

	// Search. Every page has the box; the results come best match first.
	expect("search box", index, []string{`<form class="search" action="/test/-/search" role="search">`, `name="q" value=""`}, nil)
	// rows cuts the result list into its rows and picks three things out of
	// each, tolerant of whatever else a row holds: badges came and went three
	// times while this test matched the markup exactly.
	type row struct{ title, path, percent string }
	var (
		rowTitle   = regexp.MustCompile(`<a href="[^"]*">([^<]*)</a>`)
		rowPath    = regexp.MustCompile(`<small>([^<]*)</small>`)
		rowPercent = regexp.MustCompile(`>(\d+%)<`)
	)
	rows := func(q string) []row {
		t.Helper()
		_, list, found := strings.Cut(body("/test/-/search?q="+url.QueryEscape(q)), `<ul class="list results">`)
		if !found {
			return nil
		}
		list, _, _ = strings.Cut(list, "</ul>")
		var out []row
		for _, chunk := range strings.Split(list, "<li>")[1:] {
			first := func(re *regexp.Regexp) string {
				if m := re.FindStringSubmatch(chunk); m != nil {
					return m[1]
				}
				return ""
			}
			out = append(out, row{first(rowTitle), first(rowPath), first(rowPercent)})
		}
		return out
	}
	results := func(q string) string {
		t.Helper()
		var found []string
		for _, r := range rows(q) {
			found = append(found, r.path)
		}
		return strings.Join(found, " | ")
	}
	// The ranking example, in full order: title, tag, heading, mention.
	if got, want := results("zeppelin"), "search/Zeppelin.md | search/Airships.md | search/History.md | 06 Search.md"; got != want {
		t.Errorf("ranking\n got: %s\nwant: %s", got, want)
	}
	// ...and with how well each matches, on the fixed scale the page explains.
	var percents []string
	for _, r := range rows("zeppelin") {
		percents = append(percents, r.title+" "+r.percent)
	}
	if got, want := strings.Join(percents, " | "), "Zeppelin 100% | Airships 30% | History 20% | 06 Search 13%"; got != want {
		t.Errorf("percentages\n got: %s\nwant: %s", got, want)
	}
	expect("filters alone show no percentage", body("/test/-/search?q="+url.QueryEscape("path:search/")), []string{"3 results for"}, []string{`class="match"`})

	// Page 06 quotes every query, so it is always found too. For these the
	// best match and the set of results matter, not how ties fall.
	for q, want := range map[string]string{
		"Zeppelin 1937": "search/History.md | 06 Search.md",
		`"rigid frame"`: "search/Zeppelin.md | 06 Search.md",
		`"frame rigid"`: "06 Search.md",
		"path:search/":  "search/Airships.md | search/History.md | search/Zeppelin.md",
		// 06 first: it says the word twice (link text and link address), 04 once.
		"tag:test checkerboard": "06 Search.md | 04 Missing attachments.md",
		"pixel":                 "attachments/pixel.png · attachment | 04 Missing attachments.md | 06 Search.md | sub/03 Nested.md",
		"versionNonce":          "06 Search.md", // never the drawings that contain it
	} {
		got := strings.Split(results(q), " | ")
		first := got[0]
		sort.Strings(got)
		wantSet := strings.Split(want, " | ")
		wantFirst := wantSet[0]
		sort.Strings(wantSet)
		if first != wantFirst || strings.Join(got, " | ") != strings.Join(wantSet, " | ") {
			t.Errorf("search %q\n got: %s first, of %v\nwant: %s first, of %v", q, first, got, wantFirst, wantSet)
		}
	}
	// Tasks: which notes, in what order, and the tasks themselves as the snippets.
	tasks := body("/test/-/search?q=" + url.QueryEscape(`task-todo:""`))
	if got := results(`task-todo:""`); got != "06 Search.md | 01 Formatting.md" {
		t.Errorf("open tasks: %s", got)
	}
	expect("open tasks", tasks, []string{
		`<span class="match">2 tasks</span>`, `<span class="match">1 task</span>`,
		`<p class="snippet task"><input type="checkbox" disabled> inflate the zeppelin</p>`,
		`<input type="checkbox" disabled> check the ballast</p>`, `<input type="checkbox" disabled> open</p>`,
	}, []string{"moor the zeppelin", "an example, not a task", `title="How well`})
	if got := results("task-done:zeppelin"); got != "06 Search.md" {
		t.Errorf("completed tasks: %s", got)
	}
	expect("a completed task, and a word beside the operator", body("/test/-/search?q="+url.QueryEscape(`task:"" zeppelin`)), []string{
		`<input type="checkbox" disabled checked> moor the zeppelin</p>`,
		`%</span> · 3 tasks</span>`,
	}, nil)
	if got := results(`task:""`); got != "06 Search.md | 01 Formatting.md" {
		t.Errorf("all tasks: %s", got)
	}
	if got := results("task-todo:ballast tag:test/missing"); got != "" {
		t.Errorf("a task filter and a tag filter must both hold: %s", got)
	}

	expect("search in properties", body("/test/-/search?q=dirigible"),
		[]string{"1 result for", `<p class="snippet">vessel: dirigible</p>`}, nil)
	searched := body("/test/-/search?q=" + url.QueryEscape(`"rigid frame"`))
	expect("search page", searched, []string{
		`value="&#34;rigid frame&#34;"`, // the box keeps the query
		`<p class="snippet">An airship with a rigid frame.</p>`,
		"2 results for",
	}, nil)
	expect("search escapes the query", body("/test/-/search?q="+url.QueryEscape(`<b>x</b>`)),
		[]string{`value="&lt;b&gt;x&lt;/b&gt;"`, "0 results for “&lt;b&gt;x&lt;/b&gt;”"}, []string{"<b>x"})
	expect("search without a query", body("/test/-/search"), []string{`<details class="help" open>`, "tag:project"}, []string{"results for"})

	expect("v1.2 plan", body("/test/v1.2%20plan"), []string{"this body text must still render"}, nil)

	tags := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(body("/test/-/tags"), "")
	expect("tags", tags, []string{"#test 13", "#test/slides 1", "#test/keyboard 1", "#test/print 1", "#test/links 2", "#test/embeds 1", "#test/missing 1", "#test/excalidraw 1", "#test/search 1", "#test/code 1", "#test/nested 1", "#überprüfung 1"}, []string{"notatag"})
	expect("tag page", body("/test/-/tag/test"), []string{"01 Formatting.md", "02 Code and Diagrams.md", "sub/03 Nested.md", "00 Index.md"}, []string{"v1.2"})
	expect("unicode tag", body("/test/-/tag/%C3%BCberpr%C3%BCfung"), []string{"00 Index.md"}, nil)

	expect("listing", body("/test/"), []string{
		`<a href="/test/attachments/">attachments/</a>`, // a folder's URL ends in a slash
		`<a href="/test/00%20Index">Feature Test Index</a>`,
	}, nil)
}

// The index page is how a person finds the test pages; a page that cannot be
// reached from it is a feature nobody looks at. A numbered page must be in the
// Pages table itself; what it brings along (drawings, helper notes) only has
// to be reachable through it.
func TestFixtureIndexListsEveryPage(t *testing.T) {
	const root, indexPage = "../../testdata/vault", "00 Index.md"
	idx := index.New(root, "/test")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	next := map[string][]string{}
	for _, link := range idx.Graph().Links {
		next[link.Source] = append(next[link.Source], link.Target)
	}
	direct, reachable := map[string]bool{}, map[string]bool{indexPage: true}
	for _, p := range next[indexPage] {
		direct[p] = true
	}
	for queue := []string{indexPage}; len(queue) > 0; queue = queue[1:] {
		for _, p := range next[queue[0]] {
			if !reachable[p] {
				reachable[p] = true
				queue = append(queue, p)
			}
		}
	}
	pages := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".md" {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		pages++
		numbered := !strings.Contains(rel, "/") && rel[0] >= '0' && rel[0] <= '9'
		switch {
		case numbered && !direct[rel] && rel != indexPage:
			t.Errorf("%q is not in the Pages table of %q", rel, indexPage)
		case !reachable[rel]:
			t.Errorf("%q cannot be reached from %q by following links", rel, indexPage)
		}
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
	rec := httptestGet(Home(two, "1.2.3"), "/")
	if b, _ := io.ReadAll(rec.Body); rec.StatusCode != 200 || !strings.Contains(string(b), `<a href="/test/">test</a>`) ||
		!strings.Contains(string(b), `<span class="about">markdown-webdav-backend 1.2.3</span>`) {
		t.Errorf("two vaults: status %d, body %s", rec.StatusCode, b)
	}
	rec = httptestGet(Home(two[:1], "1.2.3"), "/")
	if rec.StatusCode != http.StatusFound || rec.Header.Get("Location") != "/notes/" {
		t.Errorf("one vault: status %d, location %q", rec.StatusCode, rec.Header.Get("Location"))
	}
}
