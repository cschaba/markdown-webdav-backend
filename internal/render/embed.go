package render

import (
	"bytes"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"
	"go.abhg.dev/goldmark/wikilink"
)

// Note embeds, Obsidian's ![[Note]] and ![[Note#Heading]]: the other note, or
// one section of it, is shown in place.
//
// The embedded note is rendered as itself, from its own path, so its links and
// images mean what they mean on its own page. Three things differ from its
// page. Its headings get no ids: they would collide with the host's, and the
// host's anchors must stay the host's. A link to a heading of "this note"
// therefore leads to the note's own page. And the Mermaid script, if it has
// diagrams, is left to the host page to include once.
//
// Embedding goes one level deep, by decision: inside an embedded note, a note
// embed is a link. That also makes loops impossible, except for a note
// embedding itself, which is checked.

// transclusion is the embedded note, already rendered.
type transclusion struct {
	ast.BaseBlock
	html  []byte
	title string
	url   string
}

var kindTransclusion = ast.NewNodeKind("Transclusion")

func (n *transclusion) Kind() ast.NodeKind { return kindTransclusion }
func (n *transclusion) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"URL": n.url}, nil)
}

func renderTransclusion(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if n := node.(*transclusion); entering {
		_, _ = w.WriteString(`<div class="transclusion">` + "\n" + `<div class="transclusion-title"><a href="`)
		_, _ = w.Write(util.EscapeHTML([]byte(n.url)))
		_, _ = w.WriteString(`">`)
		_, _ = w.Write(util.EscapeHTML([]byte(n.title)))
		_, _ = w.WriteString("</a></div>\n")
		_, _ = w.Write(n.html)
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkSkipChildren, nil
}

func registerTransclusion(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindTransclusion, renderTransclusion)
}

// Why an embed stayed a link, for the link's tooltip.
const (
	embedNested = "Not embedded here: notes are embedded one level deep"
	embedSelf   = "Not embedded: a note cannot embed itself"
)

// transclude replaces the wikilink by the note it embeds. If that cannot be
// done, the wikilink stays and is rendered as a link; the reason, if there is
// one to give, goes into resolved.
//
// It reports whether the embedded content needs the Mermaid script.
func (r *Renderer) transclude(wiki *wikilink.Node, resolved *resolvedLink, from string, embedded bool) (needsMermaid bool) {
	note := resolved.embed.Note
	switch {
	case embedded:
		resolved.reason = embedNested
		return false
	case note == from:
		resolved.reason = embedSelf
		return false
	case resolved.noHeading:
		return false // the link says that the heading is missing
	}
	host, ok := splitAround(wiki)
	if !ok {
		return false // inside emphasis, a link text, a table cell: no place for a block
	}
	src, err := r.links.ReadNote(note)
	if err != nil {
		return false
	}
	doc, meta := r.parse(src, note, true, true)

	var content ast.Node = doc
	title := resolved.embed.Title
	if resolved.anchor != "" {
		// A heading and what is below it is exactly what fold.go made a
		// section of.
		section := findSection(doc, src, resolved.anchor, meta.Headings)
		if section == nil {
			return false
		}
		_, fragment := wikiTarget(wiki)
		content, title = section, title+" > "+strings.Join(strings.Split(fragment, "#"), " > ")
	}
	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, src, content); err != nil {
		return false
	}
	host(&transclusion{html: buf.Bytes(), title: title, url: resolved.href()})
	return meta.needsMermaid
}

// findSection returns the fold section whose heading has the id. The ids were
// taken off the headings (see prepareEmbedded), so it goes by position: the
// n-th heading of the document has the n-th id.
func findSection(doc ast.Node, _ []byte, id string, ids []string) ast.Node {
	want := slices.Index(ids, id)
	if want < 0 {
		return nil
	}
	var found ast.Node
	count := 0
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if _, ok := n.(*ast.Heading); ok && entering {
			if count == want {
				// heading -> summary -> section; a heading in a list or a
				// quote has no section and is shown alone with its parent
				found = n
				if summary, ok := n.Parent().(*foldSummary); ok {
					found = summary.Parent()
				}
				return ast.WalkStop, nil
			}
			count++
		}
		return ast.WalkContinue, nil
	})
	return found
}

// prepareEmbedded makes a parsed note fit for being shown inside another: no
// heading ids, no script of its own. It reports whether there was a script.
func prepareEmbedded(doc ast.Node) (hadScript bool) {
	var scripts []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			removeAttribute(n, "id")
		case *mermaid.ScriptBlock:
			scripts = append(scripts, n)
		}
		return ast.WalkContinue, nil
	})
	for _, script := range scripts {
		script.Parent().RemoveChild(script.Parent(), script)
	}
	return len(scripts) > 0
}

func removeAttribute(n ast.Node, name string) {
	kept := n.Attributes()[:0:0]
	for _, attr := range n.Attributes() {
		if string(attr.Name) != name {
			kept = append(kept, attr)
		}
	}
	n.RemoveAttributes()
	for _, attr := range kept {
		n.SetAttribute(attr.Name, attr.Value)
	}
}

// splitAround makes room for a block where an inline node stands. An embed is
// written inside a paragraph, but a note's content - headings, lists, more
// paragraphs - cannot be inside one. So the paragraph is cut in two around
// the node, and the returned function puts the block between the halves.
func splitAround(inline ast.Node) (place func(block ast.Node), ok bool) {
	parent := inline.Parent()
	var newHalf func() ast.Node
	switch parent.(type) {
	case *ast.Paragraph:
		newHalf = func() ast.Node { return ast.NewParagraph() }
	case *ast.TextBlock: // the text of a tight list item
		newHalf = func() ast.Node { return ast.NewTextBlock() }
	default:
		return nil, false
	}
	return func(block ast.Node) {
		before, after := newHalf(), newHalf()
		half := before
		for c := parent.FirstChild(); c != nil; {
			next := c.NextSibling()
			if c == inline {
				half = after
			} else {
				half.AppendChild(half, c) // moves c out of parent
			}
			c = next
		}
		outer := parent.Parent()
		for _, n := range []ast.Node{before, block, after} {
			if n == block || !blank(n) {
				outer.InsertBefore(outer, parent, n)
			}
		}
		outer.RemoveChild(outer, parent)
	}, true
}

// blank reports whether a paragraph half holds nothing to show: no children,
// or only the line break that stood next to the embed.
func blank(n ast.Node) bool {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		text, ok := c.(*ast.Text)
		if !ok || text.Segment.Len() > 0 {
			return false
		}
	}
	return true
}
