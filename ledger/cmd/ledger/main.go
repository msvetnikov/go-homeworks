package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"example.com/gateway-ledger/ledger"
)

func main() {
	budgetsFile := flag.String("budgets", "budgets.json", "JSON file with budgets to load at startup")
	flag.Parse()

	fmt.Println("Ledger service started")
	demoValidation()

	ctx := context.Background()
	if err := ledger.Connect(ctx); err != nil {
		log.Fatalf("cannot start Ledger: %v\nCheck that PostgreSQL is running, migrations are applied and DATABASE_URL (or DB_*) is set.", err)
	}
	defer ledger.Close()

	if err := loadBudgets(*budgetsFile); err != nil {
		log.Fatal(err)
	}

	budgets, err := ledger.ListBudgets()
	if err != nil {
		log.Fatalf("list budgets: %v", err)
	}
	fmt.Println("Бюджеты:")
	for _, b := range budgets {
		fmt.Printf("%s: лимит %.2f руб.\n", b.Category, b.Limit)
	}

	transactions, err := ledger.ListTransactions()
	if err != nil {
		log.Fatalf("list transactions: %v", err)
	}
	fmt.Printf("Транзакций в базе: %d\n", len(transactions))
	for _, tx := range transactions {
		fmt.Printf("ID=%d | %.2f руб. | %s | %s | %s\n", tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}

	// The second call is served from Redis (see "cache hit" in the log).
	for i := 1; i <= 2; i++ {
		report, err := ledger.GetReportSummary(ctx, "1970-01-01", "2999-12-31")
		if err != nil {
			log.Fatalf("report summary: %v", err)
		}
		fmt.Printf("Отчёт, вызов %d: всего %.2f руб., категорий %d\n", i, report.Total, len(report.Categories))
	}
}

// demoValidation shows that CheckValid works with different types
// through the Validatable interface.
func demoValidation() {
	for name, v := range map[string]ledger.Validatable{
		"транзакция":             ledger.Transaction{Amount: 450, Category: "еда", Date: "2025-09-10"},
		"транзакция (сумма 0)":   ledger.Transaction{Amount: 0, Category: "еда", Date: "2025-09-10"},
		"бюджет":                 ledger.Budget{Category: "еда", Limit: 5000},
		"бюджет (без категории)": ledger.Budget{Limit: 5000},
	} {
		if err := ledger.CheckValid(v); err != nil {
			fmt.Printf("%s: ошибка — %v\n", name, err)
			continue
		}
		fmt.Printf("%s: корректна\n", name)
	}
}

func loadBudgets(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Файл %s не найден, бюджеты остаются как в базе\n", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	if err := ledger.LoadBudgets(file); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	fmt.Printf("Бюджеты загружены из %s\n", path)
	return nil
}
