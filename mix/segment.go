// Package mix generates new usernames by recombining the parts of the ones
// you give it, rather than editing each one individually. It splits every
// input username into words, numbers and separators, pools those parts
// together, and reassembles them in every combination its structural
// patterns allow — which reaches into the billions from a seed list of only
// a few hundred names. See README.md for the full design.
package mix

import (
	"strings"
	"unicode"
)

// Kind classifies a Segment.
type Kind int

const (
	Word Kind = iota
	Number
	Separator
)

// Segment is one part of a username.
type Segment struct {
	Text string
	Kind Kind
}

// MinLen and MaxLen bound a username's length in characters, matching what
// most sites accept.
const (
	MinLen = 8
	MaxLen = 15
)

func kindOf(r rune) Kind {
	switch {
	case unicode.IsDigit(r):
		return Number
	case r == '_' || r == '-' || r == '.':
		return Separator
	default:
		return Word
	}
}

// Split breaks name into words, numbers and separators, splitting words at
// case changes: "DarkWolf_99" → Dark, Wolf, _, 99.
func Split(name string) []Segment {
	runes := []rune(name)
	if len(runes) == 0 {
		return nil
	}
	var segs []Segment
	start := 0
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || boundary(runes, i) {
			segs = append(segs, Segment{string(runes[start:i]), kindOf(runes[start])})
			start = i
		}
	}
	return segs
}

func boundary(runes []rune, i int) bool {
	prev, cur := runes[i-1], runes[i]
	if kindOf(prev) != kindOf(cur) {
		return true
	}
	if kindOf(cur) != Word {
		return false
	}
	if unicode.IsLower(prev) && unicode.IsUpper(cur) {
		return true // camelCase: DarkWolf -> Dark, Wolf
	}
	// Acronym then word: XMLParser -> XML, Parser.
	return unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
}

// Shape describes one slot in a structural pattern: what kind of segment it
// holds; for a Word, whether it is the name's first word (its role affects
// which pool it draws from, so "maria" leads and "garcia" follows); and for
// a Number, how many digits it has, so a pattern ending in a 2-digit number
// is weighted by how often real names end that way, separately from one
// ending in a year.
type Shape struct {
	Kind  Kind
	Lead  bool // only meaningful when Kind == Word
	Width int  // only meaningful when Kind == Number
}

// shapeOf converts segs to the pattern Pool and Pattern use to group
// usernames and pick pools.
func shapeOf(segs []Segment) []Shape {
	shapes := make([]Shape, len(segs))
	lead := true
	for i, seg := range segs {
		shapes[i] = Shape{Kind: seg.Kind, Lead: seg.Kind == Word && lead}
		if seg.Kind == Number {
			shapes[i].Width = len(seg.Text)
		}
		if seg.Kind == Word {
			lead = false
		}
	}
	return shapes
}

// clean turns a name's refined segments into what usermix learns from:
// connectors dropped (usernames are made without them, so kevin_smith is
// learned as kevin, smith) and words lowercased.
func clean(segs []Segment) []Segment {
	out := make([]Segment, 0, len(segs))
	for _, seg := range segs {
		switch seg.Kind {
		case Separator:
			continue
		case Word:
			seg.Text = strings.ToLower(seg.Text)
		}
		out = append(out, seg)
	}
	return out
}

// wordLike reports whether w is made only of letters and has a vowel, so
// stray symbols (%40, +) and consonant scraps never become pool words.
func wordLike(w string) bool {
	vowel := false
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
		if r >= 128 || strings.ContainsRune("aeiouy", r) {
			vowel = true
		}
	}
	return vowel
}
