# Hamburg Rails — Graph Routing REST Service

A REST service for routing queries on a directed, weighted graph of towns, built in Go 1.24+.

## Quick Start

```bash
# Build
go build -o hamburg-rails ./cmd/server

# Run with the seed graph pre-loaded (see testdata/graph.txt)
./hamburg-rails --graph=testdata/graph.txt

# Or run empty and load a graph later via POST /admin/graph
./hamburg-rails
```

The `--graph` flag points to a plain-text file with comma-separated edges (e.g. `AB5, BC4, CD8`).
A sample file is included at `testdata/graph.txt` with the seed graph from the spec.

| Flag | Default | Description |
|---|---|---|
| `--addr` | `:8080` | Listen address |
| `--graph` | (empty) | Path to edge-list file to load at startup |

## How to Test

```bash
# Run all tests
go test ./... -v

# Run fuzz test (5 seconds)
go test ./internal/graph/ -fuzz=FuzzParse -fuzztime=5s

# Run benchmarks
go test ./internal/route/ -bench=. -benchmem
```

## API Examples (curl)

### Load graph

```bash
curl -s -X POST localhost:8080/admin/graph \
  -H 'Content-Type: text/plain' \
  --data 'AB5, BC4, CD8, DC8, DE6, AD5, CE2, EB3, AE7'
```

### Get current graph

```bash
curl -s localhost:8080/graph
```

### Fixed route distance

```bash
curl -s -X POST localhost:8080/routes/distance \
  -H 'Content-Type: application/json' \
  -d '{"path":["A","B","C"]}'
# {"distance":9}
```

### Count trips by stops

```bash
curl -s -X POST localhost:8080/routes/count-by-stops \
  -H 'Content-Type: application/json' \
  -d '{"from":"C","to":"C","maxStops":3}'
# {"count":2}

# 4 stops
curl -s -X POST localhost:8080/routes/count-by-stops \
  -H 'Content-Type: application/json' \
  -d '{"from":"A","to":"C","maxStops":4,"minStops":4}'
# {"count":3}
```

### Count trips by distance

```bash
curl -s -X POST localhost:8080/routes/count-by-distance \
  -H 'Content-Type: application/json' \
  -d '{"from":"C","to":"C","maxDistance":30}'
# {"count":7}
```

### Shortest path

```bash
curl -s 'localhost:8080/routes/shortest?from=A&to=C'
# {"distance":9,"path":["A","B","C"]}

curl -s 'localhost:8080/routes/shortest?from=B&to=B'
# {"distance":9,"path":["B","C","E","B"]}
```

### Route search (optional endpoint)

```bash
curl -s -X POST localhost:8080/routes/search \
  -H 'Content-Type: application/json' \
  -d '{"from":"A","to":"C","constraints":{"maxStops":5,"maxDistance":25},"limit":5}'
```

### Health check

```bash
curl -s localhost:8080/healthz
# {"status":"ok"}
```

### Metrics

```bash
curl -s localhost:8080/metrics
```

---

## Architecture Decision Record

### 1. Data Representation: Adjacency List (map of maps)

**Decision:** `map[string]map[string]int` — from → (to → weight).

**Why:**

- O(1) edge lookup by (from, to) pair.
- O(degree) neighbor iteration — optimal for Dijkstra and DFS.
- Natural for sparse graphs (5k nodes / 50k edges). An adjacency matrix would waste O(V²) memory.

**Rejected alternatives:**

- **Adjacency matrix** — O(V²) memory, wasteful for sparse graphs.
- **Edge list** — O(E) neighbor lookup kills Dijkstra performance.

### 2. Concurrency: `sync.RWMutex`

**Decision:** Protect the graph with `RWMutex`. Reads acquire shared lock; `Replace` acquires exclusive lock.

**Why:**

- Reads don't block each other — important since routing queries dominate traffic.
- Hot reload only briefly blocks reads while swapping pointers.
- Simpler than `atomic.Value` with copy-on-write, and sufficient for this scale.

### 3. Shortest Path: Dijkstra with Binary Heap

**Decision:** Standard Dijkstra using `container/heap`, with path tracking and lexicographic tie-breaking.

**Complexity:** O((V + E) log V) time, O(V) space for distances.

**Why:**

- All edge weights are positive integers — Dijkstra is optimal.
- Binary heap from stdlib is well-tested and sufficient.
- For `from == to` (cycle detection): run Dijkstra from each neighbor of the source, add initial edge weight, find minimum cycle.

**Rejected alternatives:**

- **Floyd-Warshall** — O(V³), overkill for single-pair queries.
- **Bellman-Ford** — O(VE), slower, handles negative weights we don't have.
- **BFS** — only correct for unweighted graphs.

### 4. Route Counting: Iterative Bounded DFS

**Decision:** Explicit stack-based DFS with pruning on stops/distance bounds.

**Complexity:** O(b^d) where b = average branching factor, d = max depth/stops. Bounded by constraints.

**Why:**

- Avoids stack overflow on deep graphs (no recursion).
- Early pruning (`stops > maxStops` or `distance >= maxDistance`) keeps exploration tractable.
- Simple to reason about and debug.

---

## Interpretation Choices

- **Stops** = number of edges traversed (not intermediate nodes).
- **Distance constraints** use strict less-than (`< maxDistance`), per sample case 10.
- **Town names**: single uppercase letter per the example format. Multi-character town names would require a delimiter in the edge format.
- **Tie-breaking**: when multiple shortest paths have equal distance, the lexicographically smallest path is returned.
