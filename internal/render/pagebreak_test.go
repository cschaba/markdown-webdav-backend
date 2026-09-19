package render

import (
	"strings"
	"testing"
)

func TestPageBreaks(t *testing.T) {
	const br = `<div class="page-break" role="separator" aria-label="Page break"></div>`
	src := "one\n\n\\pagebreak\n\ntwo\n\n\\newpage\n\nthree\n\n" +
		`<div style="page-break-after: always;"></div>` + "\n\nfour\n\n" +
		`<div class="x" style='color:red; break-before:page'></div>` + "\n\nfive\n\n" +
		// none of these is a page break
		"A sentence with \\pagebreak in it.\n\n" +
		"```\n\\pagebreak\n```\n\n" +
		"`\\newpage`\n\n" +
		`<div style="color: red">not a break</div>` + "\n\n" +
		`<div style="page-break-after: always;">with content</div>` + "\n\n" +
		"# Heading\n\n\\pagebreak\n\nunder a heading\n"
	html, _, err := New(fakeLinks{}, "/-/tag/").Render([]byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	if n := strings.Count(out, br); n != 5 {
		t.Errorf("%d page breaks, want 5 (two words, two divs, one inside a section)\n%s", n, out)
	}
	for _, want := range []string{
		"<p>one</p>\n" + br + "\n<p>two</p>\n" + br + "\n<p>three</p>\n" + br + "\n<p>four</p>\n" + br + "\n<p>five</p>",
		"<p>A sentence with \\pagebreak in it.</p>",
		"<code>\\newpage</code>",
		"</summary>\n" + br + "\n<p>under a heading</p>", // breaks work inside foldable sections
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q\n%s", want, out)
		}
	}
	// raw HTML stays out, page break or not
	if strings.Contains(out, "color") || strings.Contains(out, "with content") || strings.Contains(out, "page-break-after") {
		t.Errorf("raw HTML reached the page:\n%s", out)
	}
}
