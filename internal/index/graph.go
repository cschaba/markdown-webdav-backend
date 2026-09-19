package index

import (
	"sort"
	"strings"
)

// The graph view: what links to what. It is derived from the same Links and
// Tags the rest of the site uses, so the graph cannot disagree with the
// backlinks under a note. Everything is included; the page filters.

type GraphNode struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	URL   string   `json:"url,omitempty"` // empty for a missing note: there is nowhere to go
	Kind  string   `json:"kind"`          // note, attachment, missing or tag
	Tags  []string `json:"tags,omitempty"`
}

type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// Graph returns every note, the files and missing notes they point at, and
// their tags, in a stable order.
func (idx *Index) Graph() Graph {
	idx.mu.RLock()
	notes := make([]*Note, 0, len(idx.notes))
	for _, note := range idx.notes {
		notes = append(notes, note)
	}
	idx.mu.RUnlock()
	sort.Slice(notes, func(i, j int) bool { return notes[i].Path < notes[j].Path })

	nodes := map[string]GraphNode{}
	links := map[GraphLink]bool{}
	for _, note := range notes {
		nodes[note.Path] = GraphNode{ID: note.Path, Title: note.Title, URL: note.URL, Kind: "note", Tags: note.Tags}
	}
	for _, note := range notes {
		for _, target := range note.Links {
			id, ok := idx.Resolve(target) // takes the lock itself
			switch {
			case !ok:
				// One node per missing name, however it was capitalised.
				id = "missing:" + strings.ToLower(strings.TrimSpace(target))
				if _, seen := nodes[id]; !seen {
					nodes[id] = GraphNode{ID: id, Title: target, Kind: "missing"}
				}
			case nodes[id].Kind == "":
				nodes[id] = GraphNode{ID: id, Title: id[strings.LastIndex(id, "/")+1:], URL: idx.URL(id), Kind: "attachment"}
			}
			if id != note.Path {
				links[GraphLink{note.Path, id}] = true
			}
		}
		for _, tag := range note.Tags {
			id := "tag:" + tag
			nodes[id] = GraphNode{ID: id, Title: "#" + tag, URL: idx.TagURL(tag), Kind: "tag"}
			links[GraphLink{note.Path, id}] = true
		}
	}

	graph := Graph{Nodes: make([]GraphNode, 0, len(nodes)), Links: make([]GraphLink, 0, len(links))}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	for link := range links {
		graph.Links = append(graph.Links, link)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Links, func(i, j int) bool {
		if graph.Links[i].Source != graph.Links[j].Source {
			return graph.Links[i].Source < graph.Links[j].Source
		}
		return graph.Links[i].Target < graph.Links[j].Target
	})
	return graph
}
