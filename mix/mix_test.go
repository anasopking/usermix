package mix

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func seedNames() []string {
	firsts := []string{"maria", "john", "kevin", "oliver", "sara", "tobias", "ines", "yusuf", "greta", "amir"}
	lasts := []string{"garcia", "smith", "lindqvist", "hollis", "carter", "thorne", "quigley", "ashby", "okafor", "winslow"}
	seps := []string{"", ".", "_"}
	var out []string
	i := 0
	for _, f := range firsts {
		for _, l := range lasts {
			sep := seps[i%len(seps)]
			out = append(out, f+sep+l)
			out = append(out, f+sep+l+strconv.Itoa(1980+i%40))
			out = append(out, fmt.Sprintf("%s%d", f, 1000+i))
			i++
		}
	}
	return out
}

func TestSplit(t *testing.T) {
	cases := map[string][]string{
		"DarkWolf_99": {"Dark", "Wolf", "_", "99"},
		"john.smith":  {"john", ".", "smith"},
		"XMLParser":   {"XML", "Parser"},
		"":            nil,
	}
	for name, want := range cases {
		var got []string
		for _, seg := range Split(name) {
			got = append(got, seg.Text)
		}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("Split(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestVocabSplitsJoinedWords(t *testing.T) {
	// "maria" and "garcia" are each taught by delimited names elsewhere;
	// "mariagarcia" has no separator at all, so it must come from the
	// learned vocabulary, not from being seen whole.
	names := []string{"maria.smith", "john_garcia", "garcia.john", "mariagarcia77"}
	v := buildVocab(names)
	segs := v.refine(Split("mariagarcia77"))
	var words []string
	for _, s := range segs {
		if s.Kind == Word {
			words = append(words, s.Text)
		}
	}
	if len(words) != 2 || words[0] != "maria" || words[1] != "garcia" {
		t.Fatalf("refine(mariagarcia77) words = %v, want [maria garcia]", words)
	}
}

func TestVocabDoesNotLearnFromDigitOnlyBoundary(t *testing.T) {
	// A word that only touches a bare number proves nothing about an
	// internal split, so "mayagentry" (no separator at all between "maya"
	// and "gentry") must not be learned as one atomic word just because
	// "maya2005" exists.
	names := []string{"maya2005", "oliver4006"}
	v := buildVocab(names)
	if v.lead["maya"] || v.lead["oliver"] {
		t.Fatal("buildVocab learned a word from a digit-only boundary")
	}
}

// wordsIn reports the fewest pooled words the letters of name split into
// (0 if none), and its trailing number.
func wordsIn(c *Corpus, name string) (int, string) {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	for k := 1; k <= maxWords; k++ {
		found := false
		c.pools.splits(name[:i], k, func([]int) bool { found = true; return false })
		if found {
			return k, name[i:]
		}
	}
	return 0, name[i:]
}

var namesThenDigits = regexp.MustCompile(`^[a-z]+[0-9]{1,3}$`)

func TestOutputIsNamesThenRandomDigits(t *testing.T) {
	// Every username is two or three whole words with no connector, then a
	// 1-to-3-digit number at the very end: never "kevin45smith", never
	// "maria.garcia", and never a bare name with no number.
	c := Learn(seedNames(), false)
	n := Generate(c, 20000, Options{Seed: 1}, func(name string) {
		if !namesThenDigits.MatchString(name) {
			t.Fatalf("%q is not whole names then a 1-3 digit number", name)
		}
		if k, _ := wordsIn(c, name); k < 2 {
			t.Fatalf("%q is not made of two or more whole words (got %d)", name, k)
		}
	})
	if n == 0 {
		t.Fatal("generated nothing")
	}
}

func TestTrailingNumberIsNotTakenFromInput(t *testing.T) {
	// Numbers are random, not learned: a 4-digit year the whole input ends
	// with is never reproduced (too long), and a number buried mid-name is
	// never used either.
	names := append(seedNames(), "kevin4242smith", "maria4242garcia")
	for i := range names {
		names[i] = names[i] + "1998"
	}
	c := Learn(names, false)
	Generate(c, 20000, Options{Seed: 2}, func(name string) {
		if strings.Contains(name, "1998") || strings.Contains(name, "4242") {
			t.Fatalf("%q reused a number from the input", name)
		}
	})
}

func TestDigitWidthsAreRoughlyEven(t *testing.T) {
	// Each username ends with 1, 2 or 3 digits, and the three lengths show
	// up in similar amounts rather than mostly the longest.
	c := Learn(bigNames(20000), false)
	var byWidth [4]int
	total := 0
	Generate(c, 30000, Options{Seed: 7}, func(name string) {
		_, num := wordsIn(c, name)
		byWidth[len(num)]++
		total++
	})
	for d := 1; d <= 3; d++ {
		if share := float64(byWidth[d]) / float64(total); share < 0.2 {
			t.Fatalf("%d-digit endings are %.2f of output, want a roughly even share", d, share)
		}
	}
}

func TestAllowPlainOptIn(t *testing.T) {
	c := Learn(seedNames(), true)
	foundPlain := false
	Generate(c, 50000, Options{Seed: 1}, func(name string) {
		if k, num := wordsIn(c, name); k == 1 && num != "" {
			foundPlain = true
		}
	})
	if !foundPlain {
		t.Fatal("allowPlain=true never produced a single name plus a number")
	}
}

func TestWordShapeMixFollowsInput(t *testing.T) {
	// Whichever word shapes the input favours, the output favours too:
	// here two-word names outnumber three-word ones nine to one, so most
	// output should be two words (before the trailing number).
	firsts := []string{"maria", "john", "kevin", "oliver", "sara", "tobias", "ines", "yusuf", "greta", "amir"}
	lasts := []string{"garcia", "smith", "lindqvist", "hollis", "carter", "thorne", "quigley", "ashby", "okafor", "winslow"}
	mids := []string{"rose", "lee", "jane", "max", "kai"}
	var names []string
	for i, f := range firsts {
		for j, l := range lasts {
			names = append(names, f+"_"+l)
			if (i+j)%10 == 0 {
				names = append(names, f+"_"+mids[j%len(mids)]+"_"+l)
			}
		}
	}
	c := Learn(names, false)
	var two, all float64
	Generate(c, 4000, Options{Seed: 9}, func(name string) {
		all++
		if k, _ := wordsIn(c, name); k == 2 {
			two++
		}
	})
	if got := two / all; got < 0.8 {
		t.Fatalf("%.2f of output is two words, want most of it like the input", got)
	}
}

func TestAmbiguousSplitsAreNotDuplicated(t *testing.T) {
	// "annamaria" can be built as anna+maria or as one pooled word, and
	// "rosaline"+"smith" as rosa+linesmith; each must still come out once.
	names := []string{
		"anna_maria", "anna.maria", "annamaria_lee", "annamaria.lee",
		"rosa_linesmith", "rosa.linesmith", "rosaline.smith", "rosaline_smith",
		"anna_smith", "anna.smith", "rosa_lee", "rosa.lee", "maria_lee", "maria.lee",
	}
	c := Learn(names, false)
	seen := map[string]bool{}
	Generate(c, c.Total(), Options{Seed: 4}, func(name string) {
		if seen[name] {
			t.Fatalf("duplicate output: %q", name)
		}
		seen[name] = true
	})
	if len(seen) == 0 {
		t.Fatal("generated nothing")
	}
}

func TestGenerateIsDuplicateFree(t *testing.T) {
	names := bigNames(20000)
	c := Learn(names, false)
	if c.Total() < 60000 {
		t.Fatalf("test corpus too small: %d combinations", c.Total())
	}
	const n = 20000
	seen := make(map[string]bool, n)
	input := map[string]bool{}
	for _, name := range names {
		input[strings.NewReplacer("_", "", ".", "", "-", "").Replace(strings.ToLower(name))] = true
	}
	count := Generate(c, n, Options{Seed: 42}, func(name string) {
		if seen[name] {
			t.Fatalf("duplicate output: %q", name)
		}
		seen[name] = true
		if input[strings.ToLower(name)] {
			t.Fatalf("reproduced an input username: %q", name)
		}
	})
	if count != n {
		t.Fatalf("got %d usernames, want %d (corpus has %d)", count, n, c.Total())
	}
}

func TestGenerateExhaustsSmallCorpus(t *testing.T) {
	c := Learn([]string{"john.smith", "john.carter", "john.hollis", "maria.smith", "maria.carter", "sara_hollis", "sara_smith", "sara_carter", "maria_hollis"}, false)
	total := c.Total()
	if total == 0 {
		t.Fatal("expected some combinations")
	}
	count := Generate(c, total*10, Options{Seed: 1}, func(string) {})
	if count > total {
		t.Fatalf("got %d usernames but corpus only has %d combinations", count, total)
	}
}

func TestGenerateIsRepeatableWithSameSeed(t *testing.T) {
	c := Learn(seedNames(), false)
	var a, b []string
	Generate(c, 5000, Options{Seed: 123}, func(n string) { a = append(a, n) })
	Generate(c, 5000, Options{Seed: 123}, func(n string) { b = append(b, n) })
	sortedJoin := func(s []string) string {
		cp := append([]string(nil), s...)
		for i := 0; i < len(cp); i++ {
			for j := i + 1; j < len(cp); j++ {
				if cp[j] < cp[i] {
					cp[i], cp[j] = cp[j], cp[i]
				}
			}
		}
		return strings.Join(cp, ",")
	}
	if sortedJoin(a) != sortedJoin(b) {
		t.Fatal("same seed produced a different set of usernames")
	}
}

func TestFeistelIsABijection(t *testing.T) {
	for _, total := range []uint64{1, 2, 10, 1000, 65537, 999983} {
		f := newFeistel(total, 7)
		seen := make([]bool, total)
		for i := uint64(0); i < total; i++ {
			p := f.permute(i)
			if p >= total {
				t.Fatalf("total=%d: permute(%d)=%d out of range", total, i, p)
			}
			if seen[p] {
				t.Fatalf("total=%d: permute collided at output %d", total, p)
			}
			seen[p] = true
		}
	}
}

func TestGarbageRejectsObviousJunk(t *testing.T) {
	bad := []string{"aaaaaaa", "qwertyuiop", "12345678", "admin", "hahahaha"}
	for _, name := range bad {
		if !Garbage(name) {
			t.Errorf("Garbage(%q) = false, want true", name)
		}
	}
	good := []string{"maria.garcia92", "tobiaslindqvist", "kevin_hollis"}
	for _, name := range good {
		if Garbage(name) {
			t.Errorf("Garbage(%q) = true, want false", name)
		}
	}
}

func TestLearnWithNoUsableInputReturnsEmptyCorpus(t *testing.T) {
	c := Learn([]string{"abc", "xyz"}, false) // only bare words; all filtered by default
	if c.Total() != 0 {
		t.Fatalf("expected an empty corpus, got total=%d", c.Total())
	}
	n := Generate(c, 100, Options{Seed: 1}, func(string) { t.Fatal("should not emit anything") })
	if n != 0 {
		t.Fatalf("got %d, want 0", n)
	}
}

// bigNames builds a varied list of tens of thousands of usernames with
// long words and mixed separators, so most raw combinations of the bigger
// patterns would be over MaxLen.
func bigNames(count int) []string {
	const cons, vow = "bcdfghjklmnprstvwz", "aeiou"
	x := uint64(99)
	next := func(n int) int { x = splitmix64(x); return int(x % uint64(n)) }
	word := func(lo, hi int) string {
		n := lo + next(hi-lo+1)
		b := make([]byte, 0, n+1)
		for len(b) < n {
			b = append(b, cons[next(len(cons))], vow[next(len(vow))])
		}
		return string(b[:n])
	}
	seps := []string{"_", ".", "-"}
	out := make([]string, 0, count)
	for len(out) < count {
		f, m, l := word(3, 8), word(3, 7), word(4, 10)
		num := strconv.Itoa(next(3000))
		sep := seps[next(3)]
		switch next(6) {
		case 0:
			out = append(out, f+sep+l)
		case 1:
			out = append(out, f+sep+l+num)
		case 2:
			out = append(out, f+sep+m+sep+l+sep+num)
		case 3:
			out = append(out, strings.ToUpper(f[:1])+f[1:]+strings.ToUpper(m[:1])+m[1:]+strings.ToUpper(l[:1])+l[1:]+num)
		case 4:
			out = append(out, f+sep+l+sep+num)
		default:
			out = append(out, f+m+sep+l)
		}
	}
	return out
}

func TestEveryCombinationFitsLengthLimits(t *testing.T) {
	c := Learn(bigNames(20000), false)
	if c.Total() == 0 {
		t.Fatal("expected combinations")
	}
	for ti, sp := range c.spaces {
		f := newFeistel(sp.total, 5)
		for i := uint64(0); i < 50000 && i < sp.total; i++ {
			name, ok := c.decode(ti, f.permute(i))
			if !ok {
				continue
			}
			if n := len([]rune(name)); n < MinLen || n > MaxLen {
				t.Fatalf("decoded %q has %d characters, want %d..%d", name, n, MinLen, MaxLen)
			}
		}
	}
}

func TestBigInputGeneratesQuickly(t *testing.T) {
	c := Learn(bigNames(100000), false)
	if c.Total() == 0 || c.Total() > maxTotal {
		t.Fatalf("total %d out of range", c.Total())
	}
	const n = 200000
	var tried uint64
	seen := make(map[string]bool, n)
	got := Generate(c, n, Options{Seed: 3, Progress: func(_, raw uint64) { tried = raw }}, func(name string) {
		if seen[name] {
			t.Fatalf("duplicate output: %q", name)
		}
		seen[name] = true
	})
	if got != n {
		t.Fatalf("got %d usernames, want %d", got, n)
	}
	// Nearly every raw combination should now be usable; before length
	// cells, almost all of them were too long and thrown away.
	if tried > 4*n {
		t.Fatalf("tried %d raw combinations for %d usernames; too many rejected", tried, n)
	}
}

func TestHugePoolsDoNotOverflow(t *testing.T) {
	big := make([]string, 1<<20)
	for i := range big {
		big[i] = "abc" // only the pool sizes matter here: 3 slots of 2^20 and 10^4 numbers is past 2^62
	}
	p := &Pools{numbers: map[int][]string{4: make([]string, 10000)}}
	p.leadsByLen, p.tailsByLen = byLength(big), byLength(big)
	p.leadBuckets, p.tailBuckets = bucketsOf(p.leadsByLen), bucketsOf(p.tailsByLen)
	shape := []Shape{{Kind: Word, Lead: true}, {Kind: Word}, {Kind: Word}, {Kind: Number, Width: 4}}
	cells, total := cellsOf(shape, p)
	if total > maxTotal {
		t.Fatalf("total %d exceeds cap", total)
	}
	var sum uint64
	for _, cl := range cells {
		sum += cl.total
	}
	if sum != total {
		t.Fatalf("cells sum to %d, total says %d", sum, total)
	}
}

func TestPossibleMatchesWhatGenerateWrites(t *testing.T) {
	// Small enough to be counted exactly: Possible must equal how many
	// usernames Generate can actually write before running dry.
	c := Learn(seedNames(), false)
	if c.Total() > estimateSample*uint64(len(c.spaces)) {
		t.Skip("corpus too big to be exact")
	}
	got := Generate(c, c.Total()+1, Options{Seed: 5}, func(string) {})
	if got != c.Possible() {
		t.Fatalf("Possible() = %d, Generate wrote %d", c.Possible(), got)
	}
}

func TestPossibleEstimatesBigCorpus(t *testing.T) {
	c := Learn(bigNames(20000), false)
	if p, total := c.Possible(), c.Total(); p == 0 || p > total {
		t.Fatalf("Possible() = %d, want between 1 and Total() = %d", p, total)
	}
}

func TestLearnDigitsFixedWidth(t *testing.T) {
	for _, d := range []int{1, 2, 3} {
		c := LearnDigits(seedNames(), false, d, d)
		re := regexp.MustCompile(`^[a-z]+[0-9]{` + strconv.Itoa(d) + `}$`)
		n := Generate(c, 10000, Options{Seed: 1}, func(name string) {
			if !re.MatchString(name) {
				t.Fatalf("digits=%d produced %q, want names then exactly %d digits", d, name, d)
			}
		})
		if n == 0 {
			t.Fatalf("digits=%d generated nothing", d)
		}
	}
}

func TestLearnDigitsNone(t *testing.T) {
	c := LearnDigits(bigNames(5000), false, 0, 0)
	re := regexp.MustCompile(`^[a-z]+$`)
	n := Generate(c, 10000, Options{Seed: 1}, func(name string) {
		if !re.MatchString(name) {
			t.Fatalf("digits=0 produced %q, want whole names with no number", name)
		}
	})
	if n == 0 {
		t.Fatal("digits=0 generated nothing")
	}
}
