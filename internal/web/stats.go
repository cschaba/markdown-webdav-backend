package web

import (
	"fmt"
	"strconv"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
	"github.com/cschaba/markdown-webdav-backend/internal/render"
)

// A note's statistics, shown in a panel that the key "i" or the button at the
// foot opens and closes (keys.js). The panel is in the page from the start and
// only hidden: it costs nothing that the page does not already know, and it
// needs no request of its own.

type stat struct{ Name, Value string }

// wordsPerMinute is a common estimate for reading prose on a screen.
const wordsPerMinute = 200

func (h *Handler) noteStats(rel string, src []byte, meta render.Meta, backlinks int) []stat {
	minutes := (meta.Words + wordsPerMinute - 1) / wordsPerMinute
	reading := fmt.Sprintf("%d min", minutes)
	if meta.Words == 0 {
		reading = "–"
	}
	stats := []stat{
		{"Words", thousands(meta.Words)},
		{"Characters", thousands(meta.Characters)},
		{"Reading time", reading},
		{"Headings", thousands(len(meta.Headings))},
		{"Links", thousands(len(meta.Links))},
		{"Backlinks", thousands(backlinks)},
	}
	if open, done := index.CountTasks(string(src)); open+done > 0 {
		stats = append(stats, stat{"Tasks", fmt.Sprintf("%d of %d done", done, open+done)})
	}
	if meta.Slides > 1 {
		stats = append(stats, stat{"Slides", thousands(meta.Slides)})
	}
	stats = append(stats, stat{"Size", formatSize(int64(len(src)))})
	if info, err := h.root.Stat(rel); err == nil {
		stats = append(stats, stat{"Changed", info.ModTime().Format("2 Jan 2006, 15:04")})
	}
	return stats
}

// thousands writes 12345 as "12,345".
func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
