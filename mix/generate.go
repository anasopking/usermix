package mix

import (
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Corpus is what Learn builds from your usernames: the pools recombination
// draws from, and the structural patterns found among them.
type Corpus struct {
	pools     *Pools
	templates []Template
	spaces    []space // one per template, same order
	total     uint64  // sum of every template's combinations
	possible  uint64  // estimate of how many of those pass every filter
	input     map[string]bool
}

// space is one template's combinations, numbered 0..total-1 across its
// cells, plus the share of the output it should get.
type space struct {
	cells  []*cell
	bounds []uint64 // cumulative total, exclusive upper bound, one per cell
	total  uint64
	weight float64 // share of the input that follows this template
}

// Learn builds a Corpus from a list of usernames (one per line, as given to
// the CLI). allowPlain controls whether a single word plus a number
// ("john2005") is allowed; see Templates.
func Learn(names []string, allowPlain bool) *Corpus {
	vocab := buildVocab(names)
	cleaned := make([][]Segment, len(names))
	for i, name := range names {
		cleaned[i] = clean(vocab.refine(Split(name)))
	}

	pools := LearnPools(cleaned)
	templates := Templates(cleaned, pools, allowPlain)

	c := &Corpus{pools: pools, templates: templates, input: map[string]bool{}}
	seen := 0
	for _, t := range templates {
		seen += t.seen
	}
	var total uint64
	for ti := range c.templates {
		t := &c.templates[ti]
		sp := space{weight: float64(t.seen) / float64(seen)}
		for ci := range t.cells {
			cl := &t.cells[ci]
			if total+cl.total > maxTotal {
				continue // past the cap; see maxTotal
			}
			cl.tmpl = ti
			total += cl.total
			sp.total += cl.total
			sp.cells = append(sp.cells, cl)
			sp.bounds = append(sp.bounds, sp.total)
		}
		c.spaces = append(c.spaces, sp)
	}
	c.total = total
	for _, n := range names {
		var b strings.Builder
		for _, seg := range Split(n) {
			if seg.Kind != Separator {
				b.WriteString(strings.ToLower(seg.Text))
			}
		}
		c.input[b.String()] = true
	}
	c.possible = c.estimate()
	return c
}

// estimateSample is how many combinations per template estimate checks.
const estimateSample = 20000

// estimate counts, per template, what share of a spread-out sample of its
// combinations Generate would actually write (decodes canonically, isn't
// garbage, isn't already in the input) and scales that up to the
// template's size. A template small enough to check whole is counted
// exactly.
func (c *Corpus) estimate() uint64 {
	var sum float64
	for t, sp := range c.spaces {
		if sp.total == 0 {
			continue
		}
		f := newFeistel(sp.total, 0x5eed)
		k := min(sp.total, estimateSample)
		var ok uint64
		for i := uint64(0); i < k; i++ {
			if name, good := c.decode(t, f.permute(i)); good && !Garbage(name) && !c.input[name] {
				ok++
			}
		}
		sum += float64(sp.total) * float64(ok) / float64(k)
	}
	if sum >= float64(maxTotal) {
		return maxTotal
	}
	return uint64(math.Round(sum))
}

// Total is how many distinct combinations this corpus has (before
// skipping ones that are garbage or already in the input).
func (c *Corpus) Total() uint64 { return c.total }

// Possible is about how many distinct usernames Generate can actually
// write from this corpus, once every filter has had its say: the most it
// will hand out before running dry. It is exact for small inputs and an
// estimate from sampling otherwise.
func (c *Corpus) Possible() uint64 { return c.possible }

// Templates reports the structural patterns Learn found, most used first.
func (c *Corpus) Templates() []Template { return c.templates }

func (c *Corpus) decode(t int, idx uint64) (string, bool) {
	sp := &c.spaces[t]
	i := sort.Search(len(sp.bounds), func(i int) bool { return idx < sp.bounds[i] })
	lower := uint64(0)
	if i > 0 {
		lower = sp.bounds[i-1]
	}
	cl := sp.cells[i]
	return cl.decode(c.templates[cl.tmpl].shape, idx-lower, c.pools)
}

// Options controls Generate.
type Options struct {
	Seed       uint64 // 0 picks a random one
	AllowPlain bool   // see Templates; only takes effect via Learn, kept here for the CLI
	Workers    int    // 0 means every CPU core

	// Progress, if non-nil, is called as usernames are found and as raw
	// combinations are worked through, so a caller can draw a progress
	// bar. found and triedRaw are both cumulative and only ever increase.
	Progress func(found, triedRaw uint64)
}

const (
	roundSize = 1 << 15 // usernames aimed for per round
	blockSize = 1 << 12 // raw combinations per worker job
)

// Generate streams up to n distinct new usernames to emit, using every CPU
// core, until n are found or the corpus is exhausted (then it returns a
// short count).
//
// Each template gets a share of the output equal to its share of the
// input, so if a third of your list is two names and a 2-digit number, so
// is a third of the output; how many combinations a template happens to
// allow doesn't matter. Output is made in rounds: every round, each
// template is handed as many fresh combinations as it needs to catch up
// with its share, workers decode them in parallel, and the round's
// usernames are interleaved so templates are mixed through the file
// rather than written in runs.
//
// Generation is duplicate-free by construction: each template walks its
// own combination space in an order shuffled by a Feistel permutation
// keyed on seed, never visiting an index twice, and every username has
// only one combination that can produce it. That needs no memory to track
// what has been handed out, even for billions. For a given seed, the
// output is the same on every run and for any number of workers, and
// asking for more only extends it.
func Generate(c *Corpus, n uint64, opts Options, emit func(string)) uint64 {
	if c.total == 0 || n == 0 {
		return 0
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	seed := opts.Seed
	if seed == 0 {
		seed = randomSeed()
	}

	type stream struct {
		f             *feistel
		next, emitted uint64
	}
	streams := make([]stream, len(c.spaces))
	for t, sp := range c.spaces {
		if sp.total > 0 {
			streams[t].f = newFeistel(sp.total, splitmix64(seed^uint64(t+1)*0x9E3779B97F4A7C15))
		}
	}

	type job struct {
		t          int
		start, end uint64
		names      []string
	}
	jobs := make(chan *job)
	var wg sync.WaitGroup
	for range workers {
		go func() {
			for j := range jobs {
				f := streams[j.t].f
				for raw := j.start; raw < j.end; raw++ {
					name, ok := c.decode(j.t, f.permute(raw))
					if ok && !Garbage(name) && !c.input[name] {
						j.names = append(j.names, name)
					}
				}
				wg.Done()
			}
		}()
	}
	defer close(jobs)

	var found, tried uint64
	type placed struct {
		pos  float64
		name string
	}
	var batch []placed
	for found < n {
		// Hand each live template enough raw combinations to bring it up
		// to its share of everything emitted by the end of this round,
		// allowing for the ones its filters have been rejecting.
		live := 0.0
		for t, sp := range c.spaces {
			if streams[t].f != nil && streams[t].next < sp.total {
				live += sp.weight
			}
		}
		if live == 0 {
			break // every template exhausted
		}
		target := float64(found + roundSize)
		var round []*job
		for t, sp := range c.spaces {
			s := &streams[t]
			if s.f == nil || s.next >= sp.total {
				continue
			}
			need := target*sp.weight/live - float64(s.emitted)
			if need < 0.5 {
				continue
			}
			rate := (float64(s.emitted) + 1) / (float64(s.next) + 1)
			raw := uint64(math.Ceil(need / rate))
			raw = min(raw, 4*roundSize, sp.total-s.next)
			for start := s.next; start < s.next+raw; start += blockSize {
				round = append(round, &job{t: t, start: start, end: min(start+blockSize, s.next+raw)})
			}
			s.next += raw
			tried += raw
		}
		wg.Add(len(round))
		for _, j := range round {
			jobs <- j
		}
		wg.Wait()

		// Interleave: spread each template's names evenly across the round.
		batch = batch[:0]
		perT := map[int]int{}
		for _, j := range round {
			perT[j.t] += len(j.names)
		}
		idx := map[int]int{}
		for _, j := range round {
			for _, name := range j.names {
				i := idx[j.t]
				idx[j.t]++
				batch = append(batch, placed{(float64(i) + 0.5) / float64(perT[j.t]), name})
			}
		}
		sort.SliceStable(batch, func(a, b int) bool { return batch[a].pos < batch[b].pos })
		for _, j := range round {
			streams[j.t].emitted += uint64(len(j.names))
		}
		for _, p := range batch {
			if found >= n {
				break
			}
			emit(p.name)
			found++
		}
		if opts.Progress != nil {
			opts.Progress(found, tried)
		}
	}
	return found
}
