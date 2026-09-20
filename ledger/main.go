package main

import "fmt"

func main() {
	fmt.Println("Ledger service started")
	for _, tx := range []Transaction{
		{Amount: 1200, Category: "еда", Description: "Продукты", Date: "2026-10-05"},
		{Amount: 800, Category: "транспорт", Description: "Проездной", Date: "2026-10-05"},
		{Amount: 500, Category: "досуг", Description: "Кино", Date: "2026-10-05"},
	} {
		if err := AddTransaction(tx); err != nil {
			fmt.Printf("Отказ: %s — %v\n", tx.Description, err)
		}
	}
	fmt.Println("Сохранённые транзакции:")
	for _, tx := range ListTransactions() {
		fmt.Printf("ID=%d | %.2f руб. | %s | %s | %s\n", tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
