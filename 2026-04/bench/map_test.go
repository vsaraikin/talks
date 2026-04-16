package bench

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

const smallSize = 100
const medSize = 10_000
const largeSize = 1_000_000

// pre-generate keys to avoid measuring key generation
var (
	keysSmall = genKeys(smallSize)
	keysMed   = genKeys(medSize)
	keysLarge = genKeys(largeSize)
)

func genKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("key-%016x", rand.Uint64())
	}
	return keys
}

// --- Insert ---

func benchInsert(b *testing.B, keys []string) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := make(map[string]string, len(keys))
		for _, k := range keys {
			m[k] = k
		}
	}
}

func BenchmarkInsert_100(b *testing.B)  { benchInsert(b, keysSmall) }
func BenchmarkInsert_10K(b *testing.B)  { benchInsert(b, keysMed) }
func BenchmarkInsert_1M(b *testing.B)   { benchInsert(b, keysLarge) }

// --- Insert without pre-allocation ---

func benchInsertNoHint(b *testing.B, keys []string) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := make(map[string]string)
		for _, k := range keys {
			m[k] = k
		}
	}
}

func BenchmarkInsertNoHint_100(b *testing.B) { benchInsertNoHint(b, keysSmall) }
func BenchmarkInsertNoHint_10K(b *testing.B) { benchInsertNoHint(b, keysMed) }
func BenchmarkInsertNoHint_1M(b *testing.B)  { benchInsertNoHint(b, keysLarge) }

// --- Lookup hit ---

func benchLookupHit(b *testing.B, keys []string) {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = k
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, k := range keys {
			_ = m[k]
		}
	}
}

func BenchmarkLookupHit_100(b *testing.B) { benchLookupHit(b, keysSmall) }
func BenchmarkLookupHit_10K(b *testing.B) { benchLookupHit(b, keysMed) }
func BenchmarkLookupHit_1M(b *testing.B)  { benchLookupHit(b, keysLarge) }

// --- Lookup miss ---

func benchLookupMiss(b *testing.B, keys []string) {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = k
	}
	missKeys := genKeys(len(keys)) // different keys
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, k := range missKeys {
			_ = m[k]
		}
	}
}

func BenchmarkLookupMiss_100(b *testing.B) { benchLookupMiss(b, keysSmall) }
func BenchmarkLookupMiss_10K(b *testing.B) { benchLookupMiss(b, keysMed) }
func BenchmarkLookupMiss_1M(b *testing.B)  { benchLookupMiss(b, keysLarge) }

// --- Delete ---

func benchDelete(b *testing.B, keys []string) {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = k
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// re-populate
		for _, k := range keys {
			m[k] = k
		}
		// delete all
		for _, k := range keys {
			delete(m, k)
		}
	}
}

func BenchmarkDelete_100(b *testing.B) { benchDelete(b, keysSmall) }
func BenchmarkDelete_10K(b *testing.B) { benchDelete(b, keysMed) }
func BenchmarkDelete_1M(b *testing.B)  { benchDelete(b, keysLarge) }

// --- Iteration ---

func benchIterate(b *testing.B, keys []string) {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = k
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for k, v := range m {
			_, _ = k, v
		}
	}
}

func BenchmarkIterate_100(b *testing.B) { benchIterate(b, keysSmall) }
func BenchmarkIterate_10K(b *testing.B) { benchIterate(b, keysMed) }
func BenchmarkIterate_1M(b *testing.B)  { benchIterate(b, keysLarge) }
