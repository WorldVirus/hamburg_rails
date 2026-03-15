package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"hamburg-rails/internal/graph"
	"hamburg-rails/internal/route"
)

type Handler struct {
	graph   *graph.Graph
	service *route.Service
}

func New(g *graph.Graph, svc *route.Service) *Handler {
	return &Handler{graph: g, service: svc}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) GetGraph(w http.ResponseWriter, r *http.Request) {
	edges := h.graph.Edges()
	writeJSON(w, http.StatusOK, map[string]any{
		"edges":     graph.FormatEdges(edges),
		"nodeCount": h.graph.NodeCount(),
		"edgeCount": h.graph.EdgeCount(),
	})
}

func (h *Handler) PostGraph(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	edges, err := graph.Parse(string(body))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.graph.Replace(edges); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   "graph loaded",
		"nodeCount": h.graph.NodeCount(),
		"edgeCount": h.graph.EdgeCount(),
	})
}

type distanceRequest struct {
	Path []string `json:"path"`
}

func (h *Handler) RouteDistance(w http.ResponseWriter, r *http.Request) {
	var req distanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.Path) < 2 {
		writeError(w, http.StatusUnprocessableEntity, "path must have at least 2 towns")
		return
	}

	for i := range req.Path {
		req.Path[i] = strings.ToUpper(strings.TrimSpace(req.Path[i]))
	}

	dist, err := h.service.Distance(req.Path)
	if err != nil {
		if err == route.ErrNoSuchRoute {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"distance": dist})
}

type countByStopsRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	MaxStops int    `json:"maxStops"`
	MinStops int    `json:"minStops"`
}

func (h *Handler) CountByStops(w http.ResponseWriter, r *http.Request) {
	var req countByStopsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.From = strings.ToUpper(strings.TrimSpace(req.From))
	req.To = strings.ToUpper(strings.TrimSpace(req.To))

	if req.From == "" || req.To == "" {
		writeError(w, http.StatusUnprocessableEntity, "from and to are required")
		return
	}
	if req.MaxStops <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "maxStops must be positive")
		return
	}
	if req.MinStops <= 0 {
		req.MinStops = 1
	}
	if req.MinStops > req.MaxStops {
		writeError(w, http.StatusUnprocessableEntity, "minStops must not exceed maxStops")
		return
	}

	count, err := h.service.CountByStops(req.From, req.To, req.MinStops, req.MaxStops)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

type countByDistanceRequest struct {
	From        string `json:"from"`
	To          string `json:"to"`
	MaxDistance int    `json:"maxDistance"`
}

func (h *Handler) CountByDistance(w http.ResponseWriter, r *http.Request) {
	var req countByDistanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.From = strings.ToUpper(strings.TrimSpace(req.From))
	req.To = strings.ToUpper(strings.TrimSpace(req.To))

	if req.From == "" || req.To == "" {
		writeError(w, http.StatusUnprocessableEntity, "from and to are required")
		return
	}
	if req.MaxDistance <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "maxDistance must be positive")
		return
	}

	count, err := h.service.CountByDistance(req.From, req.To, req.MaxDistance)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *Handler) ShortestPath(w http.ResponseWriter, r *http.Request) {
	from := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("from")))
	to := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("to")))

	if from == "" || to == "" {
		writeError(w, http.StatusUnprocessableEntity, "from and to query params are required")
		return
	}

	dist, path, err := h.service.ShortestPath(from, to)
	if err != nil {
		if err == route.ErrUnreachable || err == route.ErrNoSuchRoute {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"distance": dist,
		"path":     path,
	})
}

type searchRequest struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Constraints struct {
		MaxStops      int  `json:"maxStops"`
		MaxDistance    int  `json:"maxDistance"`
		DistinctNodes bool `json:"distinctNodes"`
	} `json:"constraints"`
	Limit int `json:"limit"`
}

func (h *Handler) SearchRoutes(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.From = strings.ToUpper(strings.TrimSpace(req.From))
	req.To = strings.ToUpper(strings.TrimSpace(req.To))

	if req.From == "" || req.To == "" {
		writeError(w, http.StatusUnprocessableEntity, "from and to are required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	results, err := h.service.SearchRoutes(
		req.From, req.To,
		req.Constraints.MaxStops, req.Constraints.MaxDistance,
		req.Constraints.DistinctNodes, req.Limit,
	)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"routes": results})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
