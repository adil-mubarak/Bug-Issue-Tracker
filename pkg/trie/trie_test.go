package trie

import "testing"

func TestPrefixSearch(t *testing.T) {
	tr := New()
	tr.Insert("Bug")
	tr.Insert("Build")
	tr.Insert("Feature")
	got := tr.Search("bu", 5)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if len(tr.Search("x", 5)) != 0 {
		t.Fatal("expected empty result")
	}
}
