package index

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

// Search reads the notes on every query instead of keeping their text. At the
// size of a personal vault that is fast enough (measured, see
// docs/ARCHITECTURE.md), needs no second copy of the vault that could go stale,
// and finds text that was edited on disk a second ago. Should it ever be too
// slow, keep the text in Note and change only this file.
//
// Query syntax: words and "quoted phrases" must all occur; tag:name and
// path:text narrow the notes down; task:, task-todo: and task-done: find
// tasks, see tasks.go. Everything ignores case.

type SearchResult struct {
	Title    string
	URL      string
	Path     string
	Snippets []string // lines of the note that contain a search term
	Score    int
	// Percent puts Score on a fixed scale: 100 is what a note named exactly
	// what was searched for scores. Weaker signals add up, so a note with the
	// word in its title, as a tag and in its text gets there as well. It is
	// not relative to the other results, so the top hit of a poor search does
	// not read as perfect. Results are ordered by Score, which is not capped.
	// It is 0 when only tag: and path: were given: those notes all match alike.
	Percent    int
	Tasks      int        // how many task lines match the task operators
	TaskLines  []TaskLine // the first of them, shown in place of Snippets
	Attachment bool       // matched by file name only
}

// Scores per search term, by where it was found. The order is the point, not
// the numbers: a note *named* after the term beats one *about* it, which beats
// one that mentions it. Body hits count only up to bodyHitsMax, so a long note
// repeating a word cannot outrank a note with the word in its title.
const (
	scoreTitleIs    = 100
	scoreTitleWord  = 60
	scoreTitleHas   = 40
	scoreTagIs      = 30
	scoreTagHas     = 20
	scoreProperty   = 25 // a front matter value: the author, a description, a status
	scoreHeadingHas = 15
	scorePathHas    = 10
	scoreBodyHit    = 2
	bodyHitsMax     = 5
	scoreBodyWord   = 3 // once, if the term stands as a word of its own

	maxResults  = 100
	maxSnippets = 3
	snippetLen  = 160
)

type query struct {
	terms []string // lowercased words and phrases
	tags  []string
	paths []string
	tasks []taskFilter
}

func parseQuery(q string) query {
	var out query
	for _, field := range splitQuery(strings.ToLower(q)) {
		if filter, ok := parseTaskFilter(field); ok {
			out.tasks = append(out.tasks, filter)
			continue
		}
		switch {
		case strings.HasPrefix(field, "tag:"):
			if tag := strings.Trim(field[len("tag:"):], "#/"); tag != "" {
				out.tags = append(out.tags, tag)
			}
		case strings.HasPrefix(field, "path:"):
			if p := field[len("path:"):]; p != "" {
				out.paths = append(out.paths, p)
			}
		default:
			out.terms = append(out.terms, field)
		}
	}
	return out
}

// splitQuery splits on spaces, keeping "quoted phrases" together; the quotes
// may follow an operator, as in path:"my folder".
func splitQuery(q string) []string {
	var fields []string
	var field strings.Builder
	quoted := false
	flush := func() {
		if s := strings.TrimSpace(field.String()); s != "" {
			fields = append(fields, s)
		}
		field.Reset()
	}
	for _, r := range q {
		switch {
		case r == '"':
			quoted = !quoted
		case unicode.IsSpace(r) && !quoted:
			flush()
		default:
			field.WriteRune(r)
		}
	}
	flush()
	return fields
}

func (q query) empty() bool { return len(q.terms)+len(q.tags)+len(q.paths)+len(q.tasks) == 0 }

func (q query) pathOK(rel string) bool {
	rel = strings.ToLower(rel)
	for _, p := range q.paths {
		if !strings.Contains(rel, p) {
			return false
		}
	}
	return true
}

func (q query) tagsOK(tags []string) bool {
	for _, want := range q.tags {
		found := false
		for _, tag := range tags {
			if tag == want || strings.HasPrefix(tag, want+"/") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Search returns the notes and attachments matching q, best match first.
func (idx *Index) Search(q string) []SearchResult {
	query := parseQuery(q)
	if query.empty() {
		return nil
	}
	idx.mu.RLock()
	notes := make([]*Note, 0, len(idx.notes))
	for _, note := range idx.notes {
		notes = append(notes, note)
	}
	var files []string
	if len(query.tags)+len(query.tasks) == 0 && len(query.terms) > 0 { // a file has no tags or tasks, and needs a name to match
		for _, paths := range idx.byName {
			for _, p := range paths {
				if idx.notes[p] == nil && !drawingPath(p) {
					files = append(files, p)
				}
			}
		}
	}
	idx.mu.RUnlock()

	// Reading and lowercasing the notes is the whole cost, and it spreads
	// over the cores without any shared state.
	var (
		results []SearchResult
		mu      sync.Mutex
		wg      sync.WaitGroup
		next    atomic.Int64
	)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(notes) {
					return
				}
				note := notes[i]
				// A drawing is a scene wrapped in Markdown. It is found through
				// the notes that embed it, which also say what it is about.
				if note.Drawing || !query.pathOK(note.Path) || !query.tagsOK(note.Tags) {
					continue
				}
				if result, ok := idx.scoreNote(note, query); ok {
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	for _, file := range files {
		if !query.pathOK(file) {
			continue
		}
		// A file's name is all there is to it, so it is scored like a note's
		// title: "floor plan.pdf" is as good an answer to "floor plan" as a
		// note of that name.
		lower := strings.ToLower(file)
		name := strings.TrimSuffix(path.Base(lower), path.Ext(lower))
		score := 0
		for _, term := range query.terms {
			if !strings.Contains(path.Base(lower), term) {
				score = 0
				break
			}
			score += titleScore(name, term) + scorePathHas
		}
		if score > 0 {
			results = append(results, SearchResult{Title: path.Base(file), URL: idx.URL(file), Path: file, Score: score, Attachment: true})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Tasks != b.Tasks { // "where is most to do" when only tasks were asked for
			return a.Tasks > b.Tasks
		}
		if at, bt := strings.ToLower(a.Title), strings.ToLower(b.Title); at != bt {
			return at < bt
		}
		return a.Path < b.Path
	})
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	for i := range results {
		if !results[i].Attachment && results[i].Tasks == 0 { // task results bring their task lines
			results[i].Snippets = idx.snippets(idx.Note(results[i].Path), results[i].Path, query)
		}
		if n := len(query.terms); n > 0 {
			// A title match can score above scoreTitleIs through the path and
			// the text as well; better than perfect is still 100.
			results[i].Percent = max(1, min(100, results[i].Score*100/(n*scoreTitleIs)))
		}
	}
	return results
}

// scoreNote reads a note and scores it; ok is false if a term is missing.
// It works on the whole text at once: strings.Count and strings.Index on one
// large string are far cheaper than the same work line by line.
func (idx *Index) scoreNote(note *Note, q query) (result SearchResult, ok bool) {
	result = SearchResult{Title: note.Title, URL: note.URL, Path: note.Path}
	if len(q.terms)+len(q.tasks) == 0 { // only tag: and path:, which the caller checked
		return result, true
	}
	src, err := os.ReadFile(filepath.Join(idx.root, filepath.FromSlash(note.Path)))
	if err != nil {
		return result, false
	}
	original := body(string(src))
	if len(q.tasks) > 0 {
		if result.Tasks, result.TaskLines = matchTasks(original, q.tasks); result.Tasks == 0 {
			return result, false
		}
	}
	text := strings.ToLower(original)
	title, file := strings.ToLower(note.Title), strings.ToLower(note.Path)
	var headings []string // found on first need: most notes fail on a term before

	for _, term := range q.terms {
		score := titleScore(title, term)
		for _, tag := range note.Tags {
			if tag == term {
				score += scoreTagIs
				break
			} else if strings.Contains(tag, term) {
				score += scoreTagHas
				break
			}
		}
		if strings.Contains(file, term) {
			score += scorePathHas
		}
		for _, prop := range note.props {
			if strings.Contains(prop.lower, term) {
				score += scoreProperty
				break
			}
		}
		if hits := strings.Count(text, term); hits > 0 {
			score += min(hits, bodyHitsMax) * scoreBodyHit
			if hasWord(text, term) {
				score += scoreBodyWord
			}
			if headings == nil {
				headings = headingsOf(text)
			}
			for _, heading := range headings {
				if strings.Contains(heading, term) {
					score += scoreHeadingHas
					break
				}
			}
		}
		if score == 0 {
			return result, false // every term has to be somewhere
		}
		result.Score += score
	}
	return result, true
}

func titleScore(title, term string) int {
	switch {
	case title == term:
		return scoreTitleIs
	case hasWord(title, term):
		return scoreTitleWord
	case strings.Contains(title, term):
		return scoreTitleHas
	}
	return 0
}

// property is one front matter value, as the search sees it.
type property struct {
	line  string // "key: value", for the snippet
	lower string // the value alone, lowercased
}

// properties flattens front matter for the search. Only values are searched:
// the keys are the same in every note, so "created" or "source" would find the
// whole vault. Title and tags are left out because they are scored on their
// own. It is kept in the index, which has parsed the front matter anyway.
func properties(frontmatter map[string]any) []property {
	keys := make([]string, 0, len(frontmatter))
	for key := range frontmatter {
		if key != "title" && key != "tags" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var out []property
	var add func(key string, v any)
	add = func(key string, v any) {
		switch v := v.(type) {
		case nil:
		case []any:
			for _, item := range v {
				add(key, item)
			}
		case map[string]any:
			for _, sub := range sortedKeys(v) {
				add(key+"."+sub, v[sub])
			}
		case time.Time:
			// As written, not as Go prints it: "2026-09-04", found by "2026-09".
			value := v.Format("2006-01-02 15:04:05")
			value = strings.TrimSuffix(value, " 00:00:00")
			out = append(out, property{key + ": " + value, value})
		default:
			if value := strings.TrimSpace(fmt.Sprint(v)); value != "" {
				out = append(out, property{key + ": " + value, strings.ToLower(value)})
			}
		}
	}
	for _, key := range keys {
		add(key, frontmatter[key])
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// snippets returns the first lines of a note that contain a search term,
// matching properties first. It reads the note again: only the results that
// are shown get snippets, which is cheaper than keeping the text of
// everything that matched.
func (idx *Index) snippets(note *Note, rel string, q query) []string {
	var out []string
	if note != nil {
		for _, prop := range note.props {
			for _, term := range q.terms {
				if i := strings.Index(prop.lower, term); i >= 0 && len(out) < maxSnippets {
					at := len(prop.line) - len(prop.lower) + i // the value ends the line
					out = append(out, snippet(prop.line, at >= 0 && at <= len(prop.line), max(at, 0)))
					break
				}
			}
		}
	}
	if len(out) == maxSnippets {
		return out
	}
	src, err := os.ReadFile(filepath.Join(idx.root, filepath.FromSlash(rel)))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(body(string(src)), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		for _, term := range q.terms {
			if i := strings.Index(lower, term); i >= 0 {
				out = append(out, snippet(line, len(line) == len(lower), i))
				break
			}
		}
		if len(out) == maxSnippets {
			break
		}
	}
	return out
}

// body returns a note's text without its front matter, whose values are
// searched through Note.props: as raw text its keys would match too, and
// "tags" or "created" would find every note.
func body(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	if strings.HasPrefix(src, "---\n") {
		if end := strings.Index(src[3:], "\n---"); end >= 0 {
			rest := src[3+end+4:]
			if i := strings.IndexByte(rest, '\n'); i >= 0 {
				return rest[i+1:]
			}
			return ""
		}
	}
	return src
}

// headingsOf returns the heading lines of a (lowercased) text. "# comment" in
// a code block is no heading.
func headingsOf(text string) []string {
	var headings []string
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
		} else if !fenced && strings.HasPrefix(line, "#") {
			if rest := strings.TrimLeft(line, "#"); strings.HasPrefix(rest, " ") {
				headings = append(headings, rest)
			}
		}
	}
	return headings
}

// hasWord reports whether term occurs in s with no letter or digit on either
// side: "plan" is a word of "the plan." but not of "planet".
func hasWord(s, term string) bool {
	for from := 0; from <= len(s)-len(term); {
		i := strings.Index(s[from:], term)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(term)
		before, _ := utf8.DecodeLastRuneInString(s[:start])
		after, _ := utf8.DecodeRuneInString(s[end:])
		// DecodeRune returns RuneError at the string's edge, which is no word rune.
		if !isWordRune(before) && !isWordRune(after) {
			return true
		}
		_, size := utf8.DecodeRuneInString(s[start:])
		from = start + size
	}
	return false
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// snippet cuts a window around the match out of a long line. Lowercasing can
// change a string's byte length (rarely: "İ"); then the match position does
// not apply to the original line and the line's start is shown instead.
func snippet(line string, positionsAgree bool, at int) string {
	runes := []rune(line)
	if len(runes) <= snippetLen {
		return line
	}
	start := 0
	if positionsAgree {
		start = max(len([]rune(line[:at]))-snippetLen/3, 0)
	}
	end := min(start+snippetLen, len(runes))
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}
