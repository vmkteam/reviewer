# Кеширование сессий Claude Code

## Механизм

reviewctl поддерживает два режима переиспользования сессий:

| Флаг | Claude CLI | Описание |
|------|-----------|----------|
| `--session <id>` | `--resume <id>` | Возобновить конкретную сессию по ID |
| `--continue` | `--continue` | Продолжить последнюю сессию в директории |

### Prompt cache

При `--resume` Claude переиспользует prompt cache предыдущей сессии. Экономия ~90% input tokens (cache read вместо cache creation).

Пример из реального запуска:
- Без cache: `cacheCreation=154964`, `cacheRead=0` → $2.09
- С cache: `cacheCreation=0`, `cacheRead=1391090` → $1.89

### Session ID

Session ID возвращается в `ClaudeResult.SessionID` и сохраняется в `review.modelInfo.sessionId`. Для повторного запуска:

```bash
reviewctl review --session "12308e53-d706-4afa-9ee0-aa86052e927d" ...
```

### CI интеграция

В CI session ID сохраняется между запусками через GitLab CI cache:

```yaml
cache:
  key: "claude-review-${CI_MERGE_REQUEST_IID}"
  paths:
    - .claude-cache/
```

reviewctl с `--continue` автоматически подхватывает последнюю сессию из `~/.claude/projects/`.

## Важно

Флаг `--verbose` НЕ используется при запуске Claude CLI — он заставляет выводить все сообщения (tool calls, file reads) как JSON array в stdout. При большом review это может превысить 448KB и обрезать вывод, потеряв result entry.
