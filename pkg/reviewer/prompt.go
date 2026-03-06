package reviewer

import (
	"text/template"
)

var promptTemplate = template.Must(template.New("prompt").Parse(promptTmpl))

const promptTmpl = `# Сначала сделай ревью в виде MD файлов
Сделай ревью %SOURCE_BRANCH% с актуальной веткой %TARGET_BRANCH%.

{{- if .Common}}

{{.Common}}
{{- end}}

{{- if .Instructions}}

## Инструкции по проекту
{{.Instructions}}
{{- end}}

{{- range .Types}}
{{- if .Text}}
{{.Num}}. файл R{{.Num}}.<TASK>.ru.md как {{.Text}}
{{- end}}
{{- end}}

Каждое открытое замечание в MD файле оформляй заголовком с localId: ` + "`### C1. Заголовок замечания`" + `
Префикс: A — architecture, C — code, S — security, T — tests, O — operability. Нумерация с 1 для каждого типа.

Создавай замечание если ты уверен, что это реальная проблема или стоящее улучшение. Если при анализе понял, что проблемы нет — НЕ создавай замечание, даже если начал его описывать.
НЕ создавай замечания вида "это не проблема", "это допустимо", "это нормально".

{{- if .FetchPrompt}}

## Как получить текст задачи?
{{.FetchPrompt}}

В начале КАЖДОГО MD-файла напиши строку: "Задачи: TASK-1, TASK-2, ..." с ID всех проанализированных задач через запятую.
{{- end}}

{{- if .AcceptedRisks}}

## Принятые риски

Следующие замечания были рассмотрены командой и приняты как допустимые. НЕ создавай повторные замечания по этим пунктам:

{{- range .AcceptedRisks}}
- {{.Title}} | {{.File}} | {{.IssueType}}
{{- end}}
{{- end}}
`

const promptReviewJSON = `
---

# После создай файл review.json

review.json — структурированные данные по всем замечаниям

### Правила оценки severity

- **critical** — баг, который сломает production: потеря данных, crash, race condition с потерей состояния, нарушение бизнес-логики с финансовым импактом
- **high** — серьёзный баг или проблема: некорректное поведение при edge cases, утечка ресурсов, проблемы безопасности, искажение метрик
- **medium** — проблема качества: отсутствие обработки ошибок, игнорирование ошибок API, хрупкая логика, отсутствие валидации на границах системы
- **low** — стиль, нейминг, мелкие улучшения, потенциальные проблемы без реального импакта

### Категории замечаний

Используй одну из категорий для каждого замечания:
- nil-check — отсутствие проверки на nil/null/empty
- error-handling — некорректная или отсутствующая обработка ошибок
- tests — проблемы с тестами или их отсутствие
- naming — именование переменных, функций, типов
- duplication — дублирование кода или логики
- security — уязвимости (injection, XSS, утечка секретов, ...)
- perf — проблемы производительности
- architecture — архитектурные проблемы (нарушение SRP, связанность, ...)
- logging — проблемы с логированием или мониторингом
- concurrency — проблемы многопоточности (race conditions, deadlocks, ...)
- если не хватает - можешь добавить в таком же стиле

### Формат review.json

Создай файл review.json со следующей структурой.
Блок ` + "`review`" + ` заполни данными из текущей сессии Claude Code (модель, токены, стоимость, время выполнения).

` + "```json" + `
{
  "review": {
    "externalId": "%EXTERNAL_ID%",
    "title": "%TITLE%",
    "description": "Краткий вывод по всему ревью (1-2 предложения)",
    "commitHash": "{{COMMIT_HASH}}",
    "sourceBranch": "%SOURCE_BRANCH%",
    "targetBranch": "%TARGET_BRANCH%",
    "author": "{{AUTHOR}}",
    "createdAt": "2025-01-01T00:00:00Z",
    "durationMs": 0,
    "modelInfo": {
      "model": "claude-opus-4-6",
      "inputTokens": 0,
      "outputTokens": 0,
      "costUsd": 0.00
    }
  },
  "files": [
    {
      "reviewType": "architecture | code | security | tests | operability",
      "summary": "Краткий вывод по этому типу ревью",
      "isAccepted": true
    }
  ],
  "issues": [
    {
      "localId": "C1",
      "severity": "critical | high | medium | low",
      "title": "Заголовок замечания",
      "description": "Описание проблемы (1-2 предложения, без кода)",
      "content": "полный текст из MD файла по этому замечанию с форматированием",
      "file": "path/to/file.go",
      "lines": "121-156",
      "issueType": "error-handling",
      "fileType": "architecture | code | security | tests | operability"
    }
  ]
}
` + "```" + `

Важно:
- все ключи JSON в lowerCamelCase
- review.durationMs, review.createdAt — замерь точное время:
  1. В самом начале работы выполни ` + "`date +%s%3N`" + ` через Bash и сохрани результат как START_MS
  2. Перед записью review.json выполни ` + "`date +%s%3N`" + ` ещё раз
  3. durationMs = END_MS - START_MS
  4. createdAt = текущее время в ISO 8601 (получи через ` + "`date -u +%Y-%m-%dT%H:%M:%SZ`" + `)
- review.modelInfo — заполни так:
  - model: точное имя модели из твоей сессии (например claude-opus-4-6, claude-sonnet-4-6)
  - inputTokens: примерный текущий context usage в токенах
  - outputTokens: оценочно, обычно 15-30% от inputTokens
  - costUsd: рассчитай по формуле (inputTokens * inputPrice + outputTokens * outputPrice) / 1000000, цены за 1M токенов: opus — $5/$25, sonnet — $3/$15, haiku — $1/$5
- review.externalId, review.commitHash — из контекста git и VCS
- issues в JSON должны точно соответствовать замечаниям в MD-файлах
- localId — уникальный идентификатор замечания (A1, C2, S1, T3, O1), должен совпадать с заголовком в MD файле, A/C/S/T/O определяются по типу ревью fileType
- в issues и сами ревью должны попадать только открытые замечания, не исправленные 
- isAccepted — true, если MR допустим с точки зрения данного аспекта (нет critical/high замечаний), false — если есть серьёзные проблемы
- trafficLight и issuesStats — НЕ заполняй, рассчитываются на сервере
- description — краткий, информативный, на русском
- review.externalId, review.title, review.sourceBranch, review.targetBranch в оригинальном json определены как переменные "EXTERNAL_ID", "TITLE", "SOURCE_BRANCH", "TARGET_BRANCH",
  если в json эти данные уже стоят и они не пустые (и не переменные), значит они были заменены на CI и менять их не нужно.
  Если значения всё ещё содержат %% (шаблонные переменные) — замени их самостоятельно из контекста git:
  externalId = "" (пусто при локальном запуске), title = краткое описание MR из коммитов,
  sourceBranch = текущая ветка, targetBranch = целевая ветка (master/main).
`
