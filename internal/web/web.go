// Package web serves the read-only rendered view. Each vault gets its own
// Handler, mounted below "/<name>"; Home and Style serve what they share.
package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
	"github.com/cschaba/markdown-webdav-backend/internal/render"
)

//go:embed templates/*.html static/style.css static/graph.js static/print.js static/keys.js static/slides.js static/mermaid-start.js
var assets embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"mermaidScript":    render.MermaidScript.Tag,
	"forceGraphScript": render.ForceGraphScript.Tag,
}).ParseFS(assets, "templates/*.html"))

// contentSecurityPolicy is sent with every page. A page runs with the owner's
// login and can write to the vault over WebDAV, so it matters what may run on
// it: our own files and the two pinned scripts, by their exact URL - the CDN
// as a whole would allow any package anyone has published there. No inline
// script, which is what is left of an escaping bug in the renderer: text that
// comes out as HTML after all does not run. Styles may be inline, Mermaid
// writes them; pictures and media may come from anywhere, notes embed them.
var contentSecurityPolicy = strings.Join([]string{
	"default-src 'none'",
	"script-src 'self' " + render.MermaidScript.URL + " " + render.ForceGraphScript.URL,
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data: blob: https: http:",
	"media-src 'self' data: blob: https: http:",
	"font-src 'self' data:",
	"connect-src 'self'",
	"form-action 'self'",
	"base-uri 'none'",
	"frame-ancestors 'none'",
}, "; ")

// Vault names one vault for the switcher and the start page.
type Vault struct {
	Name string
	URL  string // "/<name>/"
}

type Config struct {
	Name    string // the vault's name; the handler expects to be mounted at "/<name>"
	Version string // of the server, for the foot of the pages and the About page
	Dir     string
	Index   *index.Index
	Vaults  []Vault // all vaults, for the switcher
	// Warning returns a non-nil error while something the owner should know
	// about is broken, for example versioning. May be nil.
	Warning func() error
}

type Handler struct {
	Config
	prefix string   // "/<name>"
	root   *os.Root // confines file access to the vault, symlinks included
	mux    *http.ServeMux
}

func New(cfg Config) (*Handler, error) {
	root, err := os.OpenRoot(cfg.Dir)
	if err != nil {
		return nil, err
	}
	h := &Handler{Config: cfg, prefix: "/" + cfg.Name, root: root, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /-/tags", h.tags)
	h.mux.HandleFunc("GET /-/about", h.about)
	h.mux.HandleFunc("GET /-/export", h.export)
	h.mux.HandleFunc("GET /-/slides", h.slides)
	h.mux.HandleFunc("GET /-/search", h.search)
	h.mux.HandleFunc("POST /-/search/clear", h.clearHistory)
	h.mux.HandleFunc("GET /-/graph", h.graph)
	h.mux.HandleFunc("GET /-/graph.json", h.graphData)
	h.mux.HandleFunc("GET "+index.TagPath+"{tag...}", h.tag)
	h.mux.HandleFunc("GET /", h.vault)
	return h, nil
}

// ServeHTTP serves the vault below its prefix.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.StripPrefix(h.prefix, h.mux).ServeHTTP(w, r)
}

type crumb struct{ Name, URL string }

type page struct {
	Title     string
	Vault     string
	Vaults    []Vault
	TagsURL   string
	GraphURL  string
	SearchURL string
	AboutURL  string
	Version   string   // of the server, shown at the foot
	ExportURL string   // set on pages that can be exported to PDF
	SlidesURL string   // set on notes that have slide separators
	Stats     bool     // the page has statistics to show, see stats.go
	Query     string   // what the search box shows
	History   []string // recent searches, offered by the search box
	Wide      bool     // the page uses the whole window, not a text column
	Crumbs    []crumb
	Warning   string
	Body      any
}

// exportable is a page body that stands for a note or folder of the vault.
type exportable interface{ vaultPath() string }

func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, title string, crumbs []crumb, body any) {
	p := page{Title: title, Vault: h.Name, Vaults: h.Vaults, TagsURL: h.prefix + "/-/tags", GraphURL: h.prefix + "/-/graph",
		SearchURL: h.prefix + "/-/search", AboutURL: h.prefix + "/-/about", Version: h.Version, Wide: name == "graph.html", Crumbs: crumbs, Body: body}
	if results, ok := body.(searchBody); ok {
		p.Query, p.History = results.Query, results.History // as just updated, not as the request had it
	} else {
		p.History = h.history(r)
	}
	if e, ok := body.(exportable); ok {
		p.ExportURL = h.exportURL(e.vaultPath())
	}
	if note, ok := body.(noteBody); ok {
		if note.Slides > 1 {
			p.SlidesURL = h.slidesURL(note.Path)
		}
		p.Stats = true
	}
	if h.Warning != nil {
		if err := h.Warning(); err != nil {
			p.Warning = "Versioning is failing: " + err.Error()
		}
	}
	writePage(w, name, p)
}

func writePage(w http.ResponseWriter, name string, p page) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, p); err != nil {
		slog.Error("template failed", "template", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	_, _ = w.Write(buf.Bytes())
}

// AssetsPath is where every page expects the files all vaults share. No vault
// can be named "-", so nothing of a vault is shadowed.
const AssetsPath = "/-/"

// Assets serves the stylesheet, including the one for highlighted code, and
// the graph view's script.
func Assets() (http.Handler, error) {
	css, err := assets.ReadFile("static/style.css")
	if err != nil {
		return nil, err
	}
	style := bytes.NewBuffer(css)
	if err := render.WriteHighlightCSS(style); err != nil {
		return nil, err
	}
	script, err := assets.ReadFile("static/graph.js")
	if err != nil {
		return nil, err
	}
	printScript, err := assets.ReadFile("static/print.js")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	serve := func(name, contentType string, content []byte) {
		mux.HandleFunc("GET "+AssetsPath+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(content)
		})
	}
	serve("style.css", "text/css; charset=utf-8", style.Bytes())
	serve("graph.js", "text/javascript; charset=utf-8", script)
	serve("print.js", "text/javascript; charset=utf-8", printScript)
	keysScript, err := assets.ReadFile("static/keys.js")
	if err != nil {
		return nil, err
	}
	serve("keys.js", "text/javascript; charset=utf-8", keysScript)
	slidesScript, err := assets.ReadFile("static/slides.js")
	if err != nil {
		return nil, err
	}
	serve("slides.js", "text/javascript; charset=utf-8", slidesScript)
	mermaidStart, err := assets.ReadFile("static/mermaid-start.js")
	if err != nil {
		return nil, err
	}
	serve("mermaid-start.js", "text/javascript; charset=utf-8", mermaidStart)
	return mux, nil
}

// Home is the start page: the list of vaults, or straight into the vault
// when there is only one.
func Home(vaults []Vault, version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(vaults) == 1 {
			http.Redirect(w, r, vaults[0].URL, http.StatusFound)
			return
		}
		writePage(w, "vaults.html", page{Title: "Vaults", Vaults: vaults, Version: version})
	})
}

// vault serves a directory listing, a rendered note, or an attachment,
// depending on what the URL path names in the vault.
func (h *Handler) vault(w http.ResponseWriter, r *http.Request) {
	rel := strings.Trim(path.Clean("/"+r.URL.Path), "/")
	if index.Hidden(rel) {
		http.NotFound(w, r)
		return
	}
	if rel == "" {
		h.directory(w, r, ".")
		return
	}
	// A note and a folder may share a name ("Daily.md" beside "Daily/"), and
	// then share a URL, since a note's URL drops the ".md". The note gets it:
	// that is where links lead. The folder is the URL with a slash at its end.
	if !strings.HasSuffix(r.URL.Path, "/") {
		if info, err := h.root.Stat(rel + ".md"); err == nil && !info.IsDir() {
			if h.Index.IsDrawing(rel + ".md") {
				h.drawing(w, r, rel+".md")
			} else {
				h.note(w, r, rel+".md")
			}
			return
		}
	}
	if info, err := h.root.Stat(rel); err == nil {
		switch {
		case info.IsDir():
			h.directory(w, r, rel)
		case h.Index.IsDrawing(rel) && !r.URL.Query().Has("raw"):
			h.drawing(w, r, rel)
		case strings.EqualFold(path.Ext(rel), ".md"):
			h.note(w, r, rel)
		default:
			h.attachment(w, r, rel)
		}
		return
	}
	http.NotFound(w, r)
}

// folderURL is a folder's URL. The slash at the end tells it from a note of
// the same name.
func (h *Handler) folderURL(rel string) string { return h.Index.URL(rel) + "/" }

func (h *Handler) crumbs(rel string) []crumb {
	out := []crumb{{Name: h.Name, URL: h.prefix + "/"}}
	if rel == "." || rel == "" {
		return out
	}
	info, err := h.root.Stat(rel)
	isFolder := err == nil && info.IsDir()
	parts := strings.Split(strings.TrimSuffix(rel, ".md"), "/")
	for i, part := range parts {
		at := strings.Join(parts[:i+1], "/")
		if i < len(parts)-1 || isFolder {
			out = append(out, crumb{Name: part, URL: h.folderURL(at)})
		} else {
			out = append(out, crumb{Name: part, URL: h.Index.URL(at)})
		}
	}
	return out
}

type entry struct {
	Name, URL string
	IsDir     bool
}

type directoryBody struct {
	Path    string
	Entries []entry
}

func (b noteBody) vaultPath() string      { return b.Path }
func (b drawingBody) vaultPath() string   { return b.Path }
func (b directoryBody) vaultPath() string { return b.Path }

func (h *Handler) directory(w http.ResponseWriter, r *http.Request, rel string) {
	dir, err := h.root.Open(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer dir.Close()
	infos, err := dir.ReadDir(-1)
	if err != nil {
		http.Error(w, "cannot read directory", http.StatusInternalServerError)
		return
	}
	var entries []entry
	for _, info := range infos {
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		vaultPath := path.Join(rel, name)
		e := entry{Name: name, URL: h.Index.URL(vaultPath), IsDir: info.IsDir()}
		if e.IsDir {
			e.URL = h.folderURL(vaultPath)
		}
		if note := h.Index.Note(vaultPath); note != nil {
			e.Name = note.Title
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	title := h.Name
	if rel != "." {
		title = path.Base(rel)
	}
	h.render(w, r, "directory.html", title, h.crumbs(rel), directoryBody{Path: rel, Entries: entries})
}

type property struct {
	Key   string
	Value string
}

type tagLink struct{ Tag, URL string }

type noteBody struct {
	Path       string
	Slides     int // how many slides the note would make
	HTML       template.HTML
	Properties []property
	Tags       []tagLink
	Backlinks  []*index.Note
	Stats      []stat
}

func (h *Handler) note(w http.ResponseWriter, r *http.Request, rel string) {
	src, err := h.root.ReadFile(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	html, meta, err := h.Index.Renderer().Render(src, rel)
	if err != nil {
		slog.Error("render failed", "note", rel, "err", err)
		http.Error(w, "cannot render note", http.StatusInternalServerError)
		return
	}
	body := noteBody{Path: rel, Slides: meta.Slides, HTML: html, Backlinks: h.Index.Backlinks(rel)}
	for _, tag := range meta.Tags {
		body.Tags = append(body.Tags, tagLink{tag, h.Index.TagURL(tag)})
	}
	for key, value := range meta.Frontmatter {
		if key == "title" || key == "tags" {
			continue // shown as heading and tag list
		}
		body.Properties = append(body.Properties, property{key, formatValue(value)})
	}
	sort.Slice(body.Properties, func(i, j int) bool { return body.Properties[i].Key < body.Properties[j].Key })
	body.Stats = h.noteStats(rel, src, meta, len(body.Backlinks))

	title := meta.Title
	if title == "" {
		title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	}
	h.render(w, r, "note.html", title, h.crumbs(rel), body)
}

type drawingBody struct {
	Path             string
	Image, DarkImage string
	Help             string // set when there is no picture to show
	Backlinks        []*index.Note
}

// drawing shows an Excalidraw drawing through the picture the Obsidian plugin
// exported for it. The file itself is a compressed scene wrapped in Markdown
// and says nothing to a reader.
func (h *Handler) drawing(w http.ResponseWriter, r *http.Request, rel string) {
	body := drawingBody{Path: rel, Backlinks: h.Index.Backlinks(rel)}
	if body.Image, body.DarkImage = h.Index.DrawingImages(rel); body.Image == "" {
		body.Help = render.DrawingHelp
	}
	title := index.DrawingName(rel) // a legacy .excalidraw file is no note
	if note := h.Index.Note(rel); note != nil {
		title = note.Title
	}
	h.render(w, r, "drawing.html", title, h.crumbs(rel), body)
}

func formatValue(v any) string {
	if list, ok := v.([]any); ok {
		parts := make([]string, len(list))
		for i, item := range list {
			parts[i] = fmt.Sprint(item)
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

func (h *Handler) attachment(w http.ResponseWriter, r *http.Request, rel string) {
	file, err := h.root.Open(rel)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, fs.ErrNotExist) {
			status = http.StatusNotFound
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "cannot read file", http.StatusInternalServerError)
		return
	}
	// An attachment is whatever was synced in. Never let one run as a page
	// of this origin, where it would act with the owner's login. nosniff
	// pins the type to the file extension; the types that can carry script
	// are sandboxed. Not everything is: a sandbox also stops the browser's
	// own PDF viewer from opening a PDF.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if scriptable(rel) {
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	if mime.TypeByExtension(path.Ext(rel)) == "" {
		// Otherwise ServeContent guesses the type from the content, and a
		// file without extension holding HTML would be served as a page.
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// scriptable reports whether a browser may execute script when it displays
// the file as a document. It goes by extension because the served type does
// too: attachment never lets the type be guessed from the content.
func scriptable(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm", ".xhtml", ".xht", ".svg", ".svgz", ".xml", ".xsl", ".xslt", ".mht", ".mhtml":
		return true
	}
	return false
}

type searchBody struct {
	Query     string
	Results   []index.SearchResult
	History   []string
	SearchURL string
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	title := "Search"
	if q != "" {
		title = q + " – Search"
	}
	body := searchBody{Query: q, Results: h.Index.Search(q), SearchURL: h.prefix + "/-/search"}
	if len(body.Results) > 0 {
		// Only what found something: a typo is not worth one of few places.
		body.History = h.remember(w, r, q)
	} else {
		body.History = h.history(r)
	}
	h.render(w, r, "search.html", title, append(h.crumbs(""), crumb{"Search", h.prefix + "/-/search"}), body)
}

// graph is the page; the script on it fetches graphData.
func (h *Handler) graph(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "graph.html", "Graph", append(h.crumbs(""), crumb{"Graph", h.prefix + "/-/graph"}), nil)
}

func (h *Handler) graphData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(h.Index.Graph()); err != nil {
		slog.Error("graph failed", "err", err)
	}
}

type tagCount struct {
	Tag, URL string
	Count    int
}

func (h *Handler) tags(w http.ResponseWriter, r *http.Request) {
	var list []tagCount
	for tag, count := range h.Index.Tags() {
		list = append(list, tagCount{tag, h.Index.TagURL(tag), count})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Tag < list[j].Tag })
	h.render(w, r, "tags.html", "Tags", append(h.crumbs(""), crumb{"Tags", h.prefix + "/-/tags"}), list)
}

func (h *Handler) tag(w http.ResponseWriter, r *http.Request) {
	tag := strings.Trim(r.PathValue("tag"), "/")
	h.render(w, r, "tag.html", "#"+tag,
		append(h.crumbs(""), crumb{"Tags", h.prefix + "/-/tags"}, crumb{"#" + tag, h.Index.TagURL(tag)}),
		h.Index.Tagged(tag))
}
