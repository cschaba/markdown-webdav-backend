package web

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
)

// The slide show: a note whose slides are separated the Obsidian way, shown
// one at a time. See render/slides.go for what a slide is, static/slides.js
// for how it is shown.

// slideSize is the fixed size a slide is laid out at, in CSS pixels. The page
// it is printed on is given in the same pixels, not in inches or as "A4": the
// page then is the slide by construction. A page a rounding error smaller than
// its slide prints a blank sheet after every deck (13.3333in is 1279.997px).
// At 96 pixels to the inch these are 13.33x7.5in, 10x7.5in, and A4 landscape
// to within a tenth of a millimetre.
type slideSize struct {
	Name string
	W, H int // px
}

func (s slideSize) css() string { return fmt.Sprintf("%dpx %dpx", s.W, s.H) }

var slideSizes = []slideSize{
	{Name: "16:9", W: 1280, H: 720},
	{Name: "4:3", W: 960, H: 720},
	{Name: "A4", W: 1123, H: 794},
}

type slide struct {
	N    int
	HTML template.HTML
}

type slidesPage struct {
	Title   string
	BackURL string
	PDFURL  string
	W, H    int
	Slides  []slide
	Total   int
	Mermaid bool
}

func (h *Handler) slides(w http.ResponseWriter, r *http.Request) {
	rel := strings.Trim(path.Clean("/"+r.URL.Query().Get("path")), "/")
	note := h.Index.Note(rel)
	if index.Hidden(rel) || note == nil || note.Drawing {
		http.NotFound(w, r)
		return
	}
	src, err := h.root.ReadFile(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	deck, err := h.Index.Renderer().RenderSlides(src, rel)
	if err != nil {
		slog.Error("render failed", "note", rel, "err", err)
		http.Error(w, "cannot render note", http.StatusInternalServerError)
		return
	}
	p := slidesPage{Title: note.Title, BackURL: note.URL, PDFURL: h.exportURL(rel) + "&format=slides",
		W: slideSizes[0].W, H: slideSizes[0].H, Total: len(deck.Slides), Mermaid: deck.Mermaid}
	for i, html := range deck.Slides {
		p.Slides = append(p.Slides, slide{N: i + 1, HTML: html})
	}
	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, "slides.html", p); err != nil {
		slog.Error("template failed", "template", "slides.html", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	_, _ = w.Write([]byte(buf.String()))
}

func (h *Handler) slidesURL(rel string) string {
	return h.prefix + "/-/slides?path=" + strings.ReplaceAll(url.QueryEscape(rel), "+", "%20")
}
