package mix

import (
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
	bounds    []uint64 // cumulative total, exclusive upper bound, one per template
	total     uint64   // sum of every template's combinations
	input     map[string]bool
}

// Learn builds a Corpus from a list of usernames (one per line, as given to
// the CLI). allowPlain controls whether bare-word and bare-word-plus-number
// patterns ("john2005") are included; see Templates.
func Learn(names []string, allowPlain bool) *Corpus {
	vocab := buildVocab(names)
	refined := make([][]Segment, len(names))
	for i, name := range names {
		refined[i] = vocab.refine(Split(name))
	}

	pools := LearnPools(refined)
	templates := Templates(refined, pools, allowPlain)

	c := &Corpus{pools: pools, templates: templates, input: map[string]bool{}}
	c.bounds = make([]uint64, len(templates))
	var total uint64
	for i, t := range templates {
		total = satAdd(total, t.total)
		c.bounds[i] = total
	}
	c.total = total
	for _, n := range names {
		c.input[strings.ToLower(n)] = true
	}
	return c
}

func satAdd(a, b uint64) uint64 {
	if a+b < a { // overflow
		return ^uint64(0)
	}
	return a + b
}

// Total is how many distinct usernames this corpus can produce (before
// skipping ones that are garbage or already in the input).
func (c *Corpus) Total() uint64 { return c.total }

// Templates reports the structural patterns Learn found, most prolific
// first.
func (c *Corpus) Templates() []Template { return c.templates }

func (c *Corpus) decode(idx uint64) string {
	i := sort.Search(len(c.bounds), func(i int) bool { return idx < c.bounds[i] })
	lower := uint64(0)
	if i > 0 {
		lower = c.bounds[i-1]
	}
	return c.templates[i].decode(idx-lower, c.pools)
}

// Options controls Generate.
type Options struct {
	Seed       uint64 // 0 picks a random one
	AllowPlain bool   // see Templates; only takes effect via Learn, kept here for the CLI
	Workers    int    // 0 means every CPU core
}

// Generate streams up to n distinct new usernames to emit, using every CPU
// core, until n are found or the corpus is exhausted (then it returns a
// short count). Generation is duplicate-free by construction — every index
// in the corpus's combination space is visited at most once, in an order
// shuffled by a Feistel permutation keyed on seed, so handing out billions
// needs no memory to track what has been given out already. For a given
// seed, the usernames emitted (and their order) are the same on every run,
// regardless of how many workers ran: raw counters are processed in fixed
// blocks, and blocks are only ever consumed in order, so which ones are
// needed never depends on how threads happened to be scheduled. It returns
// the number emitted.
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
	f := newFeistel(c.total, seed)

	// Raw counters are handed out in fixed-size blocks. Workers fill
	// blocks concurrently (whichever worker is free takes the next one),
	// but a block's contents only depend on its own fixed start..end
	// range, never on timing — and the main loop below only ever reads
	// finished blocks back in the order they were dispatched. That is
	// what makes output order (and the stopping point) independent of
	// how goroutines happened to be scheduled.
	const blockSize = 1 << 14
	type block struct {
		start, end uint64
		names      []string
		done       chan struct{}
	}
	work := make(chan *block, workers)
	ordered := make(chan *block, workers*2)
	cancel := make(chan struct{})

	var workerWG sync.WaitGroup
	for range workers {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for b := range work {
				for raw := b.start; raw < b.end; raw++ {
					name := c.decode(f.permute(raw))
					if !Garbage(name) && !c.input[name] {
						b.names = append(b.names, name)
					}
				}
				close(b.done)
			}
		}()
	}

	go func() {
		defer close(work)
		defer close(ordered)
		for start := uint64(0); start < c.total; start += blockSize {
			b := &block{start: start, end: min(start+blockSize, c.total), done: make(chan struct{})}
			select {
			case <-cancel:
				return
			case ordered <- b:
				work <- b
			}
		}
	}()

	var total uint64
	for b := range ordered {
		<-b.done
		for _, name := range b.names {
			if total >= n {
				break
			}
			emit(name)
			total++
		}
		if total >= n {
			close(cancel)
			break
		}
	}
	for range ordered {
		// Drain any blocks already dispatched before cancel took effect,
		// so their workers' sends don't block forever.
	}
	workerWG.Wait()
	return total
}
