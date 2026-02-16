# CI Integration — план реализации

## Цель

На странице списка проектов в VT-админке добавить кнопку «CI», при клике на которую открывается модальное окно с готовым `.gitlab-ci.yml` фрагментом.
Пользователь копирует YAML и вставляет его в свой репозиторий.

## Общая схема работы CI

```
GitLab CI (merge request)
  → fetch prompt from reviewsrv (/v1/prompt/$PROJECT_KEY/)
  → claude-code выполняет ревью с этим промтом
  → результаты (review.json + .md файлы) загружаются на сервер (/v1/upload/...)
  → артефакты (HTML) сохраняются в GitLab
```

## Dockerfile (уже есть)

```dockerfile
FROM node:20-alpine
RUN apk add git bash curl
WORKDIR /app
RUN npm install -g @anthropic-ai/claude-code
RUN npm install -g marked
CMD ["claude-code"]
```

Образ: `registry.gitlab/docker/claude-code:latest`

## Шаблон GitLab CI

Генерируемый YAML — GitLab CI компонент. Файл шаблона: `pkg/vt/gitlab-ci.yml.tmpl` (`//go:embed`).

### Переменные

| Переменная | Тип | Откуда | Используется |
|---|---|---|---|
| `REVIEWSRV_URL` | подставляется сервером | `server.baseURL` из конфига | curl к API (`/v1/prompt/`, `/v1/upload/`) |
| `TARGET_BRANCH` | подставляется сервером | параметр метода, по умолчанию `devel` | `git fetch`, `rules` |
| `PROJECT_KEY` | CI/CD variable | настройки проекта GitLab | curl к `/v1/prompt/`, `upload.js` |
| `ANTHROPIC_API_KEY` | CI/CD variable | настройки проекта GitLab | claude-code (подхватывает автоматически) |
| `CI_COMMIT_BRANCH` | GitLab predefined | автоматически | → `%SOURCE_BRANCH%` в промте |
| `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | GitLab predefined | автоматически | → `%TARGET_BRANCH%` в промте |
| `CI_MERGE_REQUEST_IID` | GitLab predefined | автоматически | → `%EXTERNAL_ID%` в промте (ID merge request) |
| `CI_MERGE_REQUEST_TITLE` | GitLab predefined | автоматически | → `%TITLE%` в промте, `rules` (фильтр draft) |
| `CI_PIPELINE_SOURCE` | GitLab predefined | автоматически | `rules` (фильтр merge_request_event) |

### Подстановка в промте

Серверный промт (`/v1/prompt/<projectKey>/`) содержит `%`-плейсхолдеры.
В CI-скрипте они заменяются через `sed` на значения GitLab CI predefined переменных:

| Плейсхолдер | CI-переменная | Пример |
|---|---|---|
| `%SOURCE_BRANCH%` | `CI_COMMIT_BRANCH` | `feature/auth` |
| `%TARGET_BRANCH%` | `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | `devel` |
| `%EXTERNAL_ID%` | `CI_MERGE_REQUEST_IID` | `2618` |
| `%TITLE%` | `CI_MERGE_REQUEST_TITLE` | `Add user auth` |

## Реализация

### 1. Backend — `pkg/vt/gitlab-ci.yml.tmpl`

YAML-шаблон с `text/template` плейсхолдерами `{{.BaseURL}}` и `{{.TargetBranch}}`.
`$PROJECT_KEY` и `$ANTHROPIC_API_KEY` остаются как есть — это переменные GitLab CI.

### 2. Backend — метод `project.gitlabCI` в `pkg/vt/project.go`

```go
//zenrpc:targetBranch string
//zenrpc:return string
func (s ProjectService) GitlabCI(_ context.Context, targetBranch string) (string, error)
```

- Если `targetBranch` пустой — `"devel"`
- Рендерит шаблон с `BaseURL` (из `s.baseURL`) и `TargetBranch`
- `baseURL` прокинут в `ProjectService` через `vt.New()` → `NewProjectService()`

### 3. Frontend — API-клиент

Обновить сгенерированный клиент:
```bash
{ echo '// @ts-nocheck'; curl -sf http://localhost:8075/v1/vt/api.ts; } > frontend/src/api/vt.generated.ts
```

Враппер `vt.ts` адаптирует сигнатуру:
```ts
gitlabCI: (targetBranch: string) => generated.project.gitlabCI({ targetBranch }),
```

### 4. Frontend — кнопка «CI» и модалка в `ProjectsPage.vue`

Кнопка «CI» в колонке actions таблицы проектов.

Модалка с двумя табами:
- Заголовок: «GitLab CI»
- Кнопка «Close»

**Таб 1: Review (по умолчанию)**
- Имя файла: `components/claude-code/templates/review.yml`
- Поле ввода `targetBranch` (по умолчанию `devel`)
- YAML из `vtApi.project.gitlabCI(targetBranch)`
- Кнопка «Copy»

**Таб 2: Dockerfile**
- Имя файла: `docker/claude-code/Dockerfile`
- Статический контент Dockerfile
- Кнопка «Copy»
