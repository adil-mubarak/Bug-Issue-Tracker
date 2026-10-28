// Package graph provides directed dependency validation and topological ordering.
package graph

import "errors"

var ErrCycle = errors.New("dependency cycle")

type Graph struct {
	edges map[string]map[string]struct{}
}

func New() *Graph { return &Graph{edges: make(map[string]map[string]struct{})} }

func (g *Graph) AddNode(node string) {
	if node != "" {
		g.edges[node] = g.ensure(g.edges[node])
	}
}

// AddDependency means item is blocked by dependency.
func (g *Graph) AddDependency(item, dependency string) error {
	if item == "" || dependency == "" {
		return errors.New("node names must not be empty")
	}
	g.AddNode(item)
	g.AddNode(dependency)
	g.edges[item][dependency] = struct{}{}
	if _, err := g.TopologicalOrder(); err != nil {
		delete(g.edges[item], dependency)
		return err
	}
	return nil
}
func (g *Graph) ensure(m map[string]struct{}) map[string]struct{} {
	if m == nil {
		return make(map[string]struct{})
	}
	return m
}
func (g *Graph) TopologicalOrder() ([]string, error) {
	state := map[string]uint8{}
	out := make([]string, 0, len(g.edges))
	var visit func(string) error
	visit = func(n string) error {
		if state[n] == 1 {
			return ErrCycle
		}
		if state[n] == 2 {
			return nil
		}
		state[n] = 1
		for dep := range g.edges[n] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[n] = 2
		out = append(out, n)
		return nil
	}
	for n := range g.edges {
		if err := visit(n); err != nil {
			return nil, err
		}
	}
	return out, nil
}
