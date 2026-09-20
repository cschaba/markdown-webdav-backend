package render

import (
	"strings"
	"testing"
)

// Callouts and ==highlight== are two goldmark extensions, so what is tested
// here is what this project needs of them and would notice if it changed:
// which element a callout becomes, that folding needs no script, and that
// what stands inside one is part of the note like any other text.
func TestCallouts(t *testing.T) {
	r := New(fakeLinks{"My Note": "/folder/My%20Note"}, "/-/tag/")
	for name, tc := range map[string]struct {
		src          string
		want, reject []string
	}{
		"plain": {
			src:    "> [!note] A title\n> Body.",
			want:   []string{`<div class="callout callout-note`, `data-callout="note"`, "A title", "<p>Body.</p>"},
			reject: []string{"<details"},
		},
		"closed": {
			src:    "> [!tip]- Folded\n> Body.",
			want:   []string{"<details", `class="callout-title"`},
			reject: []string{"<script", " open>"},
		},
		"open": {
			src:  "> [!tip]+ Folded\n> Body.",
			want: []string{"<details", `data-callout="tip" open>`},
		},
		"quote stays a quote": {
			src:    "> Just a quote.",
			want:   []string{"<blockquote>"},
			reject: []string{"callout"},
		},
	} {
		html, _, err := r.Render([]byte(tc.src), "")
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range tc.want {
			if !strings.Contains(string(html), w) {
				t.Errorf("%s lacks %q:\n%s", name, w, html)
			}
		}
		for _, w := range tc.reject {
			if strings.Contains(string(html), w) {
				t.Errorf("%s holds %q:\n%s", name, w, html)
			}
		}
	}

	// A callout is part of the note: its tags are tags, its links are links
	// (and so make backlinks), and its words are counted - in the title too.
	_, meta, err := r.Render([]byte("> [!note] About [[My Note]]\n> #topic and more words."), "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(meta.Tags, ",") != "topic" || strings.Join(meta.Links, ",") != "My Note" || meta.Words != 7 { // "About My Note" and "#topic and more words."
		t.Errorf("a callout's content: tags %v, links %v, words %d", meta.Tags, meta.Links, meta.Words)
	}
}

func TestHighlight(t *testing.T) {
	for src, want := range map[string]string{
		"a ==marked== b":       "<p>a <mark>marked</mark> b</p>",
		"==**bold** inside==":  "<p><mark><strong>bold</strong> inside</mark></p>",
		"2 == 2 and a = b":     "<p>2 == 2 and a = b</p>",
		"`==code==` untouched": "<p><code>==code==</code> untouched</p>",
	} {
		html, _, err := New(fakeLinks{}, "/-/tag/").Render([]byte(src), "")
		if got := strings.TrimSpace(string(html)); err != nil || got != want {
			t.Errorf("%q\n got: %q\nwant: %q (err %v)", src, got, want, err)
		}
	}
}
