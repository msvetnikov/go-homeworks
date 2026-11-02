package ledger

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

const (
	reportCachePrefix = "report:summary:"
	budgetsCacheKey   = "budgets:all"

	reportCacheTTL  = 30 * time.Second
	budgetsCacheTTL = 15 * time.Second
)

// kvCache is a key-value cache with TTL. Redis implements it.
type kvCache interface {
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeletePrefix(ctx context.Context, prefix string) error
}

type CategoryTotal struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

// ReportSummary is the spending per category for a period, both ends included.
type ReportSummary struct {
	From       string          `json:"from"`
	To         string          `json:"to"`
	Total      float64         `json:"total"`
	Categories []CategoryTotal `json:"categories"`
}

// GetReportSummary sums the spending per category between from and to
// (YYYY-MM-DD, inclusive). The result is cached for a short time.
func GetReportSummary(ctx context.Context, from, to string) (ReportSummary, error) {
	fromDate, err := time.Parse(dateLayout, from)
	if err != nil {
		return ReportSummary{}, invalid("from must be a valid date in YYYY-MM-DD format")
	}
	toDate, err := time.Parse(dateLayout, to)
	if err != nil {
		return ReportSummary{}, invalid("to must be a valid date in YYYY-MM-DD format")
	}
	if fromDate.After(toDate) {
		return ReportSummary{}, invalid("from must not be after to")
	}

	key := reportCachePrefix + from + ":" + to
	var report ReportSummary
	if cacheGet(ctx, key, &report) {
		return report, nil
	}

	totals, err := current.categoryTotals(ctx, from, to)
	if err != nil {
		return ReportSummary{}, err
	}
	report = ReportSummary{From: from, To: to, Categories: totals}
	var sum int64
	for _, c := range totals {
		sum += toCents(c.Total)
	}
	report.Total = float64(sum) / 100
	cacheSet(ctx, key, report, reportCacheTTL)
	return report, nil
}

// The cache is best effort: its errors are logged and never returned, so a
// Redis outage only makes requests slower.

func cacheGet(ctx context.Context, key string, dst any) bool {
	if kv == nil {
		return false
	}
	raw, found, err := kv.Get(ctx, key)
	if err != nil {
		log.Printf("ledger: cache get %s: %v", key, err)
		return false
	}
	if !found {
		log.Printf("ledger: cache miss %s", key)
		return false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		log.Printf("ledger: cache entry %s is broken: %v", key, err)
		return false
	}
	log.Printf("ledger: cache hit %s", key)
	return true
}

func cacheSet(ctx context.Context, key string, value any, ttl time.Duration) {
	if kv == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		log.Printf("ledger: cache encode %s: %v", key, err)
		return
	}
	if err := kv.Set(ctx, key, raw, ttl); err != nil {
		log.Printf("ledger: cache set %s: %v", key, err)
	}
}

// invalidateCache drops the exact keys and every key that starts with one of
// the prefixes (names ending with ":").
func invalidateCache(ctx context.Context, names ...string) {
	if kv == nil {
		return
	}
	for _, name := range names {
		var err error
		if name[len(name)-1] == ':' {
			err = kv.DeletePrefix(ctx, name)
		} else {
			err = kv.Delete(ctx, name)
		}
		if err != nil {
			log.Printf("ledger: cache invalidate %s: %v", name, err)
		}
	}
}
