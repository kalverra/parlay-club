package eval

import (
	"context"
	"errors"
	"sync"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
)

type cacheKey struct {
	groupID int64
	season  int
}

type cacheGen struct {
	global uint64
	group  uint64
}

// Cache memoizes SuiteReports keyed by (groupID, season) until invalidated by
// leg/slip settlement or a sandbox reset. groupID 0 caches global (all-group)
// reports and is invalidated whenever any group is invalidated.
type Cache struct {
	mu       sync.RWMutex
	entries  map[cacheKey]SuiteReport
	gen      uint64
	groupGen map[int64]uint64
}

// NewCache constructs an empty in-memory eval Cache.
func NewCache() *Cache {
	return &Cache{
		entries:  make(map[cacheKey]SuiteReport),
		groupGen: make(map[int64]uint64),
	}
}

// GetOrCompute returns the cached SuiteReport for (groupID, season), or replays
// and evaluates the group's dataset from store on a cache miss.
func (c *Cache) GetOrCompute(
	ctx context.Context,
	store *db.Store,
	groupID int64,
	season int,
) (SuiteReport, error) {
	key := cacheKey{groupID: groupID, season: season}

	c.mu.RLock()
	if report, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		return report, nil
	}
	startGen := cacheGen{
		global: c.gen,
		group:  c.groupGen[groupID],
	}
	c.mu.RUnlock()

	if store == nil {
		return SuiteReport{}, errors.New("eval cache: nil store")
	}

	dataset, err := LoadDataset(ctx, store, ReplayFilter{
		GroupID: groupID,
		Season:  season,
	})
	if err != nil {
		return SuiteReport{}, err
	}
	report := EvaluateAll(dataset, analyzer.BuiltInRubrics())

	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[cacheKey]SuiteReport)
	}
	if c.gen == startGen.global && c.groupGen[groupID] == startGen.group {
		c.entries[key] = report
	}
	c.mu.Unlock()

	return report, nil
}

// InvalidateGroup drops all cached seasons for groupID as well as any global
// (groupID 0) cached entries that aggregate across groups.
func (c *Cache) InvalidateGroup(groupID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.groupGen == nil {
		c.groupGen = make(map[int64]uint64)
	}
	c.groupGen[groupID]++
	if groupID != 0 {
		c.groupGen[0]++
	}
	for key := range c.entries {
		if key.groupID == groupID || key.groupID == 0 {
			delete(c.entries, key)
		}
	}
}

// InvalidateAll clears every cached entry across all groups and seasons.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.gen++
	clear(c.entries)
}
