package route

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"hamburg-rails/internal/graph"
)

var (
	ErrNoSuchRoute = errors.New("NO SUCH ROUTE")
	ErrUnreachable = errors.New("no path exists")
)

type Service struct {
	g *graph.Graph
}

func NewService(g *graph.Graph) *Service {
	return &Service{g: g}
}

func (s *Service) Distance(path []string) (int, error) {
	if len(path) < 2 {
		return 0, nil
	}
	total := 0
	for i := 0; i < len(path)-1; i++ {
		w, ok := s.g.EdgeWeight(path[i], path[i+1])
		if !ok {
			return 0, ErrNoSuchRoute
		}
		total += w
	}
	return total, nil
}

// CountByStops counts trips where minStops <= edges_traversed <= maxStops.
func (s *Service) CountByStops(from, to string, minStops, maxStops int) (int, error) {
	if err := s.validateNodes(from, to); err != nil {
		return 0, err
	}

	snap := s.g.Snapshot()
	count := 0

	type frame struct {
		node  string
		stops int
	}
	stack := []frame{{node: from, stops: 0}}

	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if f.stops > 0 && f.node == to && f.stops >= minStops && f.stops <= maxStops {
			count++
		}
		if f.stops >= maxStops {
			continue
		}

		for neighbor := range snap[f.node] {
			stack = append(stack, frame{node: neighbor, stops: f.stops + 1})
		}
	}

	return count, nil
}

// CountByDistance counts trips where total distance is strictly less than maxDistance.
func (s *Service) CountByDistance(from, to string, maxDistance int) (int, error) {
	if err := s.validateNodes(from, to); err != nil {
		return 0, err
	}

	snap := s.g.Snapshot()
	count := 0

	type frame struct {
		node string
		dist int
	}
	stack := []frame{{node: from, dist: 0}}

	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if f.dist > 0 && f.node == to && f.dist < maxDistance {
			count++
		}

		for neighbor, w := range snap[f.node] {
			newDist := f.dist + w
			if newDist < maxDistance {
				stack = append(stack, frame{node: neighbor, dist: newDist})
			}
		}
	}

	return count, nil
}

func (s *Service) ShortestPath(from, to string) (int, []string, error) {
	if err := s.validateNodes(from, to); err != nil {
		return 0, nil, err
	}

	snap := s.g.Snapshot()

	if from == to {
		return s.shortestCycle(from, snap)
	}
	return s.dijkstra(from, to, snap)
}

func (s *Service) dijkstra(from, to string, snap map[string]map[string]int) (int, []string, error) {
	dist := make(map[string]int)
	bestPath := make(map[string][]string)

	pq := &priorityQueue{}
	heap.Init(pq)
	heap.Push(pq, &pqItem{node: from, dist: 0, path: []string{from}})

	for pq.Len() > 0 {
		item := heap.Pop(pq).(*pqItem)

		if d, seen := dist[item.node]; seen {
			if d < item.dist {
				continue
			}
			if d == item.dist && !pathLess(item.path, bestPath[item.node]) {
				continue
			}
		}

		dist[item.node] = item.dist
		bestPath[item.node] = item.path

		if item.node == to {
			return item.dist, item.path, nil
		}

		for neighbor, w := range snap[item.node] {
			newDist := item.dist + w
			if d, seen := dist[neighbor]; seen && d < newDist {
				continue
			}
			newPath := make([]string, len(item.path)+1)
			copy(newPath, item.path)
			newPath[len(item.path)] = neighbor
			heap.Push(pq, &pqItem{node: neighbor, dist: newDist, path: newPath})
		}
	}

	return 0, nil, ErrUnreachable
}

// shortestCycle finds the shortest non-zero cycle back to node.
// run dijkstra from each outgoing neighbor back to node,
// then pick the minimum total (edge weight + dijkstra result).
func (s *Service) shortestCycle(node string, snap map[string]map[string]int) (int, []string, error) {
	bestDist := math.MaxInt
	var bestPath []string

	neighbors := snap[node]
	if len(neighbors) == 0 {
		return 0, nil, ErrUnreachable
	}

	type neighborEdge struct {
		to     string
		weight int
	}
	sorted := make([]neighborEdge, 0, len(neighbors))
	for to, w := range neighbors {
		sorted = append(sorted, neighborEdge{to, w})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].to < sorted[j].to })

	for _, ne := range sorted {
		d, p, err := s.dijkstra(ne.to, node, snap)
		if err != nil {
			continue
		}
		totalDist := ne.weight + d
		totalPath := append([]string{node}, p...)

		if totalDist < bestDist || (totalDist == bestDist && pathLess(totalPath, bestPath)) {
			bestDist = totalDist
			bestPath = totalPath
		}
	}

	if bestPath == nil {
		return 0, nil, ErrUnreachable
	}
	return bestDist, bestPath, nil
}

type RouteResult struct {
	Path     []string `json:"path"`
	Distance int      `json:"distance"`
}

func (s *Service) SearchRoutes(from, to string, maxStops, maxDistance int, distinctNodes bool, limit int) ([]RouteResult, error) {
	if err := s.validateNodes(from, to); err != nil {
		return nil, err
	}

	snap := s.g.Snapshot()
	var results []RouteResult

	type frame struct {
		node    string
		dist    int
		path    []string
		visited map[string]bool
	}

	startVisited := map[string]bool{}
	if distinctNodes {
		startVisited[from] = true
	}
	stack := []frame{{node: from, dist: 0, path: []string{from}, visited: startVisited}}

	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if len(f.path) > 1 && f.node == to {
			results = append(results, RouteResult{Path: f.path, Distance: f.dist})
		}

		stops := len(f.path) - 1
		if maxStops > 0 && stops >= maxStops {
			continue
		}

		for neighbor, w := range snap[f.node] {
			newDist := f.dist + w
			if maxDistance > 0 && newDist >= maxDistance {
				continue
			}
			if distinctNodes && f.visited[neighbor] && neighbor != to {
				continue
			}
			newPath := make([]string, len(f.path)+1)
			copy(newPath, f.path)
			newPath[len(f.path)] = neighbor

			var newVisited map[string]bool
			if distinctNodes {
				newVisited = make(map[string]bool, len(f.visited)+1)
				for k, v := range f.visited {
					newVisited[k] = v
				}
				newVisited[neighbor] = true
			} else {
				newVisited = f.visited
			}
			stack = append(stack, frame{node: neighbor, dist: newDist, path: newPath, visited: newVisited})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Distance != results[j].Distance {
			return results[i].Distance < results[j].Distance
		}
		return pathLess(results[i].Path, results[j].Path)
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Service) validateNodes(nodes ...string) error {
	for _, n := range nodes {
		if !s.g.HasNode(n) {
			return fmt.Errorf("unknown town: %s", n)
		}
	}
	return nil
}

func pathLess(a, b []string) bool {
	return strings.Join(a, "") < strings.Join(b, "")
}
