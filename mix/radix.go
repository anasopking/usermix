package mix

import "strings"

// decode turns a combination index (0 <= idx < t.total) into the username
// it names: each slot's share of idx (mixed-radix, least significant slot
// last) picks that slot's pool entry.
func (t Template) decode(idx uint64, pools *Pools) string {
	// Compute each slot's radix (pool size) and pick its digit from the
	// least significant slot first, matching how size() multiplies them.
	digits := make([]uint64, len(t.shape))
	for i := len(t.shape) - 1; i >= 0; i-- {
		radix := slotSize(t.shape[i], pools)
		digits[i] = idx % radix
		idx /= radix
	}

	var b strings.Builder
	for i, s := range t.shape {
		switch s.Kind {
		case Word:
			b.WriteString(pools.wordPool(s.Lead)[digits[i]])
		case Number:
			b.WriteString(pools.numberAt(digits[i]))
		case Separator:
			b.WriteString(pools.seps[digits[i]])
		}
	}
	return b.String()
}
