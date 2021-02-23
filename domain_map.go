package crawler

import (
	"sort"
	"sync"
	"time"
)

// DomainMap stores domains and the last time a request was made to any
// URL in the domain. This is used specifically to rate-limit requests
// per-domain in the crawler.
type DomainMap struct {
	mu *sync.RWMutex

	// In-memory storage mapping domains to timestamps
	domains map[string]*time.Time

	// Array of domains sortedKeys using in ascending order according to
	// time accessed. This is used to clear old entries
	sortedKeys []string

	size    int
	MaxSize int
	Delay   time.Duration
}

func NewDomainMap(size int, delay time.Duration) *DomainMap {
	return &DomainMap{
		mu:         &sync.RWMutex{},
		domains:    make(map[string]*time.Time),
		sortedKeys: []string{},
		size:       0,
		MaxSize:    size,
		Delay:      delay,
	}
}

// Sets the domain's timestamp to the specified time, regardless of
// whether a value is already set.
func (dm *DomainMap) Set(domain string, t *time.Time) {
	dm.Update(domain, func(unused *time.Time) *time.Time {
		return t
	})
}

func (dm *DomainMap) Get(domain string) *time.Time {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	if t, ok := dm.domains[domain]; ok {
		return t
	}
	return nil
}

func (dm *DomainMap) Update(domain string, fn func(*time.Time) *time.Time) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if t, ok := dm.domains[domain]; ok {
		dm.domains[domain] = fn(t)
		return
	}

	if dm.size == dm.MaxSize {
		dm.clean()
	}

	dm.size += 1
	dm.domains[domain] = fn(nil)
	dm.sortedKeys = append(dm.sortedKeys, domain)
	dm.sort()
}

// Clear out old entries from the map
// This should only be called when the map has already been fully locked
func (dm *DomainMap) clean() {
	for _, domain := range dm.sortedKeys {
		// If the current time is after the required delay period for the given domain
		// We can safely delete the entry since we will allow any request to the domain
		if time.Now().After(dm.domains[domain].Add(dm.Delay)) {
			dm.size -= 1
			delete(dm.domains, domain)
		} else {
			break
		}
	}
}

func (dm *DomainMap) sort() {
	sort.Slice(dm.sortedKeys, func(i, j int) bool {
		return dm.domains[dm.sortedKeys[i]].After(*dm.domains[dm.sortedKeys[j]])
	})
}
