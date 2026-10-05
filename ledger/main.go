package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

func main() {
	fmt.Println("Ledger service started")
	SetBudget(Budget{Category: "еда", Limit: 5000})
	SetBudget(Budget{Category: "транспорт", Limit: 2000})

	file, err := os.Open("budgets.json")
	if err != nil {
		log.Fatalf("open budgets.json: %v", err)
	}
	loadErr := LoadBudgets(bufio.NewReader(file))
	closeErr := file.Close()
	if loadErr != nil {
		log.Fatal(loadErr)
	}
	if closeErr != nil {
		log.Fatalf("close budgets.json: %v", closeErr)
	}
	fmt.Println("Бюджеты загружены из budgets.json")

	for _, tx := range []Transaction{
		{Amount: 1200, Category: "еда", Description: "Продукты", Date: "2026-10-05"},
		{Amount: 800, Category: "транспорт", Description: "Проездной", Date: "2026-10-05"},
		{Amount: 3800, Category: "еда", Description: "Покупки до лимита бюджета", Date: "2026-10-05"},
		{Amount: 100, Category: "еда", Description: "Покупка сверх бюджета", Date: "2026-10-05"},
		{Amount: 0, Category: "транспорт", Description: "Нулевая сумма", Date: "2026-10-05"},
	} {
		if err := AddTransaction(tx); err != nil {
			fmt.Printf("Отказ: %s — %v\n", tx.Description, err)
			continue
		}
		fmt.Printf("Добавлено: %s, %.2f руб., категория %s\n", tx.Description, tx.Amount, tx.Category)
	}

	fmt.Println("Сохранённые транзакции:")
	for _, tx := range ListTransactions() {
		fmt.Printf("ID=%d | %.2f руб. | %s | %s | %s\n", tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
