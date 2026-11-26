# usermix

Give it a file of usernames. It mixes the parts of them together and hands
you new ones — millions to billions of them, straight from your own list,
with nothing built in.

```sh
go build -o usermix .
./usermix generate -n 1000000000 mine.txt > generated.txt
```

```
$ ./usermix info mine.txt
Usernames read     1,000,000
Patterns found     4
Usernames possible 476,091,132,000
```

## What it does

Most of `mine.txt`'s usernames are built the same handful of ways: a name,
a separator, a surname, a number; a name alone with a number; two names run
together. usermix finds those shapes, then pools every word, number and
separator it saw across the *whole* file by the role it played — words
that started a name, words that came later, which separators were used,
which numbers — and reassembles them in every combination the shapes
allow.

A few hundred first names and a few hundred surnames already give
thousands times thousands of combinations; a real list of a million
usernames gives you many more words to mix, and the combinations run into
the billions.

```
mine.txt:        maria.garcia92, tobiaslindqvist, kevin_hollis, dariwex07, sara.ashby88
generated.txt:   maria_lindqvist, kevin.garcia41, tobiaslhollis77, saradariwex, ...
```

Nothing here is a built-in name list. If you give it usernames in Spanish,
Arabic script, gamer tags, or anything else, it learns and mixes those —
only ever the shapes and words actually in your file.

## Why it reaches billions where editing one name at a time can't

Editing each username into a handful of variants tops out at
`usernames × edits per username`. Here, the output size is
`word pool size × word pool size × number pool size × ...` — a product,
not a sum — so a modest-sized vocabulary already multiplies out past a
billion.

Getting there safely, without storing billions of usernames in memory or
in a hash set to check for duplicates, needs a different trick:

- Every combination is numbered, 0 up to however many that pattern allows.
- Handing out combination 0, 1, 2, 3, ... in that order would be
  repetitive and dull. Instead, each number is run through a
  **[format-preserving][fpe] shuffle** (a small Feistel network with
  cycle-walking — the standard technique behind format-preserving
  encryption) before being turned into a username. That shuffle is a
  one-to-one mapping from numbers to numbers: every input number reaches
  exactly one output number, and every output number is reached by
  exactly one input. Reading them out in order 0, 1, 2, ... therefore
  visits every combination exactly once, in an order that looks random,
  using a few arithmetic operations per combination and no memory at all
  for what has already been handed out.
- That makes generation embarrassingly parallel: splitting the counter
  space into blocks and handing blocks to every CPU core needs no
  coordination beyond which block comes next.

[fpe]: https://en.wikipedia.org/wiki/Format-preserving_encryption

One consequence worth knowing: because every combination keeps its own
fixed number regardless of how many you ask for, asking for the same
`-seed` and more usernames always extends the previous list — it never
reshuffles what you already had, and running the same command twice gives
the exact same output.

## No plain names

usermix never produces:

- a bare name on its own (`oliver`) — almost certainly already somebody's
  real account;
- a name (or two names run together) followed by a number with nothing
  else — `john2005`, `oliver4006`, and just as much `johnthorne1195`, which
  reads exactly the same way even though it was built from two names. Pass
  `-allow-plain` to allow this shape back.

It also runs every candidate through a garbage filter before writing it:
no keyboard mashes (`qwertyuiop`), no repeated characters (`hahaha`,
`zzzz`), no placeholder names (`admin`, `test`, `deleted`), nothing that's
mostly digits, nothing unpronounceable, no 5-or-more digits in a row.

## Splitting joined names

Plenty of real usernames run two words together with no separator at all
(`mariagarcia92`). usermix learns to split those too — but only using
words it has already seen written separately elsewhere in your file
(`maria.smith`, `john_garcia` teach it `maria` and `garcia`), never a
guess and never a built-in dictionary. A word that's only ever touched a
bare number (`maya` in `maya2005`) doesn't count as proof either, since
that shape also covers a fully joined compound (`mayagentry` in
`mayagentry55`) — so only an explicit separator or a change of case
teaches a word.

## Commands

```sh
./usermix generate [flags] FILE     # FILE, or - for stdin; writes to stdout
./usermix info     [flags] FILE     # show what usermix learned, generate nothing
```

| Flag | Meaning |
|------|---------|
| `-n` | how many usernames to generate (default 1,000,000) |
| `-seed` | fixed seed for repeatable output (0 = random) |
| `-allow-plain` | allow bare name-plus-number usernames like `john2005` |
| `-workers` | CPU cores to use (0 = all) |
| `-out` | file to write to (default: stdout) |

If a pattern's combinations run out before `-n` is reached, usermix says
so and writes however many it found; give it a bigger or more varied input
file for a larger pool.

## Performance

On a 4-core machine, from a million-username input with a few thousand
distinct words: about 1.3 million usernames per second, so a billion take
around 8 minutes. Throughput scales with CPU cores and drops a little as
`-n` approaches how many combinations the input can actually produce
(most of the last few are found by skipping ones the garbage filter or an
exhausted block rejects).

## Tests

```sh
go test ./...
go test -race ./...
```
