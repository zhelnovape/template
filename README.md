# TripGo — репозиторий для лабораторных работ

Учебный проект курса «Разработка микросервисов на Go».

## Инструкция запуска

### Требования

| Инструмент | Назначение |
| :-- | :-- |
| **Go 1.27+** | Среда выполнения |
| **Docker** | Контейнеризация |
| **tripgoctl** | Внутренний СLI |
| **golangci-lint v2** | Линтер и форматтеры |

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

### Тесты, линтер и генерация кода

```bash
# Линтер
make lint

# Unit-тесты с -race и покрытие пакетов domain и usecase
make test

# Интеграционные тесты репозитория (нужна база с миграциями)
tripgoctl environment start && make migrate
make test-integration

# Код по OpenAPI и моки
make generate
```

`make test` запускает все unit-тесты с `-race`, затем отдельно считает покрытие `internal/domain` и `internal/usecase` и печатает итог последней строкой.

Интеграционные тесты помечены тегом `//go:build integration`, поэтому `make test` их не запускает. Адрес базы они берут из `TEST_DATABASE_URL`, а если его нет — из `DATABASE_URL` (Makefile подхватывает его из `.env`). Каждый тест сам создаёт свои поездки и удаляет их после себя, поэтому повторный запуск тоже проходит.

### Проверка телеметрии

```bash
# Ссылки на Jaeger и Grafana
tripgoctl connect

# Логи сервиса в JSON: у каждой записи внутри запроса есть trace_id и span_id
make run 2>&1 | tee service.log
```

- Jaeger: сервис `trip-service`, трейсы называются по шаблону маршрута, например `POST /api/v1/trips`.
- Grafana → Explore → Prometheus: метрики `http_server_request_duration_seconds`, `http_server_active_requests`, `db_client_operation_duration_seconds`, `tripgo_trips_created_total`, `tripgo_trips_completed_total`.

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
| ``LOG_LEVEL`` | Уровень логирования (`debug`, `info`, `warn`, `error`) |
| ``OTEL_SERVICE_NAME`` | Имя сервиса в телеметрии (`trip-service`) |
| ``OTEL_EXPORTER_OTLP_ENDPOINT`` | Адрес коллектора OpenTelemetry |
| ``OTEL_EXPORTER_OTLP_PROTOCOL`` | Протокол экспорта: `http/protobuf` или `grpc` |
| ``OTEL_RESOURCE_ATTRIBUTES`` | Общие атрибуты: `service.namespace`, `deployment.environment` |
| ``OTEL_TRACES_SAMPLER`` | Какие трейсы сохранять (`parentbased_always_on`) |
| ``OTEL_METRIC_EXPORT_INTERVAL`` | Как часто отправлять метрики, в миллисекундах |
| ``TEST_DATABASE_URL`` | Необязательная: база для интеграционных тестов, иначе берётся `DATABASE_URL` |

Переменные `OTEL_*`, кроме `OTEL_METRIC_EXPORT_INTERVAL`, записывает в `.env` сам `tripgoctl`. `OTEL_METRIC_EXPORT_INTERVAL` задана в `environment.toml`, чтобы метрики уходили раз в 5 секунд, а не раз в минуту.

## Архитектура

### Слои

```
cmd/service/main.go          читает конфиг, вызывает container.New, запускает сервер, ждёт сигнала
internal/
  config/                    чтение и проверка переменных окружения
  container/                 создаёт все зависимости и связывает их — единственное такое место
  domain/trip/               поездка: статусы, правила, переход в completed, ошибки домена
  model/                     данные запроса на создание поездки (CreateTrip)
  usecase/trip/              сценарии: create, get, finish
  http/                      транспорт
    trip/create|get|finish/  ручки, по пакету на ручку
    domainerror/             перевод ошибки в HTTP-ответ — одно место на весь сервис
    middleware/              Recover → Tracing → Logging → Metrics
    health/, ready/          служебные ручки
    response/                запись JSON-ответов
  repository/trip/           SQL для таблиц trips и trip_status_history
  infra/
    postgres/                пул pgx и трейсер запросов
    postgres/transaction/    менеджер транзакций
    telemetry/               OpenTelemetry и логгер slog
    clock/                   текущее время
  generated/api/             код, сгенерированный по OpenAPI
```

### Кто от кого зависит

Стрелка значит «импортирует». Зависимости идут внутрь, к домену:

```
                 cmd/service
                      │
               internal/container
                      │
   ┌──────────┬───────┴───────┬──────────────┐
   ▼          ▼               ▼              ▼
  http     usecase        repository       infra
   │          │               │
   └──────────┼───────────────┘
              ▼
       model, domain
```

| Пакет | Что импортирует | Что ему нельзя |
| :-- | :-- | :-- |
| `domain` | стандартную библиотеку и `uuid` | `net/http`, `chi`, `pgx`, `squirrel`, `otel`, `api` |
| `usecase` | `domain`, `model`, `slog`, `otel/metric` | `net/http`, `chi`, `pgx`, `squirrel`, `api` |
| `http` | `domain`, `model`, `api`, `chi`, `otel` | `repository`, `infra` |
| `repository` | `domain`, `pgx`, `squirrel`, менеджер транзакций | — |
| `container` | всё | — |

Запреты для `domain` и `usecase` проверяет `depguard` в `.golangci.yml`, поэтому нарушение ломает `make lint`.

### Где объявлены интерфейсы и почему там

Интерфейсы лежат в `contract.go` того пакета, который их использует, и они неэкспортируемые:

| Где | Интерфейсы |
| :-- | :-- |
| `usecase/trip/create`, `usecase/trip/finish` | `tripRepository`, `txManager`, `clock` |
| `usecase/trip/get` | `tripRepository` |
| `http/trip/create`, `get`, `finish` | `tripCreator`, `tripGetter`, `tripFinisher` — нужный ручке usecase |
| `http/ready` | `pinger` — проверка базы |
| `repository/trip` | `querier` — пул или открытая транзакция |

Почему у потребителя:

1. Usecase сам говорит, что ему нужно. Интерфейс узкий: в нём только те методы, которые этот usecase вызывает. Такой интерфейс легко замокать.
2. Реализация ничего не знает о том, кто её использует. Репозиторий просто имеет методы `Save`, `Get`, `Finish`, а Go сам проверит, что он подходит под интерфейс.
3. Usecase не импортирует ни репозиторий, ни драйвер базы. Поэтому в тестах вместо них подставляются моки.
4. В лабораторной работе 4 появится gRPC. Для него заводятся пакеты `internal/grpc/trip/create` и т. д. со своим `contract.go`, а usecase'ы не меняются: им всё равно, кто их вызывает.

Моки создаёт `mockgen` в `mocks_test.go` рядом с `contract.go`, отдельного пакета `mocks/` нет.

### Отступления от раскладки из задания

- `internal/generated/api/` вместо `internal/generated/`: у пакета своё имя `api`, а в работе 4 рядом появится код из `.proto`.
- `http/response`, `http/health`, `http/ready` — пакетов нет в схеме задания. Запись JSON нужна всем ручкам, а `health` и `ready` — тоже ручки, поэтому у каждой свой пакет.
- `http/trip/convert.go` — перевод поездки в ответ API общий для трёх ручек, чтобы не копировать его три раза.
- `cmd/service/` взят из задания ЛР2; в `conventions.md` написано `cmd/trip-service/`.
- В репозитории файлы разделены так: `save.go` — все записи (`Save`, `Finish`, `SaveStatusChange`), `fetch.go` — чтение.

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

## Лабораторная работа 2. Архитектура, качество кода и observability

Новых ручек и миграций нет. Код разложен по слоям, добавлены доменная модель, тесты, линтер, логи, метрики и трейсы.

Код из работы 1 переехал: менеджер транзакций — в `internal/infra/postgres/transaction`, проверка `isActiveDriverViolation` — в `internal/repository/trip/convert.go`, функция `executor` стала методом `conn` репозитория. Уровень Read Committed теперь задаётся внутри менеджера транзакций.

### Решения

**Сборка зависимостей – `container`**

`container.New` создаёт логгер, телеметрию, пул базы, репозиторий, usecase'ы, ручки и роутер и связывает их между собой. Больше нигде объекты не создаются, а `main` только запускает сервер и ждёт сигнала. DI-фреймворка нет, всё собрано вручную.

**Доменная модель – `domain/trip`**

- `trip.New(...)` создаёт поездку в статусе `active` и сразу проверяет её через `Validate()`: id не пустые, координаты в границах, цена не отрицательная, `active` без `finished_at`, `completed` с ним.
- `Finish(at)` переводит поездку в `completed`. Если она уже завершена, возвращается `ErrTripCompleted`. Метод не ходит в базу, поэтому проверяется обычным тестом.
- Ошибки домена `ErrTripNotFound`, `ErrTripCompleted`, `ErrDriverBusy` лежат в `domain/trip/errors.go`. Для ошибок валидации есть тип `ValidationError`, в нём поле и причина.

**Завершение поездки**

Usecase читает поездку, вызывает доменный `Finish`, затем репозиторий делает `UPDATE ... WHERE id = $1 AND status = 'active'`. Если два запроса пришли одновременно, второй `UPDATE` не найдёт активную поездку и вернёт `ErrTripCompleted`. Защита от гонки та же, что в работе 1.

**Единый маппинг ошибок – `http/domainerror`**

- `Problem(err)` — единственный `switch` по ошибкам: `ValidationError` → 400, `ErrTripNotFound` → 404, `ErrTripCompleted` и `ErrDriverBusy` → 409, всё остальное → 500.
- Ручка не разбирает ошибку, а сразу отдаёт её в `domainerror.Write`.
- Для 500 клиент видит только `internal_error`, а текст ошибки пишется в лог — один раз, в этом месте.
- Ошибки разбора JSON и неверный `tripId` тоже оформляются как `ValidationError`, поэтому все ошибки идут через один маппер.

**Время – интерфейс `clock`**

Usecase'ы не вызывают `time.Now()` сами, время приходит через интерфейс. В тестах его подменяет мок, поэтому можно проверить точное время старта и завершения.

**Тесты**

| Что | Как |
| :-- | :-- |
| домен | табличные тесты без моков |
| usecase'ы | моки `gomock` для репозитория, менеджера транзакций и `clock` |
| маппинг ошибок | табличный тест: ошибка → статус и `code`, для 500 нет деталей |
| ручки | `httptest` и мок usecase'а |
| middleware и логгер | имя спана и `http.route` — шаблон, уровень лога, метрики, `Recover` |
| репозиторий | реальный PostgreSQL, тег `integration`, есть тест на откат обеих таблиц |

Покрытие `domain` и `usecase` — 100%.

**Линтер**

`.golangci.yml` скопирован из материалов курса без изменений. Что пришлось поправить в коде:

- `main` разделён на `main` и `run`, иначе `log.Fatalf` срабатывал бы после `defer` (`gocritic: exitAfterDefer`).
- Переводы чисел в `int32` сделаны без риска переполнения (`gosec G115`).
- В лог больше не попадают данные запроса через `log.Printf` (`gosec G706`, подделка записей в логе).
- Повторяющиеся строки в репозитории вынесены в константы (`goconst`).

`nolint` в коде нет.

**Логи – `slog`**

- JSON в stdout, уровень из `LOG_LEVEL`, в каждой записи поле `service`.
- `trace_id` и `span_id` добавляет обёртка над обработчиком `slog` (`infra/telemetry/logger.go`): если в контексте есть спан, она берёт id из него.
- Логгер передаётся зависимостью, `slog.Default()` в коде нет.
- Middleware пишет одну запись `request handled` на запрос: метод, шаблон маршрута, статус, время. Для 5xx уровень `Error`, для остального `Info`. Тела запроса и ответа не пишутся.
- `trip created` и `trip completed` пишут usecase'ы, а не middleware.

**Трейсы**

- Корневой спан создаёт `otelhttp`. Его имя — `{метод} {шаблон маршрута}`, а шаблон берётся у роутера chi через `router.Find`. Так в имени и в `http.route` стоит `/api/v1/trips/{tripId}`, а не конкретный id.
- Спаны запросов к базе создаёт `otelpgx` на уровне пула, а не в каждом методе репозитория. Небольшая обёртка (`infra/postgres/tracer.go`) добавляет `db.operation.name`, `db.collection.name` и `error.type`, потому что сам `otelpgx` не ставит `db.collection.name`.
- Ошибка помечает спан статусом `Error` и записывает `error.type`. Для базы это код PostgreSQL, например `23505`, для HTTP — код 5xx.

**Метрики**

- `http.server.request.duration` и `http.server.active_requests` пишет свой middleware `Metrics`. Метрики `otelhttp` выключены, потому что он не знает шаблон маршрута chi.
- `db.client.operation.duration` пишет та же обёртка над `otelpgx`.
- `tripgo.trips.created` и `tripgo.trips.completed` увеличивают usecase'ы после успешной транзакции.
- В `http.route` только шаблон: если положить туда id, на каждую поездку появится своя серия, и метрика перестанет складываться.

**Коллектор недоступен**

Экспорт идёт в фоне, поэтому сервис работает как обычно, а ошибки экспорта пишутся в лог как `Warn`. При остановке сервис пробует отправить последние данные. Если коллектора нет, он пишет `Warn` и всё равно завершается нормально.

