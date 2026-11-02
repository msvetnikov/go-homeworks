package ledger

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// resetLedger isolates a test from the others: the shared storage is cleaned
// before the test and after it. Tests that use it must not run in parallel.
func resetLedger(t *testing.T) {
	t.Helper()
	if err := Reset(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Reset(); err != nil {
			t.Error(err)
		}
	})
}

func TestTransactionValidate(t *testing.T) {
	t.Parallel()
	valid := Transaction{Amount: 450, Category: "еда", Description: "ланч", Date: "2025-09-10"}
	for _, tc := range []struct {
		name   string
		mutate func(*Transaction)
		wantOK bool
	}{
		{"valid", func(*Transaction) {}, true},
		{"valid without description", func(tx *Transaction) { tx.Description = "" }, true},
		{"cents", func(tx *Transaction) { tx.Amount = 0.01 }, true},
		{"zero amount", func(tx *Transaction) { tx.Amount = 0 }, false},
		{"negative amount", func(tx *Transaction) { tx.Amount = -5 }, false},
		{"less than a cent", func(tx *Transaction) { tx.Amount = 0.001 }, false},
		{"too large", func(tx *Transaction) { tx.Amount = 1e13 }, false},
		{"NaN", func(tx *Transaction) { tx.Amount = math.NaN() }, false},
		{"infinity", func(tx *Transaction) { tx.Amount = math.Inf(1) }, false},
		{"empty category", func(tx *Transaction) { tx.Category = "" }, false},
		{"blank category", func(tx *Transaction) { tx.Category = "   " }, false},
		{"empty date", func(tx *Transaction) { tx.Date = "" }, false},
		{"wrong date format", func(tx *Transaction) { tx.Date = "10.09.2025" }, false},
		{"date that does not exist", func(tx *Transaction) { tx.Date = "2025-02-30" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tx := valid
			tc.mutate(&tx)
			err := tx.Validate()
			if tc.wantOK && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantOK {
				var validationErr *ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("want ValidationError, got %v", err)
				}
			}
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		budget Budget
		wantOK bool
	}{
		{"valid", Budget{Category: "еда", Limit: 5000}, true},
		{"zero limit", Budget{Category: "еда", Limit: 0}, false},
		{"negative limit", Budget{Category: "еда", Limit: -1}, false},
		{"NaN limit", Budget{Category: "еда", Limit: math.NaN()}, false},
		{"empty category", Budget{Category: "", Limit: 100}, false},
		{"blank category", Budget{Category: " ", Limit: 100}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.budget.Validate(); (err == nil) != tc.wantOK {
				t.Fatalf("Validate() = %v, want ok=%v", err, tc.wantOK)
			}
		})
	}
}

func TestCheckValidWorksWithAnyValidatable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		value  Validatable
		wantOK bool
	}{
		{"transaction", Transaction{Amount: 1, Category: "еда", Date: "2025-09-10"}, true},
		{"transaction pointer", &Transaction{Amount: 1, Category: "еда", Date: "2025-09-10"}, true},
		{"bad transaction", Transaction{Category: "еда", Date: "2025-09-10"}, false},
		{"budget", Budget{Category: "еда", Limit: 10}, true},
		{"bad budget", Budget{Category: "еда"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := CheckValid(tc.value); (err == nil) != tc.wantOK {
				t.Fatalf("CheckValid() = %v, want ok=%v", err, tc.wantOK)
			}
		})
	}
}

func TestBudgetLimitsTransactions(t *testing.T) {
	resetLedger(t)
	if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
		t.Fatal(err)
	}

	t.Run("within the limit", func(t *testing.T) {
		for _, amount := range []float64{1200, 3800} {
			tx := Transaction{Amount: amount, Category: "еда", Date: "2025-09-10"}
			if err := AddTransaction(&tx); err != nil {
				t.Fatal(err)
			}
			if tx.ID == 0 {
				t.Fatal("ID was not set")
			}
		}
	})

	t.Run("over the limit", func(t *testing.T) {
		tx := Transaction{Amount: 0.01, Category: "еда", Date: "2025-09-10"}
		if err := AddTransaction(&tx); !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("want budget exceeded, got %v", err)
		}
		list, err := ListTransactions()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 {
			t.Fatalf("rejected transaction was saved: %d transactions", len(list))
		}
	})

	t.Run("raised limit", func(t *testing.T) {
		if err := SetBudget(Budget{Category: "еда", Limit: 6000}); err != nil {
			t.Fatal(err)
		}
		tx := Transaction{Amount: 1000, Category: "еда", Date: "2025-09-11"}
		if err := AddTransaction(&tx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("category without a budget", func(t *testing.T) {
		tx := Transaction{Amount: 9000, Category: "досуг", Date: "2025-09-11"}
		if err := AddTransaction(&tx); err != nil {
			t.Fatalf("category without a budget has no limit: %v", err)
		}
	})

	t.Run("limit is compared in cents", func(t *testing.T) {
		if err := SetBudget(Budget{Category: "кофе", Limit: 0.3}); err != nil {
			t.Fatal(err)
		}
		for _, amount := range []float64{0.1, 0.2} {
			tx := Transaction{Amount: amount, Category: "кофе", Date: "2025-09-10"}
			if err := AddTransaction(&tx); err != nil {
				t.Fatalf("0.1 + 0.2 must fit the limit 0.3: %v", err)
			}
		}
	})
}

func TestInvalidInputIsNotSaved(t *testing.T) {
	resetLedger(t)
	for _, tx := range []Transaction{
		{Amount: 0, Category: "еда", Date: "2025-09-10"},
		{Amount: -1, Category: "еда", Date: "2025-09-10"},
		{Amount: 1, Category: "  ", Date: "2025-09-10"},
		{Amount: 1, Category: "еда", Date: "yesterday"},
	} {
		if err := AddTransaction(&tx); err == nil {
			t.Fatalf("invalid transaction %+v was accepted", tx)
		}
	}
	if err := AddTransaction(nil); err == nil {
		t.Fatal("nil transaction was accepted")
	}
	if err := SetBudget(Budget{Category: "еда", Limit: 0}); err == nil {
		t.Fatal("zero limit was accepted")
	}
	list, _ := ListTransactions()
	budgets, _ := ListBudgets()
	if len(list) != 0 || len(budgets) != 0 {
		t.Fatalf("invalid data was saved: %v %v", list, budgets)
	}
}

func TestListTransactionsNewestFirst(t *testing.T) {
	resetLedger(t)
	for _, tx := range []Transaction{
		{Amount: 1, Category: "а", Date: "2025-09-10"},
		{Amount: 2, Category: "а", Date: "2025-09-12"},
		{Amount: 3, Category: "а", Date: "2025-09-10"},
	} {
		if err := AddTransaction(&tx); err != nil {
			t.Fatal(err)
		}
	}
	list, err := ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	var got []float64
	for _, tx := range list {
		got = append(got, tx.Amount)
	}
	if want := []float64{2, 3, 1}; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("order by date DESC, id DESC expected %v, got %v", want, got)
	}
	list[0].Amount = 99999
	if again, _ := ListTransactions(); again[0].Amount != 2 {
		t.Fatal("ListTransactions exposed the underlying storage")
	}
}

func TestLoadBudgets(t *testing.T) {
	resetLedger(t)
	if err := SetBudget(Budget{Category: "еда", Limit: 10}); err != nil {
		t.Fatal(err)
	}
	if err := LoadBudgets(strings.NewReader(`[{"category":"еда","limit":5000},{"category":"транспорт","limit":2000}]`)); err != nil {
		t.Fatal(err)
	}
	budgets, err := ListBudgets()
	if err != nil {
		t.Fatal(err)
	}
	if len(budgets) != 2 || budgets[0].Category != "еда" || budgets[0].Limit != 5000 || budgets[1].Limit != 2000 {
		t.Fatalf("unexpected budgets: %v", budgets)
	}
	if err := AddTransaction(&Transaction{Amount: 5001, Category: "еда", Date: "2025-09-10"}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("loaded budget was not applied: %v", err)
	}
	if err := LoadBudgets(strings.NewReader(`[]`)); err != nil {
		t.Fatalf("empty array should be accepted: %v", err)
	}
}

func TestLoadBudgetsRejectsInvalidJSONWithoutChanges(t *testing.T) {
	for _, input := range []string{
		``, `null`, `{}`, `[`, `[{"category":"еда","limit":"5000"}]`,
		`[{"category":"еда","limit":1}] []`,
		`[{"category":"еда","limit":1},{"category":"транспорт","limit":-1}]`,
		`[{"category":"еда","limit":1},{"category":"транспорт","limit":0}]`,
		`[{"category":" ","limit":100}]`,
	} {
		t.Run(input, func(t *testing.T) {
			resetLedger(t)
			if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
				t.Fatal(err)
			}
			if err := LoadBudgets(strings.NewReader(input)); err == nil {
				t.Fatal("invalid input was accepted")
			}
			budgets, _ := ListBudgets()
			if len(budgets) != 1 || budgets[0].Limit != 5000 {
				t.Fatalf("failed loading changed the budgets: %v", budgets)
			}
		})
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestLoadBudgetsReadError(t *testing.T) {
	readErr := errors.New("reader failed")
	err := LoadBudgets(failingReader{err: readErr})
	if !errors.Is(err, readErr) || !strings.Contains(err.Error(), "read budgets") {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
}

func TestGetReportSummary(t *testing.T) {
	resetLedger(t)
	for _, tx := range []Transaction{
		{Amount: 100, Category: "еда", Date: "2025-09-01"},
		{Amount: 250.5, Category: "еда", Date: "2025-09-30"},
		{Amount: 40, Category: "транспорт", Date: "2025-09-15"},
		{Amount: 999, Category: "еда", Date: "2025-10-01"},
	} {
		if err := AddTransaction(&tx); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("period is inclusive and grouped by category", func(t *testing.T) {
		report, err := GetReportSummary(context.Background(), "2025-09-01", "2025-09-30")
		if err != nil {
			t.Fatal(err)
		}
		if report.Total != 390.5 || len(report.Categories) != 2 ||
			report.Categories[0] != (CategoryTotal{Category: "еда", Total: 350.5}) ||
			report.Categories[1] != (CategoryTotal{Category: "транспорт", Total: 40}) {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("empty period", func(t *testing.T) {
		report, err := GetReportSummary(context.Background(), "2024-01-01", "2024-01-31")
		if err != nil {
			t.Fatal(err)
		}
		if report.Total != 0 || report.Categories == nil || len(report.Categories) != 0 {
			t.Fatalf("unexpected report: %+v", report)
		}
	})

	t.Run("invalid period", func(t *testing.T) {
		for _, p := range [][2]string{{"", "2025-09-30"}, {"2025-09-01", "x"}, {"2025-09-30", "2025-09-01"}} {
			_, err := GetReportSummary(context.Background(), p[0], p[1])
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("period %v: want ValidationError, got %v", p, err)
			}
		}
	})
}

// fakeCache is an in-memory kvCache that remembers how it was used.
type fakeCache struct {
	mu      sync.Mutex
	values  map[string][]byte
	ttls    map[string]time.Duration
	deleted []string
}

func newFakeCache() *fakeCache {
	return &fakeCache{values: map[string][]byte{}, ttls: map[string]time.Duration{}}
}

func (c *fakeCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.values[key]
	return v, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key], c.ttls[key] = value, ttl
	return nil
}

func (c *fakeCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, key := range keys {
		delete(c.values, key)
		c.deleted = append(c.deleted, key)
	}
	return nil
}

func (c *fakeCache) DeletePrefix(_ context.Context, prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.values {
		if strings.HasPrefix(key, prefix) {
			delete(c.values, key)
		}
	}
	c.deleted = append(c.deleted, prefix+"*")
	return nil
}

func useFakeCache(t *testing.T) *fakeCache {
	t.Helper()
	fake := newFakeCache()
	kv = fake
	t.Cleanup(func() { kv = nil })
	return fake
}

func TestReportSummaryIsCached(t *testing.T) {
	resetLedger(t)
	fake := useFakeCache(t)
	ctx := context.Background()
	if err := AddTransaction(&Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"}); err != nil {
		t.Fatal(err)
	}

	first, err := GetReportSummary(ctx, "2025-09-01", "2025-09-30") // miss, then set
	if err != nil {
		t.Fatal(err)
	}
	key := "report:summary:2025-09-01:2025-09-30"
	if _, ok := fake.values[key]; !ok || fake.ttls[key] != reportCacheTTL {
		t.Fatalf("report was not cached with TTL: %v %v", fake.values, fake.ttls)
	}

	// Write to the storage behind the cache's back: a hit must not see it.
	if _, err := current.addTransaction(ctx, Transaction{Amount: 50, Category: "еда", Date: "2025-09-11"}); err != nil {
		t.Fatal(err)
	}
	second, err := GetReportSummary(ctx, "2025-09-01", "2025-09-30") // hit
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != first.Total || second.Total != 100 {
		t.Fatalf("second call did not come from the cache: %+v vs %+v", first, second)
	}

	// A transaction added through the package drops the cached reports.
	if err := AddTransaction(&Transaction{Amount: 1, Category: "еда", Date: "2025-09-12"}); err != nil {
		t.Fatal(err)
	}
	third, err := GetReportSummary(ctx, "2025-09-01", "2025-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if third.Total != 151 {
		t.Fatalf("cache was not invalidated: %+v", third)
	}
}

func TestBudgetsListIsCached(t *testing.T) {
	resetLedger(t)
	fake := useFakeCache(t)
	if err := SetBudget(Budget{Category: "еда", Limit: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := ListBudgets(); err != nil { // miss, then set
		t.Fatal(err)
	}
	if fake.ttls[budgetsCacheKey] != budgetsCacheTTL {
		t.Fatal("budgets were not cached with TTL")
	}
	if err := current.setBudget(context.Background(), Budget{Category: "транспорт", Limit: 5}); err != nil {
		t.Fatal(err)
	}
	if cached, _ := ListBudgets(); len(cached) != 1 { // hit
		t.Fatalf("second call did not come from the cache: %v", cached)
	}
	if err := SetBudget(Budget{Category: "еда", Limit: 200}); err != nil { // DEL budgets:all
		t.Fatal(err)
	}
	if fresh, _ := ListBudgets(); len(fresh) != 2 || fresh[0].Limit != 200 {
		t.Fatalf("cache was not invalidated: %v", fresh)
	}
}
