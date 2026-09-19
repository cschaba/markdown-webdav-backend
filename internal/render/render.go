// Package render turns Obsidian-flavoured Markdown into HTML and extracts the
// metadata (front matter, tags, links) the index is built from. Both use the
// same parser, so what the index believes and what a page shows cannot drift.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/frontmatter"
	"go.abhg.dev/goldmark/hashtag"
	"go.abhg.dev/goldmark/mermaid"
	"go.abhg.dev/goldmark/wikilink"
)

// LinkResolver maps a link target such as "Note", "../img/photo.png" or
// "/folder/Note" to the URL path of the file it means. What a target means
// depends on where the link stands, so from names the linking note (its vault
// path). The index implements it.
type LinkResolver interface {
	ResolveLink(from, target string) (urlPath string, ok bool)
	// ResolveEmbed says what to show for ![[target]]. It is only asked
	// about targets that ResolveLink found.
	ResolveEmbed(from, target string) Embed
	// HasHeading reports whether the note that target means has a heading
	// with that id. For anything that is not a note it reports true: there
	// is nothing to check a heading against.
	HasHeading(from, target, id string) bool
}

// Embed is what stands in for an embedded file. With no Image the embed is
// rendered as a link to the file.
type Embed struct {
	Image     string // URL of the picture to show
	DarkImage string // its variant for dark mode, if there is one
	// Drawing marks an Excalidraw drawing. Its Image is the picture the
	// Obsidian plugin exported next to it; the picture links to the drawing.
	Drawing bool
}

// Meta is everything known about a note without rendering it.
type Meta struct {
	Title       string // front matter "title", empty if unset
	Frontmatter map[string]any
	Tags        []string // front matter and inline, lowercased, sorted, unique
	Headings    []string // the ids of the note's headings, in order
	// Links holds the targets of everything that points at another vault
	// file: wikilinks, wikilinks inside front matter values, and relative
	// Markdown links. Raw targets without fragment, front matter first.
	Links []string
}

type Renderer struct {
	md    goldmark.Markdown
	links LinkResolver
}

// New builds a renderer for one vault. Hashtags link to tagURL + tag; both
// it and the resolver's URLs carry the vault's URL prefix.

func New(links LinkResolver, tagURL string) *Renderer {
	return &Renderer{links: links, md: goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			&frontmatter.Extender{},
			linkExtender{},
			&hashtag.Extender{Variant: hashtag.ObsidianVariant, Resolver: tagResolver{tagURL}},
			// Client-side: server-side rendering would need a headless
			// browser in the image.
			&mermaid.Extender{RenderMode: mermaid.RenderModeClient},
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
			foldExtender{},
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		// Raw HTML stays disabled (goldmark's default): notes may be pasted
		// from anywhere, and the viewer runs with the owner's session.
	)}
}

// Render returns the HTML of a note together with its metadata. from is the
// note's vault path: its links are resolved from where it stands.
func (r *Renderer) Render(src []byte, from string) (template.HTML, Meta, error) {
	doc, meta := r.parse(src, from, true)
	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, src, doc); err != nil {
		return "", meta, err
	}
	return template.HTML(buf.String()), meta, nil
}

// Meta parses a note without rendering it.
func (r *Renderer) Meta(src []byte) Meta {
	_, meta := r.parse(src, "", false)
	return meta
}

// parse reads a note. With forRender it also resolves what points into the
// vault: Markdown links get their real URL, and whatever has no file behind it
// becomes a missing marker. The index skips that, since it only needs Meta, and
// while it is being built there is nothing to resolve against yet.
func (r *Renderer) parse(src []byte, from string, forRender bool) (ast.Node, Meta) {
	ctx := parser.NewContext(parser.WithIDs(newHeadingIDs()))
	doc := r.md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))

	var meta Meta
	tags := map[string]bool{}
	if data := frontmatter.Get(ctx); data != nil {
		// Broken front matter must not make a note unreadable.
		if err := data.Decode(&meta.Frontmatter); err == nil {
			meta.Title, _ = meta.Frontmatter["title"].(string)
			for _, tag := range stringList(meta.Frontmatter["tags"]) {
				tags[normalizeTag(tag)] = true
			}
			meta.Links = propertyLinks(meta.Frontmatter, nil)
		}
	}
	// The headings first: a link further up may point at one further down.
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if heading, ok := n.(*ast.Heading); ok && entering {
			if id, ok := heading.AttributeString("id"); ok {
				meta.Headings = append(meta.Headings, string(id.([]byte)))
			}
		}
		return ast.WalkContinue, nil
	})

	var unresolved []func() // applied after the walk; it must not see the tree change
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var target, fragment string
		var link *ast.Link      // a Markdown link, whose URL gets rewritten
		var image *ast.Image    // likewise
		var wiki *wikilink.Node // a wikilink, which carries its URL to the renderer
		embed := false
		switch n := n.(type) {
		case *hashtag.Node:
			tags[normalizeTag(string(n.Tag))] = true
			return ast.WalkContinue, nil
		case *wikilink.Node:
			target, fragment = wikiTarget(n)
			embed, wiki = n.Embed, n
		case *ast.Link:
			target, fragment = internalTarget(string(n.Destination))
			link = n
		case *ast.Image:
			target, _ = internalTarget(string(n.Destination))
			embed, image = true, n
		default:
			return ast.WalkContinue, nil
		}
		if target == "" && fragment == "" {
			return ast.WalkContinue, nil // leads out of the vault
		}
		if target != "" {
			meta.Links = append(meta.Links, target)
		}
		if !forRender {
			return ast.WalkContinue, nil
		}

		resolved := resolvedLink{anchor: anchor(fragment)}
		if target != "" {
			var ok bool
			if resolved.url, ok = r.links.ResolveLink(from, target); !ok {
				unresolved = append(unresolved, func() { markMissing(n, src, target, embed) })
				return ast.WalkContinue, nil
			}
		}
		if resolved.anchor != "" && !embed {
			if target == "" { // [[#Heading]]: in this note
				resolved.noHeading = !slices.Contains(meta.Headings, resolved.anchor)
			} else {
				resolved.noHeading = !r.links.HasHeading(from, target, resolved.anchor)
			}
		}
		switch {
		case wiki != nil:
			// Resolved here, not in the node renderer: only here is it known
			// which note the link stands in.
			if embed {
				resolved.embed = r.links.ResolveEmbed(from, target)
			}
			wiki.SetAttribute(resolvedAttr, resolved)
		case image != nil:
			image.Destination = []byte(resolved.url)
		case link != nil:
			// The browser would resolve "Note.md" against the linking note's
			// URL, which works for a file next to it and for nothing else.
			link.Destination = []byte(resolved.href())
			if resolved.noHeading {
				link.SetAttributeString("class", []byte(noHeadingClass))
				if link.Title == nil {
					link.Title = []byte(noHeadingTitle)
				}
			}
		}
		return ast.WalkContinue, nil
	})
	for _, mark := range unresolved {
		mark()
	}
	delete(tags, "")
	for tag := range tags {
		meta.Tags = append(meta.Tags, tag)
	}
	sort.Strings(meta.Tags)
	return doc, meta
}

// propertyWikilink finds [[Target]], [[Target|label]] and [[Target#heading]]
// inside a front matter value. A regex is fine here, unlike in the note body:
// a YAML string has no code blocks that could hold a link that is not one.
var propertyWikilink = regexp.MustCompile(`\[\[([^\]|#]+)[^\]]*\]\]`)

// propertyLinks collects the wikilinks in front matter values, in a stable
// order. Obsidian treats them as links, so they must produce backlinks.
func propertyLinks(v any, out []string) []string {
	switch v := v.(type) {
	case string:
		for _, m := range propertyWikilink.FindAllStringSubmatch(v, -1) {
			out = append(out, strings.TrimSpace(m[1]))
		}
	case []any:
		for _, item := range v {
			out = propertyLinks(item, out)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out = propertyLinks(v[key], out)
		}
	}
	return out
}

// internalTarget returns what a Markdown link destination such as
// "Other%20note.md#Some%20heading", "../img/a.png", "/folder/x.md" or
// "#heading" names: the target, decoded but otherwise as written, and the
// fragment. Both are "" for anything that leaves the vault.
func internalTarget(dest string) (target, fragment string) {
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" || (u.Path == "" && u.Fragment == "") {
		return "", ""
	}
	if strings.HasPrefix(strings.TrimPrefix(path.Clean("/"+u.Path), "/"), "-/") {
		return "", "" // one of the server's own pages, such as -/search?q=...
	}
	return u.Path, u.Fragment // "./" and "../" mean something: the resolver follows them
}

// stringList accepts the shapes Obsidian allows for list properties:
// a YAML list, or a single comma- or space-separated string.
func stringList(v any) []string {
	switch v := v.(type) {
	case []any:
		var out []string
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case string:
		return strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
	}
	return nil
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(tag), "#/"))
}

type tagResolver struct{ tagURL string }

func (t tagResolver) ResolveHashtag(n *hashtag.Node) ([]byte, error) {
	return []byte(t.tagURL + escapePath(normalizeTag(string(n.Tag)))), nil
}

// escapePath escapes each segment of a slash-separated path for use in a URL.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// EscapePath is escapePath for other packages building vault URLs.
func EscapePath(p string) string { return escapePath(p) }

// WriteHighlightCSS writes the stylesheet for highlighted code blocks,
// switching style with the reader's colour scheme.
func WriteHighlightCSS(w io.Writer) error {
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	if err := formatter.WriteCSS(w, styles.Get("github")); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "@media (prefers-color-scheme: dark) {\n"); err != nil {
		return err
	}
	if err := formatter.WriteCSS(w, styles.Get("github-dark")); err != nil {
		return err
	}
	_, err := io.WriteString(w, "}\n")
	return err
}
