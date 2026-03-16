# go-highload-demo

Демонстрационный **Notification Service** на Go, имитирующий высоконагруженный сервис рассылки уведомлений. Проект демонстрирует навыки работы с многопоточностью (горутины, каналы, sync-примитивы) и highload-паттернами.

## Ключевые паттерны

| Паттерн | Реализация |
|---|---|
| **Worker pool** | Настраиваемый пул воркеров на каналах (`internal/worker/`) |
| **Fan-out** | Одно событие → параллельная отправка в несколько каналов (`service.SendAll`) |
| **Backpressure** | Буферизированные каналы, контроль перегрузки |
| **Graceful shutdown** | `context.Context` + `sync.WaitGroup` + `sync.Once` |
| **Rate limiting** | Fixed window counter на Redis (`pkg/ratelimiter/`) |
| **Retry с exponential backoff** | Повторные попытки при сбоях (`pkg/retry/`) |

## Стек технологий

- **Go** — основной язык
- **PostgreSQL** — хранение уведомлений и статусов
- **RabbitMQ** — durable очередь с dead letter queue
- **Redis** — rate limiting
- **Docker Compose** — вся инфраструктура в контейнерах
- **k6** — нагрузочное тестирование
- **pprof** — профилирование

## Архитектура

```
HTTP Request
     │
     ▼
  Handler ─── POST /notifications
     │            (fan-out: channels → goroutines + WaitGroup)
     ▼
  Service.Send() ─── Save → Publish
     │
     ▼
  Broker (RabbitMQ / LocalBroker)
     │
     ▼
  Consumer → Worker Pool
     │
     ▼
  ProcessNotification
     ├── Rate Limit (Redis)
     ├── Send (retry + exponential backoff)
     └── Update Status (PostgreSQL)
```

## Структура проекта

```
go-highload-demo/
├── cmd/server/           # Точка входа
├── internal/
│   ├── app/              # Сборка компонентов, lifecycle
│   ├── broker/           # Broker интерфейс, LocalBroker, RabbitMQBroker
│   ├── config/           # Конфигурация из env-переменных
│   ├── handler/          # HTTP-обработчики
│   ├── model/            # Модели данных (Notification, Channel, Status)
│   ├── repository/       # Слой доступа к данным (Store интерфейс)
│   ├── sender/           # Отправщики (email, push, sms, webhook)
│   ├── service/          # Бизнес-логика
│   ├── storage/          # Реализации Store (PostgreSQL, in-memory)
│   └── worker/           # Worker pool
├── pkg/
│   ├── ratelimiter/      # Rate limiter + Redis-адаптер
│   └── retry/            # Retry с exponential backoff
├── migrations/           # SQL-миграции
├── scripts/              # k6 нагрузочные скрипты
├── docker-compose.yml
├── Dockerfile
└── CHANGELOG.md
```

## Быстрый старт

### Полный стек (PostgreSQL + Redis + RabbitMQ)

```bash
docker compose up -d
```

Сервис запустится на `http://localhost:8080` с подключением ко всем зависимостям.

### Без внешних зависимостей

```bash
go run ./cmd/server/
```

Запустится с in-memory store, без rate limiting и без RabbitMQ. Каждый компонент инфраструктуры подключается автоматически по наличию env-переменной.

## API

### Создать уведомление (один канал)

```bash
curl -X POST http://localhost:8080/notifications \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user:1", "channel": "email", "payload": "Hello!"}'
```

Ответ: `201 Created`
```json
{"id": "550e8400-e29b-41d4-a716-446655440000"}
```

### Создать уведомления (fan-out, несколько каналов)

```bash
curl -X POST http://localhost:8080/notifications \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user:1", "channels": ["email", "push", "sms"], "payload": "Hello!"}'
```

Ответ: `201 Created`
```json
{"ids": ["id-1", "id-2", "id-3"]}
```

### Получить уведомление

```bash
curl http://localhost:8080/notifications/{id}
```

### Health check

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### Профилирование

```bash
go tool pprof http://localhost:8080/debug/pprof/goroutine
go tool pprof http://localhost:8080/debug/pprof/heap
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

## Конфигурация

| Переменная | По умолчанию | Описание |
|---|---|---|
| `SERVER_ADDR` | `:8080` | Адрес HTTP-сервера |
| `DB_HOST` | — | Хост PostgreSQL (если пусто — in-memory) |
| `DB_PORT` | `5432` | Порт PostgreSQL |
| `DB_USER` | `notify` | Пользователь PostgreSQL |
| `DB_PASSWORD` | — | Пароль PostgreSQL |
| `DB_NAME` | `notifications` | Имя базы данных |
| `REDIS_ADDR` | — | Адрес Redis (если пусто — rate limiting отключён) |
| `RABBITMQ_URL` | — | URL RabbitMQ (если пусто — локальный worker pool) |
| `WORKER_POOL_SIZE` | `10` | Количество воркеров |
| `WORKER_QUEUE_SIZE` | `100` | Размер очереди задач |
| `RATE_LIMIT_RPS` | `100` | Лимит запросов в секунду на пользователя |
| `RETRY_MAX_ATTEMPTS` | `3` | Максимум попыток отправки |

## Тесты

```bash
# Все тесты (93 теста)
go test ./...

# С подробным выводом
go test ./... -v

# Интеграционные тесты (нужны запущенные сервисы)
TEST_POSTGRES_DSN="postgres://notify:notify_pass@localhost:5432/notifications?sslmode=disable" \
TEST_REDIS_ADDR="localhost:6381" \
TEST_RABBITMQ_URL="amqp://guest:guest@localhost:5672/" \
go test ./... -v
```

## Нагрузочное тестирование

```bash
# Smoke-тест (5 VU, 10 секунд)
docker compose run --rm k6 run /scripts/smoke-test.js

# Полный нагрузочный тест (smoke → load → stress → spike)
docker compose run --rm k6 run /scripts/load-test.js
```

## Лицензия

MIT
