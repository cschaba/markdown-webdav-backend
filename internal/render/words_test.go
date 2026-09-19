package render

import "testing"

func TestCountText(t *testing.T) {
	r := New(fakeLinks{"Other note": "/Other%20note", "X": "/X", "pic.png": "/pic.png"}, "/-/tag/")
	for src, want := range map[string][2]int{
		"":                                  {0, 0},
		"Hello world.":                      {2, 12},
		"---\ntitle: Not counted\n---\nOne": {1, 3},
		"**bo**ld and *it*":                 {3, 11}, // markup splits no word
		"# Head\n\nBody":                    {2, 8},  // two blocks, not "HeadBody"
		"line one\nline two":                {4, 16}, // a soft break separates
		"well-known - fine":                 {2, 17}, // a lone dash is no word
		"[[Other note]] and [[X|a label]]":  {5, 22}, // what the link shows
		"![[Other note]] ![](pic.png)":      {0, 1},  // embeds and images are not text
		"#tag text":                         {2, 9},
		"`code` here":                       {2, 9},
		"```go\nfunc main() {}\n```":        {2, 14}, // "{}" is no word
		"```mermaid\ngraph TD; A-->B\n```":  {0, 0},  // a diagram's source is no text
		"Straße über 42":                    {3, 14}, // letters and digits of any script
	} {
		_, meta, err := r.Render([]byte(src), "Note.md")
		if err != nil {
			t.Fatal(err)
		}
		if got := [2]int{meta.Words, meta.Characters}; got != want {
			t.Errorf("%q: words, characters = %v, want %v", src, got, want)
		}
		if m := r.Meta([]byte(src)); m.Words != meta.Words || m.Characters != meta.Characters {
			t.Errorf("%q: Meta counts %d, %d, Render %d, %d", src, m.Words, m.Characters, meta.Words, meta.Characters)
		}
	}
}
