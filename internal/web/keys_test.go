package web

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestPagesAreBuiltForTheKeyboard(t *testing.T) {
	get := serve(t, "test", "../../testdata/vault")
	for _, p := range []string{"/test/01%20Formatting", "/test/", "/test/-/tags", "/test/-/search?q=test", "/test/-/graph", "/test/drawings/Sketch.excalidraw"} {
		res := get(p)
		b, _ := io.ReadAll(res.Body)
		html := string(b)
		for _, want := range []string{
			// the first thing Tab reaches, and where it leads
			`<body`, `<a class="skip" href="#content">Skip to content</a>`, `<main id="content" tabindex="-1">`,
			// landmarks have names, so they can be told apart
			`<nav class="crumbs" aria-label="Breadcrumb">`, `<nav aria-label="Site">`,
			// the help: a modal dialog with a name, reachable without a shortcut, with the switch in it
			`<dialog id="keys-help" aria-labelledby="keys-title">`, `<h2 id="keys-title">Keyboard shortcuts</h2>`,
			`<button type="button" id="keys-button" hidden>`, `<input type="checkbox" id="keys-enabled" checked>`,
			`<script src="/-/keys.js"></script>`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s lacks %q", p, want)
			}
		}
		if skip, header := strings.Index(html, `class="skip"`), strings.Index(html, "<header>"); skip < 0 || skip > header {
			t.Errorf("%s: the skip link must come before the header", p)
		}
	}
	// The export page is paper, not a page to move around in.
	res := get("/test/-/export?path=10%20Printing.md")
	if b, _ := io.ReadAll(res.Body); strings.Contains(string(b), "keys.js") {
		t.Error("the export page loads the keyboard script")
	}
}

// The help in the browser is made from the script's own list of keys and cannot
// drift. The table in the README can: this fails when a key is added, removed
// or renamed without it.
func TestReadmeListsEveryKey(t *testing.T) {
	script, err := os.ReadFile("static/keys.js")
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(readme), "## Keyboard")
	if !found {
		t.Fatal("README has no Keyboard section")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	inReadme := map[string]bool{}
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(section, -1) {
		inReadme[m[1]] = true
	}
	var keys []string
	for _, m := range regexp.MustCompile(`\{ keys: "([^"]+)"(?:, alt: "([^"]+)")?`).FindAllStringSubmatch(string(script), -1) {
		keys = append(keys, m[1])
		if m[2] != "" {
			keys = append(keys, m[2])
		}
	}
	if len(keys) < 20 {
		t.Fatalf("found only %d keys in keys.js; has the format of BINDINGS changed?", len(keys))
	}
	inScript := map[string]bool{}
	for _, k := range keys {
		inScript[k] = true
		if !inReadme[k] {
			t.Errorf("keys.js has %q, the README's Keyboard section does not", k)
		}
	}
	// and the other way round, for what the README shows as a key in its table
	for _, m := range regexp.MustCompile("(?m)^\\| ((?:`[^`]+`(?: / )?)+) \\|").FindAllStringSubmatch(section, -1) {
		for _, k := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(m[1], -1) {
			if !inScript[k[1]] {
				t.Errorf("the README lists the key %q, keys.js has no such key", k[1])
			}
		}
	}
}
