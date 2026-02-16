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

{{- range .Types}}
{{- if .Text}}
{{.Num}}. файл R{{.Num}}.<TASK>.ru.md как {{.Text}}
{{- end}}
{{- end}}

{{- if .FetchPrompt}}

## Как получить текст задачи?
{{.FetchPrompt}}
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
      "reviewType": "architecture | code | security | tests",
      "summary": "Краткий вывод по этому типу ревью"
    }
  ],
  "issues": [
    {
      "severity": "critical | high | medium | low",
      "title": "Заголовок замечания",
      "description": "Описание проблемы (1-2 предложения, без кода)",
      "content": "полный текст из MD файла по этому замечанию с форматированием",
      "file": "path/to/file.go",
      "lines": "121-156",
      "issueType": "error-handling",
      "fileType": "architecture | code | security | tests"
    }
  ]
}
` + "```" + `

Важно:
- все ключи JSON в lowerCamelCase
- review.createdAt, review.durationMs, review.modelInfo (model, inputTokens, outputTokens, costUsd) — заполни из данных текущей сессии Claude Code
- review.externalId, review.commitHash — из контекста git и VCS
- issues в JSON должны точно соответствовать замечаниям в MD-файлах
- trafficLight и issuesStats — НЕ заполняй, рассчитываются на сервере
- description — краткий, информативный, на русском
- поставь оценочные значения в modelInfo из claude code сессии, так как точных значений у тебя нет
`
