package render

import (
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// Rendering of everything that points into the vault.
//
// Parsing wikilinks is the library's job; rendering them is ours, for two
// things the library's renderer cannot do: Obsidian's image size syntax
// (![[photo.png|200]]), and showing a link whose file does not exist as
// missing. The library prints such a link as bare text, and a missing Markdown
// image would be an <img> pointing at a 404. A reader should see at a glance
// that a note expects a file the vault does not have.

// missing replaces a wikilink, Markdown link or image whose target is not in
// the vault. Its children are the text to show: the link label or the alt text.
type missing struct {
	ast.BaseInline
	target string
	embed  bool // an image or other embed, shown as a box rather than in line
}

var kindMissing = ast.NewNodeKind("MissingLink")

func (n *missing) Kind() ast.NodeKind { return kindMissing }
func (n *missing) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Target": n.target}, nil)
}

// markMissing swaps node for a missing marker that keeps node's label.
func markMissing(node ast.Node, src []byte, target string, embed bool) {
	m := &missing{target: target, embed: embed}
	// For ![[photo.png|200]] the "label" is a size, not something to read.
	keepLabel := !(embed && imageSize.Match(labelOf(node, src)))
	for c := node.FirstChild(); c != nil; {
		next := c.NextSibling()
		if keepLabel {
			m.AppendChild(m, c) // moves c out of node
		}
		c = next
	}
	node.Parent().ReplaceChild(node.Parent(), node, m)
}

// labelOf returns the literal text of a node whose only child is plain text,
// which is all a wikilink label can be.
func labelOf(node ast.Node, src []byte) []byte {
	if node.ChildCount() != 1 {
		return nil
	}
	if t, ok := node.FirstChild().(*ast.Text); ok {
		return t.Value(src)
	}
	return nil
}

// imageSize is Obsidian's "200" (width) or "200x100" in place of a label.
var imageSize = regexp.MustCompile(`^(\d+)(?:x(\d+))?$`)

var imageExt = map[string]bool{
	".apng": true, ".avif": true, ".bmp": true, ".gif": true, ".jpeg": true,
	".jpg": true, ".png": true, ".svg": true, ".webp": true,
}

// IsImage reports whether a file is shown as a picture when embedded.
func IsImage(name string) bool { return imageExt[strings.ToLower(path.Ext(name))] }

type linkExtender struct{}

func (linkExtender) Extend(md goldmark.Markdown) {
	// Below goldmark's own link parser (200), so "[[" is seen first.
	md.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(&wikilink.Parser{}, 199)))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(linkRenderer{}, 199)))
}

// resolvedLink is what Renderer.parse found out about a wikilink. It travels
// on the node, because a node renderer is not told which note it is rendering.
type resolvedLink struct {
	url      string
	anchor   string // id of the heading or block to land on, "" for none
	noAnchor bool   // the note exists, nothing with that id in it does
	embed    Embed
	reason   string // why a note embed stayed a link, if there is a reason to give
}

func (l resolvedLink) href() string {
	if l.anchor == "" {
		return l.url
	}
	return l.url + "#" + l.anchor
}

// A link to a heading or a block that is not there still leads to the note, as
// in Obsidian, but says so.
const (
	noAnchorClass = "missing-heading"
	noHeadingText = "The note has no heading of this name"
	noBlockText   = "The note has no block with this id"
)

// noAnchorTitle says which of the two is missing.
func (l resolvedLink) noAnchorTitle() string {
	if IsBlockAnchor(l.anchor) {
		return noBlockText
	}
	return noHeadingText
}

// wikiTarget returns what a wikilink points at: the file, and the heading or
// block in it. The parser splits at the last "#", which takes
// [[Note#Chapter#Section]] for a file called "Note#Chapter".
func wikiTarget(n *wikilink.Node) (target, fragment string) {
	full := string(n.Target)
	if n.Fragment != nil {
		full += "#" + string(n.Fragment)
	}
	target, fragment, _ = strings.Cut(full, "#")
	return strings.TrimSpace(target), fragment
}

var resolvedAttr = []byte("data-resolved")

type linkRenderer struct{}

func (r linkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	registerTransclusion(reg)
	reg.Register(wikilink.Kind, r.wikilink)
	reg.Register(kindMissing, r.missing)
}

// closeTag carries the closing tag from a wikilink's entering call to its
// leaving call, which goldmark makes even when the children were skipped.
var closeTag = []byte("data-close")

func (r linkRenderer) wikilink(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*wikilink.Node)
	if !entering {
		if tag, ok := n.Attribute(closeTag); ok {
			_, _ = w.Write(tag.([]byte))
		}
		return ast.WalkContinue, nil
	}
	// Unresolved targets were replaced by a missing marker in parse. What
	// remains is resolved, or has no target at all: [[#Heading]].
	var resolved resolvedLink
	if v, ok := n.Attribute(resolvedAttr); ok {
		resolved = v.(resolvedLink)
	}
	embed := resolved.embed
	href := string(util.EscapeHTML([]byte(resolved.href())))
	switch {
	case embed.Image == "" && embed.Drawing:
		// The server does not draw Excalidraw scenes itself; say what is
		// missing instead of showing a bare link.
		_, _ = w.WriteString(`<a class="drawing-unexported" href="` + href + `" title="` + DrawingHelp + `">`)
		n.SetAttribute(closeTag, []byte("</a>"))
		return ast.WalkContinue, nil
	case embed.Image == "":
		_, _ = w.WriteString(`<a href="` + href + `"`)
		if resolved.noAnchor {
			_, _ = w.WriteString(` class="` + noAnchorClass + `" title="` + resolved.noAnchorTitle() + `"`)
		} else if resolved.reason != "" {
			_, _ = w.WriteString(` class="not-embedded" title="` + resolved.reason + `"`)
		}
		_, _ = w.WriteString(">")
		n.SetAttribute(closeTag, []byte("</a>"))
		// Without an alias Obsidian shows "Note > Heading" rather than the
		// link as typed, and just "Heading" for a heading of the same note.
		target, fragment := wikiTarget(n)
		if typed := string(labelOf(n, src)); fragment != "" && typed == strings.TrimSpace(string(n.Target))+"#"+string(n.Fragment) {
			parts := strings.Split(fragment, "#")
			if target != "" {
				parts = append([]string{target}, parts...)
			}
			_, _ = w.Write(util.EscapeHTML([]byte(strings.Join(parts, " > "))))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	}

	if embed.Drawing {
		_, _ = w.WriteString(`<a class="drawing" href="` + href + `">`)
	}
	if embed.DarkImage != "" {
		_, _ = w.WriteString(`<picture><source media="screen and (prefers-color-scheme: dark)" srcset="` +
			string(util.EscapeHTML([]byte(embed.DarkImage))) + `">`)
	}
	_, _ = w.WriteString(`<img src="` + string(util.EscapeHTML([]byte(embed.Image))) + `"`)
	label := labelOf(n, src)
	if size := imageSize.FindSubmatch(label); size != nil {
		_, _ = w.WriteString(` width="` + string(size[1]) + `"`)
		if len(size[2]) > 0 {
			_, _ = w.WriteString(` height="` + string(size[2]) + `"`)
		}
	} else if len(label) > 0 && string(label) != string(n.Target) {
		_, _ = w.WriteString(` alt="` + string(util.EscapeHTML(label)) + `"`)
	}
	_, _ = w.WriteString(">")
	if embed.DarkImage != "" {
		_, _ = w.WriteString("</picture>")
	}
	if embed.Drawing {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkSkipChildren, nil
}

// DrawingHelp tells the owner how to make a drawing show up.
const DrawingHelp = "This drawing has no exported image. In Obsidian, turn on Excalidraw settings → " +
	"Embedding and Exporting → Auto-export SVG, then open the drawing once."

func (r linkRenderer) missing(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*missing)
	if !entering {
		_, _ = w.WriteString("</span>")
		return ast.WalkContinue, nil
	}
	class := "missing"
	if n.embed {
		class += " missing-embed"
	}
	_, _ = w.WriteString(`<span class="` + class + `" title="Not found in this vault: `)
	_, _ = w.Write(util.EscapeHTML([]byte(n.target)))
	_, _ = w.WriteString(`">`)
	if !n.HasChildren() {
		_, _ = w.Write(util.EscapeHTML([]byte(n.target)))
	}
	return ast.WalkContinue, nil
}
