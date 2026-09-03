package pidmap

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collidingKeys returns n distinct keys sharing one inline-index bucket.
func collidingKeys(n int) (keys []uint16) {
	target := hash(0x100, 32-6)
	for k := 0; k < 1<<13 && len(keys) < n; k++ {
		if hash(uint16(k), 32-6) == target {
			keys = append(keys, uint16(k))
		}
	}
	return
}

// The index must answer through a probe chain, across the move to the heap
// index, and after positions shift on Remove — none of which the parallel
// slices make visible.
func TestMapIndex(t *testing.T) {
	tests := []struct {
		name string
		keys []uint16
	}{
		{"one bucket, chained", collidingKeys(5)},
		{"inline index full", seq(0x100, inlineIndex/4)},
		{"heap index", seq(0x100, 300)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Map[int]
			for i, k := range tt.keys {
				m.Set(k, i)
			}
			for i, k := range tt.keys {
				require.Equal(t, i, *m.Get(k), "key %#x", k)
			}
			assert.False(t, m.Has(0x1fff))

			m.Remove(tt.keys[0])
			assert.False(t, m.Has(tt.keys[0]))
			for i, k := range tt.keys[1:] {
				require.Equal(t, i+1, *m.Get(k), "after Remove, key %#x", k)
			}

			m.Clear()
			assert.False(t, m.Has(tt.keys[1]))
			m.Set(tt.keys[1], 7)
			assert.Equal(t, 7, *m.Get(tt.keys[1]))
		})
	}
}

func seq(from uint16, n int) (keys []uint16) {
	for i := range n {
		keys = append(keys, from+uint16(i))
	}
	return
}

var sink int

func BenchmarkMapGet(b *testing.B) {
	for _, n := range []int{8, 67, 300} {
		var m Map[int]
		keys := seq(0x100, n)
		for i, k := range keys {
			m.Set(k, i)
		}
		rng := rand.New(rand.NewPCG(1, 2))
		probe := make([]uint16, 4096)
		for i := range probe {
			probe[i] = keys[rng.IntN(n)]
		}
		b.Run(fmt.Sprintf("%d keys", n), func(b *testing.B) {
			acc := 0
			for b.Loop() {
				for _, k := range probe {
					acc += *m.Get(k)
				}
			}
			sink = acc
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(probe)), "ns/get")
		})
	}
}
