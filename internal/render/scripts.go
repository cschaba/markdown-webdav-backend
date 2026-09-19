package render

import (
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"
)

// The scripts that come from somewhere else. A page of a vault runs with the
// owner's login and can write to the vault over WebDAV, and so could a script
// on it. Each is therefore one version, with the hash of the file: the browser
// refuses whatever the CDN sends that differs. Without the version the URL
// means "the newest", and the hash would break with every release.
//
// To move to another version, change the URL and take the new hash from
//
//	curl -s URL | openssl dgst -sha384 -binary | openssl base64 -A
//
// after looking at what changed. The pages' Content-Security-Policy allows
// these URLs and no others (web.contentSecurityPolicy).
type ExternalScript struct{ URL, Integrity string }

var (
	MermaidScript = ExternalScript{
		"https://cdn.jsdelivr.net/npm/mermaid@12.0.0/dist/mermaid.min.js",
		"sha384-xzghz1GQ5u9HCpVskeDPqMsdogD1yvuMQbEK53+wi+G70+6J1AG0L2cfi9PHjDWI",
	}
	ForceGraphScript = ExternalScript{
		"https://cdn.jsdelivr.net/npm/force-graph@1.51.4/dist/force-graph.min.js",
		"sha384-Hm6GpQcTNI5VqGgGS7lLxTGtEFcxu/kOVV0B7ozIZRu9blWVvigv5httJQZ2qZmY",
	}
)

// Tag is the script element. crossorigin is what makes the browser check the
// hash at all: without it a script from another origin is run unchecked.
func (s ExternalScript) Tag() template.HTML {
	return template.HTML(`<script src="` + template.HTMLEscapeString(s.URL) + `" integrity="` +
		template.HTMLEscapeString(s.Integrity) + `" crossorigin="anonymous"></script>`)
}

// MermaidStart draws the diagrams of a note once the page is loaded. It is a
// file, not a line in the page: the policy allows no inline script, which is
// what keeps text that slipped through as HTML from running.
const MermaidStart = `<script src="/-/mermaid-start.js"></script>`

// scriptExtender replaces the script element of the Mermaid extension, which
// names no version, has no hash and starts Mermaid with an inline script.
type scriptExtender struct{}

func (scriptExtender) Extend(md goldmark.Markdown) {
	// Above the extension's own renderer (100), so this one is used.
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(scriptRenderer{}, 99)))
}

type scriptRenderer struct{}

func (scriptRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mermaid.ScriptKind, func(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(string(MermaidScript.Tag()) + MermaidStart)
		}
		return ast.WalkContinue, nil
	})
}
