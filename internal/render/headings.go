package render

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
)

// Heading anchors. A link to a heading, [[Note#Some Heading]], only works if
// the link and the heading arrive at the same id. goldmark's own ids drop
// every non-ASCII character ("Überprüfung" becomes "berprfung"), and a second
// implementation guessing at them had already drifted. So there is one
// function, Slug, and both sides use it: headingIDs gives it to the parser for
// the headings, anchor applies it to a link's fragment.

// Slug turns a heading's text into its id: lowercased, letters and digits of
// any script kept, spaces, "-" and "_" as "-", everything else dropped.
func Slug(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "heading"
	}
	return b.String()
}

// anchor returns the id in the page a link's fragment means, or "" if it names
// none. Of a nested reference, [[Note#Chapter#Section]], the last part counts:
// that is the heading to land on. A fragment beginning with "^" names a block
// rather than a heading, see blockref.go.
func anchor(fragment string) string {
	if i := strings.LastIndex(fragment, "#"); i >= 0 {
		fragment = fragment[i+1:]
	}
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(fragment, BlockAnchorPrefix); ok {
		return blockAnchor(after)
	}
	return Slug(fragment)
}

// headingIDs implements parser.IDs with Slug. A repeated heading gets "-1",
// "-2" appended, so a link by name lands on the first, as in Obsidian.
type headingIDs struct{ used map[string]bool }

func newHeadingIDs() *headingIDs { return &headingIDs{used: map[string]bool{}} }

func (h *headingIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	slug := Slug(string(value))
	id := slug
	for i := 1; h.used[id]; i++ {
		id = slug + "-" + strconv.Itoa(i)
	}
	h.used[id] = true
	return []byte(id)
}

func (h *headingIDs) Put(value []byte) { h.used[string(value)] = true }
