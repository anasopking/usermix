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

// Templates extracts the name shapes among names' cleaned segments (see
// clean) — the number of whole words run together, 2 or 3 — and turns each
// into one pattern per trailing-number length in loDigits..hiDigits, so a
// generated username is those words followed by a number of that many
// digits. A length of 0 means no trailing number (loDigits and hiDigits
// both 0 makes every username a bare name). A bare name is otherwise never
// made; allowPlain lets a single name (plus its number) back in. Each
// shape's input count is split evenly across the allowed digit lengths, so
// when several are allowed short and long numbers appear about equally often
// whatever the input's own numbers were.
func Templates(cleaned [][]Segment, pools *Pools, allowPlain bool, loDigits, hiDigits int) []Template {
	seen := map[string]int{}
	base := map[string][]Shape{}
	var order []string
	for _, segs := range cleaned {
		words, ok := nameShape(segs, allowPlain)
		if !ok {
			continue
		}
		key := shapeKey(words)
		if _, ok := base[key]; !ok {
			base[key] = words
			order = append(order, key)
		}
		seen[key]++
	}
	var templates []Template
	for _, key := range order {
		for d := loDigits; d <= hiDigits; d++ {
			shape := append([]Shape(nil), base[key]...)
			if d > 0 {
				shape = append(shape, Shape{Kind: Number, Width: d})
			}
			t := Template{shape: shape, seen: seen[key]}
			t.cells, t.total = cellsOf(shape, pools)
			if t.total > 0 {
				templates = append(templates, t)
			}
		}
	}
	sort.SliceStable(templates, func(i, j int) bool { return templates[i].seen > templates[j].seen })
	return templates
}

// nameShape returns segs' word slots (its shape without any number) when
// segs is a usable name: whole words run together with an optional trailing
// number and nothing else. Two or three words, or a single word only when
// allowPlain is set.
func nameShape(segs []Segment, allowPlain bool) ([]Shape, bool) {
	end := len(segs)
	if end > 0 && segs[end-1].Kind == Number {
		end-- // the input's own trailing number is dropped; we add our own
	}
	words := segs[:end]
	for _, seg := range words {
		if seg.Kind != Word {
			return nil, false // a connector or a number anywhere but the end
		}
	}
	n := len(words)
	if n == 0 || n > maxWords || (n == 1 && !allowPlain) {
		return nil, false
	}
	return shapeOf(words), true
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
