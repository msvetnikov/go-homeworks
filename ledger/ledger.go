package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
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

var (
	transactions      = make([]Transaction, 0)
	budgets           = make(map[string]Budget)
	ErrBudgetExceeded = errors.New("budget exceeded")
)

func AddTransaction(tx Transaction) error {
	if tx.Amount <= 0 || math.IsNaN(tx.Amount) || math.IsInf(tx.Amount, 0) {
		return errors.New("transaction amount must be a positive finite number")
	}
	if strings.TrimSpace(tx.Category) == "" {
		return errors.New("transaction category must not be empty")
	}
	if budget, ok := budgets[tx.Category]; ok {
		total := tx.Amount
		for _, saved := range transactions {
			if saved.Category == tx.Category {
				total += saved.Amount
			}
		}
		if total > budget.Limit {
			return fmt.Errorf("%w: category %q, total %.2f, limit %.2f", ErrBudgetExceeded, tx.Category, total, budget.Limit)
		}
	}
	// Assign IDs only after validation so rejected transactions do not consume IDs.
	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)
	return nil
}

func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}

func SetBudget(b Budget) {
	budgets[b.Category] = b
}

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
	// Validate the entire file before changing the existing budgets.
	for i, b := range loaded {
		if strings.TrimSpace(b.Category) == "" {
			return fmt.Errorf("budget %d: category must not be empty", i+1)
		}
		if b.Limit < 0 || math.IsNaN(b.Limit) || math.IsInf(b.Limit, 0) {
			return fmt.Errorf("budget %d: limit must be a non-negative finite number", i+1)
		}
	}
	for _, b := range loaded {
		SetBudget(b)
	}
	return nil
}
