package web

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// A note and a folder of the same name: the note has the URL, the folder the
// URL with a slash. Links lead to notes, so without this [[10 Printing]] showed
// a folder listing.
func TestNoteAndFolderOfTheSameName(t *testing.T) {
	get := serve(t, "test", "../../testdata/vault")
	text := func(p string) string {
		res := get(p)
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("GET %s: %d", p, res.StatusCode)
		}
		return string(b)
	}
	note, folder := text("/test/10%20Printing"), text("/test/10%20Printing/")
	if !strings.Contains(note, "<article>") || !strings.Contains(note, "Printing and PDF export") {
		t.Error("/test/10 Printing is not the note")
	}
	if strings.Contains(folder, "<article>") || !strings.Contains(folder, `<a href="/test/10%20Printing/Chapter%201">Chapter 1</a>`) ||
		!strings.Contains(folder, `<a href="/test/10%20Printing/Appendix/">Appendix/</a>`) {
		t.Errorf("/test/10 Printing/ is not the folder:\n%s", folder)
	}
	// the breadcrumb of a note in the folder leads to the folder, not the note
	if chapter := text("/test/10%20Printing/Chapter%201"); !strings.Contains(chapter, `<a href="/test/10%20Printing/">10 Printing</a> / <a href="/test/10%20Printing/Chapter%201">Chapter 1</a>`) {
		t.Errorf("breadcrumb:\n%s", chapter[:strings.Index(chapter, "</header>")])
	}
	// a folder without a namesake answers with and without the slash
	for _, p := range []string{"/test/attachments", "/test/attachments/"} {
		if !strings.Contains(text(p), "pixel.png") {
			t.Errorf("%s is not the folder listing", p)
		}
	}
}

func TestExportPage(t *testing.T) {
	get := serve(t, "test", "../../testdata/vault")
	page := func(query string) string {
		t.Helper()
		res := get("/test/-/export?" + query)
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			t.Fatalf("export?%s: %d", query, res.StatusCode)
		}
		return string(b)
	}
	titles := func(html string) string {
		var out []string
		for _, m := range regexp.MustCompile(`<section class="doc">\s*(?:<h1 class="doc-title">([^<]*)</h1>)?`).FindAllStringSubmatch(html, -1) {
			out = append(out, m[1])
		}
		return strings.Join(out, "|")
	}

	one := page("path=10+Printing.md")
	for _, want := range []string{
		`<style>@page { size: A4 portrait; margin: 22mm 20mm 26mm; }</style>`,
		`style="width: 210mm; padding: 22mm 20mm 26mm;"`,
		`<option selected>A4</option>`,
		`<input type="checkbox" name="sub" value="1">`, // sub pages exist, not included
		`<a class="back" href="/test/10%20Printing">`,
		`<div class="page-break"`, `src="/-/print.js"`,
	} {
		if !strings.Contains(one, want) {
			t.Errorf("export of one note lacks %q", want)
		}
	}
	// no site furniture in the document at all, not merely hidden
	// ... and no page numbering of our own: it worked in Chromium only, and was taken out
	for _, reject := range []string{"<header>", "<aside>", `class="search"`, "Backlinks", `class="tags"`, "<footer>", `name="start"`, "counter", "data-numbered"} {
		if strings.Contains(one, reject) {
			t.Errorf("export contains %q", reject)
		}
	}
	if got := titles(one); got != "" { // the note opens with its own heading
		t.Errorf("titles = %q", got)
	}

	book := page("path=10+Printing.md&sub=1&size=a5&start=41") // a leftover "start" is ignored
	for _, want := range []string{
		`<style>@page { size: A5 portrait; margin: 17mm 15mm 21mm; }</style>`,
		`<option selected>A5</option>`, `name="sub" value="1" checked`,
		`<section class="doc contents">`, "<li>Sources</li>",
	} {
		if !strings.Contains(book, want) {
			t.Errorf("export with sub pages lacks %q", want)
		}
	}
	// reading order; only the note without a heading of its own is given its title
	if got := titles(book); got != "||||Sources" {
		t.Errorf("titles = %q", got)
	}
	if i, j := strings.Index(book, "A short chapter"), strings.Index(book, "Numbers count as numbers"); i < 0 || j < i {
		t.Error("Chapter 1 must come before Chapter 10")
	}

	if got := page("path=10+Printing/Chapter+1.md"); !strings.Contains(got, `class="disabled"`) || !strings.Contains(got, "disabled> Include sub pages") {
		t.Error("a note without sub pages must not offer them")
	}
	// a folder: its notes; with sub pages, those of the folders below too
	if got := page("path=10+Printing"); strings.Contains(got, "A note in a subfolder") || !strings.Contains(got, "A short chapter") ||
		!strings.Contains(got, `<a class="back" href="/test/10%20Printing/">`) {
		t.Error("folder export")
	}
	if got := page("path=10+Printing&sub=1"); !strings.Contains(got, "A note in a subfolder") {
		t.Error("folder export with sub pages")
	}
	// Settings apply without a button; the button is only there without script.
	if !strings.Contains(one, `<noscript><button type="submit">Apply</button></noscript>`) || !strings.Contains(one, `name="set" value="1"`) {
		t.Error("the Apply button must be a fallback only, and the form must say that its values are a choice")
	}

	// whatever is asked for, the page rule is made of known values only
	bad := page("path=10+Printing.md&size=%3C/style%3E%3Cscript%3E&sub=1;}body{display:none")
	if !strings.Contains(bad, `<style>@page { size: A4 portrait; margin: 22mm 20mm 26mm; }</style>`) || strings.Contains(bad, "<script>") && strings.Contains(bad, "display:none") {
		t.Error("query values reached the stylesheet")
	}
	for _, q := range []string{"path=.git/config", "path=../../etc/passwd", "path=No+such+note.md", "path=attachments/pixel.png"} {
		if code := get("/test/-/export?" + q).StatusCode; code != 404 {
			t.Errorf("export?%s: %d, want 404", q, code)
		}
	}
	// the link that leads here
	for p, want := range map[string]string{
		"/test/10%20Printing":              `<footer><a href="/test/-/export?path=10%20Printing.md">Export to PDF</a></footer>`,
		"/test/10%20Printing/":             `<footer><a href="/test/-/export?path=10%20Printing">Export to PDF</a></footer>`,
		"/test/":                           `<footer><a href="/test/-/export?path=">Export to PDF</a></footer>`,
		"/test/drawings/Sketch.excalidraw": `/test/-/export?path=drawings%2FSketch.excalidraw.md`,
	} {
		res := get(p)
		b, _ := io.ReadAll(res.Body)
		if !strings.Contains(string(b), want) {
			t.Errorf("%s lacks %q", p, want)
		}
	}
	if res := get("/test/-/tags"); true {
		b, _ := io.ReadAll(res.Body)
		if strings.Contains(string(b), "Export to PDF") {
			t.Error("the tags page is not something to export")
		}
	}
}

func TestExportSettingsAreRemembered(t *testing.T) {
	b := &browser{t: t, cookieName: exportCookie}
	b.h = handlerFor(t, "test", "../../testdata/vault")
	open := func(query string) string {
		t.Helper()
		code, body, _ := b.do("GET", "/test/-/export?"+query)
		if code != 200 {
			t.Fatalf("export?%s: %d", query, code)
		}
		return body
	}
	selected := func(html string) string {
		m := regexp.MustCompile(`<option selected>([^<]*)</option>`).FindStringSubmatch(html)
		sub := "sub off"
		if strings.Contains(html, `name="sub" value="1" checked`) {
			sub = "sub on"
		}
		return m[1] + ", " + sub
	}

	// a plain link, nothing chosen yet: the defaults, and nothing is stored
	if got := selected(open("path=10+Printing.md")); got != "A4, sub off" || b.cookie != nil {
		t.Errorf("first visit: %s, cookie %v", got, b.cookie)
	}
	// the form was changed: that is the choice, and it is kept
	if got := selected(open("path=10+Printing.md&set=1&size=A5&sub=1")); got != "A5, sub on" {
		t.Errorf("chosen: %s", got)
	}
	if c := b.cookie; c == nil || c.Value != "size=A5&sub=1" || c.Path != "/test/-/export" || !c.HttpOnly {
		t.Fatalf("cookie: %+v", c)
	}
	// the next export, of another note, through its footer link: as last chosen
	if got := selected(open("path=09+Embedded+notes.md")); got != "A5, sub on" {
		t.Errorf("next export: %s", got)
	}
	// unticking the switch arrives as a missing "sub": with set=1 that means off
	if got := selected(open("path=10+Printing.md&set=1&size=A5")); got != "A5, sub off" || b.cookie.Value != "size=A5" {
		t.Errorf("sub pages switched off: %s, cookie %q", got, b.cookie.Value)
	}
	// only known values are stored, and a damaged cookie is ignored
	open("path=10+Printing.md&set=1&size=%3Cscript%3E&sub=yes")
	if b.cookie.Value != "size=A4" {
		t.Errorf("stored %q", b.cookie.Value)
	}
	for _, value := range []string{"%zz", "size=Poster&sub=maybe", strings.Repeat("x", 3000)} {
		b.cookie = &http.Cookie{Name: exportCookie, Value: value}
		if got := selected(open("path=10+Printing.md")); got != "A4, sub off" {
			t.Errorf("cookie %.20q: %s", value, got)
		}
	}
}
