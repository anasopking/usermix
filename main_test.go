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
