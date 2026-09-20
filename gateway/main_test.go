package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		status             int
		body               string
	}{
		{"ping", http.MethodGet, "/ping", http.StatusOK, "pong"},
		{"wrong method", http.MethodPost, "/ping", http.StatusMethodNotAllowed, "method not allowed\n"},
		{"unknown path", http.MethodGet, "/unknown", http.StatusNotFound, "404 page not found\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			newHandler().ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if response.Code != tc.status || response.Body.String() != tc.body {
				t.Fatalf("got %d %q; want %d %q", response.Code, response.Body.String(), tc.status, tc.body)
			}
			if tc.status == http.StatusMethodNotAllowed && response.Header().Get("Allow") != http.MethodGet {
				t.Fatal("Allow header must contain GET")
			}
		})
	}
}
