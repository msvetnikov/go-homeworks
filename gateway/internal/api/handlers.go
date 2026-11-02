package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"example.com/gateway-ledger/ledger"
)

const maxBodyBytes = 1 << 20

// NewHandler returns the handler of all /api/... routes.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/transactions", byMethod(map[string]http.HandlerFunc{
		http.MethodGet:  listTransactions,
		http.MethodPost: createTransaction,
	}))
	mux.HandleFunc("/api/budgets", byMethod(map[string]http.HandlerFunc{
		http.MethodGet:  listBudgets,
		http.MethodPost: createBudget,
	}))
	mux.HandleFunc("/api/reports/summary", byMethod(map[string]http.HandlerFunc{
		http.MethodGet: reportSummary,
	}))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	return mux
}

// byMethod picks the handler by HTTP method and answers 405 in JSON otherwise.
func byMethod(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	methods := make([]string, 0, len(handlers))
	for method := range handlers {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	allow := strings.Join(methods, ", ")
	return func(w http.ResponseWriter, r *http.Request) {
		handler, ok := handlers[r.Method]
		if !ok {
			w.Header().Set("Allow", allow)
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handler(w, r)
	}
}

// decodeJSON reads exactly one JSON object from the request body.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON: unexpected data after the object")
		return false
	}
	return true
}

// writeLedgerError maps errors of the ledger package to HTTP statuses.
func writeLedgerError(w http.ResponseWriter, err error) {
	var validationErr *ledger.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Msg)
	case errors.Is(err, ledger.ErrBudgetExceeded):
		writeError(w, http.StatusConflict, "budget exceeded")
	default:
		log.Printf("api: internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func createTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	tx := req.toDomain()
	if err := tx.Validate(); err != nil {
		writeLedgerError(w, err)
		return
	}
	if err := ledger.AddTransaction(&tx); err != nil {
		writeLedgerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newTransactionResponse(tx))
}

func listTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := ledger.ListTransactions()
	if err != nil {
		writeLedgerError(w, err)
		return
	}
	response := make([]TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		response = append(response, newTransactionResponse(tx))
	}
	writeJSON(w, http.StatusOK, response)
}

func createBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	budget := req.toDomain()
	if err := budget.Validate(); err != nil {
		writeLedgerError(w, err)
		return
	}
	if err := ledger.SetBudget(budget); err != nil {
		writeLedgerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newBudgetResponse(budget))
}

func listBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := ledger.ListBudgets()
	if err != nil {
		writeLedgerError(w, err)
		return
	}
	response := make([]BudgetResponse, 0, len(budgets))
	for _, b := range budgets {
		response = append(response, newBudgetResponse(b))
	}
	writeJSON(w, http.StatusOK, response)
}

func reportSummary(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	report, err := ledger.GetReportSummary(r.Context(), query.Get("from"), query.Get("to"))
	if err != nil {
		writeLedgerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newReportSummaryResponse(report))
}
