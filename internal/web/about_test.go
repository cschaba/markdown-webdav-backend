package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cschaba/markdown-webdav-backend/internal/index"
)

func TestAbout(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"A.md": "[[B]] [[Gone]]", "B.md": "#tag", "pic.png": "12345", ".git/HEAD": "hidden"} {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx := index.New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{Name: "v", Version: "1.2.3", Dir: root, Index: idx})
	if err != nil {
		t.Fatal(err)
	}
	get := func(p string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", p, rec.Code)
		}
		b, _ := io.ReadAll(rec.Body)
		return string(b)
	}
	about := get("/v/-/about")
	for _, want := range []string{
		"<td>markdown-webdav-backend 1.2.3</td>",
		`<a href="https://github.com/cschaba/markdown-webdav-backend" rel="noreferrer">`,
		`<a href="https://github.com/cschaba/markdown-webdav-backend/releases/tag/v1.2.3" rel="noreferrer">v1.2.3</a>`,
		"<tr><th>Notes</th><td>2</td></tr>",
		"<tr><th>Attachments</th><td>1</td></tr>",
		"<tr><th>Folders</th><td>0</td></tr>",
		`<tr><th>Tags</th><td><a href="/v/-/tags">1</a></td></tr>`,
		"<td>2, 1 of them to a file that is missing</td>",
		"<td>23 B, of which 18 B are Markdown</td>",
		"<th>Last change</th>",
	} {
		if !strings.Contains(about, want) {
			t.Errorf("the About page lacks %q", want)
		}
	}
	// Every page leads there, and says which version serves it.
	if want := `<a class="about" href="/v/-/about">markdown-webdav-backend 1.2.3</a>`; !strings.Contains(get("/v/A"), want) {
		t.Errorf("a note's foot lacks %q", want)
	}
}

func TestFormatSize(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB",
		5 << 20: "5.0 MiB", 3 << 30: "3.0 GiB"} {
		if got := formatSize(n); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", n, got, want)
		}
	}
}
