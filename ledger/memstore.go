package ledger

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// memStore keeps everything in memory. It is used until Connect is called.
type memStore struct {
	mu           sync.Mutex
	transactions []Transaction
	budgets      map[string]Budget
	nextID       int
}

func newMemStore() *memStore {
	return &memStore{budgets: make(map[string]Budget), nextID: 1}
}

func (s *memStore) addTransaction(_ context.Context, tx Transaction) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if budget, ok := s.budgets[tx.Category]; ok {
		spent := toCents(tx.Amount)
		for _, saved := range s.transactions {
			if saved.Category == tx.Category {
				spent += toCents(saved.Amount)
			}
		}
		if spent > toCents(budget.Limit) {
			return 0, fmt.Errorf("%w: category %q, total %.2f, limit %.2f",
				ErrBudgetExceeded, tx.Category, float64(spent)/100, budget.Limit)
		}
	}
	// The ID is assigned only after the checks, so rejected transactions do not consume IDs.
	tx.ID = s.nextID
	s.nextID++
	s.transactions = append(s.transactions, tx)
	return tx.ID, nil
}

func (s *memStore) listTransactions(context.Context) ([]Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Transaction, len(s.transactions))
	copy(result, s.transactions)
	// Same order as the SQL query: newest first.
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Date != result[j].Date {
			return result[i].Date > result[j].Date
		}
		return result[i].ID > result[j].ID
	})
	return result, nil
}

func (s *memStore) setBudget(_ context.Context, b Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.budgets[b.Category] = b
	return nil
}

func (s *memStore) listBudgets(context.Context) ([]Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Budget, 0, len(s.budgets))
	for _, b := range s.budgets {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Category < result[j].Category })
	return result, nil
}

func (s *memStore) categoryTotals(_ context.Context, from, to string) ([]CategoryTotal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cents := make(map[string]int64)
	for _, tx := range s.transactions {
		// YYYY-MM-DD strings compare in date order.
		if tx.Date >= from && tx.Date <= to {
			cents[tx.Category] += toCents(tx.Amount)
		}
	}
	result := make([]CategoryTotal, 0, len(cents))
	for category, sum := range cents {
		result = append(result, CategoryTotal{Category: category, Total: float64(sum) / 100})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Category < result[j].Category })
	return result, nil
}

func (s *memStore) reset(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions = nil
	s.budgets = make(map[string]Budget)
	s.nextID = 1
	return nil
}

func (s *memStore) close() error { return nil }
