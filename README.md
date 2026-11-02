# ДЗ 7 — PostgreSQL и Redis в Ledger

Раньше Ledger хранил данные в памяти и терял их при перезапуске. Теперь **Ledger хранит бюджеты и транзакции в PostgreSQL**, а **отчёты и список бюджетов кэшируются в Redis** на короткий TTL. Gateway обращается к тем же HTTP-эндпойнтам, что и раньше, контракты не изменились. Gateway подключает модуль `ledger` как библиотеку, поэтому при старте сам открывает соединения с PostgreSQL и Redis.

Из прошлых заданий в проекте есть:

- ДЗ 4 — интерфейс `Validatable` с методом `Validate() error`. Его реализуют `Transaction` и `Budget`, функция `CheckValid(v Validatable)` проверяет любой из них. `AddTransaction` и `SetBudget` вызывают `Validate()` перед сохранением.
- ДЗ 5 — HTTP-слой в `gateway/internal/api`: DTO, обработчики, middleware с логом `метод путь статус время`, единый JSON-формат ошибок.
- ДЗ 6 — таблицы тестов для `Validate()`, бюджетов и обработчиков (см. «Тесты»).

## Структура

```
ledger/
  ledger.go, validate.go, report.go   бизнес-логика, валидация, отчёт и кэш
  postgres.go, memstore.go            хранилища: PostgreSQL и память (память нужна тестам)
  internal/db, internal/cache         подключение к PostgreSQL и к Redis
  migrations/                         миграции Goose
  cmd/ledger/main.go                  запуск Ledger
gateway/
  main.go                             запуск Gateway
  internal/api                        DTO, обработчики, middleware
```

## 1. Подготовка: PostgreSQL, Redis, Goose

Нужен Go 1.26 или новее.

PostgreSQL и Redis проще всего запустить в Docker (можно поставить и локально):

```sh
docker run -d --name cashcraft-pg -p 5432:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=cashapp postgres:17-alpine
docker run -d --name cashcraft-redis -p 6379:6379 redis:7-alpine
```

Установка Goose:

```sh
go install github.com/pressly/goose/v3/cmd/goose@latest
```

## 2. Миграции

Строка подключения задаётся переменной `DATABASE_URL` (bash; в PowerShell: `$env:DATABASE_URL = "..."`):

```sh
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"
```

Создать таблицы `budgets` и `expenses` (из корня репозитория):

```sh
goose -dir ./ledger/migrations postgres "$DATABASE_URL" up
```

Откатить:

```sh
goose -dir ./ledger/migrations postgres "$DATABASE_URL" down
```

Состояние миграций: `goose -dir ./ledger/migrations postgres "$DATABASE_URL" status`.

Миграцию нужно применять перед первым запуском и после каждого изменения схемы. Только после успешного `up` запускайте Ledger и Gateway.

Схема: `budgets(id, category UNIQUE, limit_amount > 0)` и `expenses(id, amount <> 0, category, description, date)` плюс индекс по `(category, date)`.

## 3. Настройка

| Переменная | Значение по умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | — | строка подключения к PostgreSQL, главная переменная |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME` | `localhost`, `5432`, `postgres`, `postgres`, `cashapp` | используются, если `DATABASE_URL` не задана (`sslmode=disable`) |
| `REDIS_ADDR` | `localhost:6379` | адрес Redis |
| `REDIS_DB` | `0` | номер базы Redis |
| `REDIS_PASSWORD` | пусто | пароль Redis |

Если PostgreSQL недоступен, программа пишет понятную ошибку и завершается. Если недоступен Redis, в лог попадёт предупреждение, а сервис продолжит работать без кэша. При успешном старте в лог пишутся строки `connected to PostgreSQL` и `connected to Redis`. Пул соединений: `SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(30m)`.

## 4. Запуск

Ledger (проверяет подключение, загружает `budgets.json`, печатает бюджеты, транзакции и дважды строит отчёт):

```sh
cd ledger
go run ./cmd/ledger
```

Gateway (порт 8080):

```sh
cd gateway
go run .
```

## 5. Проверка через cURL

```sh
# бюджет → 201
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"еда","limit":5000}'

# транзакция в пределах лимита → 201
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}'

# транзакция сверх лимита → 409 {"error":"budget exceeded"}, запись не добавляется
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":5000,"category":"еда","description":"слишком много","date":"2025-09-10"}'

# некорректные данные → 400 {"error":"..."}
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":-1,"category":"еда","date":"2025-09-10"}'

# списки из базы → 200
curl http://localhost:8080/api/transactions
curl http://localhost:8080/api/budgets

# отчёт за период, два раза подряд
curl "http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30"
curl "http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30"
```

В логе Gateway видно эффект кэша: первый запрос отчёта пишет `cache miss report:summary:2025-09-01:2025-09-30` и считается по базе, второй пишет `cache hit ...` и отвечает быстрее (время запроса есть в строке `GET /api/reports/summary -> 200 (...)`).

Чтобы убедиться, что данные переживают перезапуск, остановите Gateway, запустите снова и повторите `GET /api/transactions` и `GET /api/budgets`.

> На Windows `curl` в консоли может испортить кириллицу в аргументе `-d`. Тогда положите JSON в файл в кодировке UTF-8 и передайте `--data-binary @файл.json`.

Эндпойнты: `POST/GET /api/transactions`, `POST/GET /api/budgets`, `GET /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD`, `GET /ping`. Ошибки всегда приходят как `{"error":"..."}` с кодом 400 (валидация), 409 (превышение бюджета) или 500 (внутренняя ошибка); все ответы API имеют `Content-Type: application/json; charset=utf-8`.

## 6. Как устроено

- `SetBudget` делает UPSERT по `category`, `ListBudgets` и `ListTransactions` читают таблицы (транзакции от новых к старым).
- `AddTransaction` в одной SQL-транзакции блокирует строку бюджета категории, сравнивает потраченное плюс новую сумму с лимитом и либо возвращает `budget exceeded` без вставки, либо делает `INSERT ... RETURNING id`. Категория без бюджета лимита не имеет. Суммы сравниваются в копейках, чтобы не было ошибок float.
- `GetReportSummary(ctx, from, to)` считает `SUM(amount) GROUP BY category` за период (границы включены) и кэширует результат в Redis по ключу `report:summary:<from>:<to>` на 30 секунд. Список бюджетов лежит под ключом `budgets:all` на 15 секунд. Добавление транзакции удаляет кэш отчётов, `SetBudget` удаляет `budgets:all`, поэтому после записи читать устаревшие данные не придётся. Если Redis недоступен, ошибки кэша только логируются.
- До вызова `ledger.Connect` пакет работает с хранилищем в памяти. Так юнит-тесты запускаются без базы.

## Тесты

```sh
cd ledger  && go test ./... -cover
cd gateway && go test ./... -cover
```

Тесты `ledger` проверяют `Validate()` (таблицы случаев с `t.Parallel()`), бюджеты и лимиты, отчёт и логику кэша на фейковом кэше, а пакет `internal/cache` проверяется на `miniredis`. Тесты Gateway прогоняют обработчики через `httptest` (создание бюджета и транзакции, 400, 409, формат ошибок, заголовок `Content-Type`). Каждый тест очищает данные через `ledger.Reset()`.

Интеграционный тест PostgreSQL запускается, только если задана `TEST_DATABASE_URL` (база с применёнными миграциями, **таблицы будут очищены**), иначе пропускается:

```sh
cd ledger
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable" go test ./... -cover
```

Отчёт о покрытии:

```sh
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html
```

Покрытие тестами: Ledger ~76% (с интеграционным тестом PostgreSQL; без базы ~55%), Gateway ~82%.
