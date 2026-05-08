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

Если изменения затрагивают взаимодействие нескольких компонентов — в конце архитектурного обзора (R1) добавь Mermaid sequence diagram, показывающий ключевые взаимодействия в изменённом коде. Формат: ` + "```mermaid" + ` ... ` + "```" + `. Диаграмма должна отражать только изменённый flow, не всю архитектуру. Если изменения тривиальные (рефакторинг, переименование) — диаграмму не добавляй.

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

review.json — структурированные данные по всем замечаниям.

⚠️ STRICT SCHEMA — точные значения, без вариаций:

| Поле                | Допустимые значения                                          |
|---------------------|--------------------------------------------------------------|
| issues[].severity   | critical, high, medium, low                                  |
| issues[].fileType   | architecture, code, security, tests, operability             |
| files[].reviewType  | architecture, code, security, tests, operability             |

ЗАПРЕЩЕНО: severity = major | minor | trivial | blocker | info | warning (это другие шкалы — не используй).
ЗАПРЕЩЕНО: класть в files[] список изменённых файлов diff с полями path/kind/lines. files[] — это РОВНО 5 объектов, по одному на каждый тип ревью.

### Правила оценки severity

- **critical** — баг, который сломает production: потеря данных, crash, race condition с потерей состояния, нарушение бизнес-логики с финансовым импактом
- **high** — серьёзный баг или проблема: некорректное поведение при edge cases, утечка ресурсов, проблемы безопасности, искажение метрик
- **medium** — проблема качества: отсутствие обработки ошибок, игнорирование ошибок API, хрупкая логика, отсутствие валидации на границах системы
- **low** — стиль, нейминг, мелкие улучшения, потенциальные проблемы без реального импакта

### Категории замечаний (issueType)

Используй одну из категорий для каждого замечания (поле issues[].issueType):
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
- если не хватает — можешь добавить в таком же стиле

### Формат review.json

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
    "createdAt": "1970-01-01T00:00:00Z",
    "durationMs": 0,
    "effortMinutes": 15,
    "aiSlopScore": 0.0,
    "modelInfo": {"model": "", "inputTokens": 0, "outputTokens": 0, "costUsd": 0}
  },
  "files": [
    {"reviewType": "architecture", "summary": "Краткий вывод по архитектуре",  "isAccepted": true},
    {"reviewType": "code",         "summary": "Краткий вывод по коду",         "isAccepted": true},
    {"reviewType": "security",     "summary": "Краткий вывод по безопасности", "isAccepted": true},
    {"reviewType": "tests",        "summary": "Краткий вывод по тестам",       "isAccepted": true},
    {"reviewType": "operability",  "summary": "Краткий вывод по operability",  "isAccepted": false}
  ],
  "issues": [
    {
      "localId": "C1",
      "severity": "high",
      "title": "Заголовок замечания",
      "description": "Описание проблемы (1-2 предложения, без кода)",
      "content": "полный текст из MD файла по этому замечанию с форматированием",
      "file": "path/to/file.go",
      "lines": "121-156",
      "issueType": "error-handling",
      "fileType": "code",
      "suggestedFix": "` + "```go" + `\nresult, err := doSomething()\nif err != nil {\n    return fmt.Errorf(\"handler: %w\", err)\n}\n` + "```" + `"
    }
  ]
}
` + "```" + `

Важно:
- все ключи JSON в lowerCamelCase
- review.modelInfo, review.durationMs, review.createdAt — оставь заглушки как в примере (нули и "1970-01-01T00:00:00Z"), сервер заполнит из метрик ран'а
- review.externalId, review.title, review.sourceBranch, review.targetBranch — если в шаблоне ещё стоят %EXTERNAL_ID%, %TITLE%, %SOURCE_BRANCH%, %TARGET_BRANCH%, замени их сам из git-контекста; если CI уже подставил реальные значения — НЕ перезаписывай
- review.commitHash — из git
- review.description — краткий, информативный, на русском
- review.effortMinutes — оценка времени в минутах для ручного review опытным разработчиком: 5 = тривиальный, 15 = средний, 30 = большой, 60+ = требует обсуждения архитектуры
- review.aiSlopScore — число 0.0..1.0, вероятность AI-сгенерированного кода без human review. Признаки: избыточные docstrings, защита от невозможных сценариев, шаблонные паттерны, лишние абстракции, несоответствие стилю, большой diff с поверхностными изменениями. 0.0 = точно человек, 0.3 = подозрительно, 0.7 = высокая вероятность AI, 1.0 = AI-slop
- localId — уникальный идентификатор замечания (A1, C2, S1, T3, O1), должен совпадать с заголовком в MD файле; префикс A/C/S/T/O берётся из fileType
- issues в JSON должны точно соответствовать замечаниям в MD-файлах
- в issues и сами ревью должны попадать только открытые замечания, не исправленные
- isAccepted — true, если MR допустим с точки зрения данного аспекта (нет critical/high замечаний), false — если есть серьёзные проблемы
- issues[].suggestedFix — конкретный код исправления, markdown code block с языком; если без большего контекста предложить нельзя — пустая строка
- trafficLight и issuesStats — НЕ заполняй, рассчитываются на сервере

## Перед записью review.json — обязательная самопроверка

Прежде чем записать файл, перечитай содержимое и проверь по чек-листу:
1. ` + "`files`" + ` — массив РОВНО из 5 объектов, по одному на каждый reviewType из {architecture, code, security, tests, operability}. Если ты собираешься положить туда список изменённых файлов c полями path/kind — это ОШИБКА, перепиши.
2. Каждый ` + "`issues[*].severity`" + ` ∈ {critical, high, medium, low}. Если встретилось major/minor/trivial/blocker/info/warning — замени на ближайший допустимый уровень (major→high, minor→low, trivial→low, blocker→critical) и перепиши.
3. Каждый ` + "`issues[*].fileType`" + ` ∈ {architecture, code, security, tests, operability}.
4. Каждый ` + "`issues[*].issueType`" + ` — короткая категория из списка выше (или однословная в том же стиле); НЕ путай с fileType.
5. Все ключи JSON в lowerCamelCase. Поле НЕ называется ` + "`category`" + ` — оно называется ` + "`issueType`" + `.

Если хоть один пункт не соблюдён — перезапиши review.json и проверь снова.
`
