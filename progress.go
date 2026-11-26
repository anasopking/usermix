package main

import (
	"fmt"
	"io"
)

const barWidth = 30

// progressBar draws a bar that fills as usernames are found, redrawing in
// place on every update.
type progressBar struct {
	out   io.Writer
	total uint64
}

func newProgressBar(out io.Writer, total uint64) *progressBar {
	b := &progressBar{out: out, total: total}
	b.draw(0)
	return b
}

func (b *progressBar) draw(found uint64) {
	frac := float64(found) / float64(b.total)
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * barWidth)
	fmt.Fprintf(b.out, "\rGenerating [%s%s] %3d%% (%s/%s)",
		repeat("#", filled), repeat(".", barWidth-filled), int(frac*100),
		formatCount(found), formatCount(b.total))
}

// finish draws the bar one last time at found (which may be less than
// total, if the corpus ran out first) and moves to a new line.
func (b *progressBar) finish(found uint64) {
	b.draw(found)
	fmt.Fprintln(b.out)
}

func repeat(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for range n {
		out = append(out, s...)
	}
	return string(out)
}
