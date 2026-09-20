// Package index keeps what the web view needs to know about the vault in
// memory: which files exist, and each note's title, tags and links. It is
// rebuilt from the files at start, so there is no database to get out of sync.
package index

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cschaba/markdown-webdav-backend/internal/render"
)

type Note struct {
	Path    string // vault-relative, slash-separated, including ".md"
	URL     string // where the web view serves it
	Title   string
	Drawing bool       // an Excalidraw drawing, see drawing.go
	props   []property // front matter values for the search, see search.go
	ModTime time.Time
	render.Meta
}

// TagPath is where a vault's tag pages live, below its URL prefix.
const TagPath = "/-/tag/"

type Index struct {
	root     string
	openRoot sync.Once
	confined *os.Root // root, opened; see file
	rootErr  error
	prefix   string // URL prefix of the vault in the web view, e.g. "/notes"
	renderer *render.Renderer

	mu     sync.RWMutex
	notes  map[string]*Note    // by Path
	byName map[string][]string // lowercased base name -> vault paths, all files
}

// New indexes the vault in root, which the web view serves below urlPrefix.
func New(root, urlPrefix string) *Index {
	idx := &Index{root: root, prefix: urlPrefix}
	// The renderer resolves links through the index, and the index parses
	// notes through the renderer.
	idx.renderer = render.New(idx, urlPrefix+TagPath)
	return idx
}

// file opens a file of the vault, and nothing else: through os.Root a symbolic
// link that leads out of the vault is not followed. The index feeds search
// excerpts and note embeds, so a link to a file outside would otherwise be a
// way to read it in the web view.
func (idx *Index) file(vaultPath string) (*os.File, error) {
	idx.openRoot.Do(func() { idx.confined, idx.rootErr = os.OpenRoot(idx.root) })
	if idx.rootErr != nil {
		return nil, idx.rootErr
	}
	return idx.confined.Open(filepath.FromSlash(vaultPath))
}

// readFile reads a file of the vault, see file.
func (idx *Index) readFile(vaultPath string) ([]byte, error) {
	f, err := idx.file(vaultPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// URL maps a vault path to its URL; notes drop the ".md". Every URL into a
// vault is built here, so the prefix cannot be forgotten.
func (idx *Index) URL(vaultPath string) string {
	return idx.prefix + "/" + render.EscapePath(strings.TrimSuffix(vaultPath, ".md"))
}

// TagURL is the page listing the notes that carry tag.
func (idx *Index) TagURL(tag string) string {
	return idx.prefix + TagPath + render.EscapePath(tag)
}

func (idx *Index) Renderer() *render.Renderer { return idx.renderer }

// Rebuild scans the whole vault.
func (idx *Index) Rebuild() error {
	notes := map[string]*Note{}
	byName := map[string][]string{}
	err := filepath.WalkDir(idx.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == idx.root {
			return nil
		}
		// Dot entries are client state (.obsidian, .trash) or ours (.git).
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(idx.root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key := strings.ToLower(path.Base(rel))
		byName[key] = append(byName[key], rel)
		if note := idx.load(rel); note != nil {
			notes[rel] = note
		}
		return nil
	})
	if err != nil {
		return err
	}
	idx.mu.Lock()
	idx.notes, idx.byName = notes, byName
	idx.mu.Unlock()
	return nil
}

// Update re-reads one file after it was written. Anything that can move or
// remove whole subtrees should call Rebuild instead.
func (idx *Index) Update(rel string) {
	if Hidden(rel) {
		return
	}
	note := idx.load(rel)
	f, statErr := idx.file(rel)
	if statErr == nil {
		f.Close()
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()
	if note != nil {
		idx.notes[rel] = note
	} else {
		delete(idx.notes, rel)
	}
	key := strings.ToLower(path.Base(rel))
	paths := idx.byName[key][:0:0]
	for _, p := range idx.byName[key] {
		if p != rel {
			paths = append(paths, p)
		}
	}
	if statErr == nil {
		paths = append(paths, rel)
	}
	idx.byName[key] = paths
}

// Hidden reports whether a vault path has a dot component and therefore is
// not part of the rendered site.
func Hidden(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func (idx *Index) load(rel string) *Note {
	if !strings.EqualFold(path.Ext(rel), ".md") {
		return nil
	}
	f, err := idx.file(rel)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return nil
	}
	src, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	note := &Note{Path: rel, URL: idx.URL(rel), ModTime: info.ModTime(), Meta: idx.renderer.Meta(src)}
	note.props = properties(note.Frontmatter)
	_, hasKey := note.Frontmatter[drawingKey]
	note.Drawing = hasKey || drawingPath(rel)
	note.Title = note.Meta.Title
	if note.Title == "" {
		note.Title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		if note.Drawing {
			note.Title = DrawingName(rel)
		}
	}
	return note
}

// ResolveLink implements render.LinkResolver with Obsidian's rules: a link
// names a file by base name or by a trailing part of its path, and ".md" is
// optional. Among several matches the shortest path wins.
func (idx *Index) ResolveLink(from, target string) (string, bool) {
	p, ok := idx.Resolve(from, target)
	if !ok {
		return "", false
	}
	return idx.URL(p), true
}

// HasAnchor implements render.LinkResolver.
func (idx *Index) HasAnchor(from, target, id string) bool {
	p, ok := idx.Resolve(from, target)
	if !ok {
		return true
	}
	note := idx.Note(p)
	return note == nil || slices.Contains(note.Headings, id) || slices.Contains(note.Blocks, id)
}

// Resolve returns the vault path that target means in the note at from (a
// vault path; "" stands for a note in the root). The same rules serve
// [[wikilinks]], ![[embeds]] and Markdown links. Case never matters, and ".md"
// may be left out.
//
//	[[Name]]         next to the linking note first, then anywhere in the
//	                 vault, the shortest path winning (Obsidian's rule)
//	[[sub/Name]]     relative to the linking note, then from the vault root,
//	                 then any file whose path ends like that (Obsidian writes
//	                 such links, as short as is still unambiguous)
//	[[./x]] [[../x]] relative to the linking note, and nothing else
//	[[/folder/x]]    from the vault root, and nothing else
//
// A path that says where to look is followed strictly: if nothing is there,
// the link is missing, rather than quietly pointing at a namesake elsewhere.
func (idx *Index) Resolve(from, target string) (string, bool) {
	target = strings.ToLower(strings.TrimSpace(target))
	dir := strings.ToLower(path.Dir(from))
	if dir == "." {
		dir = ""
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	switch {
	case strings.Trim(target, "/") == "":
		return "", false
	case strings.HasPrefix(target, "/"):
		return idx.exact(path.Clean(target)[1:])
	case target == "." || target == ".." || strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../"):
		joined := path.Join(dir, target)
		if joined == ".." || strings.HasPrefix(joined, "../") {
			return "", false // above the vault
		}
		return idx.exact(joined)
	}
	target = path.Clean(target)
	if p, ok := idx.exact(path.Join(dir, target)); ok {
		return p, true
	}
	if strings.Contains(target, "/") && dir != "" {
		if p, ok := idx.exact(target); ok {
			return p, true
		}
	}
	return idx.anywhere(target)
}

// candidates: note names may contain dots ("v1.2 plan"), so an extension
// proves nothing. Try the note first, then the literal file.
func candidates(target string) []string { return []string{target + ".md", target} }

// exact finds the file at a lowercased vault path. The caller holds the lock.
func (idx *Index) exact(target string) (string, bool) {
	for _, candidate := range candidates(target) {
		for _, p := range idx.byName[path.Base(candidate)] {
			if strings.ToLower(p) == candidate {
				return p, true
			}
		}
	}
	return "", false
}

// anywhere finds a file by name or by the end of its path, the shortest path
// winning. The caller holds the lock.
func (idx *Index) anywhere(target string) (string, bool) {
	for _, candidate := range candidates(target) {
		var matches []string
		for _, p := range idx.byName[path.Base(candidate)] {
			lower := strings.ToLower(p)
			if lower == candidate || strings.HasSuffix(lower, "/"+candidate) {
				matches = append(matches, p)
			}
		}
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				if len(matches[i]) != len(matches[j]) {
					return len(matches[i]) < len(matches[j])
				}
				return matches[i] < matches[j]
			})
			return matches[0], true
		}
	}
	return "", false
}

func (idx *Index) Note(rel string) *Note {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.notes[rel]
}

// Backlinks returns the notes linking to rel, sorted by title.
func (idx *Index) Backlinks(rel string) []*Note {
	idx.mu.RLock()
	candidates := make([]*Note, 0, len(idx.notes))
	for _, note := range idx.notes {
		if note.Path != rel {
			candidates = append(candidates, note)
		}
	}
	idx.mu.RUnlock()

	var out []*Note
	for _, note := range candidates {
		for _, link := range note.Links {
			if p, ok := idx.Resolve(note.Path, link); ok && p == rel {
				out = append(out, note)
				break
			}
		}
	}
	sortNotes(out)
	return out
}

// Tags returns every tag with the number of notes carrying it. A nested tag
// counts for its parents too, as in Obsidian: "project/alpha" is also "project".
func (idx *Index) Tags() map[string]int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	counts := map[string]int{}
	for _, note := range idx.notes {
		seen := map[string]bool{}
		for _, tag := range note.Tags {
			for {
				seen[tag] = true
				i := strings.LastIndex(tag, "/")
				if i < 0 {
					break
				}
				tag = tag[:i]
			}
		}
		for tag := range seen {
			counts[tag]++
		}
	}
	return counts
}

// Tagged returns the notes carrying tag or one of its nested tags.
func (idx *Index) Tagged(tag string) []*Note {
	tag = strings.ToLower(tag)
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var out []*Note
	for _, note := range idx.notes {
		for _, t := range note.Tags {
			if t == tag || strings.HasPrefix(t, tag+"/") {
				out = append(out, note)
				break
			}
		}
	}
	sortNotes(out)
	return out
}

func sortNotes(notes []*Note) {
	sort.Slice(notes, func(i, j int) bool {
		a, b := strings.ToLower(notes[i].Title), strings.ToLower(notes[j].Title)
		if a != b {
			return a < b
		}
		return notes[i].Path < notes[j].Path
	})
}
