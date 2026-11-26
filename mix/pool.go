package mix

import (
	"sort"
	"strconv"
	"unicode/utf8"
)

// Pools are what Corpus learns from your usernames: every word and number,
// kept separately by the role it plays, so recombination only puts a part
// back where a part like it belongs.
//
// Only words that look like real parts of real names make it in: letters
// only, at least minWordLen long, and used at least minSeen times across
// the input in that role; a one-off is far more often a keyboard mash or a
// scrap left over from splitting one ("eh", "nh") than a name.
//
// Numbers are not learned from the input at all. Every generated username
// ends with a random number of minDigits..maxDigits digits, drawn from the
// full range of each length, so the pool is filled synthetically (see
// fillRandomNumbers).
type Pools struct {
	lead, tail map[string]bool // the pooled words, for checking splits

	leadsByLen, tailsByLen   map[int][]string // pooled words by length in characters
	leadBuckets, tailBuckets []lenBucket

	numbers map[int][]string // pooled trailing numbers by digit count
}

const (
	minWordLen = 3
	minSeen    = 3

	// Three-letter pieces are mostly initials and scraps ("ecp", "rpa");
	// the real ones (amy, joe, ali, lee) are common, so a three-letter
	// word has to be seen this often to be pooled.
	minSeenShort = 20

	// Every username ends with a random number this many digits long. The
	// digits are not taken from the input: minDigits-digit numbers run
	// 0..9, and longer ones run without a leading zero (10..99, 100..999),
	// so each number is a distinct string and no two combinations collide.
	minDigits = 1
	maxDigits = 3
)

// LearnPools builds Pools from names' cleaned segments (see clean).
func LearnPools(cleaned [][]Segment) *Pools {
	leadSeen, tailSeen := map[string]int{}, map[string]int{}
	for _, segs := range cleaned {
		lead := true
		for _, seg := range segs {
			if seg.Kind != Word {
				continue
			}
			if lead {
				leadSeen[seg.Text]++
			} else {
				tailSeen[seg.Text]++
			}
			lead = false
		}
	}

	p := &Pools{lead: map[string]bool{}, tail: map[string]bool{}, numbers: map[int][]string{}}
	keepWords := func(seen map[string]int, set map[string]bool) {
		for w, n := range seen {
			l := utf8.RuneCountInString(w)
			if l < minWordLen || !wordLike(w) || n < minSeen || (l == minWordLen && n < minSeenShort) {
				continue
			}
			{
				set[w] = true
			}
		}
	}
	keepWords(leadSeen, p.lead)
	keepWords(tailSeen, p.tail)
	p.leadsByLen, p.tailsByLen = byLength(sortedKeys(p.lead)), byLength(sortedKeys(p.tail))
	p.leadBuckets, p.tailBuckets = bucketsOf(p.leadsByLen), bucketsOf(p.tailsByLen)
	p.fillRandomNumbers(minDigits, maxDigits)
	return p
}

// fillRandomNumbers stocks the number pool with every number of
// loDigits..hiDigits digits: 0..9 for a one-digit length, then without a
// leading zero (10..99, 100..999) for longer ones so each number is a
// distinct string of its own length. A length of 0 adds nothing, so a range
// of 0..0 leaves the pool empty and usernames get no trailing number.
func (p *Pools) fillRandomNumbers(loDigits, hiDigits int) {
	p.numbers = map[int][]string{}
	for d := loDigits; d <= hiDigits; d++ {
		if d < 1 {
			continue
		}
		lo := 0
		if d > 1 {
			lo = pow10(d - 1)
		}
		hi := pow10(d) - 1
		nums := make([]string, 0, hi-lo+1)
		for n := lo; n <= hi; n++ {
			nums = append(nums, strconv.Itoa(n))
		}
		p.numbers[d] = nums
	}
}

// clampDigits holds a requested digit count to 0..maxDigits.
func clampDigits(d int) int {
	if d < 0 {
		return 0
	}
	if d > maxDigits {
		return maxDigits
	}
	return d
}

func pow10(n int) int {
	p := 1
	for ; n > 0; n-- {
		p *= 10
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
		n := utf8.RuneCountInString(w)
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

// buckets lists a slot's pool split by length, shortest first. A number
// slot has exactly one width, so it has at most one bucket.
func (p *Pools) buckets(s Shape) []lenBucket {
	switch s.Kind {
	case Word:
		if s.Lead {
			return p.leadBuckets
		}
		return p.tailBuckets
	case Number:
		if ns := p.numbers[s.Width]; len(ns) > 0 {
			return []lenBucket{{s.Width, uint64(len(ns))}}
		}
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
	case Number:
		return p.numbers[length][idx]
	}
	return ""
}

// splits calls fn with every way letters can be read as words pieces
// long, a pooled lead word followed by pooled tail words, giving the
// length in bytes of each piece; it stops early if fn returns false.
func (p *Pools) splits(letters string, words int, fn func(lens []int) bool) {
	lens := make([]int, 0, words)
	var walk func(rest string) bool
	walk = func(rest string) bool {
		i := len(lens)
		if i == words-1 {
			set := p.tail
			if i == 0 {
				set = p.lead
			}
			if !set[rest] {
				return true
			}
			lens = append(lens, len(rest))
			ok := fn(lens)
			lens = lens[:i]
			return ok
		}
		set := p.tail
		if i == 0 {
			set = p.lead
		}
		for j := 1; j < len(rest); j++ {
			if !utf8.RuneStart(rest[j]) || !set[rest[:j]] {
				continue
			}
			lens = append(lens, j)
			ok := walk(rest[j:])
			lens = lens[:i]
			if !ok {
				return false
			}
		}
		return true
	}
	walk(letters)
}

// canonical reports whether parts is the one way usermix builds the
// letters they spell. The same letters can come out of different picks
// ("maria"+"garcia" and "mari"+"agarcia", or one pooled word that is
// itself two), and only one of those may be used or Generate would write
// the same username twice. The chosen one uses the fewest words, and among
// splits into that many words, the longest first word, then the longest
// second.
func (p *Pools) canonical(parts []string) bool {
	letters := ""
	for _, w := range parts {
		letters += w
	}
	for k := 1; k < len(parts); k++ {
		found := false
		p.splits(letters, k, func([]int) bool { found = true; return false })
		if found {
			return false
		}
	}
	better := false
	p.splits(letters, len(parts), func(lens []int) bool {
		for i, n := range lens {
			if n != len(parts[i]) {
				better = n > len(parts[i])
				return !better
			}
		}
		return true
	})
	return !better
}
