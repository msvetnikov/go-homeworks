package main

import "testing"

func TestTransactions(t *testing.T) {
	transactions = make([]Transaction, 0)
	if err := AddTransaction(Transaction{Amount: 0, Category: "еда"}); err == nil {
		t.Fatal("zero amount was accepted")
	}
	for _, tx := range []Transaction{
		{Amount: 1200, Category: "еда"},
		{Amount: 800, Category: "транспорт"},
	} {
		if err := AddTransaction(tx); err != nil {
			t.Fatal(err)
		}
	}
	list := ListTransactions()
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
		t.Fatalf("unexpected transactions: %v", list)
	}
	list[0].Amount = 99999
	if ListTransactions()[0].Amount != 1200 {
		t.Fatal("ListTransactions exposed the underlying slice")
	}
}
