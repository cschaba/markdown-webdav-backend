package index

import (
	"sort"
	"strings"
	"unicode"
)

// NotesIn returns the notes of a folder ("" is the vault root), with those of
// the folders below it if recursive, in reading order: by path, numbers
// counting as numbers ("Chapter 2" before "Chapter 10"), a folder's own notes
// before its subfolders.
func (idx *Index) NotesIn(dir string, recursive bool) []*Note {
	prefix := ""
	if dir = strings.Trim(dir, "/"); dir != "" {
		prefix = dir + "/"
	}
	idx.mu.RLock()
	var out []*Note
	for p, note := range idx.notes {
		if rest, ok := strings.CutPrefix(p, prefix); ok && (recursive || !strings.Contains(rest, "/")) {
			out = append(out, note)
		}
	}
	idx.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return readingOrder(out[i].Path, out[j].Path) })
	return out
}

func readingOrder(a, b string) bool {
	as, bs := strings.Split(strings.ToLower(a), "/"), strings.Split(strings.ToLower(b), "/")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		// a file of this folder comes before the folders in it
		if aFile, bFile := i == len(as)-1, i == len(bs)-1; aFile != bFile {
			return aFile
		}
		return naturalLess(as[i], bs[i])
	}
	return len(as) < len(bs)
}

// naturalLess compares strings with runs of digits taken as numbers.
func naturalLess(a, b string) bool {
	ar, br := []rune(a), []rune(b)
	for i, j := 0, 0; i < len(ar) && j < len(br); {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			si, sj := i, j
			for i < len(ar) && unicode.IsDigit(ar[i]) {
				i++
			}
			for j < len(br) && unicode.IsDigit(br[j]) {
				j++
			}
			na, nb := strings.TrimLeft(string(ar[si:i]), "0"), strings.TrimLeft(string(br[sj:j]), "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if ar[i] != br[j] {
			return ar[i] < br[j]
		}
		i++
		j++
	}
	return len(ar) < len(br)
}
