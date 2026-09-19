package render

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Intentional page breaks, for printing and the PDF export. Markdown has no
// syntax for one, so the two spellings people already use are understood:
//
//	\pagebreak   or   \newpage          on a line of its own (Pandoc, LaTeX)
//	<div style="page-break-after: always;"></div>
//
// The second is what Obsidian users write, because Obsidian's own PDF export
// honours it. Raw HTML is otherwise not rendered here, so this one element is
// recognised for what it means and nothing of it is passed through.

type pageBreak struct{ ast.BaseBlock }

var kindPageBreak = ast.NewNodeKind("PageBreak")

func (n *pageBreak) Kind() ast.NodeKind { return kindPageBreak }
func (n *pageBreak) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

var (
	pageBreakWord = regexp.MustCompile(`^\\(pagebreak|newpage)$`)
	pageBreakHTML = regexp.MustCompile(`(?is)^<div\s[^>]*style\s*=\s*["'][^"']*(page-break-(after|before)\s*:\s*always|break-(after|before)\s*:\s*page)[^"']*["'][^>]*>\s*</div>$`)
)

type pageBreakExtender struct{}

func (pageBreakExtender) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		// Before the fold transformer (1000), which moves blocks into sections.
		util.Prioritized(pageBreakTransformer{}, 900),
	))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(pageBreakRenderer{}, 500)))
}

type pageBreakTransformer struct{}

func (pageBreakTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var found []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Paragraph:
			if pageBreakWord.Match(bytes.TrimSpace(blockText(n, src))) {
				found = append(found, n)
			}
			return ast.WalkSkipChildren, nil
		case *ast.HTMLBlock:
			if pageBreakHTML.Match(bytes.TrimSpace(blockText(n, src))) {
				found = append(found, n)
			}
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			return ast.WalkSkipChildren, nil // "\pagebreak" in code is an example
		}
		return ast.WalkContinue, nil
	})
	for _, n := range found {
		n.Parent().ReplaceChild(n.Parent(), n, &pageBreak{})
	}
}

// blockText returns the source lines of a block.
func blockText(n ast.Node, src []byte) []byte {
	var out []byte
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		out = append(out, line.Value(src)...)
	}
	return out
}

type pageBreakRenderer struct{}

func (pageBreakRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindPageBreak, func(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(`<div class="page-break" role="separator" aria-label="Page break"></div>` + "\n")
		}
		return ast.WalkSkipChildren, nil
	})
}
