package render

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Block references, Obsidian's way of pointing at one paragraph instead of a
// whole note. A block is named by putting "^id" at its end,
//
//	A paragraph worth quoting. ^claim
//
//	| a | table |
//	|---|---|
//
//	^numbers
//
// and [[Note#^claim]] links to it, ![[Note#^claim]] shows it in place. The id
// is written on the last line of the block, or alone on the line after it -
// the second form is what a table, a list or a code block needs, since their
// last line cannot carry it.
//
// The "^id" is taken off the text: it is a name, not something to read. The
// block gets it as its HTML id, prefixed with "^" - a character Slug never
// produces, so a block id and a heading id can never collide. anchor() in
// headings.go builds the same string from a link's fragment, which is what
// makes the two sides meet.
//
// Not every block can hold an id. goldmark writes a code block as
// <pre><code> and drops whatever attributes it carries, and a block that
// already has an id - a heading - must keep it. Those get an empty anchor
// element in front of them instead, which lands a link in the same place.
// The distinction is checked in the rendered HTML by TestBlockAnchors: an id
// that Meta.Blocks claims and the page does not write would be a link leading
// nowhere, and nothing else would notice.
//
// A "^id" at the end of a heading line stays text, as written: the heading's
// own id is the one links use.

// BlockAnchorPrefix marks the id of a named block in the page.
const BlockAnchorPrefix = "^"

// blockAnchor is the HTML id of the block called id. Ids are compared
// lowercased, as heading links are.
func blockAnchor(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return ""
	}
	return BlockAnchorPrefix + id
}

// IsBlockAnchor reports whether an anchor names a block rather than a heading.
func IsBlockAnchor(anchor string) bool {
	return strings.HasPrefix(anchor, BlockAnchorPrefix)
}

type blockRefExtender struct{}

func (blockRefExtender) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		// Before foldTransformer (1000), which moves the blocks about.
		util.Prioritized(blockRefTransformer{}, 900),
	))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(blockAnchorRenderer{}, 500),
	))
}

// blockAnchorNode is the id of a block that cannot carry one itself, standing
// in front of it.
type blockAnchorNode struct {
	ast.BaseBlock
	anchor string
}

var kindBlockAnchor = ast.NewNodeKind("BlockAnchor")

func (n *blockAnchorNode) Kind() ast.NodeKind { return kindBlockAnchor }
func (n *blockAnchorNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Anchor": n.anchor}, nil)
}

type blockAnchorRenderer struct{}

func (blockAnchorRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindBlockAnchor, func(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(`<span class="block-anchor" id="`)
			_, _ = w.Write(util.EscapeHTML([]byte(node.(*blockAnchorNode).anchor)))
			_, _ = w.WriteString(`"></span>` + "\n")
		}
		return ast.WalkSkipChildren, nil
	})
}

type blockRefTransformer struct{}

func (blockRefTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	// Collected first, applied after: the walk must not see the tree change.
	var standalone []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Paragraph, *ast.TextBlock:
		default:
			return ast.WalkContinue, nil // only a block with a last line can carry one
		}
		last, ok := n.LastChild().(*ast.Text)
		if !ok {
			return ast.WalkContinue, nil
		}
		value := string(last.Segment.Value(src))
		id, cut := trailingBlockID(value)
		if id == "" {
			return ast.WalkContinue, nil
		}
		if cut == 0 && n.ChildCount() == 1 {
			standalone = append(standalone, n) // the whole block is the id
			return ast.WalkContinue, nil
		}
		last.Segment = last.Segment.WithStop(last.Segment.Start + cut)
		if last.Segment.Len() == 0 {
			n.RemoveChild(n, last)
		}
		attach(owner(n), id)
		return ast.WalkContinue, nil
	})
	for _, n := range standalone {
		// "^id" on a line of its own names the block above it. With nothing
		// above, there is nothing to name and it stays as written.
		prev := n.PreviousSibling()
		if prev == nil {
			continue
		}
		value := string(n.LastChild().(*ast.Text).Segment.Value(src))
		id, _ := trailingBlockID(value)
		attach(prev, id)
		n.Parent().RemoveChild(n.Parent(), n)
	}
}

// owner returns the node an id written in n belongs to: the list item, if n is
// its text, so that the whole item is the target.
func owner(n ast.Node) ast.Node {
	if item, ok := n.Parent().(*ast.ListItem); ok && n.PreviousSibling() == nil {
		return item
	}
	return n
}

// attach gives a block its anchor: as the block's own id where that survives
// rendering, otherwise as an anchor element in front of it.
func attach(n ast.Node, id string) {
	anchor := blockAnchor(id)
	if anchor == "" {
		return
	}
	if _, taken := n.AttributeString("id"); !taken && carriesID(n) {
		n.SetAttributeString("id", []byte(anchor))
		return
	}
	if parent := n.Parent(); parent != nil {
		parent.InsertBefore(parent, n, &blockAnchorNode{anchor: anchor})
	}
}

// carriesID lists the blocks whose renderer writes the attributes they carry.
// A text block is none: it has no element of its own. Anything not listed -
// a code block, a Mermaid diagram, a callout - keeps its id in front of it
// instead, so a new kind is handled safely rather than silently dropped.
func carriesID(n ast.Node) bool {
	switch n.(type) {
	case *ast.Paragraph, *ast.Heading, *ast.Blockquote, *ast.List, *ast.ListItem,
		*ast.ThematicBreak, *extast.Table:
		return true
	}
	return false
}

// blockAnchorOf reports the anchor a node carries and the block it names -
// the node itself, or the one an anchor element stands in front of.
func blockAnchorOf(n ast.Node) (anchor string, block ast.Node) {
	if a, ok := n.(*blockAnchorNode); ok {
		if next := a.NextSibling(); next != nil {
			return a.anchor, next
		}
		return a.anchor, a
	}
	// Nothing else in a note has an id beginning with "^".
	if id, ok := n.AttributeString("id"); ok && IsBlockAnchor(string(id.([]byte))) {
		return string(id.([]byte)), n
	}
	return "", nil
}

// trailingBlockID finds the "^id" that ends a block's last line. It returns
// the id and where the text before it ends, so that the caller can cut it off;
// cut is 0 when the line holds nothing else.
//
// Obsidian's id is letters, digits and dashes, and "^" must start the line or
// follow a space: "2^10" and "a^b" are not block ids.
func trailingBlockID(value string) (id string, cut int) {
	trimmed := strings.TrimRight(value, " \t")
	caret := strings.LastIndexByte(trimmed, '^')
	if caret < 0 {
		return "", 0
	}
	if caret > 0 && trimmed[caret-1] != ' ' && trimmed[caret-1] != '\t' {
		return "", 0
	}
	id = trimmed[caret+1:]
	if id == "" || strings.IndexFunc(id, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-')
	}) >= 0 {
		return "", 0
	}
	return id, len(strings.TrimRight(trimmed[:caret], " \t"))
}
