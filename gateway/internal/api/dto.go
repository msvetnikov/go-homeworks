// Package api contains the HTTP layer of the gateway: DTOs, handlers and middleware.
package api

import "example.com/gateway-ledger/ledger"

// CreateTransactionRequest is the JSON body of POST /api/transactions.
type CreateTransactionRequest struct {
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

// TransactionResponse shows the public fields of ledger.Transaction.
type TransactionResponse struct {
	ID          int     `json:"id"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

// CreateBudgetRequest is the JSON body of POST /api/budgets.
type CreateBudgetRequest struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

// BudgetResponse shows the public fields of ledger.Budget.
type BudgetResponse struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

type CategoryTotalResponse struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

// ReportSummaryResponse is the body of GET /api/reports/summary.
type ReportSummaryResponse struct {
	From       string                  `json:"from"`
	To         string                  `json:"to"`
	Total      float64                 `json:"total"`
	Categories []CategoryTotalResponse `json:"categories"`
}

// ErrorResponse is the single error format of the API.
type ErrorResponse struct {
	Error string `json:"error"`
}

func (r CreateTransactionRequest) toDomain() ledger.Transaction {
	return ledger.Transaction{Amount: r.Amount, Category: r.Category, Description: r.Description, Date: r.Date}
}

func (r CreateBudgetRequest) toDomain() ledger.Budget {
	return ledger.Budget{Category: r.Category, Limit: r.Limit}
}

func newTransactionResponse(tx ledger.Transaction) TransactionResponse {
	return TransactionResponse{ID: tx.ID, Amount: tx.Amount, Category: tx.Category, Description: tx.Description, Date: tx.Date}
}

func newBudgetResponse(b ledger.Budget) BudgetResponse {
	return BudgetResponse{Category: b.Category, Limit: b.Limit}
}

func newReportSummaryResponse(r ledger.ReportSummary) ReportSummaryResponse {
	categories := make([]CategoryTotalResponse, 0, len(r.Categories))
	for _, c := range r.Categories {
		categories = append(categories, CategoryTotalResponse{Category: c.Category, Total: c.Total})
	}
	return ReportSummaryResponse{From: r.From, To: r.To, Total: r.Total, Categories: categories}
}
