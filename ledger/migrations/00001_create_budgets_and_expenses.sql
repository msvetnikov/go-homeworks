-- +goose Up
-- +goose StatementBegin
CREATE TABLE budgets (
    id           SERIAL PRIMARY KEY,
    category     TEXT NOT NULL UNIQUE,
    limit_amount NUMERIC(14,2) NOT NULL CHECK (limit_amount > 0)
);

CREATE TABLE expenses (
    id          SERIAL PRIMARY KEY,
    amount      NUMERIC(14,2) NOT NULL CHECK (amount <> 0),
    category    TEXT NOT NULL,
    description TEXT,
    date        DATE NOT NULL
);

CREATE INDEX expenses_category_date_idx ON expenses (category, date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE expenses;
DROP TABLE budgets;
-- +goose StatementEnd
