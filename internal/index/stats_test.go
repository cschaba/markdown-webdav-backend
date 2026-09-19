package index

import "testing"

func TestStats(t *testing.T) {
	root := writeVault(t, map[string]string{
		"Home.md":                    "#start [[Plan]] [[Nowhere]] ![[photo.png]]",
		"projects/Plan.md":           "#project/alpha back to [[Home]]",
		"projects/Sketch.excalidraw": "{}",
		"Board.excalidraw.md":        "",
		"img/photo.png":              "12345",
		".obsidian/app.json":         "not counted",
		".git/objects/pack":          "not counted",
	})
	idx := New(root, "")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	got, err := idx.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastChange.IsZero() {
		t.Error("no last change")
	}
	got.LastChange = Stats{}.LastChange
	notes := int64(len("#start [[Plan]] [[Nowhere]] ![[photo.png]]") + len("#project/alpha back to [[Home]]"))
	want := Stats{Notes: 2, Drawings: 2, Attachments: 1, Folders: 2,
		Tags:  3, // start, project, project/alpha
		Links: 4, MissingLinks: 1, Size: notes + 2 + 5, NotesSize: notes}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestStatsOfAnEmptyVault(t *testing.T) {
	idx := New(t.TempDir(), "")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if got, err := idx.Stats(); err != nil || got != (Stats{}) {
		t.Errorf("got %+v, %v", got, err)
	}
}
