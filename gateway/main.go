package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"example.com/gateway-ledger/gateway/internal/api"
	"example.com/gateway-ledger/ledger"
)

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})
	mux.Handle("/api/", api.NewHandler())
	return api.Logging(mux)
}

func main() {
	// The ledger package is imported as a library, so the gateway connects it
	// to PostgreSQL (DATABASE_URL) and Redis (REDIS_ADDR) when it starts.
	if err := ledger.Connect(context.Background()); err != nil {
		log.Fatalf("cannot connect Ledger storage: %v", err)
	}
	defer ledger.Close()

	server := &http.Server{
		Addr:              ":8080",
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("Gateway service started on http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}
