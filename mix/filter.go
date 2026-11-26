package mix

import (
	"strings"
	"unicode"
)

// Garbage usernames are never produced: keyboard mashes, placeholders,
// unpronounceable runs, and so on.

const (
	maxDigitRun     = 4 // a year fits; phone numbers and IDs don't
	maxRepeat       = 3 // "zzz" is fine, "zzzz" is not
	maxChunkRepeat  = 2 // "haha" is fine, "hahaha" is not
	maxConsonantRun = 6
	minVowelless    = 6
	sequenceLen     = 5
)

var placeholders = map[string]bool{
	"admin": true, "administrator": true, "anon": true, "anonymous": true,
	"default": true, "deleted": true, "example": true, "guest": true,
	"member": true, "nil": true, "none": true, "null": true, "removed": true,
	"root": true, "sample": true, "test": true, "testing": true,
	"undefined": true, "unknown": true, "user": true, "username": true,
}

var sequences = buildSequences()

func buildSequences() map[string]bool {
	seqs := map[string]bool{}
	for _, row := range []string{"qwertyuiop", "asdfghjkl", "zxcvbnm", "abcdefghijklmnopqrstuvwxyz"} {
		r := []rune(row)
		rev := make([]rune, len(r))
		for i, c := range r {
			rev[len(r)-1-i] = c
		}
		for i := 0; i+sequenceLen <= len(r); i++ {
			seqs[string(r[i:i+sequenceLen])] = true
			seqs[string(rev[i:i+sequenceLen])] = true
		}
	}
	return seqs
}

// Garbage reports whether name is not worth producing or learning from.
func Garbage(name string) bool {
	n := len([]rune(name))
	if n < MinLen || n > MaxLen {
		return true
	}
	lower := strings.ToLower(name)

	if placeholders[strings.TrimRight(lower, "0123456789_-.")] {
		return true
	}
	for i := 0; i+sequenceLen <= len(lower); i++ {
		if sequences[lower[i:i+sequenceLen]] {
			return true
		}
	}
	if repeats(lower) {
		return true
	}

	digitRun, consonantRun, latin, vowels, letters := 0, 0, 0, 0, 0
	for _, r := range name {
		if unicode.IsDigit(r) {
			if digitRun++; digitRun > maxDigitRun {
				return true
			}
		} else {
			digitRun = 0
		}
		if unicode.IsLetter(r) {
			letters++
		}
		if r < 128 && unicode.IsLetter(r) {
			latin++
			if strings.ContainsRune("aeiouyAEIOUY", r) {
				vowels++
				consonantRun = 0
			} else if consonantRun++; consonantRun > maxConsonantRun {
				return true
			}
		} else {
			consonantRun = 0
		}
	}
	if letters*2 < n {
		return true
	}
	if latin >= minVowelless && vowels == 0 {
		return true
	}
	return false
}

// repeats reports whether a 1-3 character chunk repeats back to back more
// than real names do: "hahaha" (chunk "ha" x3), "zzzz" (chunk "z" x4).
func repeats(s string) bool {
	for size := 1; size <= 3; size++ {
		limit := maxChunkRepeat
		if size == 1 {
			limit = maxRepeat
		}
		run := 0
		for i := size; i < len(s); i++ {
			if s[i] != s[i-size] {
				run = 0
				continue
			}
			if run++; run >= size*limit {
				return true
			}
		}
	}
	return false
}

// Plain reports whether segs is one or more words run together with no
// separator, followed by a bare number with nothing else around it —
// "john2005", "oliver4006", and just as much "johnthorne1195": joined with
// no separator, it reads as one plain name before the number even though
// it was built from two. Templates skips this shape by default; see
// Templates' allowPlain.
func Plain(segs []Segment) bool {
	if len(segs) < 2 || segs[len(segs)-1].Kind != Number {
		return false
	}
	for _, seg := range segs[:len(segs)-1] {
		if seg.Kind != Word {
			return false // a separator or an earlier number breaks the plain look
		}
	}
	return true
}
