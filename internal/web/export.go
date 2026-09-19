package web

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"markdown-webdav-backend/internal/index"
)

// Export to PDF. The server does not make PDFs: it serves a page laid out for
// paper - no site chrome, book margins - and the browser's own "Save as PDF"
// does the rest. That needs nothing installed beside the server
// and works where the notes are read, the phone included. What paper can do
// is CSS paged media (@page), checked by printing to PDF and reading the
// result back, see tools/check-pdf-export.sh.
//
// There are no page numbers of our own. They were built, with CSS page-margin
// boxes, and taken out again: only Chromium draws those, so in any other
// browser the numbers, and a "first page number" setting with them, silently
// did nothing. The print dialog's own headers and footers number the pages in
// every browser.

// pageSize is a paper format and the margins that suit it, in millimetres,
// the bottom one a little larger, as in a book.
type pageSize struct {
	Name              string
	css               string // what @page { size } takes
	W, H              float64
	Top, Side, Bottom float64
}

var pageSizes = []pageSize{
	{Name: "A4", css: "A4 portrait", W: 210, H: 297, Top: 22, Side: 20, Bottom: 26},
	{Name: "A5", css: "A5 portrait", W: 148, H: 210, Top: 17, Side: 15, Bottom: 21},
	{Name: "B5", css: "176mm 250mm", W: 176, H: 250, Top: 19, Side: 17, Bottom: 23},
	{Name: "Letter", css: "letter portrait", W: 215.9, H: 279.4, Top: 22, Side: 20, Bottom: 26},
	{Name: "Legal", css: "legal portrait", W: 215.9, H: 355.6, Top: 22, Side: 20, Bottom: 26},
}

// The page size and the sub pages switch are remembered in a cookie, like the
// search history and for the same reason: there is nowhere else to keep them.
const exportCookie = "export-settings"

type exportDoc struct {
	Title     string
	URL       string
	HTML      template.HTML
	ShowTitle bool // unless the note opens with its title as a heading anyway
}

type exportPage struct {
	Title   string
	Vault   string
	Path    string // what is exported, a note or a folder
	BackURL string
	Action  string
	Size    pageSize
	Sizes   []pageSize
	Sub     bool
	HasSub  bool // there are sub pages to include
	Docs    []exportDoc
	PageCSS template.CSS
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rel := strings.Trim(path.Clean("/"+q.Get("path")), "/")
	if index.Hidden(rel) {
		http.NotFound(w, r)
		return
	}
	p := exportPage{Vault: h.Name, Path: rel, Action: h.prefix + "/-/export", Sizes: pageSizes, Size: pageSizes[0]}
	// The form says set=1: then what it sends is the choice, a missing "sub"
	// included, and is remembered. A plain link, as in a page's footer, says
	// nothing, and gets what was chosen last.
	chosen := q.Get("set") == "1" || q.Has("size") || q.Has("sub")
	settings := q
	if !chosen {
		if cookie, err := r.Cookie(exportCookie); err == nil {
			if saved, err := url.ParseQuery(cookie.Value); err == nil {
				settings = saved
			}
		}
	}
	p.Sub = settings.Get("sub") == "1"
	for _, size := range pageSizes {
		if strings.EqualFold(size.Name, settings.Get("size")) {
			p.Size = size
		}
	}
	if chosen {
		saved := url.Values{"size": {p.Size.Name}} // the validated values, not the request's
		if p.Sub {
			saved.Set("sub", "1")
		}
		http.SetCookie(w, &http.Cookie{
			Name: exportCookie, Value: saved.Encode(), Path: h.prefix + "/-/export", MaxAge: 365 * 24 * 60 * 60,
			HttpOnly: true, SameSite: http.SameSiteLaxMode,
			Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		})
	}
	// A note brings the notes of the folder named like it as its sub pages
	// ("Daily.md" and "Daily/"); a folder brings the folders below it.
	var notes []*index.Note
	switch info, err := h.root.Stat(orDot(rel)); {
	case err == nil && info.IsDir():
		p.Title, p.BackURL = path.Base(rel), h.folderURL(rel)
		if rel == "" {
			p.Title, p.BackURL = h.Name, h.prefix+"/"
		}
		notes = h.Index.NotesIn(rel, p.Sub)
		p.HasSub = len(h.Index.NotesIn(rel, true)) > len(h.Index.NotesIn(rel, false))
	case err == nil && h.Index.Note(rel) != nil:
		note := h.Index.Note(rel)
		p.Title, p.BackURL = note.Title, note.URL
		notes = []*index.Note{note}
		sub := h.Index.NotesIn(strings.TrimSuffix(rel, path.Ext(rel)), true)
		if p.HasSub = len(sub) > 0; p.Sub {
			notes = append(notes, sub...)
		}
	default:
		http.NotFound(w, r)
		return
	}

	for _, note := range notes {
		doc := exportDoc{Title: note.Title, URL: note.URL}
		if note.Drawing {
			image, _ := h.Index.DrawingImages(note.Path)
			if image == "" {
				continue // nothing to put on paper
			}
			doc.ShowTitle = true
			doc.HTML = template.HTML(`<p><img src="` + template.HTMLEscapeString(image) + `" alt=""></p>`)
		} else {
			src, err := h.root.ReadFile(note.Path)
			if err != nil {
				continue
			}
			html, _, err := h.Index.Renderer().Render(src, note.Path)
			if err != nil {
				slog.Error("render failed", "note", note.Path, "err", err)
				continue
			}
			doc.HTML = html
			doc.ShowTitle = !strings.HasPrefix(strings.TrimSpace(string(html)), openingHeading)
		}
		p.Docs = append(p.Docs, doc)
	}

	p.PageCSS = template.CSS(fmt.Sprintf("@page { size: %s; margin: %gmm %gmm %gmm; }", p.Size.css, p.Size.Top, p.Size.Side, p.Size.Bottom))

	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, "export.html", p); err != nil {
		slog.Error("template failed", "template", "export.html", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(buf.String()))
}

// openingHeading is how a rendered note starts that opens with a level-one
// heading. Such a note has its title; any other is given its name as one.
const openingHeading = `<details class="fold" open>` + "\n<summary><h1"

func orDot(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}

// exportURL is the link in a page's footer.
func (h *Handler) exportURL(rel string) string {
	if rel == "." {
		rel = ""
	}
	// %20 rather than "+": html/template would write the plus as &#43;
	return h.prefix + "/-/export?path=" + strings.ReplaceAll(url.QueryEscape(rel), "+", "%20")
}
