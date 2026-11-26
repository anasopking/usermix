package mix

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Pools are what Corpus learns from your usernames: every word and number,
// kept separately by the role it plays, so recombination only puts a part
// back where a part like it belongs.
type Pools struct {
	leads []string // lowercased words seen as a name's first word
	tails []string // lowercased words seen later in a name
	seps  []string // separators used ("_", "-", ".")

	// numbers groups learned numbers by digit count and gives each
	// length's full range, so the pool is every number of that many
	// digits, not just the ones actually seen — real names use plenty of
	// numbers nobody happened to pick in a seed list. Grouping strictly by
	// digit count, with no overlap between groups, is what lets Generate
	// guarantee no two combinations decode to the same username: if two
	// groups' ranges could overlap in value (a "looks like a year" group
	// and a general 4-digit group both containing 1998, say), the same
	// number could be reached by two different combinations, and so could
	// the username built around it.
	numbers map[int]numberRange

	// The same pools split by length in characters (numbers are already
	// split by digit count above), so combinations can be built to land
	// inside MinLen..MaxLen instead of being tried and thrown away.
	leadsByLen, tailsByLen, sepsByLen    map[int][]string
	leadBuckets, tailBuckets, sepBuckets []lenBucket
}

type numberRange struct {
	lo, hi, width int // inclusive; width is the digit count, for zero-padding
}

func (nr numberRange) size() uint64 { return uint64(nr.hi-nr.lo) + 1 }

func (nr numberRange) format(n int) string {
	return fmt.Sprintf("%0*d", nr.width, n+nr.lo)
}

// LearnPools builds Pools from names' refined segments (see Vocab.refine).
func LearnPools(refined [][]Segment) *Pools {
	p := &Pools{numbers: map[int]numberRange{}}
	leadSet, tailSet, sepSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	numSeen := map[int][]int{}

	for _, segs := range refined {
		lead := true
		for _, seg := range segs {
			switch seg.Kind {
			case Word:
				w := strings.ToLower(seg.Text)
				if len([]rune(w)) < 2 {
					continue // initials aren't pooled as words
				}
				if lead {
					leadSet[w] = true
				} else {
					tailSet[w] = true
				}
				lead = false
			case Number:
				digits := len(seg.Text)
				if n, err := strconv.Atoi(seg.Text); err == nil {
					numSeen[digits] = append(numSeen[digits], n)
				}
			case Separator:
				sepSet[seg.Text] = true
			}
		}
	}

	p.leads = sortedKeys(leadSet)
	p.tails = sortedKeys(tailSet)
	p.seps = sortedKeys(sepSet)
	p.leadsByLen, p.tailsByLen, p.sepsByLen = byLength(p.leads), byLength(p.tails), byLength(p.seps)
	p.leadBuckets, p.tailBuckets, p.sepBuckets = bucketsOf(p.leadsByLen), bucketsOf(p.tailsByLen), bucketsOf(p.sepsByLen)
	for digits, ns := range numSeen {
		lo, hi := ns[0], ns[0]
		for _, n := range ns {
			lo, hi = min(lo, n), max(hi, n)
		}
		p.numbers[digits] = numberRange{lo: lo, hi: hi, width: digits}
	}
	return p
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// lenBucket is every entry of one slot's pool that has the same length in
// characters: a slot filled only from one bucket always adds exactly that
// many characters to the username.
type lenBucket struct {
	length int
	size   uint64
}

// byLength groups words (already sorted) by length in characters, keeping
// their order within each length.
func byLength(words []string) map[int][]string {
	m := map[int][]string{}
	for _, w := range words {
		n := len([]rune(w))
		m[n] = append(m[n], w)
	}
	return m
}

func bucketsOf(m map[int][]string) []lenBucket {
	out := make([]lenBucket, 0, len(m))
	for l, ws := range m {
		out = append(out, lenBucket{l, uint64(len(ws))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].length < out[j].length })
	return out
}

// buckets lists a slot's pool split by length, shortest first. Numbers
// longer than maxDigitRun digits are left out: Garbage rejects every
// username containing one, so they could never be used.
func (p *Pools) buckets(s Shape) []lenBucket {
	switch s.Kind {
	case Word:
		if s.Lead {
			return p.leadBuckets
		}
		return p.tailBuckets
	case Separator:
		return p.sepBuckets
	case Number:
		var out []lenBucket
		for width, nr := range p.numbers {
			if width <= maxDigitRun {
				out = append(out, lenBucket{width, nr.size()})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].length < out[j].length })
		return out
	}
	return nil
}

// pick returns the idx'th entry of the slot's pool among entries of the
// given length (idx < that bucket's size).
func (p *Pools) pick(s Shape, length int, idx uint64) string {
	switch s.Kind {
	case Word:
		if s.Lead {
			return p.leadsByLen[length][idx]
		}
		return p.tailsByLen[length][idx]
	case Separator:
		return p.sepsByLen[length][idx]
	case Number:
		return p.numbers[length].format(int(idx))
	}
	return ""
}
