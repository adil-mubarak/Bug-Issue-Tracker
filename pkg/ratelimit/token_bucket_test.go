package ratelimit

import "testing"

func TestBucketCapacity(t *testing.T) {
	b := New(2, 0.001)
	first, second := b.Allow(), b.Allow()
	if !first || !second {
		t.Fatal("initial tokens missing")
	}
	if b.Allow() {
		t.Fatal("bucket should be empty")
	}
}
