package render

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"go.abhg.dev/goldmark/mermaid"
)

// Slides, the way Obsidian's Slides plugin reads a note: a line of "---" with
// a blank line before and after it ends one slide and begins the next. (Without
// the blank line before it, "---" makes the line above a heading, in Obsidian
// as here.) Front matter is no slide.
//
// A slide is a run of the note's top-level blocks. The note is parsed without
// foldable sections for this: a section would run across separators, and a
// slide has nothing to fold.

// separatesSlides reports whether a thematic break stands at the top level of
// the note, as opposed to inside a quote or a list item, where it is a rule.
// In the normal parse the top level includes the sections fold.go builds.
func separatesSlides(n *ast.ThematicBreak) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.(type) {
		case *ast.Document, *foldSection:
		default:
			return false
		}
	}
	return true
}

// Deck is a note rendered as slides.
type Deck struct {
	Slides  []template.HTML
	Meta    Meta
	Mermaid bool // a slide has a diagram: the page must load the script, once
}

// RenderSlides renders a note as slides. A note without a separator is a deck
// of one slide.
func (r *Renderer) RenderSlides(src []byte, from string) (Deck, error) {
	doc, meta := r.parse(r.flat, src, from, true, false)
	deck := Deck{Meta: meta}
	var slide bytes.Buffer
	var footnotes ast.Node
	flush := func() {
		deck.Slides = append(deck.Slides, template.HTML(slide.String()))
		slide.Reset()
	}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.ThematicBreak:
			flush()
			continue
		case *mermaid.ScriptBlock:
			deck.Mermaid = true // the page includes it; inside a slide it would load per slide
			continue
		case *pageBreak:
			continue // slides are the pages here
		case *extast.FootnoteList:
			footnotes = n // they go where they are read: see below
			continue
		}
		if err := r.md.Renderer().Render(&slide, src, n); err != nil {
			return deck, err
		}
	}
	if footnotes != nil {
		// A note's footnotes are collected at its end, which is the last slide.
		if err := r.md.Renderer().Render(&slide, src, footnotes); err != nil {
			return deck, err
		}
	}
	flush()
	return deck, nil
}
