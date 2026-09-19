package index

import (
	"io/fs"
	"path"
	"strings"
	"time"
)

// Stats is what the About page tells about a vault.
type Stats struct {
	Notes        int // without the drawings
	Drawings     int
	Attachments  int // every file that is neither
	Folders      int
	Tags         int // as the Tags page counts them, parents of nested tags included
	Links        int // links and embeds in notes, into the vault
	MissingLinks int // those with no file behind them
	Size         int64
	NotesSize    int64     // the Markdown files' share of Size, drawings included
	LastChange   time.Time // zero in an empty vault
}

// Stats counts the vault. What a note says comes from the index; files,
// folders and sizes are read from the disk on every call, the index does not
// keep them. Dot entries count nowhere, as they show nowhere - so Size is
// without the git history.
func (idx *Index) Stats() (Stats, error) {
	var s Stats
	idx.mu.RLock()
	notes := make([]*Note, 0, len(idx.notes))
	for _, note := range idx.notes {
		notes = append(notes, note)
	}
	idx.mu.RUnlock()
	for _, note := range notes {
		if note.Drawing {
			s.Drawings++
		} else {
			s.Notes++
		}
		for _, link := range note.Links {
			s.Links++
			if _, ok := idx.Resolve(note.Path, link); !ok {
				s.MissingLinks++
			}
		}
	}
	s.Tags = len(idx.Tags())

	// Opens the vault if nothing has yet.
	if f, err := idx.file("."); err != nil {
		return s, err
	} else {
		f.Close()
	}
	err := fs.WalkDir(idx.confined.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			s.Folders++
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // gone since the directory was read
		}
		s.Size += info.Size()
		if info.ModTime().After(s.LastChange) {
			s.LastChange = info.ModTime()
		}
		switch {
		case strings.EqualFold(path.Ext(p), ".md"):
			s.NotesSize += info.Size()
		case drawingPath(p):
			s.Drawings++ // a legacy drawing is no note, so the index did not count it
		default:
			s.Attachments++
		}
		return nil
	})
	return s, err
}
