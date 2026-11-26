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
	"time"

	"github.com/anasopking/usermix/mix"
)

const usage = `usermix recombines the usernames you give it into billions of new ones.

Usage:
  usermix generate [flags] FILE   read usernames from FILE ("-" for stdin),
                                   write new ones to stdout
  usermix info     [flags] FILE   show what usermix learned from FILE,
                                   without generating anything

Run "usermix <command> -h" for a command's flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "generate", "gen":
		err = generate(args)
	case "info":
		err = info(args)
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

func generate(args []string) error {
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	n := flags.Uint64("n", 1_000_000, "how many usernames to generate")
	seed := flags.Uint64("seed", 0, "random seed for repeatable output (0 = random)")
	allowPlain := flags.Bool("allow-plain", false, "allow bare name-plus-number usernames like john2005 or oliver4006 (almost always already taken)")
	workers := flags.Int("workers", 0, "CPU cores to use (0 = all)")
	out := flags.String("out", "", "file to write to (default: stdout)")
	flags.Parse(args)
	if flags.NArg() != 1 {
		return errors.New("generate needs exactly one file of usernames (or - for stdin)")
	}

	names, err := readNames(flags.Arg(0))
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s has no usernames", flags.Arg(0))
	}
	corpus := mix.Learn(names, *allowPlain)
	if corpus.Total() == 0 {
		return errors.New("found no usable patterns; give usermix more usernames, or try -allow-plain")
	}

	w, closeW, err := openOutput(*out)
	if err != nil {
		return err
	}
	defer closeW()
	bw := bufio.NewWriterSize(w, 1<<20)

	start := time.Now()
	written := mix.Generate(corpus, *n, mix.Options{Seed: *seed, Workers: *workers}, func(name string) {
		bw.WriteString(name)
		bw.WriteByte('\n')
	})
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "usermix: wrote %s usernames in %s (%s available from this input)\n",
		formatCount(written), time.Since(start).Round(100*time.Millisecond), formatCount(corpus.Total()))
	if written < *n {
		fmt.Fprintln(os.Stderr, "usermix: fewer than requested; give usermix more or more varied usernames for a bigger pool")
	}
	return nil
}

func info(args []string) error {
	flags := flag.NewFlagSet("info", flag.ExitOnError)
	allowPlain := flags.Bool("allow-plain", false, "count bare name-plus-number patterns like john2005")
	flags.Parse(args)
	if flags.NArg() != 1 {
		return errors.New("info needs exactly one file of usernames (or - for stdin)")
	}
	names, err := readNames(flags.Arg(0))
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s has no usernames", flags.Arg(0))
	}
	corpus := mix.Learn(names, *allowPlain)
	fmt.Printf("Usernames read     %s\n", formatCount(len(names)))
	fmt.Printf("Patterns found     %s\n", formatCount(len(corpus.Templates())))
	fmt.Printf("Usernames possible %s\n", formatCount(corpus.Total()))
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

// formatCount writes n with thousands separators.
func formatCount[T int | uint64](n T) string {
	s := strconv.FormatInt(int64(n), 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
