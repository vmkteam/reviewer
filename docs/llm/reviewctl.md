# reviewctl — Go CLI Orchestrator

## Обзор

`reviewctl` — Go CLI-утилита для полного цикла AI code review: fetch prompt → Claude Code subprocess → parse review.json + R*.md → upload на сервер → post MR comments в GitLab → generate HTML.

**Репозиторий:** monorepo `reviewsrv`, пакет `cmd/reviewctl/`
**Зависимости:** переиспользует `pkg/rest` (ReviewDraft), `pkg/reviewer` (константы, типы)

---

## Архитектура

```
GitLab CI Job / Local
  └── reviewctl review
        ├── 1. GET /v1/prompt/{projectKey}/     → prompt text
        ├── 2. claude --print --output-format json --model opus -p "$prompt"
        │       → stdout: ClaudeResult JSON (cost, usage, result)
        │       → files: review.json, R1.md, R2.md, R3.md, R4.md, R5.md
        ├── 3. Parse review.json → ReviewDraft
        │       Merge ClaudeResult cost → ReviewDraft.ModelInfo
        ├── 4. POST /v1/upload/{projectKey}/    → reviewId
        │       POST /v1/upload/{projectKey}/{reviewId}/{type}/  × N files
        ├── 5. GitLab MR comments:
        │       - Cleanup old inline discussions (без ответов)
        │       - POST summary note (история прогресса)
        │       - POST inline discussions (critical + high issues)
        └── 6. Generate HTML artifact (goldmark)
```

---

## CLI Interface

### Subcommands

```
reviewctl review    — полный цикл review
reviewctl upload    — только upload review.json + R*.md (без Claude)
reviewctl comment   — только post MR comments (без review)
reviewctl version   — версия бинарника
```

### Флаги

| Флаг | Env Variable | Default | Описание |
|------|-------------|---------|----------|
| `--key` | `$PROJECT_KEY` | required | UUID проекта |
| `--url` | `$REVIEWSRV_URL` | required | URL сервера reviewsrv |
| `--model` | `$REVIEW_MODEL` | `opus` | Модель Claude |
| `--dir` | `$REVIEW_DIR` | `.` | Рабочая директория |
| `--verbose` | `$REVIEW_VERBOSE` | `false` | Подробный вывод |
| `--session` | — | — | Claude session ID для `--resume` (prompt cache) |
| `--continue` | — | `false` | Продолжить последнюю сессию Claude |
| `--gitlab-url` | `$CI_API_V4_URL` | — | GitLab API URL |
| `--gitlab-token` | `$REVIEWER_GITLAB_TOKEN` | — | GitLab API token |
| `--mr-iid` | `$CI_MERGE_REQUEST_IID` | — | MR IID |
| `--project-id` | `$CI_PROJECT_ID` | — | GitLab Project ID |
| `--source-branch` | `$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME` | — | Source branch |
| `--target-branch` | `$CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | — | Target branch |
| `--commit` | `$CI_COMMIT_SHA` | — | Commit SHA |
| `--author` | `$GITLAB_USER_LOGIN` | — | Автор MR |
| `--mr-title` | `$CI_MERGE_REQUEST_TITLE` | — | Заголовок MR |
| `--external-id` | `$CI_MERGE_REQUEST_IID` | — | External ID |
| `--diff-base-sha` | `$CI_MERGE_REQUEST_DIFF_BASE_SHA` | — | Base SHA для inline comments |

### comment subcommand

| Флаг | Описание |
|------|----------|
| `--review-id` | ID существующего review (для повторной отправки комментариев) |

---

## Claude Code subprocess

```bash
claude --print \
  --output-format json \
  --model $MODEL \
  --permission-mode bypassPermissions \
  -p "$PROMPT"
```

**Важно:** флаг `--verbose` НЕ используется — он заставляет Claude CLI выводить все сообщения как JSON array, что при большом review может превысить буфер (448KB) и обрезать вывод.

### Session caching

- `--session <id>` → `claude --resume <id>` — переиспользует prompt cache (~90% экономии input tokens)
- `--continue` → `claude --continue` — продолжает последнюю сессию

### Парсинг вывода

`ParseClaudeResult` поддерживает:
- Одиночный JSON-объект (нормальный режим без `--verbose`)
- JSON array (от `--resume` / `--verbose`) — потоковый декодер (`json.Decoder`), толерантен к обрезанным массивам

---

## GitLab MR Comments

### Поведение при перезапуске

1. **Summary note** — всегда создаётся новый. Старые НЕ удаляются — показывают историю прогресса ревью (🔴 → 🟡 → 🟢)
2. **Inline discussions** — перед созданием новых, удаляются старые reviewer-discussions БЕЗ ответов (`notes_count == 1`). Discussions с ответами сохраняются
3. Все reviewer-комменты помечаются скрытым маркером `<!-- reviewer -->` для идентификации

### Фильтрация discussions при cleanup

- Удаляются только `DiffNote` (inline) — summary notes (`type: null`) не трогаются
- Удаляются только discussions с маркером `<!-- reviewer -->`
- Discussions с ответами (`notes_count > 1`) пропускаются

### Severity фильтр для inline comments

Inline discussions создаются для issues с severity `critical` и `high`.

### Summary comment формат

```markdown
## 🔴 Reviewer

**Model:** opus | **Cost:** $1.89 | **Duration:** 5m 28s | **Effort:** ~30 min

{review.description — общее описание ревью}

| Type | Summary | Issues | Traffic Light |
|------|---------|--------|--------------|
| Architecture | {file.summary} | 1 high, 1 medium | 🟡 |
| Code | {file.summary} | 2 low | 🟢 |
...

### Critical & High Issues

**A1.** Issue title in `file.go:42-45` (issue-type)
> Description

---
*[Full review →](https://reviewer.example.com/reviews/42/)*
<!-- reviewer -->
```

### Inline comment формат

```markdown
🔴 **A1. Issue title** (issue-type)

Description

**Suggested fix:**
{suggestedFix if present}

<!-- reviewer -->
```

### Fallback

Если inline comment не удаётся (строка вне diff, GitLab 400) — issue постится как обычный note без position.

---

## Структура кода

```
cmd/reviewctl/
  main.go              — entrypoint, cobra commands, flag→Config

pkg/reviewer/ctl/
  config.go            — Config struct, Validate(), HasGitLab()
  ctl.go               — Controller: Review(), Upload(), Comment(), postComments()
  claude.go            — ClaudeResult, ParseClaudeResult (streaming JSON decoder)
  upload.go            — HTTP client: upload review.json + R*.md
  prompt.go            — HTTP client: fetch prompt + CI variable substitution
  gitlab.go            — GitLab client: summary, inline, cleanup
  html.go              — goldmark markdown → HTML rendering
  review.html.tmpl     — HTML template (embedded)
  gitlab_comment.tmpl  — MR comment markdown template (embedded)
```

---

## Releases

### GoReleaser

Бинарник `reviewctl` публикуется через GoReleaser в GitHub Releases при создании тега `v*`.

- Платформа: `linux/amd64`, статическая линковка (`CGO_ENABLED=0`)
- Архив: `reviewctl_{version}_linux_amd64.tar.gz`
- Версия инжектится через `-X main.version={{.Version}}`
- Конфиг: `.goreleaser.yml`

```bash
# создание релиза
git tag v0.1.3
git push origin v0.1.3
# GitHub Actions: test → goreleaser release → GitHub Release с артефактом
```

### Установка reviewctl

```bash
# скачать последнюю версию
LATEST=$(curl -sL -o /dev/null -w '%{url_effective}' https://github.com/vmkteam/reviewer/releases/latest | grep -oE '[^/]+$')
curl -sL "https://github.com/vmkteam/reviewer/releases/download/${LATEST}/reviewctl_${LATEST#v}_linux_amd64.tar.gz" \
  | tar -xz -C /usr/local/bin reviewctl
```

---

## Docker / CI

### Dockerfile (reviewsrv)

```dockerfile
COPY --from=builder /go/bin/reviewsrv .
COPY --from=builder /go/bin/reviewctl .
COPY docs/patches/*.sql /patches/

ENTRYPOINT ["/reviewsrv"]
```

Docker-образ `vmkteam/reviewer` публикуется через `.github/workflows/docker.yml` при создании GitHub Release.

### CI image (vmkteam/claude-ci)

```dockerfile
FROM node:20-alpine
ARG REVIEWCTL_VERSION=latest
RUN apk add --no-cache curl git \
 && LATEST=$(curl -sL -o /dev/null -w '%{url_effective}' https://github.com/vmkteam/reviewer/releases/${REVIEWCTL_VERSION} | grep -oE '[^/]+$') \
 && curl -sL "https://github.com/vmkteam/reviewer/releases/download/${LATEST}/reviewctl_${LATEST#v}_linux_amd64.tar.gz" \
    | tar -xz -C /usr/local/bin reviewctl
RUN npm install -g @anthropic-ai/claude-code
```

Бинарник `reviewctl` скачивается из GitHub Releases — не требуется пересборка Docker-образа `vmkteam/reviewer` при обновлении только `reviewctl`.

### GitLab CI

```yaml
review:
  stage: review
  image: vmkteam/claude-ci:latest
  script:
    - reviewctl review
  artifacts:
    paths:
      - review.html
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

### CI Variables

| Variable | Описание | Protected |
|----------|----------|-----------|
| `PROJECT_KEY` | UUID проекта | No |
| `REVIEWSRV_URL` | URL сервера | No |
| `ANTHROPIC_API_KEY` | API ключ Claude | Yes |
| `REVIEWER_GITLAB_TOKEN` | GitLab token для MR comments | Yes |
| `REVIEW_MODEL` | Модель (default: opus) | No |
