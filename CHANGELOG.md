# Changelog

Все значимые изменения в проекте документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
проект придерживается [семантического версионирования](https://semver.org/lang/ru/).

## [1.0.0] - 2026-03-16

### Добавлено

- RabbitMQ брокер с durable queue, dead letter queue и persistent messages
- Абстракция `Broker` (интерфейс Publish/Run/Shutdown)
- `LocalBroker` — fallback на in-memory worker pool при отсутствии RabbitMQ
- Интеграция retry с exponential backoff в `ProcessNotification`
- Fan-out: `SendAll()` — параллельная отправка в несколько каналов (goroutines + WaitGroup)
- Поддержка `channels` (массив) в POST /notifications для fan-out
- Автовыбор брокера по наличию `RABBITMQ_URL`

### Изменено

- Service упрощён: `Publisher` интерфейс вместо прямого управления worker pool
- `ProcessNotification` экспортирован для вызова из consumer

## [0.5.0] - 2026-03-16

### Добавлено

- PostgreSQL Store (`pgxpool`) с поддержкой Save, GetByID, UpdateStatus, GetPending
- SQL-миграция для таблицы `notifications` с индексами
- Redis-адаптер для rate limiter (`go-redis/v9`)
- Автовыбор хранилища: PostgreSQL при `DB_HOST`, иначе in-memory
- Автовыбор rate limiter: Redis при `REDIS_ADDR`, иначе noop
- Нагрузочные скрипты k6: smoke, load, stress, spike
- Сервис k6 в docker-compose (профиль `test`)
- Интеграционные тесты для PostgreSQL и Redis (skip без env)

### Изменено

- Конфигурация PostgreSQL: DSN собирается из `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`

## [0.4.0] - 2026-03-16

### Добавлено

- Компонент `App` — сборка всех зависимостей и управление lifecycle
- In-memory Store (`MemoryStore`) для разработки без PostgreSQL
- `cmd/server/main.go` — точка входа с graceful shutdown (SIGINT/SIGTERM)
- Интеграция worker pool — асинхронная обработка уведомлений через `service.Send()`
- Health check эндпоинты: `GET /health`, `GET /ready`
- pprof эндпоинты для профилирования (`/debug/pprof/*`)
- HTTP-роутинг на стандартном `http.ServeMux` (Go 1.22+)
- Идемпотентный `Shutdown` через `sync.Once`

## [0.3.0] - 2026-03-16

### Добавлено

- Rate limiter на Redis — алгоритм fixed window counter
- Repository с интерфейсом `Store` и обёрткой `NotificationRepository`
- Service — бизнес-логика: save → rate limit → send → update status
- HTTP-обработчики: `CreateNotification` (POST), `GetNotification` (GET)
- Полное покрытие тестами (TDD: красные тесты → реализация)

## [0.2.0] - 2026-03-16

### Добавлено

- Конфигурация из env-переменных с значениями по умолчанию
- Retry с exponential backoff (`pkg/retry`)
- Worker pool с graceful shutdown и backpressure (`internal/worker`)
- Отправщики уведомлений: email, push, sms, webhook (`internal/sender`)
- Registry отправщиков с потокобезопасным доступом (`sync.RWMutex`)
- Зависимость `testify` для тестов

## [0.1.0] - 2026-03-16

### Добавлено

- Инициализация Go-модуля и структуры проекта
- Docker Compose: Go app (air hot-reload), PostgreSQL 16, RabbitMQ 3, Redis 7
- Dockerfile на базе `golang:1.25-alpine`
- Модели данных: `Notification`, `Channel`, `NotificationStatus`
- Валидация, переходы статусов (pending → processing → sent/failed)
- Godoc-комментарии для всех экспортируемых символов
- Лицензия MIT

[1.0.0]: https://github.com/sneaking777/go-highload-demo/compare/v0.5.0...v1.0.0
[0.5.0]: https://github.com/sneaking777/go-highload-demo/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/sneaking777/go-highload-demo/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/sneaking777/go-highload-demo/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/sneaking777/go-highload-demo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/sneaking777/go-highload-demo/releases/tag/v0.1.0
