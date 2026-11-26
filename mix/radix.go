package mix

import "strings"

// cell is the part of a template's combinations whose slots have one fixed
// length each (lens), chosen so the whole username is MinLen..MaxLen
// characters long. A template's cells don't overlap and together hold every
// combination of it that fits the length limit, so no index is ever spent
// on a username that is too long or too short.
type cell struct {
	tmpl  int     // index into Corpus.templates (set by Learn)
	lens  []uint8 // each slot's length in characters
	radix []uint64
	total uint64
}

// decode turns a combination index (0 <= idx < c.total) into the username
// it names: each slot's share of idx (mixed-radix, least significant slot
// last) picks that slot's entry among pool entries of its length. ok is
// false when the words picked spell letters that another combination
// spells too and this one isn't the canonical way (see Pools.canonical),
// so every username has exactly one combination that produces it.
func (c *cell) decode(shape []Shape, idx uint64, pools *Pools) (name string, ok bool) {
	var digits [maxWords + 1]uint64
	for i := len(shape) - 1; i >= 0; i-- {
		digits[i] = idx % c.radix[i]
		idx /= c.radix[i]
	}
	var parts [maxWords]string
	nwords := 0
	var b strings.Builder
	b.Grow(MaxLen + 8)
	for i, s := range shape {
		part := pools.pick(s, int(c.lens[i]), digits[i])
		if s.Kind == Word {
			parts[nwords] = part
			nwords++
		}
		b.WriteString(part)
	}
	if nwords > 1 && !pools.canonical(parts[:nwords]) {
		return "", false
	}
	return b.String(), true
}
