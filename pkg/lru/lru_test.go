package lru

import "testing"

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := New[int, string](2)
	c.Put(1, "one")
	c.Put(2, "two")
	if _, ok := c.Get(1); !ok {
		t.Fatal("expected cache hit")
	}
	c.Put(3, "three")
	if _, ok := c.Get(2); ok {
		t.Fatal("expected key 2 eviction")
	}
	if c.Len() != 2 {
		t.Fatalf("len=%d", c.Len())
	}
}
func BenchmarkCacheGet(b *testing.B) {
	c := New[int, int](1024)
	for i := 0; i < 1024; i++ {
		c.Put(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(i % 1024)
	}
}
