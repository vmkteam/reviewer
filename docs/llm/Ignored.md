# Ignored: третья кнопка фидбэка для issues

## Проблема

Бинарный выбор Valid/FP не покрывает случай, когда замечание AI формально верное, но сознательно
игнорируется (например: тест не нужен, nil не обрабатывается намеренно). Нужна третья кнопка **Ignored**.

## Решение

Расширить таблицу `statuses` новыми значениями и перенести resolution из `isFalsePositive` в `statusId`.

### Значения statusId для issues

| statusId | Alias          | Описание                                         |
|----------|----------------|--------------------------------------------------|
| 1        | enabled        | Не обработано (текущий дефолт)                   |
| 4        | valid          | Подтверждено, нужно исправить (зелёная кнопка)   |
| 5        | falsePositive  | AI ошибся (красная кнопка)                       |
| 6        | ignored        | Верное замечание, но сознательно игнорируется (жёлтая кнопка) |

Поле `processedAt` остаётся без изменений.

---

## 1. SQL-миграция (docs/patches/2026-02-27-fp.sql)

```sql
-- new statuses for issue resolution
INSERT INTO "statuses" ("statusId", "title", "alias") VALUES
    (4, 'Valid', 'valid'),
    (5, 'FalsePositive', 'falsePositive'),
    (6, 'Ignored', 'ignored');

-- migrate data from isFalsePositive to statusId
UPDATE "issues" SET "statusId" = 4 WHERE "isFalsePositive" = false;
UPDATE "issues" SET "statusId" = 5 WHERE "isFalsePositive" = true;

-- drop old column
DROP INDEX IF EXISTS "IX_issues_isFalsePositive";
ALTER TABLE "issues" DROP COLUMN "isFalsePositive";
```

## 2. docs/reviewsrv.sql

- Убрать `"isFalsePositive" bool,` (строка 126)
- Убрать индекс `IX_issues_isFalsePositive` (строки 134-136)
- Добавить записи в `statuses` (4, 5, 6)

## 3. MFD XML: docs/model/review.xml

Удалить атрибут `IsFalsePositive` из Entity `Issue` (строка 17):
```xml
<!-- УДАЛИТЬ -->
<Attribute Name="IsFalsePositive" DBName="isFalsePositive" DBType="bool" GoType="*bool" ...></Attribute>
```

Добавить поиск по `StatusID` в `<Searches>`:
```xml
<Search Name="StatusIDs" AttrName="StatusID" SearchType="SEARCHTYPE_ARRAY"></Search>
```

## 4. MFD-генерация

```bash
make mfd-model              # pkg/db/model.go, model_search.go, model_validate.go
make mfd-repo NS=review     # pkg/db/review.go
make mfd-db-test            # pkg/db/test/review.go
```

Результат:
- `pkg/db/model.go` — из структуры `Issue` исчезнет `IsFalsePositive`, из `Columns.Issue` тоже
- `pkg/db/model_search.go` — из `IssueSearch` исчезнет `IsFalsePositive`, появится `StatusIDs`

## 5. Go: константы и фильтр (pkg/db/options.go)

Добавить после `StatusDeleted`:

```go
StatusValid         = 4
StatusFalsePositive = 5
StatusIgnored       = 6
```

Добавить фильтр для issues (включает все допустимые statusId для issues):

```go
IssueStatusFilter = Filter{
    Field: "statusId",
    Value: []int{StatusEnabled, StatusValid, StatusFalsePositive, StatusIgnored},
    SearchType: SearchTypeArray,
}
```

## 5a. Go: поправить StatusFilter в pkg/db/review.go (после mfd-repo)

`make mfd-repo NS=review` перегенерирует `pkg/db/review.go` и сбросит фильтры на дефолтные.
После генерации заменить `StatusFilter` на `IssueStatusFilter` для issues в `NewReviewRepo` (строки 22-26):

```go
filters: map[string][]Filter{
    Tables.Issue.Name:      {IssueStatusFilter},  // было StatusFilter
    Tables.ReviewFile.Name: {StatusFilter},
    Tables.Review.Name:     {StatusFilter},
},
```

Без этого issues со statusId 4, 5, 6 будут отфильтрованы базовым `StatusFilter` (который пропускает только 1 и 2).

## 6. Go: domain-слой

### pkg/reviewer/model.go — `IssueSearch` (строки 217-248)
- Удалить `IsFalsePositive *bool` (строка 221)
- Добавить `StatusIDs []int`
- В `ToDB()` (строка 229): убрать маппинг `IsFalsePositive`, добавить маппинг `StatusIDs`

### pkg/reviewer/manager.go — `SetFeedback` (строки 308-316)

Заменить на:

```go
// SetFeedback updates the statusId on an issue and sets processedAt.
func (rm *ReviewManager) SetFeedback(ctx context.Context, issueID int, statusID int) (bool, error) {
    if !isValidIssueResolution(statusID) {
        return false, fmt.Errorf("invalid statusId: %d", statusID)
    }

    issue := &db.Issue{ID: issueID, StatusID: statusID}
    if statusID != db.StatusEnabled {
        now := time.Now()
        issue.ProcessedAt = &now
    } else {
        issue.ProcessedAt = nil
    }
    return rm.repo.UpdateIssue(ctx, issue, db.WithColumns(
        db.Columns.Issue.StatusID, db.Columns.Issue.ProcessedAt,
    ))
}

// isValidIssueResolution checks that statusID is a valid issue resolution value.
func isValidIssueResolution(statusID int) bool {
    switch statusID {
    case db.StatusEnabled, db.StatusValid, db.StatusFalsePositive, db.StatusIgnored:
        return true
    }
    return false
}
```

## 7. Go: RPC (pkg/rpc/)

### model.go — `Issue` (строки 206-221)
- Заменить `IsFalsePositive *bool` (строка 219) на `StatusID int`
- В `newIssue` (строка 223): заменить `IsFalsePositive: in.IsFalsePositive` на `StatusID: in.StatusID`

### model.go — `IssueFilters` (строки 275-280)
- Заменить `IsFalsePositive *bool` на `StatusIDs []int`
- В `ToDomain` (строка 283) и `ToDomainByProject` (строка 297): заменить маппинг

### review.go — `Feedback` (строки 246-257)

```go
// Feedback updates resolution status for an issue.
//
//zenrpc:issueId Issue ID
//zenrpc:statusId Resolution (1 = unprocessed, 4 = valid, 5 = false positive, 6 = ignored)
//zenrpc:return bool
//zenrpc:404 Not Found
//zenrpc:500 Internal Error
func (s ReviewService) Feedback(ctx context.Context, issueId int, statusId int) (bool, error) {
    if err := s.checkIssue(ctx, issueId); err != nil {
        return false, err
    }

    ok, err := s.rm.SetFeedback(ctx, issueId, statusId)
    if err != nil {
        return false, newInternalError(err)
    }

    return ok, nil
}
```

Перегенерировать:
```bash
make generate           # zenrpc + colgen
make type-script-client # frontend TS-клиенты
```

Добавить `// @ts-nocheck` на 3-ю строку сгенерированных `.ts` файлов.

## 8. Frontend: константы статусов

Создать файл `frontend/src/constants/status.ts`:

```typescript
export const StatusEnabled = 1
export const StatusValid = 4
export const StatusFalsePositive = 5
export const StatusIgnored = 6
```

Использовать эти константы во всех компонентах вместо магических чисел.

## 9. Frontend: FeedbackButtons.vue

Три кнопки вместо двух. Props: `statusId: number`. Emit: `feedback(statusId)`.

- **Valid** (зелёная) — `StatusValid`, повторный клик → `StatusEnabled`
- **FP** (красная) — `StatusFalsePositive`, повторный клик → `StatusEnabled`
- **Ignored** (жёлтая) — `StatusIgnored`, повторный клик → `StatusEnabled`

## 10. Frontend: IssuesTable.vue

- Props FeedbackButtons: заменить `:is-false-positive` на `:status-id` (строки 62, 139)
- Emit `feedback`: заменить `[issue: Issue, value: boolean | null]` на `[issue: Issue, statusId: number]` (строка 193)

## 11. Frontend: MarkdownContent.vue

`IssueBadgeInfo` (строка 14): заменить `isFalsePositive` на `statusId`.

`buildBadgeHtml` (строки 130-140): использовать константы:
- `statusId === StatusFalsePositive` → бейдж FP (красный)
- `statusId === StatusValid` → бейдж Valid (зелёный)
- `statusId === StatusIgnored` → бейдж Ignored (жёлтый)

## 12. Frontend: ReviewPage.vue

`setFeedback` (строки 421-428):
```typescript
async function setFeedback(issue: Issue, statusId: number) {
    await api.review.feedback({ issueId: issue.issueId, statusId })
    issue.statusId = statusId
}
```

`issuesForReviewFile` (строки 430-439): заменить `isFalsePositive` на `statusId`.

## 13. Frontend: ReviewsPage.vue

`risksFilters` (строка 167): заменить `{ isFalsePositive: true }` на `{ statusIds: [StatusFalsePositive] }`.

`setRiskFeedback` (строки 282-293): обновить аналогично `setFeedback`. Удалять из risks если `statusId !== StatusFalsePositive`.

---

## Порядок выполнения

1. SQL-миграция `docs/patches/2026-02-27-fp.sql`
2. Обновить `docs/reviewsrv.sql`
3. Обновить `docs/model/review.xml` (удалить атрибут, добавить поиск)
4. Go: константы и `IssueStatusFilter` в `pkg/db/options.go`
5. `make mfd-model` + `make mfd-repo NS=review` + `make mfd-db-test`
6. Поправить `StatusFilter` → `IssueStatusFilter` для issues в `pkg/db/review.go`
7. Go: domain-слой (`pkg/reviewer/model.go`, `pkg/reviewer/manager.go`)
8. Go: RPC (`pkg/rpc/model.go`, `pkg/rpc/review.go`)
9. `make generate` + `make type-script-client`
10. Frontend: константы `frontend/src/constants/status.ts`
11. Frontend: компоненты + страницы
12. Тесты
