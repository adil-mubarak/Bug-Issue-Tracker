package trie

import (
	"fmt"
	"testing"
)

func BenchmarkTrieSearch(b *testing.B) {
	t := New()
	for i := 0; i < 10000; i++ {
		t.Insert(fmt.Sprintf("label-%05d", i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = t.Search("label-09", 10)
	}
}
