package reviewer

import (
	"text/template"
)

var promptTemplate = template.Must(template.New("prompt").Parse(promptTmpl))

const promptTmpl = `# Шаг 1 — сделай ревью в виде MD файлов
Сделай ревью %SOURCE_BRANCH% с актуальной веткой %TARGET_BRANCH%.

Финальный артефакт — заполненный ` + "`review.json`" + `. MD-файлы — основа, по которой ты заполнишь JSON в Шаге 2. Оба артефакта обязательны.

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

⚠️ Все файлы ` + "`R*.md`" + ` клади в КОРЕНЬ текущей рабочей директории (cwd), плоско. НЕ создавай поддиректории вроде ` + "`docs/`" + `, ` + "`docs/reviews/`" + `, ` + "`reviews/`" + ` — reviewctl ищет MD-файлы только в cwd.

⚠️ Это review, а НЕ разработка. Единственные файлы, которые ты создаёшь/редактируешь — это ` + "`R*.md`" + ` (Шаг 1) и ` + "`review.json`" + ` (Шаг 2). НЕ пиши тесты, НЕ правь исходники, НЕ создавай вспомогательные файлы (helpers, README, скрипты, конфиги). Анализ кода — только через **чтение** (Read tool) и shell-команды (git/grep/ls/cat). Любая правка кода = выход за пределы задачи.

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

Когда все запрошенные выше MD-файлы готовы — выведи в чат строку: ` + "`✓ Шаг 1: MD готовы`" + `. Затем переходи к Шагу 2.
`

// promptStep2Body is the universal Step 2 instruction set: how to fill
// review.json from MD files, schema rules, severity scale, isAccepted logic,
// JSON-safety guidance, and the self-check. Shared by promptReviewJSON (used
// in the main prompt's Step 2 section) and PromptStep2Retry (sent when the
// runner skipped Step 2 in the first pass) so the rules can't drift apart.
const promptStep2Body = `Ты в Шаге 1 написал MD-файлы (по одному на каждый запрошенный reviewType). Прочитай каждый и для каждого заголовка ` + "`### LocalID. Заголовок замечания`" + ` добавь объект в ` + "`issues[]`" + `. Это extraction-шаг, не новый анализ: ` + "`issues[]`" + ` должны соответствовать заголовкам в MD-файлах 1:1.

reviewctl уже положил на диск ` + "`review.json`" + ` со скелетом: блок ` + "`review`" + ` с подставленными CI-данными, ` + "`files[]`" + ` из 5 объектов с правильными ` + "`reviewType`" + `, и пустой ` + "`issues[]`" + `. Твоя задача — ОТКРЫТЬ этот файл, ЗАПОЛНИТЬ summary в каждом из 5 элементов files[], ДОБАВИТЬ найденные замечания в issues[], СОХРАНИТЬ. НЕ создавай файл заново.

⚠️ Сохраняй структуру корня без изменений. Корневые ключи: ` + "`review`" + `, ` + "`files`" + `, ` + "`issues`" + `. НЕ добавляй другие (` + "`branch`" + `, ` + "`baseBranch`" + `, ` + "`tasks`" + `, ` + "`summary`" + `, ` + "`reviewer`" + `).
⚠️ НЕ перемещай ` + "`issues`" + ` внутрь ` + "`files[]`" + `. issues[] — плоский top-level массив.
⚠️ НЕ меняй ` + "`reviewType`" + ` в существующих ` + "`files[]`" + ` элементах: они стоят в каноническом порядке (architecture, code, security, tests, operability).
⚠️ Если в Шаге 1 ты НЕ писал MD-файл по какому-то ` + "`reviewType`" + ` (его не было в списке запрошенных) — УДАЛИ соответствующий объект из ` + "`files[]`" + `. Skeleton по умолчанию содержит все 5 reviewTypes; оставь только те, по которым реально написан MD.

⚠️ ИЗБЕГАЙ символов ` + "`\"`" + ` и ` + "`\\`" + ` внутри простых строковых полей (` + "`title`" + `, ` + "`description`" + `, ` + "`summary`" + `). Переформулируй естественным языком: вместо ` + "`cr.SessionID == \"\"`" + ` пиши «проверка ` + "`cr.SessionID`" + ` на пустую строку»; вместо ` + "`if err != nil`" + ` — «проверка ` + "`err`" + ` на nil». Это убирает риск переэкранирования: типичная поломка — модель пишет ` + "`\\\\\\\"`" + ` (двойной escape) вместо ` + "`\\\"`" + ` (одинарный), парсер ловит «invalid character ... after object key:value pair», JSON ломается. Кавычки и слеши оставляй ТОЛЬКО в ` + "`content`" + `/` + "`suggestedFix`" + ` (markdown с кодом) — там экранируй аккуратно: ` + "`\"`" + ` → ` + "`\\\"`" + `, ` + "`\\`" + ` → ` + "`\\\\`" + `, без удвоения.

⚠️ STRICT SCHEMA — точные значения, без вариаций:

| Поле                | Допустимые значения                                          |
|---------------------|--------------------------------------------------------------|
| issues[].severity   | critical, high, medium, low                                  |
| issues[].fileType   | architecture, code, security, tests, operability             |
| files[].reviewType  | architecture, code, security, tests, operability             |

ЗАПРЕЩЕНО: severity = major | minor | trivial | blocker | info | warning (это другие шкалы — не используй).

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

### Что заполнить в review.json

В ` + "`review`" + ` (метаданные):
- ` + "`description`" + ` — краткий вывод по всему ревью (1-2 предложения, на русском)
- ` + "`effortMinutes`" + ` — оценка времени для ручного ревью опытным разработчиком: 5 = тривиальный, 15 = средний, 30 = большой, 60+ = требует обсуждения архитектуры
- ` + "`aiSlopScore`" + ` — 0.0..1.0, вероятность AI-генерации без human review. Признаки: избыточные docstrings, защита от невозможных сценариев, шаблонные паттерны, лишние абстракции, несоответствие стилю, большой diff с поверхностными изменениями. 0.0 = точно человек, 0.3 = подозрительно, 0.7 = высокая вероятность AI, 1.0 = AI-slop. Для обычного human-written MR ставь 0.0–0.1
- ` + "`modelInfo`" + `, ` + "`durationMs`" + `, ` + "`createdAt`" + ` — оставь как в скелете (нули, ` + `"1970-01-01T00:00:00Z"` + `): сервер перепишет из метрик ран'а
- ` + "`externalId`" + `, ` + "`title`" + `, ` + "`sourceBranch`" + `, ` + "`targetBranch`" + `, ` + "`author`" + `, ` + "`commitHash`" + ` — если стоит конкретное значение (CI его подставил), НЕ трогай. Если стоит плейсхолдер — резолви:
  - ` + "`%TITLE%`" + ` → subject последнего коммита (` + "`git log -1 --pretty=%s`" + `)
  - ` + "`%COMMIT_HASH%`" + ` → ` + "`git rev-parse HEAD`" + `
  - ` + "`%SOURCE_BRANCH%`" + ` → ` + "`git rev-parse --abbrev-ref HEAD`" + `
  - ` + "`%TARGET_BRANCH%`" + ` → ` + "`git symbolic-ref --short refs/remotes/origin/HEAD`" + ` (отрежь префикс ` + "`origin/`" + `), fallback ` + "`master`" + `
  - ` + "`%AUTHOR%`" + ` → ` + "`git log -1 --pretty=%an HEAD`" + `
  - ` + "`%EXTERNAL_ID%`" + ` → ВСЕГДА замени на пустую строку ` + "`\"\"`" + ` (плейсхолдер = нет CI/MR; НЕ оставляй ` + "`\"%EXTERNAL_ID%\"`" + ` в значении)

В каждом ` + "`files[i]`" + ` (по одному на каждый MD-файл из Шага 1):
- ` + "`summary`" + ` — краткий вывод по этому типу ревью
- ` + "`isAccepted`" + ` — флаг «можно ли мержить без блокирующих замечаний по этому reviewType»:
  - ` + "`true`" + ` — если в ` + "`issues[]`" + ` НЕТ ни одного объекта с ` + "`fileType`" + ` равным этому ` + "`reviewType`" + ` И ` + "`severity`" + ` ∈ {` + "`critical`" + `, ` + "`high`" + `}. Замечания со severity ` + "`medium`" + ` или ` + "`low`" + ` НЕ делают ` + "`isAccepted = false`" + `. Чистый reviewType (без issues) — тоже ` + "`true`" + `.
  - ` + "`false`" + ` — ТОЛЬКО если есть хотя бы одно critical/high по этому reviewType.
  - Типичная ошибка: «есть несколько medium → ставлю false». Это неверно: medium/low НЕ блокируют мерж.

В ` + "`issues[]`" + ` (изначально пустой) — добавь по одному объекту на каждое открытое замечание:

` + "```json" + `
{
  "localId": "C1",
  "severity": "high",
  "title": "Заголовок замечания",
  "description": "Описание проблемы (1-2 предложения, без кода)",
  "content": "полный текст секции под заголовком ### {localId}. из соответствующего MD-файла, verbatim с форматированием",
  "file": "path/to/file.go",
  "lines": "121-156",
  "issueType": "error-handling",
  "fileType": "code",
  "suggestedFix": "` + "```go" + `\nresult, err := doSomething()\nif err != nil {\n    return fmt.Errorf(\"handler: %w\", err)\n}\n` + "```" + `"
}
` + "```" + `

Важно:
- все ключи JSON в lowerCamelCase
- ` + "`localId`" + ` — уникальный идентификатор (A1, C2, S1, T3, O1), должен совпадать с заголовком в MD файле; префикс A/C/S/T/O берётся из ` + "`fileType`" + `
- issues в JSON должны точно соответствовать замечаниям в MD-файлах; включай только открытые, не исправленные
- ` + "`issues[].suggestedFix`" + ` — конкретный код исправления, markdown code block с языком; если без большего контекста предложить нельзя — пустая строка
- ` + "`trafficLight`" + ` и ` + "`issuesStats`" + ` — НЕ заполняй, рассчитываются на сервере

## Перед сохранением review.json — обязательная самопроверка

1. Корневые ключи остались: ` + "`review`" + `, ` + "`files`" + `, ` + "`issues`" + `. Никаких ` + "`branch`" + `/` + "`baseBranch`" + `/` + "`tasks`" + `/` + "`summary`" + `/` + "`reviewer`" + ` на корне.
2. ` + "`files`" + ` — по одному элементу на каждый MD-файл из Шага 1, в каноническом порядке (architecture, code, security, tests, operability — но без пропущенных). У каждого только три поля: ` + "`reviewType`" + `, ` + "`summary`" + `, ` + "`isAccepted`" + ` (никаких ` + "`path`" + `/` + "`reviewer`" + `/` + "`issues`" + ` внутри).
3. ` + "`issues`" + ` — на верхнем уровне, плоский массив. НЕ внутри ` + "`files[]`" + `.
4. Каждый ` + "`issues[*].severity`" + ` ∈ {critical, high, medium, low}. Если встретилось major/minor/trivial/blocker/info/warning — замени (major→high, minor→low, trivial→low, blocker→critical) и перепиши.
5. Каждый ` + "`issues[*].fileType`" + ` ∈ {architecture, code, security, tests, operability}.
6. Поле НЕ называется ` + "`category`" + ` — оно называется ` + "`issueType`" + `.
7. После Write/Edit на ` + "`review.json`" + ` — прочитай файл обратно через **Read tool** и убедись, что JSON валиден. Если парсер выдаёт ошибку (типично «invalid character ... after object key:value pair») — найди позицию по line/column, посмотри какое строковое поле сломано (обычно ` + "`description`" + `/` + "`title`" + ` с лишним escape вроде ` + "`\\\\\\\"`" + ` вместо ` + "`\\\"`" + `), исправь, перезапиши, перечитай. НЕ заканчивай работу с broken JSON.
8. ` + "`review.externalId`" + ` ≠ ` + "`\"%EXTERNAL_ID%\"`" + ` — если плейсхолдер остался в значении, замени на пустую строку ` + "`\"\"`" + `.
9. Каждый ` + "`files[i].isAccepted`" + ` логически согласован с ` + "`issues[]`" + `: если в issues есть critical/high с ` + "`fileType == files[i].reviewType`" + ` → ` + "`isAccepted = false`" + `; иначе → ` + "`isAccepted = true`" + `.

Если хоть один пункт не соблюдён — перезапиши ` + "`review.json`" + ` и проверь снова.

Готовый review должен иметь все запрошенные MD-файлы из Шага 1 и заполненный review.json (` + "`files[].summary`" + ` и ` + "`issues[]`" + ` соответствуют заголовкам в MD). Не заканчивай работу, пока оба условия не выполнены.

После сохранения review.json и успешной самопроверки — выведи в чат: ` + "`✓ Шаг 2: review.json заполнен`" + `.
`

const promptReviewJSON = `
---

# Шаг 2 — заполни review.json извлечением из MD-файлов

` + promptStep2Body

// PromptStep2Retry is the focused prompt sent on a Step 2 retry — when the
// initial runner pass produced MD files but never filled review.json. Caller
// resumes the previous session so the original prompt is cached; the retry
// header tells the model what failed, then re-injects the same Step 2 body so
// the rules can't drift between the original and retry paths.
const PromptStep2Retry = `# Шаг 2 был пропущен — выполни его сейчас

В предыдущем шаге ты остановился после написания MD-файлов, но review.json остался скелетом. Сейчас ВЫПОЛНИ ТОЛЬКО Шаг 2 — те же инструкции, что были в исходном промпте:

` + promptStep2Body
