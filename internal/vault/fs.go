// Package vault exposes the notes directory as a webdav.FileSystem and reports
// every change, so the index and the git committer never have to poll.
package vault

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/webdav"
)

// Change describes one modification made through WebDAV. Path is
// slash-separated and relative to the vault root, without a leading slash.
type Change struct {
	Path string
	// FileWrite is true when a single regular file was written. Everything
	// else (delete, rename, mkdir) may touch whole subtrees.
	FileWrite bool
}

// FS wraps a directory. It hides the git repository from clients: a sync
// client that uploads or deletes inside .git destroys the history.
//
// Every access goes through os.Root, as in the web view: a symbolic link in
// the vault that points out of it is not followed. webdav.Dir would follow
// it, and hand a sync client whatever the link leads to, to read and to write.
type FS struct {
	root     *os.Root
	onChange func(Change)
}

func New(dir string, onChange func(Change)) (*FS, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &FS{root: root, onChange: onChange}, nil
}

// Hidden reports whether a vault path must not be visible over WebDAV. The
// comparison ignores case, and the dots and spaces that some file systems
// drop from the end of a name: there ".GIT" and ".git." are the repository.
func Hidden(name string) bool {
	for _, part := range strings.Split(path.Clean("/"+name), "/") {
		if strings.EqualFold(strings.TrimRight(part, ". "), ".git") {
			return true
		}
	}
	return false
}

func rel(name string) string {
	return strings.TrimPrefix(path.Clean("/"+name), "/")
}

// local is name as os.Root wants it: relative, "." for the vault itself.
func local(name string) string {
	if r := rel(name); r != "" {
		return filepath.FromSlash(r)
	}
	return "."
}

func (f *FS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if Hidden(name) {
		return os.ErrPermission
	}
	if err := f.root.Mkdir(local(name), perm); err != nil {
		return err
	}
	f.onChange(Change{Path: rel(name)})
	return nil
}

func (f *FS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if Hidden(name) {
		return nil, os.ErrNotExist
	}
	file, err := f.root.OpenFile(local(name), flag, perm)
	if err != nil {
		return nil, err
	}
	w := &watchedFile{File: file}
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		w.onClose = func() { f.onChange(Change{Path: rel(name), FileWrite: true}) }
	}
	return w, nil
}

func (f *FS) RemoveAll(ctx context.Context, name string) error {
	if Hidden(name) {
		return os.ErrPermission
	}
	if rel(name) == "" {
		return os.ErrInvalid // the vault itself stays
	}
	if err := f.root.RemoveAll(local(name)); err != nil {
		return err
	}
	f.onChange(Change{Path: rel(name)})
	return nil
}

func (f *FS) Rename(ctx context.Context, oldName, newName string) error {
	if Hidden(oldName) || Hidden(newName) {
		return os.ErrPermission
	}
	if rel(oldName) == "" || rel(newName) == "" {
		return os.ErrInvalid
	}
	if err := f.root.Rename(local(oldName), local(newName)); err != nil {
		return err
	}
	f.onChange(Change{Path: rel(newName)})
	return nil
}

func (f *FS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if Hidden(name) {
		return nil, os.ErrNotExist
	}
	return f.root.Stat(local(name))
}

// watchedFile reports a write once the client has finished it, and filters
// .git out of directory listings.
type watchedFile struct {
	webdav.File
	onClose func()
}

func (w *watchedFile) Close() error {
	err := w.File.Close()
	if w.onClose != nil && err == nil {
		w.onClose()
	}
	return err
}

func (w *watchedFile) Readdir(count int) ([]fs.FileInfo, error) {
	infos, err := w.File.Readdir(count)
	visible := infos[:0]
	for _, info := range infos {
		if !Hidden(info.Name()) {
			visible = append(visible, info)
		}
	}
	return visible, err
}
