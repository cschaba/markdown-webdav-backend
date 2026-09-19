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

// handlerFor mounts the vault in dir as "/<name>".
func handlerFor(t *testing.T, name, dir string) http.Handler {
	t.Helper()
	idx := index.New(dir, "/"+name)
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{Name: name, Dir: dir, Index: idx})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// serve mounts the vault in dir as "/<name>" and returns a GET function.
func serve(t *testing.T, name, dir string) func(string) *http.Response {
	t.Helper()
	idx := index.New(dir, "/"+name)
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	vaults := []Vault{{Name: name, URL: "/" + name + "/"}, {Name: "other", URL: "/other/"}}
	h, err := New(Config{Name: name, Dir: dir, Index: idx, Vaults: vaults})
	if err != nil {
		t.Fatal(err)
	}
	return func(p string) *http.Response {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		return rec.Result()
	}
}

func TestAttachmentsCannotScriptTheOrigin(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"evil.html": "<script>alert(1)</script>",
		"evil.SVG":  `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"doc.pdf":   "%PDF-1.4",
		"notes.txt": "<script>alert(1)</script>",
		"page":      "<html><script>alert(1)</script></html>",
		".git/HEAD": "ref",
	} {
		full := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	get := serve(t, "files", root)
	for p, sandboxed := range map[string]bool{
		"/files/evil.html": true, "/files/evil.SVG": true,
		"/files/doc.pdf":   false, // a sandbox would block the browser's PDF viewer
		"/files/notes.txt": false,
	} {
		res := get(p)
		if res.StatusCode != 200 || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: status %d, nosniff %q", p, res.StatusCode, res.Header.Get("X-Content-Type-Options"))
		}
		if got := res.Header.Get("Content-Security-Policy") == "sandbox"; got != sandboxed {
			t.Errorf("%s: sandboxed = %v, want %v", p, got, sandboxed)
		}
	}
	if ct := get("/files/notes.txt").Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("notes.txt served as %q; with nosniff it must not become HTML", ct)
	}
	if ct := get("/files/page").Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("file without extension served as %q; its type must not be guessed from the content", ct)
	}
	if code := get("/files/.git/HEAD").StatusCode; code != 404 {
		t.Errorf(".git reachable through the web view: %d", code)
	}
}

func TestAssets(t *testing.T) {
	h, err := Assets()
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string][2]string{
		"/-/style.css": {"text/css", ".chroma"}, // includes the highlight stylesheet
		"/-/graph.js":  {"text/javascript", "ForceGraph"},
		"/-/print.js":  {"text/javascript", "beforeprint"},
		"/-/keys.js":   {"text/javascript", "BINDINGS"},
	} {
		res := httptestGet(h, p)
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), want[0]) || !strings.Contains(string(b), want[1]) {
			t.Errorf("%s: status %d, type %q, %d bytes", p, res.StatusCode, res.Header.Get("Content-Type"), len(b))
		}
	}
	if code := httptestGet(h, "/-/nothing").StatusCode; code != 404 {
		t.Errorf("unknown asset: %d", code)
	}
}

// A sync client may upload a note before the image it embeds.
func TestAttachmentArrivingLater(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "Note.md"), []byte("![[late.png]]"), 0o644)
	idx := index.New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{Name: "v", Dir: root, Index: idx})
	if err != nil {
		t.Fatal(err)
	}
	page := func() string {
		b, _ := io.ReadAll(httptestGet(h, "/v/Note").Body)
		return string(b)
	}
	if html := page(); !strings.Contains(html, `class="missing missing-embed"`) || strings.Contains(html, "<img") {
		t.Errorf("before the upload:\n%s", html)
	}
	os.MkdirAll(filepath.Join(root, "img"), 0o755)
	os.WriteFile(filepath.Join(root, "img", "late.png"), []byte("png"), 0o644)
	idx.Update("img/late.png") // what a WebDAV PUT triggers
	if html := page(); !strings.Contains(html, `<img src="/v/img/late.png">`) || strings.Contains(html, "missing") {
		t.Errorf("after the upload:\n%s", html)
	}
	os.Remove(filepath.Join(root, "img", "late.png"))
	idx.Update("img/late.png")
	if html := page(); !strings.Contains(html, `class="missing missing-embed"`) {
		t.Errorf("after the file was deleted again:\n%s", html)
	}
}

func httptestGet(h http.Handler, p string) *http.Response {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
	return rec.Result()
}
