package index

import (
	"io/fs"
	"path"
	"strings"

	"markdown-webdav-backend/internal/render"
)

// Excalidraw drawings.
//
// The server does not draw scenes. The Obsidian Excalidraw plugin can export
// a picture next to every drawing ("Auto-export SVG"), produced by Excalidraw
// itself, and that picture is what gets shown: exact, and without script in
// the page. The ready-made browser renderers were tried instead: the current
// one is a 20 MB download, the old small one draws current scenes wrong.
//
// A drawing is "Name.excalidraw.md" (the plugin's format: Markdown around a
// compressed scene), a legacy "Name.excalidraw" JSON file, or any note whose
// front matter carries the plugin's key.

const drawingKey = "excalidraw-plugin"

func drawingPath(rel string) bool {
	lower := strings.ToLower(rel)
	return strings.HasSuffix(lower, ".excalidraw.md") || strings.HasSuffix(lower, ".excalidraw")
}

// DrawingName strips what makes a file name a drawing's: "a/Name.excalidraw.md"
// becomes "Name".
func DrawingName(rel string) string {
	name := path.Base(rel)
	for _, suffix := range []string{".md", ".excalidraw"} {
		if len(name) > len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
			name = name[:len(name)-len(suffix)]
		}
	}
	return name
}

// IsDrawing reports whether the file at rel is an Excalidraw drawing.
func (idx *Index) IsDrawing(rel string) bool {
	if drawingPath(rel) {
		return true
	}
	note := idx.Note(rel)
	return note != nil && note.Drawing
}

// DrawingImages returns the URLs of the pictures exported for the drawing at
// rel: the one to show, and its dark-mode variant if the plugin wrote both
// ("Export both dark- and light-themed image"). Both are empty when the
// drawing was never exported.
func (idx *Index) DrawingImages(rel string) (image, dark string) {
	name := DrawingName(rel)
	find := func(file string) string {
		// Resolved like a link in the drawing: beside it is where the plugin
		// puts the export; anywhere else covers a configured export folder.
		if p, ok := idx.Resolve(rel, file); ok {
			return idx.URL(p)
		}
		return ""
	}
	for _, ext := range []string{".svg", ".png"} {
		// The plugin keeps ".excalidraw" in the export's name or not,
		// depending on a setting.
		for _, stem := range []string{name + ".excalidraw", name} {
			image, dark = find(stem+".light"+ext), find(stem+".dark"+ext)
			if image == "" {
				image = find(stem + ext)
			}
			if image == "" {
				image, dark = dark, ""
			}
			if image != "" {
				return image, dark
			}
		}
	}
	return "", ""
}

// ResolveEmbed implements render.LinkResolver.
func (idx *Index) ResolveEmbed(from, target string) render.Embed {
	p, ok := idx.Resolve(from, target)
	switch {
	case !ok:
		return render.Embed{}
	case render.IsImage(p):
		return render.Embed{Image: idx.URL(p)}
	case idx.IsDrawing(p):
		embed := render.Embed{Drawing: true}
		embed.Image, embed.DarkImage = idx.DrawingImages(p)
		return embed
	}
	if note := idx.Note(p); note != nil {
		return render.Embed{Note: note.Path, Title: note.Title}
	}
	return render.Embed{}
}

// ReadNote implements render.LinkResolver. Only what the index knows as a note
// is read: the path comes out of ResolveEmbed, but nothing forces a caller.
func (idx *Index) ReadNote(vaultPath string) ([]byte, error) {
	if idx.Note(vaultPath) == nil {
		return nil, fs.ErrNotExist
	}
	return idx.readFile(vaultPath)
}
