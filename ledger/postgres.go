package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// pgStore keeps everything in PostgreSQL (see migrations/).
type pgStore struct {
	db *sql.DB
}

func (s *pgStore) addTransaction(ctx context.Context, tx Transaction) (int, error) {
	date, err := time.Parse(dateLayout, tx.Date)
	if err != nil {
		return 0, invalid("date must be a valid date in YYYY-MM-DD format")
	}

	// The budget row is locked until commit, so two parallel requests for the
	// same category cannot both pass the limit check.
	dbTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer dbTx.Rollback()

	var limit float64
	err = dbTx.QueryRowContext(ctx,
		`SELECT limit_amount::float8 FROM budgets WHERE category = $1 FOR UPDATE`, tx.Category).Scan(&limit)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// A category without a budget has no limit.
	case err != nil:
		return 0, fmt.Errorf("select budget: %w", err)
	default:
		var spent float64
		if err := dbTx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0)::float8 FROM expenses WHERE category = $1`, tx.Category).Scan(&spent); err != nil {
			return 0, fmt.Errorf("sum expenses: %w", err)
		}
		if total := toCents(spent) + toCents(tx.Amount); total > toCents(limit) {
			return 0, fmt.Errorf("%w: category %q, total %.2f, limit %.2f",
				ErrBudgetExceeded, tx.Category, float64(total)/100, limit)
		}
	}

	var id int
	err = dbTx.QueryRowContext(ctx,
		`INSERT INTO expenses(amount, category, description, date) VALUES ($1, $2, $3, $4) RETURNING id`,
		float64(toCents(tx.Amount))/100, tx.Category, tx.Description, date).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert expense: %w", err)
	}
	if err := dbTx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return id, nil
}

func (s *pgStore) listTransactions(ctx context.Context) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, amount::float8, category, COALESCE(description, ''), date FROM expenses ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("select expenses: %w", err)
	}
	defer rows.Close()

	result := make([]Transaction, 0)
	for rows.Next() {
		var tx Transaction
		var date time.Time
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &tx.Description, &date); err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		tx.Date = date.Format(dateLayout)
		result = append(result, tx)
	}
	return result, rows.Err()
}

func (s *pgStore) setBudget(ctx context.Context, b Budget) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO budgets(category, limit_amount) VALUES ($1, $2)
		 ON CONFLICT (category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`,
		b.Category, float64(toCents(b.Limit))/100)
	if err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}
	return nil
}

func (s *pgStore) listBudgets(ctx context.Context) ([]Budget, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT category, limit_amount::float8 FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("select budgets: %w", err)
	}
	defer rows.Close()

	result := make([]Budget, 0)
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.Category, &b.Limit); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (s *pgStore) categoryTotals(ctx context.Context, from, to string) ([]CategoryTotal, error) {
	fromDate, err := time.Parse(dateLayout, from)
	if err != nil {
		return nil, invalid("from must be a valid date in YYYY-MM-DD format")
	}
	toDate, err := time.Parse(dateLayout, to)
	if err != nil {
		return nil, invalid("to must be a valid date in YYYY-MM-DD format")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT category, SUM(amount)::float8 FROM expenses
		 WHERE date BETWEEN $1 AND $2 GROUP BY category ORDER BY category`, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("select report: %w", err)
	}
	defer rows.Close()

	result := make([]CategoryTotal, 0)
	for rows.Next() {
		var c CategoryTotal
		if err := rows.Scan(&c.Category, &c.Total); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *pgStore) reset(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `TRUNCATE expenses, budgets RESTART IDENTITY`); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	return nil
}

func (s *pgStore) close() error { return s.db.Close() }
