package graph

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var tokenRegex = regexp.MustCompile(`^([A-Za-z])([A-Za-z])(\d+)$`)

func Parse(input string) ([]Edge, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	tokens := strings.Split(input, ",")
	edges := make([]Edge, 0, len(tokens))
	seen := make(map[string]struct{})

	for i, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		matches := tokenRegex.FindStringSubmatch(token)
		if matches == nil {
			return nil, fmt.Errorf("token %d: invalid format %q (expected e.g. AB5)", i+1, token)
		}

		from := strings.ToUpper(matches[1])
		to := strings.ToUpper(matches[2])
		weight, err := strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("token %d: invalid weight in %q: %w", i+1, token, err)
		}

		if weight <= 0 {
			return nil, fmt.Errorf("token %d: weight must be positive in %q", i+1, token)
		}
		if from == to {
			return nil, fmt.Errorf("token %d: self-loop not allowed: %q", i+1, token)
		}

		key := from + "->" + to
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("token %d: duplicate edge %s -> %s", i+1, from, to)
		}
		seen[key] = struct{}{}

		edges = append(edges, Edge{From: from, To: to, Weight: weight})
	}

	return edges, nil
}

func FormatEdges(edges []Edge) string {
	parts := make([]string, len(edges))
	for i, e := range edges {
		parts[i] = fmt.Sprintf("%s%s%d", e.From, e.To, e.Weight)
	}
	return strings.Join(parts, ", ")
}
