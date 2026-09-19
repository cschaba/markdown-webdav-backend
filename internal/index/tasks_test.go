package index

import (
	"strings"
	"testing"
)

func TestParseTask(t *testing.T) {
	for line, want := range map[string]string{
		"- [ ] call the dentist":    "open|call the dentist",
		"- [x] call the dentist":    "done|call the dentist",
		"- [X] shouting":            "done|shouting",
		"- [-] cancelled":           "done|cancelled", // any other character is "not open", as in Obsidian
		"- [/] in progress":         "done|in progress",
		"* [ ] star":                "open|star",
		"+ [ ] plus":                "open|plus",
		"1. [ ] numbered":           "open|numbered",
		"12) [x] numbered":          "done|numbered",
		"> - [ ] in a quote":        "open|in a quote",
		"> > - [ ] nested quote":    "open|nested quote",
		"- [ ]":                     "open|",
		"- [ ] [[Link]] and #tag":   "open|[[Link]] and #tag",
		"- [] no space inside":      "",
		"- [xx] two characters":     "",
		"- [ ]no space after":       "",
		"[ ] not a list item":       "",
		"- a [ ] later in the line": "",
		"-[ ] no space after dash":  "",
		"- [link](http://x)":        "",
		"1.[ ] no space":            "",
		"plain text":                "",
		"":                          "",
	} {
		done, text, ok := parseTask(line)
		got := ""
		if ok {
			got = map[bool]string{false: "open|", true: "done|"}[done] + text
		}
		if got != want {
			t.Errorf("parseTask(%q) = %q, want %q", line, got, want)
		}
	}
}

func taskVault(t *testing.T) *Index {
	t.Helper()
	root := writeVault(t, map[string]string{
		"Busy.md":              "# Week\n\n- [ ] call the dentist\n- [ ] Buy milk\n- [x] pay rent\n\t- [ ] nested: renew passport\n> - [ ] quoted task\n",
		"Done.md":              "- [x] call the plumber\n- [X] file taxes\n",
		"One.md":               "---\ntags: [home]\n---\n- [ ] fix the dentist chair\n",
		"Example.md":           "How to write one:\n\n```md\n- [ ] call the dentist\n```\n",
		"Prose.md":             "I should call the dentist. [ ] is not a task here.",
		"Many.md":              strings.Repeat("- [ ] chore\n", 9),
		"Sketch.excalidraw.md": "---\nexcalidraw-plugin: parsed\n---\n- [ ] drawn task\n",
	})
	idx := New(root, "/v")
	if err := idx.Rebuild(); err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestSearchTasks(t *testing.T) {
	idx := taskVault(t)
	for q, want := range map[string]string{
		// every note with such a task, the one with most of them first
		`task-todo:""`: "Many.md | Busy.md | One.md",
		`task-todo:`:   "Many.md | Busy.md | One.md", // the quotes are optional
		`task-done:""`: "Done.md | Busy.md",
		`task:""`:      "Many.md | Busy.md | Done.md | One.md",
		// a word has to be in a task of that state, not just in the note
		"task-todo:dentist":       "Busy.md | One.md", // not the code block, not the prose
		"task-done:dentist":       "",
		"task:plumber":            "Done.md",
		"task-todo:plumber":       "",
		`task-todo:"the dentist"`: "Busy.md | One.md",
		"TASK-TODO:Milk":          "Busy.md",
		// combined with everything else
		"task-todo:dentist tag:home":     "One.md",
		"task-todo:dentist path:busy":    "Busy.md",
		`task-todo:"" week`:              "Busy.md",
		"task-todo:milk task-done:rent":  "Busy.md", // both operators must find a task
		"task-todo:milk task-done:taxes": "",
		// the plain word search is untouched: it finds prose and code too
		"dentist": "Busy.md | Example.md | One.md | Prose.md",
	} {
		if got := paths(idx.Search(q)); got != want {
			t.Errorf("Search(%q)\n got: %s\nwant: %s", q, got, want)
		}
	}
}

func TestSearchTaskResults(t *testing.T) {
	idx := taskVault(t)
	busy := idx.Search(`task-todo:""`)[1]
	if busy.Path != "Busy.md" || busy.Tasks != 4 || busy.Percent != 0 {
		t.Errorf("Busy.md: %d tasks, %d%%; want 4 open tasks and no percentage", busy.Tasks, busy.Percent)
	}
	show := func(lines []TaskLine) string {
		var out []string
		for _, l := range lines {
			out = append(out, map[bool]string{false: "[ ] ", true: "[x] "}[l.Done]+l.Text)
		}
		return strings.Join(out, " | ")
	}
	want := "[ ] call the dentist | [ ] Buy milk | [ ] nested: renew passport | [ ] quoted task"
	if got := show(busy.TaskLines); got != want || len(busy.Snippets) != 0 {
		t.Errorf("task lines:\n got: %s\nwant: %s", got, want)
	}
	all := idx.Search(`task:""`)[1]
	if all.Tasks != 5 || !strings.Contains(show(all.TaskLines), "| [x] pay rent |") {
		t.Errorf("task: shows both states, in the note's order: %d, %s", all.Tasks, show(all.TaskLines))
	}
	if many := idx.Search(`task-todo:""`)[0]; many.Tasks != 9 || len(many.TaskLines) != maxTaskSnippets {
		t.Errorf("Many.md: %d tasks counted, %d shown; want 9 and %d", many.Tasks, len(many.TaskLines), maxTaskSnippets)
	}
	// With a word, the word ranks and the tasks are still what is shown.
	mixed := idx.Search(`task-todo:"" week`)[0]
	if mixed.Percent == 0 || mixed.Tasks != 4 || mixed.TaskLines[0].Text != "call the dentist" {
		t.Errorf("mixed query: %+v", mixed)
	}
}
