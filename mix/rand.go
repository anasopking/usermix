package mix

import "math/rand/v2"

// randomSeed returns a non-zero random seed (0 is reserved to mean "pick
// one").
func randomSeed() uint64 {
	for {
		if s := rand.Uint64(); s != 0 {
			return s
		}
	}
}
