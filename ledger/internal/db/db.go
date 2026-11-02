// Package db opens the PostgreSQL connection from environment variables.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const pingTimeout = 5 * time.Second

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// DSN returns DATABASE_URL or builds a DSN from DB_HOST, DB_PORT, DB_USER,
// DB_PASS and DB_NAME (local development defaults).
func DSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(getenv("DB_USER", "postgres"), getenv("DB_PASS", "postgres")),
		Host:     net.JoinHostPort(getenv("DB_HOST", "localhost"), getenv("DB_PORT", "5432")),
		Path:     "/" + getenv("DB_NAME", "cashapp"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// Describe returns the DSN without the password, for logs and error messages.
func Describe() string {
	u, err := url.Parse(DSN())
	if err != nil {
		return "DATABASE_URL"
	}
	return u.Redacted()
}

// Open connects to PostgreSQL, sets the pool limits and checks the connection.
func Open(ctx context.Context) (*sql.DB, error) {
	db, err := sql.Open("pgx", DSN())
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}
