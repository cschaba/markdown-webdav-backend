package render

import (
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"go.abhg.dev/goldmark/mermaid"
	"go.abhg.dev/goldmark/wikilink"
)

// countText counts the words and characters a reader sees in a note: its text,
// code included, the labels of its links. Not counted: front matter, markup,
// %%comments%%, the source of diagrams, what images and embedded notes show -
// an embed is another note's text. Obsidian's own count is roughly the same,
// but not exactly: it counts the Markdown source.
//
// A word is a run of anything but whitespace with a letter or digit in it, so
// "well-known" is one word and a lone "-" none. Characters are those of the
// text, spaces included, line breaks not.
func countText(doc ast.Node, src []byte) (words, chars int) {
	var text strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Type() == ast.TypeBlock {
			text.WriteByte('\n') // "end.Start" of two paragraphs is two words
		}
		switch n := n.(type) {
		case *mermaid.Block, *ast.Image:
			return ast.WalkSkipChildren, nil
		case *commentBlock, *commentInline:
			return ast.WalkSkipChildren, nil // a comment is not part of the note
		case *wikilink.Node:
			if n.Embed {
				return ast.WalkSkipChildren, nil
			}
		case *ast.Text:
			text.Write(n.Segment.Value(src))
			if n.SoftLineBreak() || n.HardLineBreak() {
				text.WriteByte('\n')
			}
		case *ast.String:
			text.Write(n.Value)
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			lines := n.Lines()
			for i := range lines.Len() {
				line := lines.At(i)
				text.Write(line.Value(src))
			}
		}
		return ast.WalkContinue, nil
	})
	for _, r := range text.String() {
		if r != '\n' && r != '\r' {
			chars++
		}
	}
	for _, field := range strings.FieldsFunc(text.String(), unicode.IsSpace) {
		if strings.IndexFunc(field, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			words++
		}
	}
	return words, chars
}
