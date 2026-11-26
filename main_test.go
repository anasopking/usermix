package main

import "testing"

func TestFormatShort(t *testing.T) {
	cases := map[uint64]string{
		950: "950", 1000: "1K", 12_345: "12K", 1_000_000: "1M", 1_450_000: "1.4M",
		1_000_000_000: "1B", 22_300_000_000: "22B", 161_000_000_000_000: "161T",
	}
	for n, want := range cases {
		if got := formatShort(n); got != want {
			t.Errorf("formatShort(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestEmailSuffix(t *testing.T) {
	cases := map[string]string{
		"hotmail.com":   "@hotmail.com",
		"@hotmail.com":  "@hotmail.com",
		"  gmail.com  ": "@gmail.com",
		"":              "",
		"   ":           "",
		"@":             "",
	}
	for in, want := range cases {
		if got := emailSuffix(in); got != want {
			t.Errorf("emailSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDigitRange(t *testing.T) {
	cases := []struct {
		in     int
		lo, hi int
		ok     bool
	}{
		{-1, 1, 3, true}, {0, 0, 0, true}, {1, 1, 1, true},
		{2, 2, 2, true}, {3, 3, 3, true}, {4, 0, 0, false}, {-2, 1, 3, true},
	}
	for _, c := range cases {
		lo, hi, ok := digitRange(c.in)
		if ok != c.ok || (ok && (lo != c.lo || hi != c.hi)) {
			t.Errorf("digitRange(%d) = %d,%d,%v, want %d,%d,%v", c.in, lo, hi, ok, c.lo, c.hi, c.ok)
		}
	}
}

func TestParseDigits(t *testing.T) {
	if lo, hi, ok := parseDigits("  "); !ok || lo != 1 || hi != 3 {
		t.Errorf("parseDigits(blank) = %d,%d,%v, want 1,3,true", lo, hi, ok)
	}
	if lo, hi, ok := parseDigits("0"); !ok || lo != 0 || hi != 0 {
		t.Errorf("parseDigits(0) = %d,%d,%v, want 0,0,true", lo, hi, ok)
	}
	if _, _, ok := parseDigits("9"); ok {
		t.Error("parseDigits(9) should be invalid")
	}
	if _, _, ok := parseDigits("x"); ok {
		t.Error("parseDigits(x) should be invalid")
	}
}
