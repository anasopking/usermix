package mix

// feistel is a format-preserving shuffle: it maps the integers
// [0, total) to themselves, one-to-one, in an order that looks random, so
// that handing out combination 0, 1, 2, ... in feistel order gives
// non-repeating, scattered picks from a space of billions without ever
// storing which ones have been given out. It is not cryptographic — it only
// needs to look shuffled, not resist an attacker — so a few rounds of a
// simple mixing function are enough.
//
// The standard trick (format-preserving encryption) is a Feistel network
// sized to the smallest even number of bits that covers total, with cycle
// walking: encrypt, and if the result lands outside [0, total), encrypt
// again. Because the network is a bijection on the full [0, 2^bits) range,
// repeatedly encrypting and rejecting out-of-range results is itself a
// bijection on the sub-range [0, total).
type feistel struct {
	seed     uint64
	halfBits uint
	half     uint64 // (1 << halfBits) - 1
	total    uint64
}

const feistelRounds = 4

func newFeistel(total, seed uint64) *feistel {
	bits := bitLen(total)
	if bits < 2 {
		bits = 2
	}
	if bits%2 == 1 {
		bits++
	}
	half := bits / 2
	return &feistel{seed: seed, halfBits: half, half: (uint64(1) << half) - 1, total: total}
}

func bitLen(n uint64) uint {
	var b uint
	for v := n; v > 0; v >>= 1 {
		b++
	}
	return b
}

// round is the Feistel round function: a keyed mix of the right half,
// distinct per round, truncated to halfBits.
func (f *feistel) round(r int, x uint64) uint64 {
	h := x ^ f.seed ^ uint64(r)*0x9E3779B97F4A7C15
	return splitmix64(h) & f.half
}

func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// encrypt runs the Feistel network once, a bijection on [0, 2^bits).
func (f *feistel) encrypt(x uint64) uint64 {
	l, r := x>>f.halfBits, x&f.half
	for i := 0; i < feistelRounds; i++ {
		l, r = r, (l^f.round(i, r))&f.half
	}
	return l<<f.halfBits | r
}

// permute returns the shuffled position of x, cycle-walking back into
// [0, total) whenever encrypt lands outside it.
func (f *feistel) permute(x uint64) uint64 {
	for {
		x = f.encrypt(x)
		if x < f.total {
			return x
		}
	}
}
