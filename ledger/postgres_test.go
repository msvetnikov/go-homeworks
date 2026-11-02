package ledger

import (
	"context"
	"errors"
	"os"
	"testing"
)

// The test needs a migrated PostgreSQL database and cleans its tables:
//
//	TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable go test ./...
//
// Set REDIS_ADDR as well to run it with the Redis cache.
func TestPostgresStorage(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	connect := func() {
		t.Helper()
		if err := Connect(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	connect()
	t.Cleanup(func() { Close() })
	resetLedger(t)

	if err := SetBudget(Budget{Category: "еда", Limit: 5000}); err != nil {
		t.Fatal(err)
	}
	if err := SetBudget(Budget{Category: "еда", Limit: 5000.5}); err != nil { // upsert, no duplicate
		t.Fatal(err)
	}
	for _, amount := range []float64{1200.25, 3800} {
		tx := Transaction{Amount: amount, Category: "еда", Description: "тест", Date: "2025-09-10"}
		if err := AddTransaction(&tx); err != nil {
			t.Fatal(err)
		}
		if tx.ID == 0 {
			t.Fatal("ID was not returned by INSERT ... RETURNING")
		}
	}
	if err := AddTransaction(&Transaction{Amount: 0.25, Category: "еда", Date: "2025-09-10"}); err != nil {
		t.Fatalf("limit 5000.50 is reached exactly: %v", err)
	}
	if err := AddTransaction(&Transaction{Amount: 0.01, Category: "еда", Date: "2025-09-10"}); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("want budget exceeded, got %v", err)
	}

	// Data must survive a reconnect (a restart of the service).
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	connect()
	budgets, err := ListBudgets()
	if err != nil || len(budgets) != 1 || budgets[0] != (Budget{Category: "еда", Limit: 5000.5}) {
		t.Fatalf("budgets after reconnect: %v, %v", budgets, err)
	}
	list, err := ListTransactions()
	if err != nil || len(list) != 3 || list[0].Date != "2025-09-10" || list[0].Amount != 0.25 || list[2].Amount != 1200.25 {
		t.Fatalf("transactions after reconnect: %v, %v", list, err)
	}
	report, err := GetReportSummary(context.Background(), "2025-09-01", "2025-09-30")
	if err != nil || report.Total != 5000.5 || len(report.Categories) != 1 {
		t.Fatalf("report: %+v, %v", report, err)
	}
}
