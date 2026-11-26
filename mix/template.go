package mix

import "sort"

// Template is one structural pattern found in the input, such as
// Word(lead) + Separator + Word — and how many distinct usernames it can
// produce once every slot is filled from Pools.
type Template struct {
	shape []Shape
	seen  int    // how many input usernames had this shape
	total uint64 // combinations this shape can produce from pools
}

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
		t := &Template{shape: shape, seen: 1, total: size(shape, pools)}
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
	sort.Slice(templates, func(i, j int) bool { return templates[i].total > templates[j].total })
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

// size is the product of every slot's pool size: how many distinct
// usernames this shape can produce.
func size(shape []Shape, pools *Pools) uint64 {
	total := uint64(1)
	for _, s := range shape {
		n := slotSize(s, pools)
		if n == 0 {
			return 0
		}
		total *= n // see note in Generate on overflow: pools this large don't occur in practice
	}
	return total
}

func slotSize(s Shape, pools *Pools) uint64 {
	switch s.Kind {
	case Word:
		return uint64(len(pools.wordPool(s.Lead)))
	case Number:
		return pools.numberPoolSize()
	case Separator:
		return uint64(len(pools.seps))
	}
	return 0
}
