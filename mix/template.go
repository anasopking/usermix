package mix

import "sort"

// Template is one structural pattern found in the input, such as two
// words then a 2-digit number, how often the input uses it, and how many
// distinct usernames it can produce once every slot is filled from Pools.
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

// maxWords is the most words a generated username joins together; more
// than three run-together words stops reading as a name.
const maxWords = 3

// Templates extracts every usable structural pattern among names' cleaned
// segments (see clean), with how many input names follow each and how many
// combinations it can produce. Usable means letters first and at most one
// number, at the very end: kevinsmith45, never kevin45smith. It also means
// at least two words, so a bare name on its own ("oliver") or a bare name
// plus a number ("john2005", almost always already taken) is never made;
// allowPlain lets the second back in.
func Templates(cleaned [][]Segment, pools *Pools, allowPlain bool) []Template {
	seen := map[string]*Template{}
	var order []string
	for _, segs := range cleaned {
		if !usable(segs, allowPlain) {
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
	sort.SliceStable(templates, func(i, j int) bool { return templates[i].seen > templates[j].seen })
	return templates
}

func usable(segs []Segment, allowPlain bool) bool {
	words := 0
	for i, seg := range segs {
		switch seg.Kind {
		case Word:
			words++
		case Number:
			if i != len(segs)-1 || len(seg.Text) > maxDigitRun {
				return false
			}
		default:
			return false
		}
	}
	if words > maxWords || words == 0 {
		return false
	}
	if words == 1 {
		return allowPlain && len(segs) == 2
	}
	return true
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
			b[i] = byte('0' + s.Width)
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
