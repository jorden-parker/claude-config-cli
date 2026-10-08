// Package match ranks settings against a search term. The shown name counts
// most, then the full key, the section, and finally words in the description,
// so a short term like "on" no longer surfaces every row whose description
// happens to contain "configuration".
package match

import (
	"sort"
	"strings"
	"unicode"
)

// Tier says where the term was found; lower is a better match.
type Tier int

const (
	None         Tier = iota
	NameExact         // the shown name is the term
	NamePrefix        // the shown name starts with the term
	NameContains      // the shown name contains the term
	KeyContains       // the full dotted key contains the term
	SectionWord       // the term starts a word in the section name
	DescWord          // the term starts a word in the description
)

// minDescTerm is the shortest term that may match inside a description.
// Shorter terms match only names, keys, and sections.
const minDescTerm = 3

// Fields are the searchable parts of one row. Name is what the UI shows.
type Fields struct{ Name, Key, Section, Desc string }

// Hit is one matched row. Name holds rune indexes into Fields.Name for
// name-tier hits, so the UI can underline them; it is nil otherwise.
type Hit struct {
	Index int
	Tier  Tier
	Name  []int
}

// Match scores one row. Tier None means the term was not found. A term with
// several words matches only when every word does, at the worst word's tier.
func Match(term string, f Fields) Hit {
	words := strings.Fields(term)
	if len(words) == 0 {
		return Hit{}
	}
	name := lower(f.Name)
	key := lower(f.Key)
	section := lower(f.Section)
	desc := lower(f.Desc)
	hit := Hit{}
	seen := map[int]bool{}
	for _, w := range words {
		word := lower(w)
		t, idx := matchWord(word, name, key, section, desc)
		if t == None {
			return Hit{}
		}
		if t > hit.Tier {
			hit.Tier = t
		}
		for _, i := range idx {
			if !seen[i] {
				seen[i] = true
				hit.Name = append(hit.Name, i)
			}
		}
	}
	sort.Ints(hit.Name)
	return hit
}

func matchWord(word, name, key, section, desc []rune) (Tier, []int) {
	if i := index(name, word); i >= 0 {
		idx := make([]int, len(word))
		for j := range word {
			idx[j] = i + j
		}
		switch {
		case len(name) == len(word):
			return NameExact, idx
		case i == 0:
			return NamePrefix, idx
		default:
			return NameContains, idx
		}
	}
	if index(key, word) >= 0 {
		return KeyContains, nil
	}
	if wordPrefix(section, word) {
		return SectionWord, nil
	}
	if len(word) >= minDescTerm && wordPrefix(desc, word) {
		return DescWord, nil
	}
	return None, nil
}

// Rank returns the rows that match, best tier first and otherwise in input
// order. An empty term returns every row in input order.
func Rank(term string, rows []Fields) []Hit {
	out := make([]Hit, 0, len(rows))
	if len(strings.Fields(term)) == 0 {
		for i := range rows {
			out = append(out, Hit{Index: i})
		}
		return out
	}
	for i, f := range rows {
		if h := Match(term, f); h.Tier != None {
			h.Index = i
			out = append(out, h)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Tier < out[b].Tier })
	return out
}

// sep separates the fields in a joined string; it never appears in settings text.
const sep = "\x00"

// Join packs fields into one string for list filters that take a single value.
func Join(f Fields) string {
	return strings.Join([]string{f.Name, f.Key, f.Section, f.Desc}, sep)
}

// Split unpacks a string made by Join. Other strings become a bare Name.
func Split(s string) Fields {
	parts := strings.SplitN(s, sep, 4)
	for len(parts) < 4 {
		parts = append(parts, "")
	}
	return Fields{Name: parts[0], Key: parts[1], Section: parts[2], Desc: parts[3]}
}

// lower folds case rune by rune, so indexes stay aligned with the original.
func lower(s string) []rune {
	r := []rune(s)
	for i, c := range r {
		r[i] = unicode.ToLower(c)
	}
	return r
}

// index is strings.Index over rune slices.
func index(s, sub []rune) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		j := 0
		for j < len(sub) && s[i+j] == sub[j] {
			j++
		}
		if j == len(sub) {
			return i
		}
	}
	return -1
}

// wordPrefix reports whether sub starts a word in s. A word starts at the
// beginning or after any rune that is not a letter or digit, so "_" and "."
// split words too.
func wordPrefix(s, sub []rune) bool {
	if len(sub) == 0 {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if i > 0 && isWordRune(s[i-1]) {
			continue
		}
		j := 0
		for j < len(sub) && s[i+j] == sub[j] {
			j++
		}
		if j == len(sub) {
			return true
		}
	}
	return false
}

func isWordRune(c rune) bool { return unicode.IsLetter(c) || unicode.IsDigit(c) }
