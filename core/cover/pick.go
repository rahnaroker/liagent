package cover

import "math/rand"

// PickRandom returns the index of a random template. If avoid is a valid index
// and there is more than one template, the result differs from avoid (so the
// "another template" button always changes the background). Returns -1 for an
// empty list.
func PickRandom(n, avoid int) int {
	switch {
	case n <= 0:
		return -1
	case n == 1:
		return 0
	}
	for {
		i := rand.Intn(n)
		if i != avoid {
			return i
		}
	}
}
