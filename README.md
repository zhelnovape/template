# TripGo — репозиторий для лабораторных работ

Учебный проект курса «Разработка микросервисов на Go».

## Инструкция запуска

### Требования

| Инструмент | Назначение |
| :-- | :-- |
| **Go 1.24+** | Среда выполнения |
| **Docker** | Контейнеризация |
| **tripgoctl** | Внутренний СLI |

Локальные интеграции: ``curl``, ``jq``, ``psql``, ``goose`` и ``shasum``.

```bash
# Установка CLI
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL \
  https://github.com/course-go-autumn-2026/course-infra/releases/latest/download/install-tripgoctl \
  | sh
```

### Команды запуска

```bash
# Первичный запуск (Один раз при настройке)
tripgoctl cluster start

# Подготовка окружения
tripgoctl environment start && make migrate

# Запуск микросервиса
make run

# Вывод адресов и ссылок (Jaeger, Grafana, Prometheus и Console)
tripgoctl connect 
```

### Список env-переменных

| Переменная | Назначение |
| :-- | :-- |
| ``HTTP_ADDR`` | Хост и порт для запуска |
| ``HTTP_READ_TIMEOUT`` | Макс. время чтения всего запроса |
| ``HTTP_WRITE_TIMEOUT`` | Макс. время записи ответа клиенту |
| ``HTTP_READ_HEADER_TIMEOUT``| Время чтения заголовков запроса |
| ``HTTP_IDLE_TIMEOUT`` | Время ожидания Keep-alive соединения |
| ``DATABASE_URL`` | DSN-ссылка для подключения к БД |
| ``DATABASE_MAX_CONNS`` | Макс. число соединений в пуле |
| ``DATABASE_MIN_CONNS`` | Мин. число соединений в пуле |
| ``DATABASE_MAX_CONN_LIFETIME`` | Макс. время жизни соединения |
| ``DATABASE_CONNECT_TIMEOUT`` | Таймаут на установку связи с БД |
| ``DATABASE_QUERY_TIMEOUT`` | Таймаут на выполнение одного SQL-запроса |
| ``SHUTDOWN_TIMEOUT``| Время Graceful Shutdown |
| ``LOG_LEVEL`` | Уровень логирования|

## Лабораторная работа 1. HTTP API и PostgreSQL

В лабораторной работе 1 сделан работающий сервис: поднят веб-сервер, накачены миграции, запись и чтение данных через pgx, реализован свой менеджер транзакций.

### Решения

**Менеджер транзакций**

Менеджер транзакций (`internal/trip/tx_manager.go`) – это интерфейс `TxManager` с одним методом:

```go
Do(ctx context.Context, fn func(ctx context.Context) error) error
```

Как он работает:

1. `Do` открывает транзакцию через `pool.BeginTx` и кладёт её в `context`. Если `fn` вернула ошибку, происходит `Rollback`, иначе – `Commit`.
2. `Rollback` выполняется с `context.WithoutCancel(ctx)`, чтобы он прошёл, даже если клиент уже отменил запрос.
3. На `Begin`, `Commit` и `Rollback` стоит таймаут `DATABASE_QUERY_TIMEOUT`, чтобы сервис не зависал, когда БД не отвечает.
4. Репозиторий сам про транзакции ничего не знает. Метод `executor(ctx, pool)` проверяет контекст: если есть транзакция, запрос идёт в неё, если нет — напрямую в пул.

В транзакции выполняются две операции сервиса:

- `CreateTrip` — создание поездки в `trips` + запись в `trip_status_history`.
- `FinishTrip` — перевод поездки в статус `completed` + запись в историю.

`GetTrip` — это одно чтение, поэтому транзакция ему не нужна.

**Уровень изоляции – Read Committed**

`Read Committed` – стандартный для PostgreSQL уровень изоляции.

Почему его достаточно:

- Транзакции короткие: два запроса, каждый работает с одной строкой.
- От гонок защищают сами механизмы БД: уникальный индекс (см. ниже) и условие `UPDATE ... WHERE id = $1 AND status = 'active'`.
- Повтор транзакции: более строгие уровни (`Repeatable Read`, `Serializable`) при конфликтах возвращают ошибку сериализации (`40001`).

**Запрет двух активных поездок – частичный уникальный индекс**

Запрет сделан на уровне БД, в миграции `00001_create_trips.sql`:

```sql
CREATE UNIQUE INDEX trips_active_driver_idx ON trips (driver_id) WHERE status = 'active';
```

Индекс учитывает только активные поездки. Когда поездка завершается, она выпадает из индекса, и водитель снова может взять заказ.

Если водитель уже занят, `INSERT` падает с ошибкой `23505` (`unique_violation`). 

Функция `isActiveDriverViolation` в `repository.go` проверяет код ошибки и имя индекса и превращает её в `ErrDriverBusy`, а хендлер отдаёт `409 Conflict` с кодом `driver_busy`. Транзакция при этом откатывается, поэтому лишняя запись в историю тоже не появляется.
