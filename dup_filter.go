package crawler

import (
	"sync"
)

type DuplicateFilter interface {
	Visited(string)
	HasVisited(string) bool
}

// In-memory duplicates filter
type InMemoryDupFilter struct {
	visited sync.Map
}

func (m *InMemoryDupFilter) Visited(u string) {
	m.visited.Store(u, true)
}

func (m *InMemoryDupFilter) HasVisited(u string) bool {
	_, ok := m.visited.Load(u)
	return ok
}
