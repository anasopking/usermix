package mix

import "sort"

// Template is one structural pattern found in the input, such as
// Word(lead) + Separator + Word — and how many distinct usernames it can
// produce once every slot is filled from Pools.
type Template struct {
	shape []Shape
	seen  int    // how many input usernames had this shape
	total uint64 // combinations this shape can produce from pools within MinLen..MaxLen
	cells []cell
}

// maxTotal caps how many combinations a corpus counts, far beyond anything
// that could ever be generated, so sums and products of pool sizes can
// never overflow uint64. Combinations past the cap are skipped.
const maxTotal = uint64(1) << 62

// Templates extracts every distinct structural pattern among names'
// refined segments (see Vocab.refine), with each one's size: the number of
// combinations it can produce once word, number and separator slots are
// filled from pools. Patterns that are just a bare word (a name on its
// own) or a bare word followed by a bare number ("john2005", "oliver4006"
// — almost always already taken) are left out unless allowPlain is true.
func Templates(refined [][]Segment, pools *Pools, allowPlain bool) []Template {
	seen := map[string]*Template{}
	var order []string
	for _, segs := range refined {
		if len(segs) == 0 {
			continue
		}
		if !allowPlain && (len(segs) == 1 || Plain(segs)) {
			continue
		}
		shape := shapeOf(segs)
		key := shapeKey(shape)
		if t, ok := seen[key]; ok {
			t.seen++
			continue
		}
		t := &Template{shape: shape, seen: 1}
		t.cells, t.total = cellsOf(shape, pools)
		seen[key] = t
		order = append(order, key)
	}
	templates := make([]Template, 0, len(order))
	for _, key := range order {
		t := seen[key]
		if t.total > 0 {
			templates = append(templates, *t)
		}
	}
	sort.SliceStable(templates, func(i, j int) bool { return templates[i].total > templates[j].total })
	return templates
}

func shapeKey(shape []Shape) string {
	b := make([]byte, len(shape))
	for i, s := range shape {
		switch {
		case s.Kind == Word && s.Lead:
			b[i] = 'L'
		case s.Kind == Word:
			b[i] = 'T'
		case s.Kind == Number:
			b[i] = 'N'
		default:
			b[i] = 'S'
		}
	}
	return string(b)
}

// cellsOf splits shape's combinations by the length of each slot and
// keeps only the length choices that add up to MinLen..MaxLen, so a big
// shape (three words and a number, say) contributes just the combinations
// that fit instead of billions that are all too long. It returns the cells
// and their combined size, capped at maxTotal.
func cellsOf(shape []Shape, pools *Pools) ([]cell, uint64) {
	k := len(shape)
	opts := make([][]lenBucket, k)
	for i, s := range shape {
		opts[i] = pools.buckets(s)
		if len(opts[i]) == 0 {
			return nil, 0
		}
	}
	// minRest[i] / maxRest[i]: the shortest / longest slots i..k-1 can be.
	minRest, maxRest := make([]int, k+1), make([]int, k+1)
	for i := k - 1; i >= 0; i-- {
		minRest[i] = minRest[i+1] + opts[i][0].length
		maxRest[i] = maxRest[i+1] + opts[i][len(opts[i])-1].length
	}
	if minRest[0] > MaxLen || maxRest[0] < MinLen {
		return nil, 0
	}

	var cells []cell
	var total uint64
	lens := make([]uint8, k)
	radix := make([]uint64, k)
	var walk func(i, used int, product uint64)
	walk = func(i, used int, product uint64) {
		if i == k {
			if total+product > maxTotal {
				return
			}
			total += product
			cells = append(cells, cell{
				lens:  append([]uint8(nil), lens...),
				radix: append([]uint64(nil), radix...),
				total: product,
			})
			return
		}
		for _, b := range opts[i] {
			n := used + b.length
			if n+minRest[i+1] > MaxLen {
				break // buckets are shortest first; the rest are longer still
			}
			if n+maxRest[i+1] < MinLen {
				continue
			}
			if product > maxTotal/b.size {
				continue // would overflow the cap
			}
			lens[i], radix[i] = uint8(b.length), b.size
			walk(i+1, n, product*b.size)
		}
	}
	walk(0, 0, 1)
	return cells, total
}
