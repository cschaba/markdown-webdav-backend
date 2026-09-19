package index

import "strings"

// Task search, with Obsidian's operators so that what works there works here:
//
//	task:word        a task that mentions word, open or completed
//	task-todo:word   an open task          - [ ] ...
//	task-done:word   a completed task      - [x] ...
//	task-todo:""     any open task; likewise for the other two
//
// As in Obsidian, only "[ ]" is open. Any other character between the
// brackets counts as completed, which covers "[X]" and the custom statuses
// some plugins use ("[-]" cancelled, "[/]" in progress).

type taskState int

const (
	taskAny taskState = iota
	taskTodo
	taskDone
)

type taskFilter struct {
	state taskState
	text  string // lowercased; empty matches every task of that state
}

func parseTaskFilter(field string) (taskFilter, bool) {
	for _, op := range []struct {
		prefix string
		state  taskState
	}{{"task-todo:", taskTodo}, {"task-done:", taskDone}, {"task:", taskAny}} {
		if text, ok := strings.CutPrefix(field, op.prefix); ok {
			return taskFilter{op.state, strings.TrimSpace(text)}, true
		}
	}
	return taskFilter{}, false
}

const maxTaskSnippets = 5

// TaskLine is one task of a note, for display.
type TaskLine struct {
	Done bool
	Text string
}

// matchTasks returns how many task lines of a note match one of the filters,
// and the first of those lines for display. It returns 0 unless every filter
// is matched by some task: "task-todo:a task-done:b" means both.
func matchTasks(text string, filters []taskFilter) (count int, lines []TaskLine) {
	satisfied := make([]bool, len(filters))
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue // "- [ ]" in a code block is an example, not a task
		}
		done, what, ok := parseTask(trimmed)
		if !ok {
			continue
		}
		lower, matched := strings.ToLower(what), false
		for i, f := range filters {
			if (f.state == taskTodo && done) || (f.state == taskDone && !done) || !strings.Contains(lower, f.text) {
				continue
			}
			satisfied[i], matched = true, true
		}
		if matched {
			if count++; len(lines) < maxTaskSnippets {
				lines = append(lines, TaskLine{done, snippet(what, true, 0)})
			}
		}
	}
	for _, ok := range satisfied {
		if !ok {
			return 0, nil
		}
	}
	return count, lines
}

// parseTask recognises a task list item: a list marker, "[c]", and the text.
// Tasks in block quotes and callouts ("> - [ ] ...") count, as in Obsidian.
func parseTask(line string) (done bool, text string, ok bool) {
	for strings.HasPrefix(line, ">") {
		line = strings.TrimSpace(line[1:])
	}
	switch {
	case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "), strings.HasPrefix(line, "+ "):
		line = line[2:]
	default: // "1. " or "1) "
		digits := 0
		for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits+1 >= len(line) || (line[digits] != '.' && line[digits] != ')') || line[digits+1] != ' ' {
			return false, "", false
		}
		line = line[digits+2:]
	}
	line = strings.TrimLeft(line, " ")
	// "[c]" where c is exactly one character, then a space or the end
	if !strings.HasPrefix(line, "[") {
		return false, "", false
	}
	end := strings.Index(line, "]")
	if end < 2 || len([]rune(line[1:end])) != 1 || (len(line) > end+1 && line[end+1] != ' ') {
		return false, "", false
	}
	return line[1:end] != " ", strings.TrimSpace(line[end+1:]), true
}
