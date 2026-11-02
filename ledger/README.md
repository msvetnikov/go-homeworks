# Ledger

Бизнес-логика и хранилище: бюджеты и транзакции в PostgreSQL, отчёты и список бюджетов в кэше Redis. Подробная инструкция (миграции Goose, переменные `DATABASE_URL`, `REDIS_ADDR`, `REDIS_DB`, `REDIS_PASSWORD`, примеры cURL) лежит в [корневом README](../README.md).

```sh
goose -dir ./migrations postgres "$DATABASE_URL" up
go run ./cmd/ledger
```
