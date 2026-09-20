package render

import (
	"strings"
	"testing"
)

func TestComments(t *testing.T) {
	r := New(fakeLinks{"My Note": "/folder/My%20Note"}, "/-/tag/")
	for src, want := range map[string]string{
		// inline, and its neighbours left as they stand
		"before %%hidden%% after":     "<p>before  after</p>",
		"%%whole line%%":              "", // no empty paragraph left behind
		"%%a%% %%b%%":                 "",
		"a%%b%%c":                     "<p>ac</p>",
		"%%one%% and %%two%%":         "<p> and </p>",
		"%%it may hold %% signs%% no": "<p> signs%% no</p>",
		// an opener with no closer on the line is no comment
		"before %%never closed":       "<p>before %%never closed</p>",
		"stray %% sign":               "<p>stray %% sign</p>",
		"`%%code%%` stays":            "<p><code>%%code%%</code> stays</p>",
		"```\n%%code%%\n```":          "<pre><code>%%code%%\n</code></pre>",
		"%%\nblock\n%%":               "",
		"a\n\n%%\nblock\n%%\n\nb":     "<p>a</p>\n<p>b</p>",
		"%%\n# Heading\n\n- item\n%%": "",
		// a block comment that is never closed runs to the end
		"a\n\n%%\nrest of the note": "<p>a</p>",
	} {
		html, _, err := r.Render([]byte(src), "")
		if got := strings.TrimSpace(string(html)); err != nil || got != want {
			t.Errorf("%q\n got: %q\nwant: %q (err %v)", src, got, want, err)
		}
	}
}

// What a comment holds is not in the note at all: not a tag, not a link, not
// in the word count. A tag or a link found there would show on the Tags page
// or as a backlink, where nobody would connect it with a comment.
func TestCommentsAreNotPartOfTheNote(t *testing.T) {
	r := New(fakeLinks{"My Note": "/folder/My%20Note"}, "/-/tag/")
	const src = "Visible #shown [[My Note]] words.\n\n" +
		"%%\n#hidden [[My Note]] more words here\n%%\n\n" +
		"Tail %%#also-hidden [[My Note]]%% end."
	html, meta, err := r.Render([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "hidden") {
		t.Errorf("a comment reached the page:\n%s", html)
	}
	if got := strings.Join(meta.Tags, ","); got != "shown" {
		t.Errorf("tags = %q, want only the one outside the comment", got)
	}
	if len(meta.Links) != 1 {
		t.Errorf("links = %v, want only the one outside the comment", meta.Links)
	}
	// "Visible #shown My Note words." and "Tail end."
	if meta.Words != 7 {
		t.Errorf("words = %d, want 7: a comment is not read", meta.Words)
	}
}
