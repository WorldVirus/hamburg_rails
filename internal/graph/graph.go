package graph

import (
	"fmt"
	"sync"
)

type Edge struct {
	From   string
	To     string
	Weight int
}

type Graph struct {
	mu    sync.RWMutex
	adj   map[string]map[string]int
	nodes map[string]struct{}
}

func New() *Graph {
	return &Graph{
		adj:   make(map[string]map[string]int),
		nodes: make(map[string]struct{}),
	}
}

func LoadEdges(edges []Edge) (*Graph, error) {
	g := New()
	for _, e := range edges {
		if e.From == e.To {
			return nil, fmt.Errorf("self-loop not allowed: %s%s%d", e.From, e.To, e.Weight)
		}
		if e.Weight <= 0 {
			return nil, fmt.Errorf("weight must be positive: %s%s%d", e.From, e.To, e.Weight)
		}
		if _, exists := g.adj[e.From][e.To]; exists {
			return nil, fmt.Errorf("duplicate edge: %s -> %s", e.From, e.To)
		}
		if g.adj[e.From] == nil {
			g.adj[e.From] = make(map[string]int)
		}
		g.adj[e.From][e.To] = e.Weight
		g.nodes[e.From] = struct{}{}
		g.nodes[e.To] = struct{}{}
	}
	return g, nil
}

func (g *Graph) Replace(edges []Edge) error {
	built, err := LoadEdges(edges)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.adj = built.adj
	g.nodes = built.nodes
	return nil
}

func (g *Graph) EdgeWeight(from, to string) (int, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if neighbors, ok := g.adj[from]; ok {
		w, exists := neighbors[to]
		return w, exists
	}
	return 0, false
}

func (g *Graph) Neighbors(node string) map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make(map[string]int)
	for k, v := range g.adj[node] {
		result[k] = v
	}
	return result
}

func (g *Graph) HasNode(node string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.nodes[node]
	return ok
}

func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, neighbors := range g.adj {
		count += len(neighbors)
	}
	return count
}

func (g *Graph) Edges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []Edge
	for from, neighbors := range g.adj {
		for to, w := range neighbors {
			result = append(result, Edge{From: from, To: to, Weight: w})
		}
	}
	return result
}

func (g *Graph) Nodes() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		result = append(result, n)
	}
	return result
}

// Snapshot returns a deep copy of the adjacency list so callers
// can iterate without holding the lock for the entire algorithm run.
func (g *Graph) Snapshot() map[string]map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	snap := make(map[string]map[string]int, len(g.adj))
	for from, neighbors := range g.adj {
		m := make(map[string]int, len(neighbors))
		for to, w := range neighbors {
			m[to] = w
		}
		snap[from] = m
	}
	return snap
}
