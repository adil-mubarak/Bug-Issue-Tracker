// Package trie provides a case-insensitive prefix index.
package trie

import "strings"

type node struct {
	children map[rune]*node
	word     string
}
type Trie struct{ root *node }

func New() *Trie { return &Trie{root: &node{children: make(map[rune]*node)}} }
func (t *Trie) Insert(word string) {
	word = strings.TrimSpace(word)
	if word == "" {
		return
	}
	n := t.root
	for _, r := range strings.ToLower(word) {
		if n.children[r] == nil {
			n.children[r] = &node{children: make(map[rune]*node)}
		}
		n = n.children[r]
	}
	n.word = word
}
func (t *Trie) Search(prefix string, limit int) []string {
	if limit <= 0 {
		return []string{}
	}
	n := t.root
	for _, r := range strings.ToLower(strings.TrimSpace(prefix)) {
		n = n.children[r]
		if n == nil {
			return []string{}
		}
	}
	out := []string{}
	var walk func(*node)
	walk = func(cur *node) {
		if cur == nil || len(out) >= limit {
			return
		}
		if cur.word != "" {
			out = append(out, cur.word)
		}
		for _, child := range cur.children {
			walk(child)
		}
	}
	walk(n)
	return out
}
