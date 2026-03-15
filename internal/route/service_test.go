package route

import (
	"testing"

	"hamburg-rails/internal/graph"
)

// seedGraph returns the sample graph from the spec:
// AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7
func seedGraph(t *testing.T) *graph.Graph {
	t.Helper()
	edges, err := graph.Parse("AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7")
	if err != nil {
		t.Fatal(err)
	}
	g, err := graph.LoadEdges(edges)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestDistance(t *testing.T) {
	svc := NewService(seedGraph(t))

	tests := []struct {
		name    string
		path    []string
		want    int
		wantErr error
	}{
		{name: "A-B-C", path: []string{"A", "B", "C"}, want: 9},
		{name: "A-D", path: []string{"A", "D"}, want: 5},
		{name: "A-D-C", path: []string{"A", "D", "C"}, want: 13},
		{name: "A-E-B-C-D", path: []string{"A", "E", "B", "C", "D"}, want: 22},
		{name: "A-E-D no route", path: []string{"A", "E", "D"}, wantErr: ErrNoSuchRoute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.Distance(tt.path)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountByStops(t *testing.T) {
	svc := NewService(seedGraph(t))

	tests := []struct {
		name     string
		from, to string
		min, max int
		want     int
	}{
		{name: "C-C max 3 stops", from: "C", to: "C", min: 1, max: 3, want: 2},
		{name: "A-C exactly 4 stops", from: "A", to: "C", min: 4, max: 4, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CountByStops(tt.from, tt.to, tt.min, tt.max)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountByDistance(t *testing.T) {
	svc := NewService(seedGraph(t))

	tests := []struct {
		name     string
		from, to string
		maxDist  int
		want     int
	}{
		{name: "C-C dist < 30", from: "C", to: "C", maxDist: 30, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.CountByDistance(tt.from, tt.to, tt.maxDist)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestShortestPath(t *testing.T) {
	svc := NewService(seedGraph(t))

	tests := []struct {
		name     string
		from, to string
		wantDist int
		wantErr  bool
	}{
		{name: "A-C shortest", from: "A", to: "C", wantDist: 9},
		{name: "B-B shortest cycle", from: "B", to: "B", wantDist: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist, path, err := svc.ShortestPath(tt.from, tt.to)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dist != tt.wantDist {
				t.Fatalf("got distance %d, want %d", dist, tt.wantDist)
			}
			if len(path) < 2 {
				t.Fatalf("path too short: %v", path)
			}
			if path[0] != tt.from {
				t.Fatalf("path should start with %s, got %s", tt.from, path[0])
			}
			if path[len(path)-1] != tt.to {
				t.Fatalf("path should end with %s, got %s", tt.to, path[len(path)-1])
			}
		})
	}
}

func TestShortestPathUnreachable(t *testing.T) {
	edges, _ := graph.Parse("AB5, CD3")
	g, _ := graph.LoadEdges(edges)
	svc := NewService(g)

	_, _, err := svc.ShortestPath("A", "D")
	if err == nil {
		t.Fatal("expected error for unreachable destination")
	}
}

func BenchmarkShortestPath(b *testing.B) {
	edges, _ := graph.Parse("AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7")
	g, _ := graph.LoadEdges(edges)
	svc := NewService(g)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.ShortestPath("A", "C")
	}
}

func BenchmarkCountByDistance(b *testing.B) {
	edges, _ := graph.Parse("AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7")
	g, _ := graph.LoadEdges(edges)
	svc := NewService(g)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.CountByDistance("C", "C", 30)
	}
}
