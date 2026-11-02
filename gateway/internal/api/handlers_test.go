package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/gateway-ledger/ledger"
)

// The ledger package keeps its data in shared storage, so every test cleans it
// and the tests in this package do not run in parallel.
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	if err := ledger.Reset(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ledger.Reset(); err != nil {
			t.Error(err)
		}
	})
	return Logging(NewHandler())
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	request := httptest.NewRequest(method, path, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	return response
}

func assertJSON(t *testing.T, response *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, status, response.Body)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var got, expected any
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %q", response.Body)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(expected)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("body = %s, want %s", gotJSON, wantJSON)
	}
}

func TestBudgets(t *testing.T) {
	h := newTestHandler(t)

	t.Run("empty list is an empty array", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodGet, "/api/budgets", ""), http.StatusOK, `[]`)
	})
	t.Run("create", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`),
			http.StatusCreated, `{"category":"еда","limit":5000}`)
	})
	t.Run("created budget is listed", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodGet, "/api/budgets", ""), http.StatusOK, `[{"category":"еда","limit":5000}]`)
	})
	t.Run("update", func(t *testing.T) {
		do(t, h, http.MethodPost, "/api/budgets", `{"category":"еда","limit":7000}`)
		assertJSON(t, do(t, h, http.MethodGet, "/api/budgets", ""), http.StatusOK, `[{"category":"еда","limit":7000}]`)
	})
	for name, body := range map[string]string{
		"negative limit": `{"category":"еда","limit":-1}`,
		"zero limit":     `{"category":"еда","limit":0}`,
		"empty category": `{"category":"","limit":100}`,
		"missing fields": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := do(t, h, http.MethodPost, "/api/budgets", body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
			assertJSON(t, response, http.StatusBadRequest, response.Body.String())
			var errBody ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &errBody); err != nil || errBody.Error == "" {
				t.Fatalf("expected {\"error\":\"...\"}, got %s", response.Body)
			}
		})
	}
}

func TestTransactions(t *testing.T) {
	h := newTestHandler(t)
	const create = `{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}`

	t.Run("empty list is an empty array", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodGet, "/api/transactions", ""), http.StatusOK, `[]`)
	})
	t.Run("create", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodPost, "/api/transactions", create), http.StatusCreated,
			`{"id":1,"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}`)
	})
	t.Run("created transaction is listed", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodGet, "/api/transactions", ""), http.StatusOK,
			`[{"id":1,"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}]`)
	})
}

func TestTransactionErrors(t *testing.T) {
	h := newTestHandler(t)
	do(t, h, http.MethodPost, "/api/budgets", `{"category":"еда","limit":1000}`)
	if response := do(t, h, http.MethodPost, "/api/transactions", `{"amount":900,"category":"еда","date":"2025-09-10"}`); response.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d %s", response.Code, response.Body)
	}

	t.Run("budget exceeded", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodPost, "/api/transactions", `{"amount":101,"category":"еда","date":"2025-09-10"}`),
			http.StatusConflict, `{"error":"budget exceeded"}`)
		assertJSON(t, do(t, h, http.MethodGet, "/api/transactions", ""), http.StatusOK,
			`[{"id":1,"amount":900,"category":"еда","description":"","date":"2025-09-10"}]`)
	})

	for name, body := range map[string]string{
		"zero amount":       `{"amount":0,"category":"еда","date":"2025-09-10"}`,
		"negative amount":   `{"amount":-5,"category":"еда","date":"2025-09-10"}`,
		"empty category":    `{"amount":5,"category":"","date":"2025-09-10"}`,
		"bad date":          `{"amount":5,"category":"еда","date":"10.09.2025"}`,
		"broken JSON":       `{"amount":`,
		"not JSON":          `hello`,
		"wrong field type":  `{"amount":"много","category":"еда","date":"2025-09-10"}`,
		"unknown field":     `{"amount":5,"category":"еда","date":"2025-09-10","extra":1}`,
		"data after object": `{"amount":5,"category":"еда","date":"2025-09-10"} {}`,
		"empty body":        ``,
	} {
		t.Run(name, func(t *testing.T) {
			response := do(t, h, http.MethodPost, "/api/transactions", body)
			var errBody ErrorResponse
			if response.Code != http.StatusBadRequest || json.Unmarshal(response.Body.Bytes(), &errBody) != nil || errBody.Error == "" {
				t.Fatalf("want 400 with an error, got %d %s", response.Code, response.Body)
			}
			if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
				t.Fatalf("Content-Type = %q", got)
			}
		})
	}
}

func TestReportSummary(t *testing.T) {
	h := newTestHandler(t)
	do(t, h, http.MethodPost, "/api/transactions", `{"amount":100,"category":"еда","date":"2025-09-10"}`)
	do(t, h, http.MethodPost, "/api/transactions", `{"amount":50,"category":"еда","date":"2025-09-11"}`)
	do(t, h, http.MethodPost, "/api/transactions", `{"amount":30,"category":"транспорт","date":"2025-10-01"}`)

	t.Run("period", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodGet, "/api/reports/summary?from=2025-09-01&to=2025-09-30", ""), http.StatusOK,
			`{"from":"2025-09-01","to":"2025-09-30","total":150,"categories":[{"category":"еда","total":150}]}`)
	})
	t.Run("empty period", func(t *testing.T) {
		assertJSON(t, do(t, h, http.MethodGet, "/api/reports/summary?from=2024-01-01&to=2024-01-31", ""), http.StatusOK,
			`{"from":"2024-01-01","to":"2024-01-31","total":0,"categories":[]}`)
	})
	for name, query := range map[string]string{
		"no period":    "",
		"bad date":     "?from=x&to=2025-09-30",
		"reversed":     "?from=2025-09-30&to=2025-09-01",
		"missing 'to'": "?from=2025-09-01",
	} {
		t.Run(name, func(t *testing.T) {
			response := do(t, h, http.MethodGet, "/api/reports/summary"+query, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; %s", response.Code, response.Body)
			}
		})
	}
}

func TestRouting(t *testing.T) {
	h := newTestHandler(t)
	for _, tc := range []struct {
		name, method, path string
		status             int
		allow              string
	}{
		{"unknown api path", http.MethodGet, "/api/unknown", http.StatusNotFound, ""},
		{"delete transactions", http.MethodDelete, "/api/transactions", http.StatusMethodNotAllowed, "GET, POST"},
		{"post report", http.MethodPost, "/api/reports/summary", http.StatusMethodNotAllowed, "GET"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := do(t, h, tc.method, tc.path, "")
			assertJSON(t, response, tc.status, response.Body.String())
			if tc.allow != "" && response.Header().Get("Allow") != tc.allow {
				t.Fatalf("Allow = %q, want %q", response.Header().Get("Allow"), tc.allow)
			}
		})
	}
}

func TestInternalErrorIsHiddenFromClient(t *testing.T) {
	response := httptest.NewRecorder()
	log.SetOutput(&bytes.Buffer{})
	t.Cleanup(func() { log.SetOutput(log.Writer()) })
	writeLedgerError(response, http.ErrServerClosed)
	assertJSON(t, response, http.StatusInternalServerError, `{"error":"internal server error"}`)
}

func TestLoggingMiddleware(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/test", nil))

	for _, want := range []string{"PUT", "/api/test", "418", "s)"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("log %q does not contain %q", logs.String(), want)
		}
	}
}

func TestDTOConversion(t *testing.T) {
	t.Parallel()
	tx := CreateTransactionRequest{Amount: 10, Category: "еда", Description: "д", Date: "2025-09-10"}.toDomain()
	if tx != (ledger.Transaction{Amount: 10, Category: "еда", Description: "д", Date: "2025-09-10"}) {
		t.Fatalf("unexpected transaction: %+v", tx)
	}
	if got := newTransactionResponse(ledger.Transaction{ID: 3, Amount: 1}); got.ID != 3 || got.Amount != 1 {
		t.Fatalf("unexpected response: %+v", got)
	}
	if b := (CreateBudgetRequest{Category: "а", Limit: 5}).toDomain(); b != (ledger.Budget{Category: "а", Limit: 5}) {
		t.Fatalf("unexpected budget: %+v", b)
	}
	if got := newBudgetResponse(ledger.Budget{Category: "а", Limit: 5}); got != (BudgetResponse{Category: "а", Limit: 5}) {
		t.Fatalf("unexpected response: %+v", got)
	}
}
