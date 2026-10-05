package main

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func resetStore(t *testing.T) {
	t.Helper()
	transactions = make([]Transaction, 0)
	budgets = make(map[string]Budget)
}

func TestBudgetAndTransactions(t *testing.T) {
	resetStore(t)
	SetBudget(Budget{Category: "еда", Limit: 5000})
	for _, amount := range []float64{1200, 3800} {
		if err := AddTransaction(Transaction{Amount: amount, Category: "еда"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := AddTransaction(Transaction{Amount: 1, Category: "еда"}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("want budget exceeded, got %v", err)
	}
	if len(ListTransactions()) != 2 {
		t.Fatal("rejected transaction was saved")
	}
	SetBudget(Budget{Category: "еда", Limit: 6000})
	if err := AddTransaction(Transaction{ID: 99, Amount: 500, Category: "еда"}); err != nil {
		t.Fatal(err)
	}
	if err := AddTransaction(Transaction{Amount: 9000, Category: "досуг"}); err != nil {
		t.Fatalf("category without a budget should be allowed: %v", err)
	}
	list := ListTransactions()
	for i, tx := range list {
		if tx.ID != i+1 {
			t.Fatalf("unexpected ID: %d", tx.ID)
		}
	}
	list[0].Amount = 99999
	if ListTransactions()[0].Amount != 1200 {
		t.Fatal("ListTransactions exposed the underlying slice")
	}
}

func TestInvalidTransaction(t *testing.T) {
	resetStore(t)
	for _, amount := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := AddTransaction(Transaction{Amount: amount, Category: "еда"}); err == nil {
			t.Fatalf("invalid amount %v was accepted", amount)
		}
	}
	if err := AddTransaction(Transaction{Amount: 1, Category: "  "}); err == nil {
		t.Fatal("empty category was accepted")
	}
	if len(ListTransactions()) != 0 {
		t.Fatal("invalid transactions were saved")
	}
}

func TestLoadBudgets(t *testing.T) {
	resetStore(t)
	SetBudget(Budget{Category: "еда", Limit: 10})
	if err := LoadBudgets(strings.NewReader(`[{"category":"еда","limit":5000},{"category":"транспорт","limit":2000}]`)); err != nil {
		t.Fatal(err)
	}
	if len(budgets) != 2 || budgets["еда"].Limit != 5000 || budgets["транспорт"].Limit != 2000 {
		t.Fatalf("unexpected budgets: %v", budgets)
	}
	if err := AddTransaction(Transaction{Amount: 5001, Category: "еда"}); !errors.Is(err, ErrBudgetExceeded) {
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
		`[{"category":" ","limit":100}]`,
	} {
		t.Run(input, func(t *testing.T) {
			resetStore(t)
			SetBudget(Budget{Category: "еда", Limit: 5000})
			if err := LoadBudgets(strings.NewReader(input)); err == nil {
				t.Fatal("invalid input was accepted")
			}
			if len(budgets) != 1 || budgets["еда"].Limit != 5000 {
				t.Fatal("failed loading changed the budgets")
			}
		})
	}
}

type failingReader struct{ err error }

func (r failingReader) Read(p []byte) (int, error) { return 0, r.err }

func TestLoadBudgetsReadError(t *testing.T) {
	resetStore(t)
	readErr := errors.New("reader failed")
	err := LoadBudgets(failingReader{err: readErr})
	if !errors.Is(err, readErr) || !strings.Contains(err.Error(), "read budgets") {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
}
