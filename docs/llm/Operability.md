# Operability: 5-й необязательный тип ревью

## Проблема

Нет анализа операционной готовности кода: логирование, метрики, трейсинг, алерты,
feature flags, план отката, миграции БД, graceful degradation.

## Решение

Добавить необязательный тип ревью **operability** (O). Шаблон уже поддерживает
необязательность — `{{- if .Text}}` в промпте пропускает тип с пустым текстом.
Если поле `prompt.Operability` пустое — тип не попадёт в промпт и файл R5 не создастся.

### Эксперты для шаблона (из docs/llm/code-reviewers.md)

| Язык | Эксперт | Описание |
|------|---------|----------|
| Go | Peter Bourgon | Автор Go kit, евангелист observability (логи, метрики, трейсинг) |
| Swift/iOS | Felix Krause | Создатель Fastlane, CI/CD, feature flags, релизные пайплайны |
| Kotlin/Android | Chet Haase | Google Android team, перформанс, профилирование, production-диагностика |
| Vue+Nuxt | Sébastien Chopin | Создатель Nuxt, деплой, edge rendering, graceful degradation SSR |
| Python | Hynek Schlawack | Автор structlog, attrs, structured logging, observability |
| TS/React | Guillermo Rauch | CEO Vercel, деплой, edge runtime, feature flags, observability |

### Пример текста для prompt.Operability (Go)

```
Peter Bourgon. Проведи ревью операционной готовности этого MR:
логирование, метрики, трейсинг, алерты, feature flags, graceful degradation,
миграции БД, план отката.
```

---

## 1. SQL-миграция

```sql
ALTER TABLE "prompts" ADD COLUMN "operability" text NOT NULL DEFAULT '';
```

## 2. docs/reviewsrv.sql

Добавить поле `"operability" text NOT NULL` в таблицу `prompts` (после `tests`).

## 3. MFD XML: docs/model/project.xml

Добавить атрибут в Entity `Prompt`:
```xml
<Attribute Name="Operability" DBName="operability" DBType="text" GoType="string" PK="false" Nullable="No" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
```

Добавить поиск:
```xml
<Search Name="OperabilityILike" AttrName="Operability" SearchType="SEARCHTYPE_ILIKE"></Search>
```

## 4. MFD-генерация

```bash
make mfd-model              # pkg/db/model.go — добавит Operability в Prompt
make mfd-repo NS=project    # pkg/db/project.go
make mfd-db-test            # pkg/db/test/project.go
```

## 5. Go: константы (pkg/reviewer/model.go)

Добавить константу и расширить массив:

```go
ReviewTypeOperability = "operability"
```

В `ReviewTypes` добавить `ReviewTypeOperability`.

Массив `localIdPrefixes` (если есть) — добавить `"O"` для operability.

## 6. Go: промпт (pkg/reviewer/prompt.go)

### promptTmpl (строка 30)
Обновить описание префиксов:
```
Префикс: A — architecture, C — code, S — security, T — tests, O — operability. Нумерация с 1 для каждого типа.
```

### promptReviewJSON (строки 105, 120)
Обновить документацию:
```json
"reviewType": "architecture | code | security | tests | operability"
"fileType": "architecture | code | security | tests | operability"
```

## 7. Go: сборка промпта (pkg/reviewer/project.go, строки 85-90)

Добавить 5-й тип:
```go
Types: []promptType{
    {1, prompt.Architecture},
    {2, prompt.Code},
    {3, prompt.Security},
    {4, prompt.Tests},
    {5, prompt.Operability},
},
```

Если `prompt.Operability == ""` — шаблон пропустит его автоматически (`{{- if .Text}}`).

## 8. Go: VT (pkg/vt/)

Генерация `make mfd-vt-rpc NS=project` обновит модели Prompt в VT автоматически.
Ручных изменений не требуется.

## 9. Go: RPC + zenrpc (pkg/rpc/)

Обновить комментарий в `pkg/rpc/model.go`:
```go
// ReviewFile — таб Architecture/Code/Security/Tests/Operability.
```

Перегенерировать:
```bash
make generate
make type-script-client
```

## 10. Frontend: useFormat.ts

Добавить маппинг:

```typescript
const reviewTypeLabels: Record<string, string> = {
  architecture: 'A',
  code: 'C',
  security: 'S',
  tests: 'T',
  operability: 'O',    // новое
}

const reviewTypeFullNames: Record<string, string> = {
  architecture: 'Architecture',
  code: 'Code',
  security: 'Security',
  tests: 'Tests',
  operability: 'Operability',  // новое
}
```

## 11. Frontend: ReviewPage.vue

### typeOrder (строка 275)
```typescript
const typeOrder = ['architecture', 'code', 'security', 'tests', 'operability']
```

### Фильтр reviewType (строки 176-182)
Добавить option:
```html
<option value="operability">Operability</option>
```

## 12. upload.cjs (строка 8)

```javascript
const TYPES = { R1: "architecture", R2: "code", R3: "security", R4: "tests", R5: "operability" };
```

---

## Порядок выполнения

1. SQL-миграция (добавить колонку operability)
2. Обновить `docs/reviewsrv.sql`
3. Обновить `docs/model/project.xml` (атрибут + поиск)
4. `make mfd-model` + `make mfd-repo NS=project` + `make mfd-db-test`
5. `make mfd-vt-rpc NS=project`
6. Go: константа в `pkg/reviewer/model.go`
7. Go: промпт (`pkg/reviewer/prompt.go`) + сборка (`pkg/reviewer/project.go`)
8. `make generate` + `make type-script-client`
9. Frontend: `useFormat.ts`, `ReviewPage.vue`
10. `upload.cjs`
