// Command usermix generates new usernames by recombining the parts of the
// ones you give it, at a scale of billions, rather than editing each one
// individually. See README.md for how it works.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/anasopking/usermix/mix"
)

const usage = `usermix recombines the usernames you give it into billions of new ones.

Usage:
  usermix generate [flags] FILE   read usernames from FILE ("-" for stdin),
                                   write new ones to stdout
  usermix info     [flags] FILE   show what usermix learned from FILE,
                                   without generating anything
  usermix gui                    open a simple window in your browser that
                                   asks for the file and how many to make

Run "usermix <command> -h" for a command's flags.
`

func main() {
	if len(os.Args) < 2 {
		if err := interactive(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "usermix:", err)
			os.Exit(1)
		}
		return
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "generate", "gen":
		err = generate(args)
	case "info":
		err = info(args)
	case "gui":
		err = gui(args)
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "usermix:", err)
		os.Exit(1)
	}
}

// interactive runs usermix with no setup: it asks for a file of usernames
// and how many to generate, then writes generated_<count>.txt while
// showing progress. Nothing else: no menu, no settings.
func interactive(in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)

	var names []string
	for {
		path, err := ask(r, out, "Usernames file: ")
		if err != nil {
			return err
		}
		names, err = readNames(path)
		if err != nil {
			fmt.Fprintf(out, "Can't read that: %s\n", err)
			continue
		}
		if len(names) == 0 {
			fmt.Fprintf(out, "%s has no usernames in it.\n", path)
			continue
		}
		break
	}

	var lo, hi int
	for {
		s, err := ask(r, out, "Digits to add at the end (0 none, 1, 2 or 3, blank for random 1-3): ")
		if err != nil {
			return err
		}
		l, h, ok := parseDigits(s)
		if !ok {
			fmt.Fprintln(out, "Enter 0, 1, 2, 3, or leave it blank.")
			continue
		}
		lo, hi = l, h
		break
	}

	corpus := mix.LearnDigits(names, false, lo, hi)
	if corpus.Possible() == 0 {
		return errors.New("found no usable patterns in that file; try one with more usernames")
	}
	fmt.Fprintf(out, "Learned from %s usernames; %s possible from them.\n",
		formatCount(len(names)), formatShort(corpus.Possible()))

	var n uint64
	for {
		s, err := ask(r, out, "How many usernames to generate: ")
		if err != nil {
			return err
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil || v == 0 {
			fmt.Fprintln(out, "Enter a whole number above 0.")
			continue
		}
		n = v
		break
	}

	domain, err := ask(r, out, "Email domain to add (blank for none): ")
	if err != nil {
		return err
	}
	suffix := emailSuffix(domain)

	tmp, err := os.CreateTemp(".", ".usermix-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	bw := bufio.NewWriterSize(tmp, 1<<20)

	bar := newProgressBar(out, n)
	start := time.Now()
	written := mix.Generate(corpus, n, mix.Options{
		Progress: func(found, _ uint64) { bar.draw(found) },
	}, func(name string) {
		bw.WriteString(name)
		bw.WriteString(suffix)
		bw.WriteByte('\n')
	})
	bar.finish(written)
	if err := bw.Flush(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	path, err := placeNumbered(tmp.Name(), "generated", written)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s usernames to %s in %s.\n",
		formatCount(written), path, time.Since(start).Round(100*time.Millisecond))
	if written < n {
		fmt.Fprintln(out, "That's fewer than asked for; give usermix more or more varied usernames for a bigger pool.")
	}
	return nil
}

// digitRange turns a digits choice into the loDigits..hiDigits range for
// mix.LearnDigits: -1 means a random 1-3 digit number (the default), 0 means
// no trailing number, and 1, 2 or 3 means exactly that many digits. ok is
// false for anything else.
func digitRange(d int) (lo, hi int, ok bool) {
	switch {
	case d < 0:
		return 1, 3, true
	case d == 0:
		return 0, 0, true
	case d <= 3:
		return d, d, true
	}
	return 0, 0, false
}

// parseDigits reads a user's digits answer: a blank one keeps the random 1-3
// default, otherwise it must be 0, 1, 2 or 3.
func parseDigits(s string) (lo, hi int, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return digitRange(-1)
	}
	d, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, false
	}
	return digitRange(d)
}

// emailSuffix turns a user-typed domain into the "@domain" that gets
// appended to every username, or "" for none. A leading "@" and any
// surrounding whitespace are trimmed, so "hotmail.com" and "@hotmail.com"
// both work.
func emailSuffix(domain string) string {
	d := strings.TrimPrefix(strings.TrimSpace(domain), "@")
	if d == "" {
		return ""
	}
	return "@" + d
}

func ask(r *bufio.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// placeNumbered gives tmp its final name, <prefix>_<count>.txt, never
// overwriting: if that name exists it tries <prefix>_<count>_2.txt, and so
// on.
func placeNumbered(tmp, prefix string, count uint64) (string, error) {
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s_%d.txt", prefix, count)
		if i > 1 {
			name = fmt.Sprintf("%s_%d_%d.txt", prefix, count, i)
		}
		if _, err := os.Lstat(name); errors.Is(err, os.ErrNotExist) {
			return name, os.Rename(tmp, name)
		} else if err != nil {
			return "", err
		}
	}
}

func generate(args []string) error {
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	n := flags.Uint64("n", 1_000_000, "how many usernames to generate")
	seed := flags.Uint64("seed", 0, "random seed for repeatable output (0 = random)")
	allowPlain := flags.Bool("allow-plain", false, "allow single-name-plus-number usernames like john2005 or oliver4006 (almost always already taken)")
	workers := flags.Int("workers", 0, "CPU cores to use (0 = all)")
	domain := flags.String("domain", "", "append @DOMAIN to every username, e.g. -domain hotmail.com")
	digits := flags.Int("digits", -1, "digits to add at the end: 0 none, 1, 2 or 3, or -1 for a random 1-3")
	out := flags.String("out", "", "file to write to (default: stdout)")
	flags.Parse(args)
	if flags.NArg() != 1 {
		return errors.New("generate needs exactly one file of usernames (or - for stdin)")
	}
	lo, hi, ok := digitRange(*digits)
	if !ok {
		return errors.New("-digits must be 0, 1, 2, 3, or -1 for a random 1-3")
	}

	names, err := readNames(flags.Arg(0))
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s has no usernames", flags.Arg(0))
	}
	corpus := mix.LearnDigits(names, *allowPlain, lo, hi)
	if corpus.Possible() == 0 {
		return errors.New("found no usable patterns; give usermix more usernames, or try -allow-plain")
	}

	w, closeW, err := openOutput(*out)
	if err != nil {
		return err
	}
	defer closeW()
	bw := bufio.NewWriterSize(w, 1<<20)

	suffix := emailSuffix(*domain)
	start := time.Now()
	written := mix.Generate(corpus, *n, mix.Options{Seed: *seed, Workers: *workers}, func(name string) {
		bw.WriteString(name)
		bw.WriteString(suffix)
		bw.WriteByte('\n')
	})
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "usermix: wrote %s usernames in %s (about %s possible from this input)\n",
		formatCount(written), time.Since(start).Round(100*time.Millisecond), formatShort(corpus.Possible()))
	if written < *n {
		fmt.Fprintln(os.Stderr, "usermix: fewer than requested; give usermix more or more varied usernames for a bigger pool")
	}
	return nil
}

func info(args []string) error {
	flags := flag.NewFlagSet("info", flag.ExitOnError)
	allowPlain := flags.Bool("allow-plain", false, "count single-name-plus-number patterns like john2005")
	digits := flags.Int("digits", -1, "digits to add at the end: 0 none, 1, 2 or 3, or -1 for a random 1-3")
	flags.Parse(args)
	if flags.NArg() != 1 {
		return errors.New("info needs exactly one file of usernames (or - for stdin)")
	}
	lo, hi, ok := digitRange(*digits)
	if !ok {
		return errors.New("-digits must be 0, 1, 2, 3, or -1 for a random 1-3")
	}
	names, err := readNames(flags.Arg(0))
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s has no usernames", flags.Arg(0))
	}
	corpus := mix.LearnDigits(names, *allowPlain, lo, hi)
	fmt.Printf("Usernames read     %s\n", formatCount(len(names)))
	fmt.Printf("Patterns found     %s\n", formatCount(len(corpus.Templates())))
	fmt.Printf("Usernames possible %s\n", formatShort(corpus.Possible()))
	return nil
}

func readNames(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	var names []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			names = append(names, line)
		}
	}
	return names, sc.Err()
}

func openOutput(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// formatShort writes n the short way, rounded: 950, 12K, 1.4M, 22B, 3.1T.
func formatShort(n uint64) string {
	units := []struct {
		size   float64
		suffix string
	}{{1e15, "Q"}, {1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}}
	v := float64(n)
	for _, u := range units {
		if v >= u.size {
			x := v / u.size
			if x < 10 {
				return strings.TrimSuffix(fmt.Sprintf("%.1f", x), ".0") + u.suffix
			}
			return fmt.Sprintf("%.0f", x) + u.suffix
		}
	}
	return strconv.FormatUint(n, 10)
}

// formatCount writes n with thousands separators.
func formatCount[T int | uint64](n T) string {
	s := strconv.FormatUint(uint64(n), 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
