package main

import (
	"context"
	_ "embed"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hamburg-rails/internal/graph"
	"hamburg-rails/internal/handler"
	"hamburg-rails/internal/metrics"
	"hamburg-rails/internal/route"
)

//go:embed openapi.json
var openAPISpec []byte

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	graphFile := flag.String("graph", "", "path to graph file (edge list)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	g := graph.New()

	if *graphFile != "" {
		data, err := os.ReadFile(*graphFile)
		if err != nil {
			slog.Error("failed to read graph file", "path", *graphFile, "error", err)
			os.Exit(1)
		}
		edges, err := graph.Parse(string(data))
		if err != nil {
			slog.Error("failed to parse graph file", "error", err)
			os.Exit(1)
		}
		if err := g.Replace(edges); err != nil {
			slog.Error("failed to load graph", "error", err)
			os.Exit(1)
		}
		slog.Info("graph loaded from file", "path", *graphFile, "nodes", g.NodeCount(), "edges", g.EdgeCount())
	}

	svc := route.NewService(g)
	h := handler.New(g, svc)
	collector := metrics.NewCollector()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /openapi.json", serveOpenAPI)
	mux.HandleFunc("GET /graph", h.GetGraph)
	mux.HandleFunc("POST /admin/graph", h.PostGraph)
	mux.HandleFunc("POST /routes/distance", h.RouteDistance)
	mux.HandleFunc("POST /routes/count-by-stops", h.CountByStops)
	mux.HandleFunc("POST /routes/count-by-distance", h.CountByDistance)
	mux.HandleFunc("GET /routes/shortest", h.ShortestPath)
	mux.HandleFunc("POST /routes/search", h.SearchRoutes)
	mux.HandleFunc("GET /metrics", collector.Handler())

	var root http.Handler = mux
	root = handler.MetricsMiddleware(collector)(root)
	root = handler.Logging(root)
	root = handler.RequestIDMiddleware(root)
	root = handler.Recovery(root)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      root,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting", "addr", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}

func serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(openAPISpec)
}
