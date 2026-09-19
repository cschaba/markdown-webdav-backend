package web

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
)

func TestNoteStats(t *testing.T) {
	root := t.TempDir()
	src := "---\ntitle: Counted\n---\n# One\n\nTwo words, [[B|three]].\n\n- [x] done\n- [ ] open\n"
	for name, content := range map[string]string{"A.md": src, "B.md": "[[A]]"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx := index.New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{Name: "v", Dir: root, Index: idx})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/v/A", nil))
	page := rec.Body.String()
	for _, want := range []string{
		"<dt>Words</dt><dd>6</dd>", // One, Two, words, three, done, open
		"<dt>Reading time</dt><dd>1 min</dd>",
		"<dt>Headings</dt><dd>1</dd>",
		"<dt>Links</dt><dd>1</dd>",
		"<dt>Backlinks</dt><dd>1</dd>",
		"<dt>Tasks</dt><dd>1 of 2 done</dd>",
		"<dt>Size</dt><dd>" + formatSize(int64(len(src))) + "</dd>",
		"<dt>Changed</dt>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the statistics lack %q", want)
		}
	}
	if strings.Contains(page, "<dt>Slides</dt>") {
		t.Error("a note without slides shows a slide count")
	}
}

func TestThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567", -1234: "-1,234"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
}
