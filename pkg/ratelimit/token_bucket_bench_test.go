package ratelimit

import "testing"

func BenchmarkTokenBucketAllow(b *testing.B) {
	bucket := New(1_000_000, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bucket.Allow()
	}
}
