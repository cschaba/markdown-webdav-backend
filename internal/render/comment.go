package render

import (
	"bytes"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Obsidian's comments, %%…%%: notes to the author that no reader sees.
//
// Written twice in one line they hide what is between them; alone on a line
// they open and close a comment that may span blocks:
//
//	Visible %%not visible%% visible.
//
//	%%
//	A whole block, headings and lists included.
//	%%
//
// There is no goldmark extension for them (the one Obsidian bundle that comes
// close lists comments as not implemented), so they are parsed here - as a
// block parser and an inline parser, not by cutting the source, because "%%"
// inside a code span or a fenced block is text like any other and the parsers
// never see it.
//
// A comment is dropped, not hidden: it produces a node with no content, so it
// is out of the page, out of the word count (words.go), and out of the graph
// and backlinks, whatever it contained. The search strips it too, on the raw
// text it reads, see stripComments in internal/index/search.go.

var (
	kindCommentBlock  = ast.NewNodeKind("Comment")
	kindCommentInline = ast.NewNodeKind("CommentInline")
)

type commentBlock struct{ ast.BaseBlock }

func (n *commentBlock) Kind() ast.NodeKind { return kindCommentBlock }
func (n *commentBlock) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

type commentInline struct{ ast.BaseInline }

func (n *commentInline) Kind() ast.NodeKind { return kindCommentInline }
func (n *commentInline) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

// commentFence is the delimiter, and the whole line when it opens or closes a
// block comment.
var commentFence = []byte("%%")

func isCommentFence(line []byte) bool {
	return bytes.Equal(bytes.TrimSpace(line), commentFence)
}

type commentExtender struct{}

func (commentExtender) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(
		// Above the paragraph parser, so a "%%" line starts a comment rather
		// than a paragraph; below the fenced code block parser (700), so a
		// "%%" inside ``` stays code.
		parser.WithBlockParsers(util.Prioritized(commentBlockParser{}, 750)),
		parser.WithInlineParsers(util.Prioritized(commentInlineParser{}, 500)),
		parser.WithASTTransformers(util.Prioritized(commentTransformer{}, 800)),
	)
	md.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(commentRenderer{}, 500),
	))
}

type commentBlockParser struct{}

func (commentBlockParser) Trigger() []byte { return []byte{'%'} }

func (commentBlockParser) Open(_ ast.Node, reader text.Reader, _ parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	if !isCommentFence(line) {
		return nil, parser.NoChildren
	}
	advanceLine(reader, line, segment)
	return &commentBlock{}, parser.NoChildren
}

// advanceLine moves the reader past the line it peeked. goldmark's own block
// parsers stop one byte short, on the line's newline - but the last line of a
// note need not have one, and there stopping short leaves half the "%%" behind
// to be read as a paragraph.
func advanceLine(reader text.Reader, line []byte, segment text.Segment) {
	n := segment.Len()
	if len(line) > 0 && line[len(line)-1] == '\n' {
		n--
	}
	reader.Advance(n)
}

// Continue swallows every line up to the closing "%%". A comment that is never
// closed runs to the end of the note, as it does in Obsidian.
func (commentBlockParser) Continue(_ ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if isCommentFence(line) {
		advanceLine(reader, line, segment)
		return parser.Close
	}
	return parser.Continue | parser.NoChildren
}

func (commentBlockParser) Close(ast.Node, text.Reader, parser.Context) {}

// A "%%" line does not cut a running paragraph in two: in Obsidian a comment
// begun inside a paragraph is inline, and this parser only handles the block
// form, which stands on its own.
func (commentBlockParser) CanInterruptParagraph() bool { return false }
func (commentBlockParser) CanAcceptIndentedLine() bool { return false }

type commentInlineParser struct{}

func (commentInlineParser) Trigger() []byte { return []byte{'%'} }

// Parse consumes %%…%% within one line. Obsidian's inline comment closes on
// the line it opened on; what spans lines is the block form above.
func (commentInlineParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if !bytes.HasPrefix(line, commentFence) {
		return nil
	}
	end := bytes.Index(line[len(commentFence):], commentFence)
	if end < 0 {
		return nil // not a comment: a stray "%%" is shown as written
	}
	block.Advance(2*len(commentFence) + end)
	return &commentInline{}
}

// commentTransformer drops a paragraph that a comment left with nothing in it.
// "%%a whole line%%" would otherwise render as an empty <p>, which is a gap on
// the page where Obsidian shows nothing at all.
type commentTransformer struct{}

func (commentTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var empty []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Paragraph, *ast.TextBlock:
		default:
			return ast.WalkContinue, nil
		}
		hadComment := false
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch c := c.(type) {
			case *commentInline:
				hadComment = true
			case *ast.Text:
				if len(bytes.TrimFunc(c.Segment.Value(src), unicode.IsSpace)) > 0 {
					return ast.WalkContinue, nil
				}
			default:
				return ast.WalkContinue, nil
			}
		}
		if hadComment {
			empty = append(empty, n)
		}
		return ast.WalkContinue, nil
	})
	for _, n := range empty {
		n.Parent().RemoveChild(n.Parent(), n)
	}
}

type commentRenderer struct{}

func (commentRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindCommentBlock, renderNothing)
	reg.Register(kindCommentInline, renderNothing)
}

func renderNothing(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
}
