package mix

import "strings"

// Many real usernames join two words with no separator and no case change
// ("mariagarcia98"), which Split alone sees as a single opaque word. Vocab
// lets usermix split those using only words it has seen written separately
// elsewhere in the same input — never a built-in word list, just what the
// million-odd usernames you give it already show it.
type Vocab struct {
	lead map[string]bool // words seen starting a name ("maria")
	tail map[string]bool // words seen later in a name ("garcia")
}

const minPieceLen = 3 // shortest piece Vocab will split off; avoids "ha"+junk

// buildVocab learns which words start names and which come later, but only
// from a boundary it can trust: an explicit separator, or a change of
// case. A word that only touches digits with no separator ("maya" in
// "maya2005") proves nothing about where IT starts or ends, because the
// same shape also covers joined compounds ("mayagentry" in "mayagentry55")
// — so digit-only boundaries teach nothing here, or "mayagentry" would be
// learned as one atomic word and block itself from ever being split.
func buildVocab(names []string) *Vocab {
	v := &Vocab{lead: map[string]bool{}, tail: map[string]bool{}}
	for _, name := range names {
		if !hasTrustedBoundary(name) {
			continue
		}
		lead := true
		for _, seg := range Split(name) {
			if seg.Kind != Word {
				continue
			}
			if len([]rune(seg.Text)) < 2 {
				continue // initials don't teach words
			}
			w := strings.ToLower(seg.Text)
			if lead {
				v.lead[w] = true
			} else {
				v.tail[w] = true
			}
			lead = false
		}
	}
	return v
}

// hasTrustedBoundary reports whether name contains a separator or a
// lowercase-to-uppercase case change, either of which proves where a word
// actually starts or ends, unlike sitting next to a bare digit.
func hasTrustedBoundary(name string) bool {
	runes := []rune(name)
	for i, r := range runes {
		if r == '_' || r == '-' || r == '.' {
			return true
		}
		if i > 0 {
			prev := runes[i-1]
			if (prev >= 'a' && prev <= 'z') && (r >= 'A' && r <= 'Z') {
				return true
			}
		}
	}
	return false
}

// refine rewrites segs, splitting any lowercase joined word it can into the
// two learned words it is made of: "mariagarcia" -> "maria", "garcia".
// Words already known whole, words with any uppercase or non-letter
// content, and words usermix can't split are left as they are.
func (v *Vocab) refine(segs []Segment) []Segment {
	out := make([]Segment, 0, len(segs)+1)
	for _, seg := range segs {
		if first, rest, ok := v.split(seg); ok {
			out = append(out, Segment{first, Word}, Segment{rest, Word})
			continue
		}
		out = append(out, seg)
	}
	return out
}

func (v *Vocab) split(seg Segment) (first, rest string, ok bool) {
	if seg.Kind != Word || seg.Text != strings.ToLower(seg.Text) {
		return "", "", false
	}
	w := []rune(seg.Text)
	if len(w) < 2*minPieceLen || v.lead[string(w)] || v.tail[string(w)] {
		return "", "", false // whole word already known; splitting would hide it
	}
	// Prefer the longest valid lead piece, so "danielcarter" splits as
	// "daniel"+"carter" rather than "dan"+"ielcarter" when both exist.
	for i := len(w) - minPieceLen; i >= minPieceLen; i-- {
		a, b := string(w[:i]), string(w[i:])
		if v.lead[a] && v.tail[b] {
			return a, b, true
		}
	}
	return "", "", false
}
