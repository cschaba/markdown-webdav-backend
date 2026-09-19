package web

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cschaba/markdown-webdav-backend/internal/render"
)

// A symbolic link in the vault that leads out of it shows nothing of what it
// leads to: not as a note, not embedded in another, not as an attachment, and
// not as an excerpt in the search.
func TestSymlinksDoNotLeaveTheVault(t *testing.T) {
	outside := t.TempDir()
	const secret = "zeppelin-outside-the-vault"
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("# Secret\n\n"+secret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Host.md"), []byte("![[Leak]]\n\n![[inside]]\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "Real.md"), []byte("zeppelin-inside\n"), 0o644)
	for link, to := range map[string]string{
		"Leak.md": filepath.Join(outside, "secret.md"), "leak.txt": filepath.Join(outside, "secret.md"),
		"out": outside, "inside.md": "Real.md", // a link that stays inside still works
	} {
		if err := os.Symlink(to, filepath.Join(dir, link)); err != nil {
			t.Skip("no symbolic links here:", err)
		}
	}
	get := serve(t, "v", dir)
	for _, p := range []string{"/v/Leak", "/v/Host", "/v/leak.txt", "/v/out/secret", "/v/out/", "/v/-/search?q=zeppelin", "/v/-/export?path=Leak.md", "/v/-/slides?path=Leak.md"} {
		body, _ := io.ReadAll(get(p).Body)
		if strings.Contains(string(body), secret) {
			t.Errorf("%s shows a file from outside the vault", p)
		}
	}
	if body, _ := io.ReadAll(get("/v/Host").Body); !strings.Contains(string(body), "zeppelin-inside") {
		t.Errorf("a link that stays in the vault should still be embedded:\n%s", body)
	}
}

func TestPagesCarryAPolicy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Note.md"), []byte("```mermaid\ngraph TD; A-->B\n```\n\n---\n\nsecond\n"), 0o644)
	get := serve(t, "v", dir)
	for _, p := range []string{"/v/", "/v/Note", "/v/-/tags", "/v/-/search?q=x", "/v/-/graph", "/v/-/export?path=Note.md", "/v/-/slides?path=Note.md", "/v/-/export?path=Note.md&format=slides"} {
		res := get(p)
		body, _ := io.ReadAll(res.Body)
		csp := res.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self' https://") || strings.Contains(strings.SplitN(csp, "style-src", 2)[0], "unsafe-inline") {
			t.Errorf("%s: policy %q", p, csp)
		}
		// The policy allows no inline script, so a page must not rely on one:
		// it would silently not run, and only a browser would show it.
		for _, tag := range strings.Split(string(body), "<script")[1:] {
			if !strings.HasPrefix(tag, ` src="`) {
				t.Errorf("%s has an inline script: <script%.80s", p, tag)
			}
			if strings.HasPrefix(tag, ` src="http`) && !strings.Contains(strings.SplitN(tag, ">", 2)[0], `integrity="sha384-`) {
				t.Errorf("%s loads a script from elsewhere without a hash: <script%.120s", p, tag)
			}
		}
	}
	// every script from elsewhere names its version, and is what the policy allows
	for _, s := range []render.ExternalScript{render.MermaidScript, render.ForceGraphScript} {
		if !strings.Contains(s.URL, "@") || !strings.Contains(contentSecurityPolicy, s.URL) {
			t.Errorf("%s: no version, or not in the policy", s.URL)
		}
	}
}
