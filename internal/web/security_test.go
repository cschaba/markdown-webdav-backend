package web

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
