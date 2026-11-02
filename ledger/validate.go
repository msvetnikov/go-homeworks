package ledger

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	dateLayout = "2006-01-02"
	// maxCents is the largest value that fits NUMERIC(14,2).
	maxCents = 99_999_999_999_999
)

// Validatable is implemented by every structure that can check its own data.
// Validate only returns an error and never logs: logging is up to the caller.
type Validatable interface {
	Validate() error
}

// ValidationError describes invalid input data. Callers (for example the
// HTTP layer) can detect it with errors.As and answer with 400 Bad Request.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// toCents rounds a money amount to whole cents, so limits are compared
// without floating point noise (0.1 + 0.2 == 0.3).
func toCents(v float64) int64 {
	return int64(math.Round(v * 100))
}

func checkMoney(name string, v float64) error {
	switch {
	case math.IsNaN(v) || math.IsInf(v, 0):
		return invalid("%s must be a finite number", name)
	case toCents(v) <= 0:
		return invalid("%s must be at least 0.01", name)
	case v*100 > maxCents:
		return invalid("%s is too large", name)
	}
	return nil
}

func (tx Transaction) Validate() error {
	if err := checkMoney("amount", tx.Amount); err != nil {
		return err
	}
	if strings.TrimSpace(tx.Category) == "" {
		return invalid("category must not be empty")
	}
	if _, err := time.Parse(dateLayout, tx.Date); err != nil {
		return invalid("date must be a valid date in YYYY-MM-DD format")
	}
	return nil
}

func (b Budget) Validate() error {
	if err := checkMoney("limit", b.Limit); err != nil {
		return err
	}
	if strings.TrimSpace(b.Category) == "" {
		return invalid("category must not be empty")
	}
	return nil
}

// CheckValid works with any Validatable, whatever its concrete type.
func CheckValid(v Validatable) error {
	return v.Validate()
}
