# reviewctl v1 — Go CLI Orchestrator

## Обзор

`reviewctl` — Go CLI-утилита, заменяющая bash (~80 строк) + Node.js (`upload.cjs`) в CI. Единый бинарник для полного цикла review: fetch prompt → Claude Code subprocess → parse review.json + R*.md → upload на сервер → post MR summary comment в GitLab.

**Версия:** v0.2.0
**Репозиторий:** monorepo `reviewsrv`, пакет `cmd/reviewctl/`
**Зависимости:** переиспользует `pkg/rest` (ReviewDraft), `pkg/reviewer` (константы, типы)

---

## Архитектура

```
GitLab CI Job
  └── reviewctl review
        ├── 1. GET /v1/prompt/{projectKey}/     → prompt text
        ├── 2. claude --print --output-format json --model opus -p "$prompt"
        │       → stdout: ClaudeResult JSON (cost, usage, result)
        │       → files: review.json, R1.md, R2.md, R3.md, R4.md, R5.md
        ├── 3. Parse review.json → ReviewDraft
        │       Merge ClaudeResult cost → ReviewDraft.ModelInfo
        ├── 4. POST /v1/upload/{projectKey}/    → reviewId
        │       POST /v1/upload/{projectKey}/{reviewId}/{type}/  × N files
        ├── 5. POST GitLab MR note (summary + critical issues)
        └── 6. Generate HTML artifact (goldmark)
```

---

## CLI Interface

### Subcommands

```
reviewctl review    — полный цикл review (default)
reviewctl upload    — только upload review.json + R*.md (без Claude)
reviewctl comment   — только post MR comment (без review)
reviewctl version   — версия бинарника
```

### Флаги и переменные окружения

Backward compatibility: все CI переменные продолжают работать.

| Флаг | Env Variable | Default | Описание |
|------|-------------|---------|----------|
| `--key` | `$PROJECT_KEY` | required | UUID проекта |
| `--url` | `$REVIEWSRV_URL` | required | URL сервера reviewsrv |
| `--model` | `$REVIEW_MODEL` | `opus` | Модель Claude |
| `--dir` | `$REVIEW_DIR` | `.` | Рабочая директория с review.json |
| `--verbose` | `$REVIEW_VERBOSE` | `false` | Подробный вывод |
| `--gitlab-url` | `$CI_API_V4_URL` | — | GitLab API URL (из CI) |
| `--gitlab-token` | `$REVIEWER_GITLAB_TOKEN` | — | GitLab API token для MR comments |
| `--mr-iid` | `$CI_MERGE_REQUEST_IID` | — | MR IID для комментария |
| `--project-id` | `$CI_PROJECT_ID` | — | GitLab Project ID |
| `--source-branch` | `$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME` | — | Source branch |
| `--target-branch` | `$CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | — | Target branch |
| `--commit` | `$CI_COMMIT_SHA` | — | Commit SHA |
| `--author` | `$GITLAB_USER_LOGIN` | — | Автор MR |
| `--mr-title` | `$CI_MERGE_REQUEST_TITLE` | — | Заголовок MR |
| `--external-id` | `$CI_MERGE_REQUEST_IID` | — | External ID (MR IID) |
| `--diff-base-sha` | `$CI_MERGE_REQUEST_DIFF_BASE_SHA` | — | Base SHA для inline comments |

### Примеры

```bash
# Полный цикл в CI (все переменные из CI environment)
reviewctl review

# Явные флаги
reviewctl review --key $PROJECT_KEY --url https://reviewer.example.com --model sonnet

# Только upload (после ручного запуска Claude)
reviewctl upload --key $PROJECT_KEY --url https://reviewer.example.com --dir .

# Только комментарий в MR
reviewctl comment --key $PROJECT_KEY --url https://reviewer.example.com --review-id 42
```

---

## Subcommand: review

Основной flow. Последовательность шагов:

### Step 1: Fetch Prompt

```
GET {serverURL}/v1/prompt/{projectKey}/
```

Подстановка CI переменных в prompt template (как в текущем bash):
- `%SOURCE_BRANCH%` → `$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME`
- `%TARGET_BRANCH%` → `$CI_MERGE_REQUEST_TARGET_BRANCH_NAME`
- `%MR_TITLE%` → `$CI_MERGE_REQUEST_TITLE`
- `%EXTERNAL_ID%` → `$CI_MERGE_REQUEST_IID`

### Step 2: Run Claude Code

```bash
claude --print \
  --output-format json \
  --model $MODEL \
  --permission-mode bypassPermissions \
  --verbose \
  -p "$PROMPT"
```

**Важно:** `--output-format json` (не `stream-json`) — для v1 достаточно. Результат — один JSON объект в stdout.

Claude Code создаёт файлы в рабочей директории:
- `review.json` — structured review data
- `R1.*.md` — architecture
- `R2.*.md` — code
- `R3.*.md` — security
- `R4.*.md` — tests
- `R5.*.md` — operability (опционально)

### Step 3: Parse Results

1. Parse Claude stdout → `ClaudeResult` (cost, usage, duration)
2. Parse `review.json` → `ReviewDraft`
3. Merge cost data: `ClaudeResult` → `ReviewDraft.Review.ModelInfo`
4. Заполнить MR metadata из CI env variables

### Step 4: Upload to Server

```
POST {serverURL}/v1/upload/{projectKey}/
Body: review.json (с обогащённой ModelInfo)
Response: reviewId (plain text)

POST {serverURL}/v1/upload/{projectKey}/{reviewId}/architecture/
Body: R1.*.md content
...повторить для каждого R*.md файла
```

### Step 5: Post GitLab MR Comment

Если `--gitlab-token` и `--mr-iid` заданы — постим summary comment.

### Step 6: Generate HTML Artifact

Markdown → HTML через goldmark. Сохраняется как `review.html` для GitLab CI artifacts.

---

## Subcommand: upload

Только шаги 4-6 из review flow. Для случаев когда Claude запускался отдельно.

```bash
reviewctl upload --key $PROJECT_KEY --url $REVIEWSRV_URL --dir .
```

Читает `review.json` и `R*.md` из `--dir`, загружает на сервер.

---

## Subcommand: comment

Только шаг 5 из review flow. Для повторной отправки комментария.

```bash
reviewctl comment --key $PROJECT_KEY --url $REVIEWSRV_URL --review-id 42
```

Получает review данные с сервера и постит MR comment.

---

## Cost Tracking

### Текущая модель

```go
// pkg/db/model_params.go
type ReviewModelInfo struct {
    Model        string  `json:"model"`
    InputTokens  int     `json:"inputTokens"`
    OutputTokens int     `json:"outputTokens"`
    CostUsd      float64 `json:"costUsd"`
}
```

### Новая модель (расширенная)

```go
type ReviewModelInfo struct {
    // Existing
    Model        string  `json:"model"`
    InputTokens  int     `json:"inputTokens"`
    OutputTokens int     `json:"outputTokens"`
    CostUsd      float64 `json:"costUsd"`

    // New: cache tokens from Claude output
    CacheCreationInputTokens int `json:"cacheCreationInputTokens,omitempty"`
    CacheReadInputTokens     int `json:"cacheReadInputTokens,omitempty"`

    // New: session metadata
    NumTurns     int    `json:"numTurns,omitempty"`
    SessionID    string `json:"sessionId,omitempty"`
    DurationApiMs int   `json:"durationApiMs,omitempty"`
}
```

**Backward compatible:** новые поля с `omitempty`, старые reviews без них — работают. JSONB в PostgreSQL хранит как есть, новые ключи появляются при заполнении.

### Claude JSON Output → ReviewModelInfo

```go
// ClaudeResult — parsed stdout от claude --output-format json
type ClaudeResult struct {
    Type         string  `json:"type"`          // "result"
    Subtype      string  `json:"subtype"`       // "success" | "error"
    Result       string  `json:"result"`        // text output
    TotalCostUSD float64 `json:"total_cost_usd"`
    DurationMs   int     `json:"duration_ms"`
    DurationApiMs int    `json:"duration_api_ms"`
    NumTurns     int     `json:"num_turns"`
    SessionID    string  `json:"session_id"`
    Usage        struct {
        InputTokens              int `json:"input_tokens"`
        CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
        CacheReadInputTokens     int `json:"cache_read_input_tokens"`
        OutputTokens             int `json:"output_tokens"`
    } `json:"usage"`
}

// toModelInfo converts ClaudeResult to ReviewModelInfo
func (cr ClaudeResult) toModelInfo(model string) ReviewModelInfo {
    return ReviewModelInfo{
        Model:                    model,
        InputTokens:             cr.Usage.InputTokens,
        OutputTokens:            cr.Usage.OutputTokens,
        CostUsd:                 cr.TotalCostUSD,
        CacheCreationInputTokens: cr.Usage.CacheCreationInputTokens,
        CacheReadInputTokens:    cr.Usage.CacheReadInputTokens,
        NumTurns:                cr.NumTurns,
        SessionID:               cr.SessionID,
        DurationApiMs:           cr.DurationApiMs,
    }
}
```

### REST ReviewDraft.ModelInfo

```go
// pkg/rest/model.go — расширить ModelInfo в ReviewDraft
ModelInfo struct {
    Model                    string  `json:"model"`
    InputTokens              int     `json:"inputTokens"`
    OutputTokens             int     `json:"outputTokens"`
    CostUsd                  float64 `json:"costUsd"`
    CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
    CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
    NumTurns                 int     `json:"numTurns"`
    SessionID                string  `json:"sessionId"`
    DurationApiMs            int     `json:"durationApiMs"`
} `json:"modelInfo"`
```

### RPC ModelInfo

```go
// pkg/rpc/model.go — расширить ModelInfo для фронтенда
type ModelInfo struct {
    Model                    string  `json:"model"`
    InputTokens              int     `json:"inputTokens"`
    OutputTokens             int     `json:"outputTokens"`
    CostUsd                  float64 `json:"costUsd"`
    CacheCreationInputTokens int     `json:"cacheCreationInputTokens,omitempty"`
    CacheReadInputTokens     int     `json:"cacheReadInputTokens,omitempty"`
    NumTurns                 int     `json:"numTurns,omitempty"`
    SessionID                string  `json:"sessionId,omitempty"`
    DurationApiMs            int     `json:"durationApiMs,omitempty"`
}
```

---

## GitLab MR Comment

### Два типа комментариев

1. **Summary comment** — один комментарий с общей статистикой review
2. **Inline comments** — critical issues прямо в diff на соответствующих строках

### Summary Comment

```markdown
## 🔴 Code Review — Red Light

**Model:** claude-opus-4-6 | **Cost:** $1.52 | **Duration:** 1m 33s | **Effort:** ~15 min

| Type | Issues | Traffic Light |
|------|--------|--------------|
| Architecture | 1 critical, 2 medium | 🔴 |
| Code | 3 high, 5 medium, 2 low | 🔴 |
| Security | 0 | 🟢 |
| Tests | 1 medium | 🟢 |

### Critical Issues

**C1.** Missing error handling in `pkg/api/handler.go:42-45` (error-handling)
> Handler ignores error from database query, leading to potential nil pointer dereference.

**C2.** SQL injection in `pkg/db/query.go:18` (security)
> Raw string concatenation in SQL query allows injection attacks.

---
*[Full review →](https://reviewer.example.com/reviews/42/)*
```

### Inline Comments (Critical Issues)

Critical issues постятся как inline discussions прямо в diff MR.

**GitLab API — Create Discussion:**

```
POST /api/v4/projects/{projectId}/merge_requests/{mrIid}/discussions
Authorization: Bearer $REVIEWER_GITLAB_TOKEN
Content-Type: application/json

{
  "body": "🔴 **C1. Missing error handling** (error-handling)\n\nHandler ignores error from database query, leading to potential nil pointer dereference.\n\n**Suggested fix:**\n```go\nresult, err := doSomething()\nif err != nil {\n    return fmt.Errorf(\"handler: %w\", err)\n}\n```",
  "position": {
    "base_sha": "<MR diff base SHA>",
    "head_sha": "<MR head SHA>",
    "start_sha": "<MR diff start SHA>",
    "position_type": "text",
    "new_path": "pkg/api/handler.go",
    "new_line": 42
  }
}
```

### Маппинг Issue → Inline Position

```go
// parseLinePosition extracts first line number from issue.Lines ("42-45" → 42, "42" → 42)
func parseLinePosition(lines string) (int, bool) {
    parts := strings.SplitN(lines, "-", 2)
    if len(parts) == 0 || parts[0] == "" {
        return 0, false
    }
    n, err := strconv.Atoi(parts[0])
    return n, err == nil
}
```

**Данные из issue:**
- `issue.File` → `position.new_path`
- `issue.Lines` → `position.new_line` (первая строка диапазона)
- `issue.SuggestedFix` → включается в body комментария, если непустой

**SHA параметры из CI:**
- `$CI_MERGE_REQUEST_DIFF_BASE_SHA` → `position.base_sha`, `position.start_sha`
- `$CI_COMMIT_SHA` → `position.head_sha`

### Fallback

Если inline comment не удаётся (файл/строка вне diff, GitLab 400) — issue постится как обычный note без position. Это гарантирует что все critical issues видны в MR.

```go
func (g *GitLabClient) PostInlineComment(ctx context.Context, issue Issue) error {
    // 1. Try creating discussion with position
    err := g.createDiscussion(ctx, issue, withPosition)
    if err == nil {
        return nil
    }

    // 2. Fallback: plain note without position
    return g.createNote(ctx, formatIssueAsNote(issue))
}
```

### Summary Comment API

```
POST /api/v4/projects/{projectId}/merge_requests/{mrIid}/notes
Authorization: Bearer $REVIEWER_GITLAB_TOKEN
Content-Type: application/json

{"body": "<summary markdown>"}
```

Token — CI env variable `$REVIEWER_GITLAB_TOKEN`. Рекомендуется Project Access Token с scope `api`.

### Дополнительные CI Variables

| Флаг | Env Variable | Описание |
|------|-------------|----------|
| `--diff-base-sha` | `$CI_MERGE_REQUEST_DIFF_BASE_SHA` | Base SHA для inline position |

### Логика

1. Собрать issue stats из review.json
2. POST summary comment (всегда)
3. Отфильтровать critical issues с непустыми `file` и `lines`
4. POST inline discussion для каждого critical issue (с fallback)
5. Ошибки MR comments не блокируют — warning в лог

---

## HTML Generation (goldmark)

### Замена marked.js

Текущий CI использует `marked` (Node.js) для markdown → HTML. reviewctl генерирует HTML через [goldmark](https://github.com/yuin/goldmark) — pure Go markdown parser.

### Что генерируем

Один `review.html` файл из R*.md файлов. Используется как GitLab CI artifact.

### Зависимости

```go
import (
    "github.com/yuin/goldmark"
    highlighting "github.com/yuin/goldmark-highlighting/v2"
)
```

### Шаблон

```go
// Встроенный HTML template с inline CSS
//go:embed review.html.tmpl
var reviewHTMLTmpl string

type ReviewHTML struct {
    Title       string
    TrafficLight string
    Sections    []ReviewSection // architecture, code, security, tests, operability
}

type ReviewSection struct {
    Type    string
    Content template.HTML // rendered markdown
}
```

---

## Docker / CI Integration

### Стратегия

reviewctl собирается вместе с reviewsrv в основном `Dockerfile`. Для CI image копируется из `vmkteam/reviewer`:

```dockerfile
# Основной Dockerfile — backend stage:
RUN cd /build && go install -mod=vendor ./cmd/reviewsrv
RUN cd /build && CGO_ENABLED=0 go build -mod=vendor -ldflags "-s -w" -o /go/bin/reviewctl ./cmd/reviewctl

# Final stage:
COPY --from=builder /go/bin/reviewsrv .
COPY --from=builder /go/bin/reviewctl .
```

```dockerfile
# Dockerfile.ci (vmkteam/claude-ci)
FROM vmkteam/reviewer:latest AS source

FROM node:20-alpine
RUN apk add --no-cache git bash curl
RUN npm install -g @anthropic-ai/claude-code
COPY --from=source /reviewctl /usr/local/bin/reviewctl
WORKDIR /workspace
```

### Makefile targets

```makefile
build-reviewctl:
	@CGO_ENABLED=0 go build $(GOFLAGS) \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		-o bin/reviewctl ./cmd/reviewctl
```

### GitLab CI Template (новый)

```yaml
review:
  stage: review
  image: vmkteam/claude-ci:latest
  variables:
    GIT_DEPTH: 100
  script:
    - reviewctl review
  artifacts:
    paths:
      - review.html
    expire_in: 30 days
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

**До:** ~80 строк bash + upload.cjs
**После:** 1 строка `reviewctl review`

### Требуемые CI Variables (в GitLab Project Settings)

| Variable | Описание | Protected |
|----------|----------|-----------|
| `PROJECT_KEY` | UUID проекта в reviewsrv | No |
| `REVIEWSRV_URL` | URL сервера | No |
| `ANTHROPIC_API_KEY` | API ключ Claude | Yes |
| `REVIEWER_GITLAB_TOKEN` | GitLab token для MR comments | Yes |
| `REVIEW_MODEL` | Модель (опционально, default: opus) | No |

Автоматически из GitLab CI: `$CI_API_V4_URL`, `$CI_PROJECT_ID`, `$CI_MERGE_REQUEST_IID`, `$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME`, `$CI_MERGE_REQUEST_TARGET_BRANCH_NAME`, `$CI_MERGE_REQUEST_TITLE`, `$CI_COMMIT_SHA`, `$GITLAB_USER_LOGIN`.

---

## Структура кода

`cmd/reviewctl/main.go` — минимальный entrypoint (cobra setup, flag parsing → Config → вызов `pkg/reviewer/ctl`).
Вся бизнес-логика — в `pkg/reviewer/ctl/`.

```
cmd/reviewctl/
  main.go              — entrypoint, cobra commands, flag→Config, вызов pkg/reviewer/ctl

pkg/reviewer/ctl/
  config.go            — Config struct
  ctl.go               — Controller: Review(), Upload(), Comment() — orchestration
  claude.go            — ClaudeResult struct + JSON parsing
  upload.go            — HTTP client: upload review.json + R*.md to server
  prompt.go            — HTTP client: fetch prompt + CI variable substitution
  gitlab.go            — GitLab MR notes API client + summary template
  html.go              — goldmark markdown → HTML rendering
  review.html.tmpl     — HTML template (embedded)
  gitlab_comment.tmpl  — MR comment markdown template (embedded)
```

### Зависимости (go.mod)

```
github.com/spf13/cobra                  — CLI framework (только в cmd/)
github.com/yuin/goldmark                — markdown → HTML
github.com/yuin/goldmark-highlighting/v2 — syntax highlighting
```

Переиспользуется из monorepo:
- `reviewsrv/pkg/rest` — `ReviewDraft`, `ReviewDraft.Validate()`
- `reviewsrv/pkg/reviewer` — константы (`ReviewTypes`, `Severities`), `IsValidReviewType()`, `IsValidSeverity()`

---

## DB Changes

### ReviewModelInfo JSONB

Нет DDL миграции — `modelInfo` это JSONB поле. Новые ключи добавляются автоматически при записи расширенного JSON. Старые записи с 4 полями продолжают читаться.

### model_params.go

Единственное изменение — расширить struct `ReviewModelInfo` (описано в секции Cost Tracking).

---

## Backward Compatibility

| Что | Статус |
|-----|--------|
| Старый bash CI + upload.cjs | Работает — server API не меняется |
| Старые reviews без cache tokens | Работают — `omitempty` в новых полях |
| ReviewDraft без новых полей ModelInfo | Работает — zero values при десериализации |
| GitLab MR comment | Опциональный — без token просто не постится |
| HTML generation | Опциональный — если goldmark недоступен, skip |

---

## Порядок реализации

```
 1. pkg/db/model_params.go              — расширить ReviewModelInfo
 2. pkg/rest/model.go                   — расширить ModelInfo в ReviewDraft
 3. pkg/rpc/model.go                    — расширить ModelInfo для фронтенда
 4. make generate                       — перегенерация zenrpc SMD
 5. pkg/reviewer/ctl/config.go          — Config struct
 6. pkg/reviewer/ctl/claude.go          — ClaudeResult struct + JSON parsing
 7. pkg/reviewer/ctl/prompt.go          — fetch prompt + CI variable substitution
 8. pkg/reviewer/ctl/upload.go          — HTTP upload review.json + R*.md
 9. pkg/reviewer/ctl/gitlab.go          — GitLab MR comment
10. pkg/reviewer/ctl/html.go            — goldmark HTML generation
11. pkg/reviewer/ctl/ctl.go             — Controller orchestration
12. cmd/reviewctl/main.go               — cobra entrypoint (thin)
13. Makefile                            — build-reviewctl target
14. go build ./cmd/reviewctl            — verify
```

---

## Тестирование

Все HTTP-взаимодействия тестируются через `net/http/httptest`. Claude subprocess не тестируется (внешний бинарник), но парсинг его output — да.

### Структура тестов

```
pkg/reviewer/ctl/
  claude_test.go         — парсинг ClaudeResult JSON
  upload_test.go         — upload review.json + R*.md через httptest
  prompt_test.go         — fetch prompt + variable substitution через httptest
  gitlab_test.go         — GitLab MR comment через httptest
  html_test.go           — goldmark rendering
  ctl_test.go            — integration: Review() / Upload() / Comment() через httptest
  testdata/
    claude_result.json        — полный Claude --output-format json output
    claude_result_error.json  — Claude error output
    review.json               — валидный review.json
    review_minimal.json       — минимальный review.json (без optional полей)
    review_invalid.json       — невалидный review.json
    R1.architecture.md        — тестовый markdown (с mermaid)
    R2.code.md
    R3.security.md
    R4.tests.md
```

### claude_test.go

```go
func TestParseClaudeResult(t *testing.T) {
    // success: парсит все поля включая cache tokens, cost, sessionId
    // error: subtype="error" → возвращает ошибку
    // empty usage: zero values, не паникует
}

func TestClaudeResultToModelInfo(t *testing.T) {
    // маппинг всех полей ClaudeResult.Usage → ReviewModelInfo
    // cacheCreationInputTokens, cacheReadInputTokens заполнены
}
```

### upload_test.go

```go
func TestUploadReview(t *testing.T) {
    // httptest server имитирует POST /v1/upload/{projectKey}/
    // проверяет: Content-Type application/json, body = review.json
    // возвращает reviewId, проверяет парсинг ответа

    // httptest server имитирует POST /v1/upload/{projectKey}/{reviewId}/{type}/
    // проверяет: Content-Type application/octet-stream, body = md content
    // проверяет: правильный маппинг R1→architecture, R2→code, R3→security, R4→tests, R5→operability
}

func TestUploadReview_ServerError(t *testing.T) {
    // server возвращает 500 → ошибка
    // server возвращает 404 → project not found
    // server возвращает 400 → validation error
}
```

### prompt_test.go

```go
func TestFetchPrompt(t *testing.T) {
    // httptest server возвращает prompt text
    // проверяет: GET /v1/prompt/{projectKey}/
}

func TestSubstituteVariables(t *testing.T) {
    // %SOURCE_BRANCH% → "feature/foo"
    // %TARGET_BRANCH% → "master"
    // %MR_TITLE% → "Add new feature"
    // %EXTERNAL_ID% → "123"
    // без переменных — текст не меняется
    // неизвестные %PLACEHOLDER% — остаются как есть
}
```

### gitlab_test.go

```go
func TestPostSummaryComment(t *testing.T) {
    // httptest server имитирует POST /api/v4/projects/{id}/merge_requests/{iid}/notes
    // проверяет: Authorization header = "Bearer <token>"
    // проверяет: body содержит JSON с "body" полем
    // проверяет: markdown содержит traffic light, issue stats, critical issues
}

func TestPostInlineComment(t *testing.T) {
    // httptest server имитирует POST /api/v4/projects/{id}/merge_requests/{iid}/discussions
    // проверяет: position.new_path = issue.File
    // проверяет: position.new_line = первая строка из issue.Lines
    // проверяет: position.base_sha, head_sha, start_sha заполнены
    // проверяет: body содержит severity emoji, title, description
    // проверяет: body содержит suggestedFix если непустой
}

func TestPostInlineComment_Fallback(t *testing.T) {
    // httptest server возвращает 400 на discussions
    // проверяет: fallback на POST /notes (без position)
    // проверяет: содержимое fallback note
}

func TestPostInlineComment_NoFileOrLines(t *testing.T) {
    // issue без file или lines — skip inline, не ошибка
}

func TestPostMRComment_NoToken(t *testing.T) {
    // без gitlab token — skip всё, не ошибка
}

func TestRenderSummaryComment(t *testing.T) {
    // red light: содержит 🔴, critical issues listed
    // yellow light: содержит 🟡, no critical section
    // green light: содержит 🟢, congratulatory message
    // с effort: содержит "~15 min"
    // с cost: содержит "$1.52"
}

func TestParseLinePosition(t *testing.T) {
    // "42-45" → 42, true
    // "42" → 42, true
    // "" → 0, false
    // "abc" → 0, false
}
```

### html_test.go

```go
func TestRenderHTML(t *testing.T) {
    // markdown с code blocks → HTML с syntax highlighting
    // markdown с mermaid blocks → HTML с <pre class="mermaid">
    // пустой markdown → пустой HTML body
}
```

### ctl_test.go

Integration тесты — полный flow через httptest.

```go
func TestController_Upload(t *testing.T) {
    // httptest server имитирует reviewsrv API
    // читает testdata/review.json + R*.md
    // проверяет: review создан, файлы загружены, reviewId получен
}

func TestController_Review(t *testing.T) {
    // httptest для reviewsrv (prompt + upload)
    // claude subprocess заменяется на testdata/claude_result.json (через interface/mock)
    // проверяет: полный flow от prompt до upload
    // проверяет: ModelInfo заполнена из ClaudeResult
}

func TestController_Comment(t *testing.T) {
    // httptest для GitLab API
    // проверяет: MR comment создан с правильным содержимым
}
```

### Подход к тестированию Claude subprocess

Claude Code — внешний бинарник, не мокаем. Вместо этого:

1. `ClaudeRunner` interface в `ctl.go`:
```go
type ClaudeRunner interface {
    Run(ctx context.Context, prompt string) (*ClaudeResult, error)
}
```

2. Реальная реализация `ExecClaudeRunner` — запускает `claude` subprocess
3. В тестах — `TestClaudeRunner` возвращает фикстуру из `testdata/claude_result.json`

Это позволяет тестировать весь flow без реального Claude.

---

## Open Questions (для отдельного обсуждения)

### Claude Code subprocess (пункт 3)

- `--output-format json` vs `stream-json` — в v1 используем `json` (проще парсить)
- `--continue` / session cache — откладываем до interactive session (v2)
- `--permission-mode` — `bypassPermissions` в CI (trusted environment)
- Параметризуемая модель через `--model` флаг (default: opus)
- Timeout: наследуется от CI job timeout (default 45m)
- Обработка ошибок: если Claude вернул `subtype: "error"` — exit code 1, лог ошибки
