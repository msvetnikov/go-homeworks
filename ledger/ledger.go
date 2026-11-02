// Package ledger keeps transactions and category budgets.
//
// Data lives in PostgreSQL after Connect is called; before that (for example
// in unit tests) an in-memory store is used. Report summaries and the budget
// list are cached in Redis when it is available.
package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"example.com/gateway-ledger/ledger/internal/cache"
	"example.com/gateway-ledger/ledger/internal/db"
)

type Transaction struct {
	ID          int     `json:"id"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

// Budget is a fixed limit for a category, without a monthly reset.
type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

var ErrBudgetExceeded = errors.New("budget exceeded")

const opTimeout = 5 * time.Second

// store is where transactions and budgets are kept.
type store interface {
	addTransaction(ctx context.Context, tx Transaction) (id int, err error)
	listTransactions(ctx context.Context) ([]Transaction, error)
	setBudget(ctx context.Context, b Budget) error
	listBudgets(ctx context.Context) ([]Budget, error)
	categoryTotals(ctx context.Context, from, to string) ([]CategoryTotal, error)
	reset(ctx context.Context) error
	close() error
}

var (
	current store = newMemStore()
	// kv is nil when Redis is not configured or not reachable.
	kv kvCache
)

// Connect opens PostgreSQL (required) and Redis (optional) using environment
// variables and makes them the storage of the package.
func Connect(ctx context.Context) error {
	sqlDB, err := db.Open(ctx)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL (%s): %w", db.Describe(), err)
	}
	log.Printf("ledger: connected to PostgreSQL (%s)", db.Describe())
	current = &pgStore{db: sqlDB}

	redisCache, err := cache.Open(ctx)
	if err != nil {
		log.Printf("ledger: Redis is unavailable, caching is disabled: %v", err)
		return nil
	}
	log.Printf("ledger: connected to Redis (%s)", cache.Describe())
	kv = redisCache
	return nil
}

// Close releases database and Redis connections.
func Close() error {
	var errs []error
	if err := current.close(); err != nil {
		errs = append(errs, err)
	}
	if c, ok := kv.(interface{ Close() error }); ok {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	current, kv = newMemStore(), nil
	return errors.Join(errs...)
}

// Reset removes all transactions and budgets. It is meant for tests.
func Reset() error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := current.reset(ctx); err != nil {
		return err
	}
	invalidateCache(ctx, budgetsCacheKey, reportCachePrefix)
	return nil
}

// AddTransaction validates and saves tx and stores the generated ID in tx.ID.
// A transaction that would push its category over the budget is rejected
// with ErrBudgetExceeded and is not saved.
func AddTransaction(tx *Transaction) error {
	if tx == nil {
		return errors.New("transaction must not be nil")
	}
	if err := tx.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	id, err := current.addTransaction(ctx, *tx)
	if err != nil {
		return err
	}
	tx.ID = id
	// Reports for any period may have changed.
	invalidateCache(ctx, reportCachePrefix)
	return nil
}

func ListTransactions() ([]Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	return current.listTransactions(ctx)
}

// SetBudget creates the budget of a category or updates its limit.
func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := current.setBudget(ctx, b); err != nil {
		return err
	}
	invalidateCache(ctx, budgetsCacheKey)
	return nil
}

func ListBudgets() ([]Budget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	var budgets []Budget
	if cacheGet(ctx, budgetsCacheKey, &budgets) {
		return budgets, nil
	}
	budgets, err := current.listBudgets(ctx)
	if err != nil {
		return nil, err
	}
	cacheSet(ctx, budgetsCacheKey, budgets, budgetsCacheTTL)
	return budgets, nil
}

// LoadBudgets reads a JSON array of budgets and saves them. The whole input
// is validated before anything is changed.
func LoadBudgets(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read budgets: %w", err)
	}
	var loaded []Budget
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parse budgets JSON (expected an array): %w", err)
	}
	if loaded == nil {
		return errors.New("budgets JSON must be an array, not null")
	}
	for i, b := range loaded {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("budget %d: %w", i+1, err)
		}
	}
	for _, b := range loaded {
		if err := SetBudget(b); err != nil {
			return err
		}
	}
	return nil
}
