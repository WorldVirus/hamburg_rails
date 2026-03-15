package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hamburg-rails/internal/graph"
	"hamburg-rails/internal/route"
)

func setupHandler(t *testing.T) *Handler {
	t.Helper()
	edges, err := graph.Parse("AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7")
	if err != nil {
		t.Fatal(err)
	}
	g, err := graph.LoadEdges(edges)
	if err != nil {
		t.Fatal(err)
	}
	return New(g, route.NewService(g))
}

func TestHealthz(t *testing.T) {
	h := setupHandler(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.Healthz(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected ok, got %s", resp["status"])
	}
}

func TestRouteDistance(t *testing.T) {
	h := setupHandler(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantDist   int
	}{
		{name: "A-B-C", body: `{"path":["A","B","C"]}`, wantStatus: 200, wantDist: 9},
		{name: "A-D", body: `{"path":["A","D"]}`, wantStatus: 200, wantDist: 5},
		{name: "no route", body: `{"path":["A","E","D"]}`, wantStatus: 404},
		{name: "too short", body: `{"path":["A"]}`, wantStatus: 422},
		{name: "bad json", body: `{bad}`, wantStatus: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/routes/distance", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.RouteDistance(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantStatus == 200 {
				var resp map[string]int
				json.NewDecoder(w.Body).Decode(&resp)
				if resp["distance"] != tt.wantDist {
					t.Fatalf("expected distance %d, got %d", tt.wantDist, resp["distance"])
				}
			}
		})
	}
}

func TestCountByStops(t *testing.T) {
	h := setupHandler(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "C-C max 3",
			body:       `{"from":"C","to":"C","maxStops":3}`,
			wantStatus: 200,
			wantCount:  2,
		},
		{
			name:       "A-C exactly 4",
			body:       `{"from":"A","to":"C","maxStops":4,"minStops":4}`,
			wantStatus: 200,
			wantCount:  3,
		},
		{
			name:       "missing from",
			body:       `{"to":"C","maxStops":3}`,
			wantStatus: 422,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/routes/count-by-stops", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.CountByStops(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantStatus == 200 {
				var resp map[string]int
				json.NewDecoder(w.Body).Decode(&resp)
				if resp["count"] != tt.wantCount {
					t.Fatalf("expected count %d, got %d", tt.wantCount, resp["count"])
				}
			}
		})
	}
}

func TestCountByDistance(t *testing.T) {
	h := setupHandler(t)

	req := httptest.NewRequest("POST", "/routes/count-by-distance",
		bytes.NewBufferString(`{"from":"C","to":"C","maxDistance":30}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CountByDistance(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]int
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["count"] != 7 {
		t.Fatalf("expected 7, got %d", resp["count"])
	}
}

func TestShortestPath(t *testing.T) {
	h := setupHandler(t)

	tests := []struct {
		name       string
		from, to   string
		wantStatus int
		wantDist   int
	}{
		{name: "A-C", from: "A", to: "C", wantStatus: 200, wantDist: 9},
		{name: "B-B cycle", from: "B", to: "B", wantStatus: 200, wantDist: 9},
		{name: "missing param", from: "", to: "C", wantStatus: 422},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/routes/shortest?from="+tt.from+"&to="+tt.to, nil)
			w := httptest.NewRecorder()
			h.ShortestPath(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantStatus == 200 {
				var resp map[string]any
				json.NewDecoder(w.Body).Decode(&resp)
				dist := int(resp["distance"].(float64))
				if dist != tt.wantDist {
					t.Fatalf("expected distance %d, got %d", tt.wantDist, dist)
				}
			}
		})
	}
}

func TestPostGraph(t *testing.T) {
	h := setupHandler(t)

	// Valid reload
	req := httptest.NewRequest("POST", "/admin/graph", bytes.NewBufferString("XY3, YZ7"))
	w := httptest.NewRecorder()
	h.PostGraph(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid reload — graph should stay as XY3, YZ7
	req = httptest.NewRequest("POST", "/admin/graph", bytes.NewBufferString("invalid!!!"))
	w = httptest.NewRecorder()
	h.PostGraph(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Verify old graph is intact
	req = httptest.NewRequest("GET", "/graph", nil)
	w = httptest.NewRecorder()
	h.GetGraph(w, req)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["nodeCount"].(float64)) != 3 {
		t.Fatalf("expected 3 nodes after failed reload, got %v", resp["nodeCount"])
	}
}
