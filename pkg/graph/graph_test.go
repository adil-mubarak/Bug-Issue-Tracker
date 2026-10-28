package graph

import "testing"

func TestDependencyOrderAndCycle(t *testing.T) {
	g := New()
	if err := g.AddDependency("ship", "test"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddDependency("test", "build"); err != nil {
		t.Fatal(err)
	}
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if pos["build"] > pos["test"] || pos["test"] > pos["ship"] {
		t.Fatalf("bad order: %v", order)
	}
	if err := g.AddDependency("build", "ship"); err != ErrCycle {
		t.Fatalf("err=%v", err)
	}
}
