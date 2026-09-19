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

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
		if got, ok := idx.Resolve("", target); !ok || got != want {
			t.Errorf("Resolve(%q) = %q, %v; want %q", target, got, ok, want)
		}
	}
	for _, target := range []string{"Missing", "Deleted", "app.json", "", "lan"} {
		if got, ok := idx.Resolve("", target); ok {
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
	if got, ok := idx.Resolve("", "New"); !ok || got != "New.md" {
		t.Errorf("new note not resolvable: %q %v", got, ok)
	}
	if n := len(idx.byName["new.md"]); n != 1 {
		t.Errorf("byName has %d entries for new.md", n)
	}
	os.Remove(filepath.Join(root, "New.md"))
	idx.Update("New.md")
	if _, ok := idx.Resolve("", "New"); ok {
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

func TestResolveFromTheLinkingNote(t *testing.T) {
	root := writeVault(t, map[string]string{
		"Twin.md":           "",
		"area/Twin.md":      "",
		"area/Page.md":      "[[Twin]] [[../Twin]]",
		"area/deep/Twin.md": "",
		"area/deep/Page.md": "[[Twin]]",
		"area/deep/pic.png": "",
		"img/pic.png":       "",
		"other/Only.md":     "",
		"other/sub/Leaf.md": "",
		"v1.2 plan.md":      "",
	})
	idx := New(root, "")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	type c struct{ from, target, want string }
	for _, tc := range []c{
		// a name: next to the linking note first...
		{"area/Page.md", "Twin", "area/Twin.md"},
		{"area/deep/Page.md", "Twin", "area/deep/Twin.md"},
		{"Home.md", "Twin", "Twin.md"},
		{"", "Twin", "Twin.md"},
		{"area/deep/Page.md", "pic.png", "area/deep/pic.png"},
		// ...then anywhere, the shortest path winning
		{"area/Page.md", "Only", "other/Only.md"},
		{"other/Only.md", "Twin", "Twin.md"},
		{"area/Page.md", "pic.png", "img/pic.png"},
		{"area/Page.md", "v1.2 plan", "v1.2 plan.md"},
		// a plain path: relative to the note, then from the root, then as the end of a path
		{"area/Page.md", "deep/Twin", "area/deep/Twin.md"},
		{"area/Page.md", "other/Only", "other/Only.md"},
		{"area/Page.md", "sub/Leaf", "other/sub/Leaf.md"},
		{"area/Page.md", "area/Twin", "area/Twin.md"},
		{"area/Page.md", "DEEP/twin.MD", "area/deep/Twin.md"},
		// ./ and ../ are followed, and nothing else is tried
		{"area/Page.md", "./Twin", "area/Twin.md"},
		{"area/Page.md", "./deep/Twin", "area/deep/Twin.md"},
		{"area/Page.md", "../Twin", "Twin.md"},
		{"area/deep/Page.md", "../Twin", "area/Twin.md"},
		{"area/deep/Page.md", "../../Twin", "Twin.md"},
		{"area/deep/Page.md", "../../img/pic.png", "img/pic.png"},
		{"area/Page.md", "./Only", ""},     // exists, but not there
		{"area/Page.md", "../Only", ""},    // likewise
		{"area/Page.md", "../../Twin", ""}, // above the vault
		{"area/Page.md", "../../../etc/passwd", ""},
		// a leading slash means the vault root, and nothing else is tried
		{"area/Page.md", "/Twin", "Twin.md"},
		{"area/Page.md", "/area/deep/Twin", "area/deep/Twin.md"},
		{"area/Page.md", "/img/pic.png", "img/pic.png"},
		{"area/Page.md", "/Only", ""},
		{"area/Page.md", "/deep/Twin", ""},
		{"area/Page.md", "/../Twin", "Twin.md"}, // cannot climb out of the root
		{"area/Page.md", "/", ""},
		{"area/Page.md", "", ""},
		{"area/Page.md", "Nowhere", ""},
	} {
		got, ok := idx.Resolve(tc.from, tc.target)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("Resolve(%q, %q) = %q, %v; want %q", tc.from, tc.target, got, ok, tc.want)
		}
	}

	// Backlinks and the graph follow the same rules: each Twin has its own.
	for twin, want := range map[string]string{"area/Twin.md": "area/Page.md", "area/deep/Twin.md": "area/deep/Page.md", "Twin.md": "area/Page.md"} {
		var from []string
		for _, n := range idx.Backlinks(twin) {
			from = append(from, n.Path)
		}
		if got := strings.Join(from, ","); got != want {
			t.Errorf("backlinks of %s = %q, want %q", twin, got, want)
		}
	}
	links := map[string]bool{}
	for _, l := range idx.Graph().Links {
		links[l.Source+" -> "+l.Target] = true
	}
	for _, want := range []string{"area/Page.md -> area/Twin.md", "area/Page.md -> Twin.md", "area/deep/Page.md -> area/deep/Twin.md"} {
		if !links[want] {
			t.Errorf("graph lacks %s (has %v)", want, links)
		}
	}
	if links["area/deep/Page.md -> Twin.md"] {
		t.Error("graph resolved [[Twin]] vault-wide instead of next to the note")
	}
}
