package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGitIsInvisibleAndChangesAreReported(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref"), 0o644)

	var changes []Change
	fs := New(root, func(c Change) { changes = append(changes, c) })
	ctx := context.Background()

	for _, name := range []string{"/.git", "/.git/HEAD", "/sub/../.git/HEAD"} {
		if _, err := fs.Stat(ctx, name); !os.IsNotExist(err) {
			t.Errorf("Stat(%q) err = %v, want not exist", name, err)
		}
		if _, err := fs.OpenFile(ctx, name, os.O_RDWR|os.O_CREATE, 0o644); err == nil {
			t.Errorf("OpenFile(%q) succeeded", name)
		}
	}
	if err := fs.RemoveAll(ctx, "/.git"); err == nil {
		t.Error("RemoveAll(.git) succeeded")
	}
	if err := fs.Rename(ctx, "/.git", "/stolen"); err == nil {
		t.Error("Rename(.git) succeeded")
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "HEAD")); err != nil {
		t.Fatalf(".git was damaged: %v", err)
	}

	f, err := fs.OpenFile(ctx, "/Note.md", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("hello"))
	if len(changes) != 0 {
		t.Error("change reported before the file was closed")
	}
	f.Close()
	if len(changes) != 1 || changes[0] != (Change{Path: "Note.md", FileWrite: true}) {
		t.Errorf("changes = %+v", changes)
	}

	dir, _ := fs.OpenFile(ctx, "/", os.O_RDONLY, 0)
	infos, _ := dir.Readdir(-1)
	dir.Close()
	if len(infos) != 1 || infos[0].Name() != "Note.md" {
		t.Errorf("listing = %v", infos)
	}
	if len(changes) != 1 {
		t.Errorf("reading reported a change: %+v", changes)
	}
}
