# Кеширование сессий Claude Code на CI для экономии токенов

## Контекст

При повторном запуске review на одном MR, Claude Code заново читает все файлы и строит контекст с нуля. Если использовать `--continue`, Claude подхватит предыдущую сессию, уже содержащую file reads, diff и анализ — и потратит меньше токенов на повторную работу.

Prompt caching на уровне API (TTL 5 мин) между запусками CI не поможет, но экономия за счёт контекста реальна: меньше tool calls, меньше повторного чтения файлов.

### Project hash и разные runner-ы

Claude Code хранит сессии в `~/.claude/projects/<project-hash>/`, где `project-hash` вычисляется из абсолютного пути к репозиторию (замена `/` на `-`, удаление ведущего `-`). На разных CI runner-ах `$PWD` может отличаться (например `/builds/abc123/project` vs `/builds/xyz789/project`), поэтому hash будет разный. При restore нужно находить сессию в кеше по любому hash и копировать её в директорию с текущим hash.

## Что меняется

### `pkg/vt/gitlab-review.yml.tmpl`

**1. Добавить секцию `cache`** — кеш по MR IID, сохраняет `.claude-cache/`:

```yaml
  cache:
    key: "claude-review-${CI_MERGE_REQUEST_IID}"
    paths:
      - .claude-cache/
```

**2. Перед запуском claude** — восстановить сессию из кеша в `~/.claude/projects/` с учётом project hash:

```bash
- |
  # Restore cached session into current project hash directory
  CACHED_SESSION=$(find .claude-cache/projects -name '*.jsonl' -print -quit 2>/dev/null)
  if [ -n "$CACHED_SESSION" ]; then
    CACHED_PROJECT_DIR=$(dirname "$CACHED_SESSION")
    CURRENT_HASH=$(echo "$PWD" | sed 's|/|-|g; s|^-||')
    TARGET_DIR="/root/.claude/projects/$CURRENT_HASH"
    mkdir -p "$TARGET_DIR"
    cp -r "$CACHED_PROJECT_DIR"/* "$TARGET_DIR/"
    echo "Restored cached Claude session into $TARGET_DIR"
  fi
```

**3. Определить флаг `--continue`** — только если есть сохранённая сессия:

```bash
- |
  CLAUDE_RESUME=""
  if find /root/.claude/projects -name '*.jsonl' -print -quit 2>/dev/null | grep -q .; then
    CLAUDE_RESUME="--continue"
    echo "Resuming previous session"
  fi
```

**4. Запуск claude** — добавить `$CLAUDE_RESUME`:

```bash
- |
  claude $CLAUDE_RESUME \
    --model opus \
    --permission-mode acceptEdits \
    --allowedTools "Bash(*) Read(*) Edit(*) Write(*) WebFetch(*)" \
    -p "$PROMPT"
```

**5. После запуска** — сохранить сессию текущего project hash в кеш:

```bash
- |
  # Save current session for next run
  CURRENT_HASH=$(echo "$PWD" | sed 's|/|-|g; s|^-||')
  SOURCE_DIR="/root/.claude/projects/$CURRENT_HASH"
  if [ -d "$SOURCE_DIR" ]; then
    mkdir -p ".claude-cache/projects/$CURRENT_HASH"
    cp -r "$SOURCE_DIR"/* ".claude-cache/projects/$CURRENT_HASH/"
    echo "Saved Claude session from $SOURCE_DIR"
  fi
```

## Файлы

| Файл | Что меняется |
|------|-------------|
| `pkg/vt/gitlab-review.yml.tmpl` | Добавление cache, restore/save сессий, условный `--continue` |

## Итоговая структура job

```yaml
$[[ inputs.job_name ]]:
  image: ...
  stage: ...
  tags: ...
  variables:
    GIT_DEPTH: 100
  cache:
    key: "claude-review-${CI_MERGE_REQUEST_IID}"
    paths:
      - .claude-cache/
  script:
    - # ... env validation (без изменений)
    - # ... git fetch (без изменений)
    - # ... prompt fetch (без изменений)
    - # restore cached session (with project hash remapping)
    - # detect --continue flag
    - # claude $CLAUDE_RESUME ... -p "$PROMPT"
    - # ... upload & html (без изменений)
    - # save current session to cache
  artifacts: ...
  rules: ...
```

## Проверка

1. Первый запуск MR: `CLAUDE_RESUME=""`, обычный запуск, сессия сохраняется в `.claude-cache/`
2. Повторный запуск MR: `.claude-cache/` восстановлен из CI cache, сессия скопирована из старого hash в текущий, `CLAUDE_RESUME="--continue"`, Claude подхватывает предыдущую сессию
3. Новый MR (другой IID): другой ключ кеша → чистый старт
4. Смена runner-а (другой `$PWD`): сессия найдена в кеше по старому hash, скопирована в директорию с новым hash → `--continue` работает
