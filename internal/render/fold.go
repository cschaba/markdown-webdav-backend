package render

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"
)

// Foldable headings, as in Obsidian: a heading and everything below it, up to
// the next heading of the same or a higher level, can be collapsed.
//
// It is done with <details open> rather than script. That needs no state, works
// without JavaScript, and browsers open a collapsed section by themselves when
// a link targets a heading inside it. The heading keeps its id, so anchors and
// [[Note#Heading]] links are unaffected.
//
//	<details class="fold" open>
//	  <summary><h2 id="..">Heading</h2></summary>
//	  content, including nested <details> for deeper headings
//	</details>

var (
	kindFoldSection = ast.NewNodeKind("FoldSection")
	kindFoldSummary = ast.NewNodeKind("FoldSummary")
)

type foldSection struct {
	ast.BaseBlock
	level int
}

func (n *foldSection) Kind() ast.NodeKind { return kindFoldSection }
func (n *foldSection) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

type foldSummary struct{ ast.BaseBlock }

func (n *foldSummary) Kind() ast.NodeKind { return kindFoldSummary }
func (n *foldSummary) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

type foldExtender struct{}

func (foldExtender) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		// After the other transformers, so that whatever they append to the
		// document (the Mermaid script) is already there to be sorted in.
		util.Prioritized(foldTransformer{}, 1000),
	))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(foldRenderer{}, 500),
	))
}

type foldTransformer struct{}

// Transform nests the document's top-level blocks into sections. Headings
// inside lists or block quotes are left alone: there is no sensible extent
// for a section that starts in the middle of a list item.
func (foldTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var blocks []ast.Node
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		blocks = append(blocks, c)
	}
	doc.RemoveChildren(doc)

	var open []*foldSection // innermost last
	for _, block := range blocks {
		// Footnotes and the Mermaid script belong to the whole note. Inside
		// the last section they would vanish when it is collapsed.
		if k := block.Kind(); k == extast.KindFootnoteList || k == mermaid.ScriptKind {
			open = nil
		}
		heading, ok := block.(*ast.Heading)
		if !ok {
			if len(open) > 0 {
				open[len(open)-1].AppendChild(open[len(open)-1], block)
			} else {
				doc.AppendChild(doc, block)
			}
			continue
		}
		for len(open) > 0 && open[len(open)-1].level >= heading.Level {
			open = open[:len(open)-1]
		}
		section := &foldSection{level: heading.Level}
		summary := &foldSummary{}
		summary.AppendChild(summary, heading)
		section.AppendChild(section, summary)
		if len(open) > 0 {
			open[len(open)-1].AppendChild(open[len(open)-1], section)
		} else {
			doc.AppendChild(doc, section)
		}
		open = append(open, section)
	}
}

type foldRenderer struct{}

func (foldRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindFoldSection, wrap("<details class=\"fold\" open>\n", "</details>\n"))
	reg.Register(kindFoldSummary, wrap("<summary>", "</summary>\n"))
}

func wrap(before, after string) renderer.NodeRendererFunc {
	return func(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(before)
		} else {
			_, _ = w.WriteString(after)
		}
		return ast.WalkContinue, nil
	}
}
