# Gateway

HTTP-сервис на порту 8080. Подключает пакет `ledger` как библиотеку и при старте открывает PostgreSQL и Redis (переменные описаны в [корневом README](../README.md)). Эндпойнты: `/ping`, `POST/GET /api/transactions`, `POST/GET /api/budgets`, `GET /api/reports/summary`.

```sh
go run .
```
