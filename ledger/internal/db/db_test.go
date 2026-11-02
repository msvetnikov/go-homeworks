package db

import (
	"context"
	"strings"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME"} {
		t.Setenv(key, "")
	}
}

func TestDSN(t *testing.T) {
	t.Run("defaults for local development", func(t *testing.T) {
		clearEnv(t)
		if got, want := DSN(), "postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"; got != want {
			t.Fatalf("DSN() = %q, want %q", got, want)
		}
	})
	t.Run("separate variables", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_HOST", "db.local")
		t.Setenv("DB_PORT", "6543")
		t.Setenv("DB_USER", "max")
		t.Setenv("DB_PASS", "p@ss:word")
		t.Setenv("DB_NAME", "cashcraft")
		if got, want := DSN(), "postgres://max:p%40ss%3Aword@db.local:6543/cashcraft?sslmode=disable"; got != want {
			t.Fatalf("DSN() = %q, want %q", got, want)
		}
	})
	t.Run("DATABASE_URL wins", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_HOST", "ignored")
		t.Setenv("DATABASE_URL", "postgres://u:secret@h:1/n?sslmode=disable")
		if got := DSN(); got != "postgres://u:secret@h:1/n?sslmode=disable" {
			t.Fatalf("DSN() = %q", got)
		}
	})
}

func TestDescribeHidesPassword(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://u:secret@h:1/n")
	if got := Describe(); strings.Contains(got, "secret") || !strings.Contains(got, "h:1/n") {
		t.Fatalf("Describe() = %q", got)
	}
}

func TestOpenReportsConnectionError(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:1/n?sslmode=disable&connect_timeout=1")
	if _, err := Open(context.Background()); err == nil {
		t.Fatal("expected a connection error")
	}
}
