package graph

import (
	"strconv"
	"testing"
)

func BenchmarkGraphTopologicalOrder(b *testing.B) {
	g := New()
	for i := 0; i < 1000; i++ {
		g.AddNode(strconv.Itoa(i))
		if i > 0 {
			_ = g.AddDependency(strconv.Itoa(i), strconv.Itoa(i-1))
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.TopologicalOrder(); err != nil {
			b.Fatal(err)
		}
	}
}
