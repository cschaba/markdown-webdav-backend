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
	fs, err := New(root, func(c Change) { changes = append(changes, c) })
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// also as a file system that ignores case or trailing dots would see it
	for _, name := range []string{"/.git", "/.git/HEAD", "/sub/../.git/HEAD", "/.GIT/HEAD", "/.git./HEAD", "/.git /HEAD"} {
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

// A symbolic link in the vault that leads out of it is no way out: a sync
// client can neither read nor write what it points at.
func TestSymlinksDoNotLeaveTheVault(t *testing.T) {
	outside, root := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "leak.txt"))
	fs, err := New(root, func(Change) {})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, name := range []string{"/out/secret.txt", "/leak.txt"} {
		if f, err := fs.OpenFile(ctx, name, os.O_RDONLY, 0); err == nil {
			f.Close()
			t.Errorf("%s can be read", name)
		}
		if f, err := fs.OpenFile(ctx, name, os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			f.Close()
			t.Errorf("%s can be written", name)
		}
	}
	if f, err := fs.OpenFile(ctx, "/out/new.txt", os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
		f.Close()
		t.Error("a file can be created outside the vault")
	}
	if err := fs.RemoveAll(ctx, "/out/secret.txt"); err == nil {
		t.Error("a file outside the vault can be removed")
	}
	if content, _ := os.ReadFile(filepath.Join(outside, "secret.txt")); string(content) != "secret" {
		t.Errorf("the file outside is now %q", content)
	}
	if err := fs.RemoveAll(ctx, "/"); err == nil {
		t.Error("the vault itself can be removed")
	}
}
