# reviewsrv

AI code review сервер с интеграцией в CI, трекингом ревью, нотификацией в Slack, светофором и статистикой.

## Концепция

1. `reviewctl review` запускает Claude Code с промтом. На выходе — review.json + R*.md файлы
2. В JSON есть structured issues из MD файлов (генерит LLM)
3. Данные загружаются на сервер, комментарии постятся в GitLab MR

## Компоненты

| Компонент | Описание |
|-----------|----------|
| `reviewsrv` | HTTP-сервер: API, RPC, фронтенды |
| `reviewctl` | CLI-оркестратор: Claude → upload → GitLab comments |
| `frontend/` | Два SPA: review (публичный) и VT (админка) |

## Соглашения

- JSON: `lowerCamelCase` для всех ключей (`trafficLight`, `issuesStats`, `fileType`, ...)

## Файлы

* [reviewsrv.sql](../reviewsrv.sql) — БД-схема (источник истины)
* ObjectModel.md — объектная модель (RU)
* Model.md — детальная объектная модель (EN)
* [reviewctl.md](reviewctl.md) — CLI-оркестратор (детальная документация)

## reviewsrv

### Флаги

| Флаг | Default | Описание |
|------|---------|----------|
| `--config` | `config.toml` | Путь к конфигу |
| `--verbose` | `false` | Debug output |
| `--json` | `false` | JSON логи |
| `--dev` | `false` | Dev mode |
| `--patches` | — | Путь к SQL-патчам для авто-миграции |
| `--ts_client` | — | Генерация TS-клиента (exit после) |

### Авто-миграции (pgmigrator)

При запуске с `--patches /patches` сервер автоматически применяет SQL-миграции перед стартом:

```
reviewsrv --config config.toml --patches /patches
```

Используется библиотека `github.com/vmkteam/pgmigrator/pkg/migrator` — тот же `*pg.DB` что и для приложения, отдельный конфиг не нужен.

Миграции лежат в `docs/patches/*.sql`, формат: `YYYY-MM-DD-description.sql`.

### Dockerfile

```dockerfile
# Build
FROM golang:1.25-alpine AS builder
RUN cd /build && go install -mod=vendor ./cmd/reviewsrv
RUN cd /build && CGO_ENABLED=0 go build -mod=vendor -ldflags "-s -w" -o /go/bin/reviewctl ./cmd/reviewctl

# Final
FROM alpine:latest
COPY --from=builder /go/bin/reviewsrv .
COPY --from=builder /go/bin/reviewctl .
COPY docs/patches/*.sql /patches/
ENTRYPOINT ["/reviewsrv"]
```

Патчи копируются как `*.sql` из корня `docs/patches/` (без подпапок). pgmigrator читает только файлы из корневой директории.

## TypeScript API-клиенты

Оба фронтенда используют сгенерированные API-клиенты с враппером.

### Схема

```
*.generated.ts  ← curl с сервера (не редактируется)
*.ts            ← враппер: адаптирует сигнатуры и реэкспортирует типы
```

### Обновление

```bash
# Review API (основной фронтенд)
{ echo '// @ts-nocheck'; curl -sf http://localhost:8075/v1/rpc/api.ts; } > frontend/src/api/factory.generated.ts

# VT API (админка)
{ echo '// @ts-nocheck'; curl -sf http://localhost:8075/v1/vt/api.ts; } > frontend/src/api/vt.generated.ts
```

### Файлы

| Сгенерированный | Враппер | RPC-клиент | Эндпоинт |
|---|---|---|---|
| `factory.generated.ts` | `factory.ts` | `client.ts` (`/v1/rpc/`) | `/v1/rpc/api.ts` |
| `vt.generated.ts` | `vt.ts` | `vtClient.ts` (`/v1/vt/`) | `/v1/vt/api.ts` |

## REST API загрузки

Используется `reviewctl`, но можно вызывать напрямую:

```
POST /v1/upload/{projectKey}/                    → reviewId (plain text)
POST /v1/upload/{projectKey}/{reviewId}/{type}/  → 200
GET  /v1/prompt/{projectKey}/                    → prompt text
```

Коды ответов: 200 — ок, 404 — project key не найден, 400 — ошибка данных, 500 — ошибка сервера.
