package graph

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int // number of edges
		wantErr bool
	}{
		{name: "seed graph", input: "AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7", want: 9},
		{name: "empty input", input: "", want: 0},
		{name: "single edge", input: "AB5", want: 1},
		{name: "lowercase normalized", input: "ab5, bc4", want: 2},
		{name: "extra spaces", input: "  AB5 ,  BC4  ", want: 2},
		{name: "trailing comma", input: "AB5, BC4,", want: 2},
		{name: "self loop", input: "AA5", wantErr: true},
		{name: "duplicate edge", input: "AB5, AB3", wantErr: true},
		{name: "invalid format", input: "ABC", wantErr: true},
		{name: "no weight", input: "AB", wantErr: true},
		{name: "zero weight", input: "AB0", wantErr: true},
		{name: "non-alpha chars", input: "A15", wantErr: true},
		{name: "three char town", input: "ABC5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edges, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(edges) != tt.want {
				t.Fatalf("got %d edges, want %d", len(edges), tt.want)
			}
		})
	}
}

func TestParseNormalization(t *testing.T) {
	edges, err := Parse("ab5")
	if err != nil {
		t.Fatal(err)
	}
	if edges[0].From != "A" || edges[0].To != "B" {
		t.Fatalf("expected A->B, got %s->%s", edges[0].From, edges[0].To)
	}
}

func FuzzParse(f *testing.F) {
	f.Add("AB5, BC4, CD8")
	f.Add("")
	f.Add("AA5")
	f.Add("A15")
	f.Add("AB0")
	f.Add("!!!")
	f.Add("AB5, AB5")

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = Parse(input)
	})
}
