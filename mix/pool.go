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

// wordPool returns the lead or tail pool.
func (p *Pools) wordPool(lead bool) []string {
	if lead {
		return p.leads
	}
	return p.tails
}

// digitGroups lists p.numbers' keys in a fixed order, so number slots can
// be indexed into a single flat pool: every learned digit-length's range,
// one after another. A slot drawing from this pool can land on any digit
// length (a 2-digit number here, a 4-digit one there), which is how real
// usernames vary and multiplies how many numbers are available far beyond
// what a single length's range alone would give.
func (p *Pools) digitGroups() []int {
	groups := make([]int, 0, len(p.numbers))
	for k := range p.numbers {
		groups = append(groups, k)
	}
	sort.Ints(groups)
	return groups
}

// numberPoolSize is the combined size of every digit length's range.
func (p *Pools) numberPoolSize() uint64 {
	var total uint64
	for _, digits := range p.digitGroups() {
		total += p.numbers[digits].size()
	}
	return total
}

// numberAt returns the idx'th number in the flat pool numberPoolSize
// describes (idx < numberPoolSize()).
func (p *Pools) numberAt(idx uint64) string {
	for _, digits := range p.digitGroups() {
		nr := p.numbers[digits]
		size := nr.size()
		if idx < size {
			return nr.format(int(idx))
		}
		idx -= size
	}
	return "" // unreachable if idx < numberPoolSize()
}
