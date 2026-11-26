package mix

import (
	"fmt"
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

func TestNoPlainNames(t *testing.T) {
	// john2005, oliver4006, johnthorne1195 (joined with no separator, so
	// it reads the same way) and a bare "oliver" must never be produced.
	// Checking this needs the same vocabulary Learn used, so that a
	// legitimate two-name join like "mariathorne" (no number, so it
	// doesn't have the unwanted look) isn't mistaken for a bare word.
	names := seedNames()
	v := buildVocab(names)
	c := Learn(names, false)
	seen := map[string]bool{}
	n := Generate(c, 200000, Options{Seed: 1}, func(name string) {
		segs := v.refine(Split(name))
		if len(segs) == 1 {
			t.Fatalf("produced a bare name: %q", name)
		}
		if Plain(segs) {
			t.Fatalf("produced a plain name-plus-number: %q", name)
		}
		seen[name] = true
	})
	if n == 0 {
		t.Fatal("generated nothing")
	}
}

func TestAllowPlainOptIn(t *testing.T) {
	c := Learn(seedNames(), true)
	foundPlain := false
	Generate(c, 50000, Options{Seed: 1}, func(name string) {
		if Plain(Split(name)) {
			foundPlain = true
		}
	})
	if !foundPlain {
		t.Fatal("allowPlain=true never produced a plain name-plus-number")
	}
}

func TestGenerateIsDuplicateFree(t *testing.T) {
	c := Learn(seedNames(), false)
	if c.Total() < 100000 {
		t.Fatalf("test corpus too small: %d combinations", c.Total())
	}
	const n = 100000
	seen := make(map[string]bool, n)
	input := map[string]bool{}
	for _, name := range seedNames() {
		input[strings.ToLower(name)] = true
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
	c := Learn([]string{"john.smith", "john.carter", "amy.smith"}, false)
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
