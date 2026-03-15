package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Collector struct {
	mu       sync.RWMutex
	counters map[string]*atomic.Int64
	durSum   map[string]*atomic.Int64 // microseconds
}

func NewCollector() *Collector {
	return &Collector{
		counters: make(map[string]*atomic.Int64),
		durSum:   make(map[string]*atomic.Int64),
	}
}

func (c *Collector) Record(method, path string, duration time.Duration) {
	key := method + " " + path

	c.mu.RLock()
	counter, cOk := c.counters[key]
	dur, dOk := c.durSum[key]
	c.mu.RUnlock()

	if !cOk || !dOk {
		c.mu.Lock()
		if _, ok := c.counters[key]; !ok {
			c.counters[key] = &atomic.Int64{}
			c.durSum[key] = &atomic.Int64{}
		}
		counter = c.counters[key]
		dur = c.durSum[key]
		c.mu.Unlock()
	}

	counter.Add(1)
	dur.Add(duration.Microseconds())
}

func (c *Collector) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		keys := make([]string, 0, len(c.counters))
		for k := range c.counters {
			keys = append(keys, k)
		}
		c.mu.RUnlock()

		sort.Strings(keys)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, key := range keys {
			c.mu.RLock()
			count := c.counters[key].Load()
			durUs := c.durSum[key].Load()
			c.mu.RUnlock()

			var avgMs float64
			if count > 0 {
				avgMs = float64(durUs) / float64(count) / 1000.0
			}
			fmt.Fprintf(w, "http_requests_total{endpoint=%q} %d\n", key, count)
			fmt.Fprintf(w, "http_request_duration_avg_ms{endpoint=%q} %.3f\n", key, avgMs)
		}
	}
}
