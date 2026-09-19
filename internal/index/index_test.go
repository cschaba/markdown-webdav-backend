package index

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeVault(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestResolve(t *testing.T) {
	root := writeVault(t, map[string]string{
		"Home.md":             "",
		"projects/Plan.md":    "",
		"archive/old/Plan.md": "",
		"notes/v1.2 plan.md":  "",
		"img/photo.png":       "",
		".obsidian/app.json":  "",
		".trash/Deleted.md":   "",
	})
	idx := New(root, "")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{
		"Home":          "Home.md",
		"home":          "Home.md", // case-insensitive, as in Obsidian
		"Home.md":       "Home.md",
		"Plan":          "projects/Plan.md", // shortest path wins
		"old/Plan":      "archive/old/Plan.md",
		"v1.2 plan":     "notes/v1.2 plan.md", // a dot is not an extension
		"photo.png":     "img/photo.png",
		"img/photo.png": "img/photo.png",
	} {
		if got, ok := idx.Resolve(target); !ok || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", target, got, ok, want)
		}
	}
	for _, target := range []string{"Missing", "Deleted", "app.json", "", "lan"} {
		if got, ok := idx.Resolve(target); ok {
			t.Errorf("Resolve(%q) = %q; want no match", target, got)
		}
	}
}

func TestTagsAndBacklinks(t *testing.T) {
	root := writeVault(t, map[string]string{
		"A.md":     "---\ntitle: Alpha\ntags: [Project/Alpha, idea]\n---\nSee [[B]] and [[B|again]].",
		"B.md":     "Text with #idea and `#notatag` in code.\n\n```\n#alsonot\n```\n",
		"C.md":     "---\ntags: project\n---\n[[b#Heading]]",
		"D.md":     "---\nrelated: \"[[B]]\"\n---\nlinked from a property only",
		"sub/E.md": "[markdown link](B.md) from another folder",
		"F.md":     "mentions B and https://example.com/B.md but links nothing",
	})
	idx := New(root, "")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if got := idx.Note("A.md").Title; got != "Alpha" {
		t.Errorf("title = %q", got)
	}
	tags := idx.Tags()
	want := map[string]int{"project": 2, "project/alpha": 1, "idea": 2}
	if len(tags) != len(want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
	for tag, n := range want {
		if tags[tag] != n {
			t.Errorf("tag %q = %d, want %d (all: %v)", tag, tags[tag], n, tags)
		}
	}
	var names []string
	for _, n := range idx.Backlinks("B.md") {
		names = append(names, n.Path)
	}
	if got := strings.Join(names, ","); got != "A.md,C.md,D.md,sub/E.md" {
		t.Errorf("backlinks = %q", got)
	}
}

func TestUpdate(t *testing.T) {
	root := writeVault(t, map[string]string{"A.md": "#one"})
	idx := New(root, "")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "A.md"), []byte("#two"), 0o644)
	os.WriteFile(filepath.Join(root, "New.md"), []byte("[[A]]"), 0o644)
	idx.Update("A.md")
	idx.Update("New.md")
	idx.Update("New.md") // twice must not register the file twice
	if tags := idx.Tags(); tags["two"] != 1 || tags["one"] != 0 {
		t.Errorf("tags after update = %v", tags)
	}
	if got, ok := idx.Resolve("New"); !ok || got != "New.md" {
		t.Errorf("new note not resolvable: %q %v", got, ok)
	}
	if n := len(idx.byName["new.md"]); n != 1 {
		t.Errorf("byName has %d entries for new.md", n)
	}
	os.Remove(filepath.Join(root, "New.md"))
	idx.Update("New.md")
	if _, ok := idx.Resolve("New"); ok {
		t.Error("removed note still resolvable")
	}
}

func TestGraph(t *testing.T) {
	root := writeVault(t, map[string]string{
		"A.md":           "---\ntags: [topic]\n---\n[[B]] [[B|twice]] [[A]] ![[img/p.png]] [[Ghost]] [[ghost]]",
		"B.md":           "#topic/sub back to [[A]]",
		"Orphan.md":      "nobody links here",
		"img/p.png":      "",
		"img/unused.png": "",
	})
	idx := New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	g := idx.Graph()
	var nodes, links []string
	for _, n := range g.Nodes {
		nodes = append(nodes, n.Kind+" "+n.ID+" "+n.URL)
	}
	for _, l := range g.Links {
		links = append(links, l.Source+" -> "+l.Target)
	}
	wantNodes := []string{
		"note A.md /v/A", "note B.md /v/B", "note Orphan.md /v/Orphan",
		"attachment img/p.png /v/img/p.png", // linked files only; unused.png is not part of the graph
		"missing missing:ghost ",            // one node for [[Ghost]] and [[ghost]], and no URL
		"tag tag:topic /v/-/tag/topic", "tag tag:topic/sub /v/-/tag/topic/sub",
	}
	wantLinks := []string{ // no duplicate for the second [[B]], no self-link for [[A]]
		"A.md -> B.md", "A.md -> img/p.png", "A.md -> missing:ghost", "A.md -> tag:topic",
		"B.md -> A.md", "B.md -> tag:topic/sub",
	}
	sort.Strings(wantNodes)
	sort.Strings(nodes)
	if strings.Join(nodes, "\n") != strings.Join(wantNodes, "\n") {
		t.Errorf("nodes:\n%s\nwant:\n%s", strings.Join(nodes, "\n"), strings.Join(wantNodes, "\n"))
	}
	if strings.Join(links, "\n") != strings.Join(wantLinks, "\n") {
		t.Errorf("links:\n%s\nwant:\n%s", strings.Join(links, "\n"), strings.Join(wantLinks, "\n"))
	}
}
