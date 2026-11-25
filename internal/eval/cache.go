package eval

import (
	"context"
	"errors"
	"sync"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
)

type cacheKey struct {
	mode    ReplayMode
	groupID int64
	season  int
}

type cacheGen struct {
	global uint64
	group  uint64
	market uint64
}

// Cache memoizes SuiteReports keyed by (mode, groupID, season) until invalidated
// by leg/slip settlement, market data updates, or a sandbox reset. groupID 0
// caches global (all-group) slip reports and is invalidated whenever any group
// is invalidated.
type Cache struct {
	mu        sync.RWMutex
	entries   map[cacheKey]SuiteReport
	gen       uint64
	groupGen  map[int64]uint64
	marketGen uint64
}

// NewCache constructs an empty in-memory eval Cache.
func NewCache() *Cache {
	return &Cache{
		entries:  make(map[cacheKey]SuiteReport),
		groupGen: make(map[int64]uint64),
	}
}

// GetOrCompute returns the cached ModeSlips SuiteReport for (groupID, season),
// or replays and evaluates the group's dataset from store on a cache miss.
func (c *Cache) GetOrCompute(
	ctx context.Context,
	store *db.Store,
	groupID int64,
	season int,
) (SuiteReport, error) {
	return c.GetOrComputeMode(ctx, store, ModeSlips, groupID, season)
}

// GetOrComputeMode returns the cached SuiteReport for (mode, groupID, season),
// or replays and evaluates the dataset from store on a cache miss.
func (c *Cache) GetOrComputeMode(
	ctx context.Context,
	store *db.Store,
	mode ReplayMode,
	groupID int64,
	season int,
) (SuiteReport, error) {
	if mode == "" {
		mode = ModeSlips
	}
	if mode == ModeMarket {
		groupID = 0
	}
	key := cacheKey{mode: mode, groupID: groupID, season: season}

	c.mu.RLock()
	if report, ok := c.entries[key]; ok {
		c.mu.RUnlock()
		return report, nil
	}
	startGen := cacheGen{
		global: c.gen,
		group:  c.groupGen[groupID],
		market: c.marketGen,
	}
	c.mu.RUnlock()

	if store == nil {
		return SuiteReport{}, errors.New("eval cache: nil store")
	}

	dataset, err := LoadDataset(ctx, store, ReplayFilter{
		GroupID: groupID,
		Season:  season,
		Mode:    mode,
	})
	if err != nil {
		return SuiteReport{}, err
	}
	report := EvaluateAllEngines(dataset, analyzer.BuiltInEngines())

	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[cacheKey]SuiteReport)
	}
	genMatch := c.gen == startGen.global &&
		((mode == ModeSlips && c.groupGen[groupID] == startGen.group) ||
			(mode == ModeMarket && c.marketGen == startGen.market))
	if genMatch {
		c.entries[key] = report
	}
	c.mu.Unlock()

	return report, nil
}

// InvalidateGroup drops all cached ModeSlips seasons for groupID as well as any
// global (groupID 0) ModeSlips cached entries that aggregate across groups.
// ModeMarket entries are left intact.
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
		if key.mode == ModeSlips && (key.groupID == groupID || key.groupID == 0) {
			delete(c.entries, key)
		}
	}
}

// InvalidateMarket drops all cached ModeMarket entries without evicting
// ModeSlips entries.
func (c *Cache) InvalidateMarket() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.marketGen++
	for key := range c.entries {
		if key.mode == ModeMarket {
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
