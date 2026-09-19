package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
)

// RepoURL is where the server's source, its releases and its issues live.
const RepoURL = "https://github.com/cschaba/markdown-webdav-backend"

type aboutBody struct {
	RepoURL    string
	ReleaseURL string // the notes of the running version; empty for a build without one
	Stats      index.Stats
	Size       string
	NotesSize  string
	LastChange time.Time
}

// about tells which server this is and how large the vault has grown.
func (h *Handler) about(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Index.Stats()
	if err != nil {
		slog.Error("statistics failed", "vault", h.Name, "err", err)
		http.Error(w, "cannot read the vault", http.StatusInternalServerError)
		return
	}
	body := aboutBody{RepoURL: RepoURL, Stats: stats, Size: formatSize(stats.Size),
		NotesSize: formatSize(stats.NotesSize), LastChange: stats.LastChange}
	if h.Version != "" {
		body.ReleaseURL = RepoURL + "/releases/tag/v" + h.Version
	}
	h.render(w, r, "about.html", "About", append(h.crumbs(""), crumb{"About", h.prefix + "/-/about"}), body)
}

// formatSize writes a number of bytes the way file managers do, in powers of
// 1024 with one decimal.
func formatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := "KMGTPE"
	value, unit := float64(n)/1024, 0
	for value >= 1024 && unit < len(units)-1 {
		value, unit = value/1024, unit+1
	}
	return fmt.Sprintf("%.1f %ciB", value, units[unit])
}
