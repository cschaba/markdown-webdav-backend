package web

import (
	"io"
	"regexp"
	"strings"
	"testing"
)

func TestSlideShowPage(t *testing.T) {
	get := serve(t, "test", "../../testdata/vault")
	text := func(p string) string {
		t.Helper()
		res := get(p)
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("GET %s: %d", p, res.StatusCode)
		}
		return string(b)
	}
	deck := text("/test/-/slides?path=12%20Slides.md")
	if n := strings.Count(deck, `<section class="slide"`); n != 8 {
		t.Fatalf("%d slides, want 8", n)
	}
	for _, want := range []string{
		`<title>12 Slides – Slides</title>`,
		`style="--slide-w: 1280; --slide-h: 720;"`,
		// every slide says which one it is
		`<section class="slide" id="slide-1" aria-roledescription="slide" aria-label="Slide 1 of 8">`,
		`id="slide-8" aria-roledescription="slide" aria-label="Slide 8 of 8">`,
		`<h1 id="a-deck-of-slides">A deck of slides</h1>`,
		// changes are announced; the controls are real buttons with names
		`id="deck-counter" aria-live="polite" aria-atomic="true">Slide 1 of 8</span>`,
		`<button type="button" id="deck-prev" aria-label="Previous slide"`, `<button type="button" id="deck-next" aria-label="Next slide"`,
		`<a href="/test/12%20Slides" id="deck-exit">Exit</a>`,
		`<a href="/test/-/export?path=12%20Slides.md&amp;format=slides">PDF</a>`,
		`<img src="/test/links/dot.png" width="120">`,
		`<pre class="mermaid">`, `<script src="/-/slides.js"></script>`,
		// what is not a separator stays in its slide
		"<hr>", `<h2 id="like-this-one">Like this one</h2>`,
	} {
		if !strings.Contains(deck, want) {
			t.Errorf("slide show lacks %q", want)
		}
	}
	if n := strings.Count(deck, "mermaid.min.js"); n != 1 {
		t.Errorf("the diagram script is included %d times", n)
	}
	for _, reject := range []string{"<header>", `class="search"`, "Backlinks", "<details", "test/slides", "keys.js"} {
		if strings.Contains(deck, reject) {
			t.Errorf("slide show contains %q", reject)
		}
	}
	// a note without separators is a deck of one; no diagram, no script
	if one := text("/test/-/slides?path=01%20Formatting.md"); strings.Count(one, `<section class="slide"`) != 1 || strings.Contains(one, "mermaid.min.js") {
		t.Error("a note without separators must be one slide")
	}
	for _, q := range []string{"path=drawings/Sketch.excalidraw.md", "path=attachments/pixel.png", "path=10%20Printing", "path=../x.md", "path=.git/config", "path=Nope.md", ""} {
		if code := get("/test/-/slides?" + q).StatusCode; code != 404 {
			t.Errorf("slides?%s: %d, want 404", q, code)
		}
	}

	// the link to it: only where there are slides to show
	if page := text("/test/12%20Slides"); !strings.Contains(page, `<footer><a href="/test/-/slides?path=12%20Slides.md">Slide show</a><a href="/test/-/export`) {
		t.Errorf("no Slide show link on the deck's page")
	}
	if page := text("/test/01%20Formatting"); strings.Contains(page, "Slide show") {
		t.Error("a note without separators offers a slide show")
	}
}

func TestExportAsSlides(t *testing.T) {
	get := serve(t, "test", "../../testdata/vault")
	text := func(q string) string {
		t.Helper()
		res := get("/test/-/export?" + q)
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("export?%s: %d", q, res.StatusCode)
		}
		return string(b)
	}
	out := text("path=12+Slides.md&format=slides")
	for _, want := range []string{
		`<style>@page { size: 1280px 720px; margin: 0; }</style>`, // the slide's own size, in its own unit
		`<option value="slides" selected>Slides</option>`,
		`<label>Slide size <select name="size"><option selected>16:9</option><option>4:3</option><option>A4</option></select>`,
		`<main class="paper sheets" id="sheets" style="--slide-w: 1280; --slide-h: 720;">`,
		"1 note, 8 slides", `<script src="/-/slides.js"></script>`, `<script src="/-/print.js"></script>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("slides export lacks %q", want)
		}
	}
	if n := strings.Count(out, `<section class="slide">`); n != 8 {
		t.Errorf("%d slides, want 8", n)
	}
	if strings.Contains(out, "Page size") || strings.Contains(out, `class="doc"`) || strings.Count(out, "mermaid.min.js") != 1 {
		t.Error("the slide format must not show book settings or book pages, and load the diagram script once")
	}
	for q, want := range map[string]string{
		"path=12+Slides.md&format=slides&set=1&size=4:3": `size: 960px 720px; margin: 0;`,
		"path=12+Slides.md&format=slides&set=1&size=a4":  `size: 1123px 794px; margin: 0;`,
		"path=12+Slides.md&format=slides&set=1&size=A5":  `size: 1280px 720px; margin: 0;`,  // a book size is no slide size
		"path=12+Slides.md&format=nonsense":              `size: A4 portrait; margin: 22mm`, // an unknown format is a book
	} {
		if got := text(q); !strings.Contains(got, want) {
			t.Errorf("export?%s lacks %q", q, want)
		}
	}
	// sub pages: every note's slides, in reading order
	book := text("path=10+Printing.md&format=slides&sub=1")
	if n := strings.Count(book, `<section class="slide">`); n != 5 || !strings.Contains(book, "5 notes, 5 slides") {
		t.Errorf("sub pages as slides: %d slides", n)
	}
	if i, j := strings.Index(book, "A short chapter"), strings.Index(book, "Numbers count as numbers"); i < 0 || j < i {
		t.Error("reading order")
	}

	// The two formats remember their sizes apart; the format itself is not remembered.
	b := &browser{t: t, cookieName: exportCookie, h: handlerFor(t, "test", "../../testdata/vault")}
	open := func(q string) string { _, body, _ := b.do("GET", "/test/-/export?"+q); return body }
	selected := func(html string) string {
		return strings.Join(regexp.MustCompile(`<option[^>]*selected>([^<]*)</option>`).FindAllString(html, -1), "")
	}
	open("path=12+Slides.md&set=1&format=book&size=A5")
	open("path=12+Slides.md&set=1&format=slides&size=4:3")
	if b.cookie.Value != "size=A5&ssize=4%3A3" {
		t.Errorf("cookie = %q", b.cookie.Value)
	}
	// Switching the format in the form sends the other format's size along. It
	// must neither apply nor overwrite what is remembered.
	if got := selected(open("path=12+Slides.md&set=1&format=book&size=4:3")); !strings.Contains(got, ">A5<") || b.cookie.Value != "size=A5&ssize=4%3A3" {
		t.Errorf("switching to the book with a slide size: %s, cookie %q", got, b.cookie.Value)
	}
	if got := selected(open("path=12+Slides.md&set=1&format=slides&size=A5")); !strings.Contains(got, ">4:3<") || b.cookie.Value != "size=A5&ssize=4%3A3" {
		t.Errorf("switching to slides with a book size: %s, cookie %q", got, b.cookie.Value)
	}
	if got := selected(open("path=12+Slides.md")); !strings.Contains(got, ">Book<") || !strings.Contains(got, ">A5<") {
		t.Errorf("a plain link opens the book format with its size: %s", got)
	}
	if got := selected(open("path=12+Slides.md&format=slides")); !strings.Contains(got, ">Slides<") || !strings.Contains(got, ">4:3<") {
		t.Errorf("the slide format opens with its own size: %s", got)
	}
}
