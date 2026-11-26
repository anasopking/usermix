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
// last) picks that slot's entry among pool entries of its length.
func (c *cell) decode(shape []Shape, idx uint64, pools *Pools) string {
	digits := make([]uint64, len(shape))
	for i := len(shape) - 1; i >= 0; i-- {
		digits[i] = idx % c.radix[i]
		idx /= c.radix[i]
	}
	var b strings.Builder
	b.Grow(MaxLen)
	for i, s := range shape {
		b.WriteString(pools.pick(s, int(c.lens[i]), digits[i]))
	}
	return b.String()
}
