package render

import (
	"strings"
	"testing"
)

func TestRenderSlides(t *testing.T) {
	links := embedVault(map[string]string{"Part": "# Embedded\n\ntext\n\n---\n\nnot a slide of the host\n"})
	src := "---\ntitle: Deck\ntags: [talk]\n---\n" +
		"# First\n\nintro [[Part]]\n\n---\n\n" +
		"## Second\n\n- a\n- b\n\n```mermaid\ngraph TD; A-->B;\n```\n\n---\n\n" +
		"Setext heading\n---\n\n> quote\n>\n> ---\n>\n> still the quote\n\n" + // neither of these separates slides
		"\\pagebreak\n\n![[Part]]\n\n***\n\n" + // *** is a thematic break too, as in Obsidian
		"Last[^1]\n\n[^1]: a footnote\n"
	deck, err := New(links, "/-/tag/").RenderSlides([]byte(src), "Deck.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 4 || deck.Meta.Slides != 4 {
		t.Fatalf("%d slides, Meta says %d; want 4\n%q", len(deck.Slides), deck.Meta.Slides, deck.Slides)
	}
	for i, want := range [][]string{
		{`<h1 id="first">First</h1>`, `<a href="/Part">Part</a>`},
		{`<h2 id="second">Second</h2>`, "<li>a</li>", `<pre class="mermaid">`},
		{`<h2 id="setext-heading">Setext heading</h2>`, "<blockquote>", "<hr>", "still the quote", `<div class="transclusion">`},
		{"Last", `class="footnotes"`},
	} {
		for _, w := range want {
			if !strings.Contains(string(deck.Slides[i]), w) {
				t.Errorf("slide %d lacks %q\n%s", i+1, w, deck.Slides[i])
			}
		}
	}
	all := ""
	for _, s := range deck.Slides {
		all += string(s)
	}
	for _, reject := range []string{"page-break", "mermaid.min.js", "Deck", "talk"} {
		if strings.Contains(all, reject) {
			t.Errorf("slides contain %q: no page breaks, no script, no front matter", reject)
		}
	}
	// The deck's own headings are not foldable. (An embedded note is rendered
	// the usual way and keeps its sections; that is what section embeds use.)
	if own := string(deck.Slides[0] + deck.Slides[1] + deck.Slides[3]); strings.Contains(own, "<details") {
		t.Errorf("the deck's own headings must not fold:\n%s", own)
	}
	if !deck.Mermaid {
		t.Error("a deck with a diagram must say that it needs the script")
	}
	// the embedded note keeps its own rule as a rule: one level, one deck
	if strings.Count(string(deck.Slides[2]), "not a slide of the host") != 1 {
		t.Errorf("the embedded note must be shown whole on its slide:\n%s", deck.Slides[2])
	}

	// the ordinary page of the same note knows how many slides it would make
	_, meta, _ := New(links, "/-/tag/").Render([]byte(src), "Deck.md")
	if meta.Slides != 4 {
		t.Errorf("Render: Meta.Slides = %d, want 4", meta.Slides)
	}
	if one, _ := New(links, "/-/tag/").RenderSlides([]byte("just a note\n"), "N.md"); len(one.Slides) != 1 || one.Meta.Slides != 1 || one.Mermaid {
		t.Errorf("a note without a separator: %+v", one)
	}
	// a deck whose only diagram is in an embedded note still needs the script
	flow := embedVault(map[string]string{"Flow": "```mermaid\ngraph TD; A-->B;\n```\n"})
	if d, _ := New(flow, "/-/tag/").RenderSlides([]byte("a\n\n---\n\n![[Flow]]\n"), "N.md"); !d.Mermaid {
		t.Error("diagram in an embedded note: the script is needed")
	}
}
