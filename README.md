# usermix

Give it a file of usernames. It mixes the parts of them together and hands
you new ones — millions to billions of them, straight from your own list,
with nothing built in.

```sh
go build -o usermix .
./usermix
```

```
Usernames file: mine.txt
Learned from 912,153 usernames; 161T possible from them.
How many usernames to generate: 1000000000
Generating [##############################] 100% (1,000,000,000/1,000,000,000)
Wrote 1,000,000,000 usernames to generated_1000000000.txt in 18m.
```

That's the whole interface: a file, a count, a progress bar, and a numbered
output file it never overwrites. Flags and subcommands below are there for
scripting; the plain command above is all you need otherwise.

Prefer a window to a terminal? Run `./usermix gui`. It opens a page in your
browser with a box for the usernames file, a box for how many to make, a
Generate button, an optional email-domain box and a progress bar, and
writes to `generated_<count>.txt` the same way.

Every username is 8 to 15 characters: two or three whole names run
together with no connector (`_ - .`), then optionally one number at the
very end, like `kevinsmith45` or `mariagarcia`. Never a number in the
middle, never a connector. The "possible" count is how many it can
actually write after every filter below, shown short: `950`, `1.4M`,
`22B`, `161T`.

Want email addresses instead of bare handles? Give it a domain and every
line comes out as `kevinsmith45@hotmail.com`: type one in the window's
email box, answer the interactive prompt, or pass `-domain hotmail.com` to
`generate`.

## What it does

Most of `mine.txt`'s usernames are built the same handful of ways: two
names; two names and a 2-digit number; two names and a year. usermix
splits every username into its names and its trailing number (dropping
connectors, so `kevin_smith` is learned as `kevin` + `smith`), pools every
name by the role it played — names that started a username, names that
came later — and every ending number, and reassembles them.

What keeps the output looking real:

- **Patterns in the same mix as your list.** If half your list is two
  names with nothing else and a quarter is two names and a 2-digit number,
  so is the output. A pattern doesn't win just because it can be built
  more ways.
- **Only names your list really uses.** A word has to show up at least 3
  times in your list to be pooled (20 times for a 3-letter one), so
  one-off mashes and scraps left over from splitting (`eh`, `jasm`) never
  get in.
- **Only numbers your list really ends with.** For each length, the most
  common endings that cover half of your list's endings of that length:
  years, `07`, `123`, `786` and the like, not every possible 4-digit
  value.

```
mine.txt:        maria.garcia92, tobiaslindqvist, kevin_hollis, sara.ashby88
generated.txt:   marialindqvist, kevingarcia92, tobiashollis, saragarcia88, ...
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
- a single name followed by a number (`john2005`, `oliver4006`). Pass
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
./usermix gui                       # open a simple window in your browser
```

| Flag | Meaning |
|------|---------|
| `-n` | how many usernames to generate (default 1,000,000) |
| `-seed` | fixed seed for repeatable output (0 = random) |
| `-allow-plain` | allow single-name-plus-number usernames like `john2005` |
| `-workers` | CPU cores to use (0 = all) |
| `-domain` | append `@DOMAIN` to every username, e.g. `-domain hotmail.com` |
| `-out` | file to write to (default: stdout) |

If a pattern's combinations run out before `-n` is reached, usermix says
so and writes however many it found; give it a bigger or more varied input
file for a larger pool.

## Performance

On a 4-core machine, from a 900,000-username input: about 900,000
usernames per second, so a billion take around 18 minutes. Throughput scales with CPU cores and drops a little as
`-n` approaches how many combinations the input can actually produce
(most of the last few are found by skipping ones the garbage filter or an
exhausted block rejects).

## Tests

```sh
go test ./...
go test -race ./...
```
