# UI

Папка frontend
Отдается напрямую через echo по адресу /reviews/ если продакшн
Но можно запустить и в дев режиме через node

## Стек
Vue, SPA, Tailwind, JSON-RPC 2.0

https://localhost:8075/v1/rpc/
    review.Projects() []Project
    review.Get(projectId,filters,fromReviewId=null) []ReviewSummary
    review.Count(projectId,filters) int
    review.GetByID(reviewId) Review
    review.Issues(reviewId,filters) []Issue
    review.CountIssues(reviewId,filters)
    review.Feedback(isFalsePositive)

http://localhost:8075/v1/rpc/api.ts - вот тут последняя спецификация для клиента

## URLs

### /reviews/ — Список проектов

Карточки проектов в виде grid.

Каждая карточка:
- title, language
- количество ревью
- последнее ревью: дата, автор, trafficLight (цветной индикатор)

Клик по карточке -> `/reviews/project/<projectId>/`

### /reviews/project/\<projectId\>/ — Ревью проекта

Заголовок: название проекта, language, ссылка на VCS

Таблица с бесконечным скроллом (по 50 элементов):

| Колонка | Данные |
|---------|--------|
| trafficLight | цветной кружок (red/yellow/green) |
| title | заголовок ревью, ниже мелко externalId |
| author | автор |
| branch | sourceBranch -> targetBranch |
| A / C / S / T | 4 мини-кружка trafficLight по каждому reviewType |
| issues | total из issueStats (сумма по всем файлам) |
| createdAt | дата, относительное время |

Фильтры: author, trafficLight, dateRange

Клик по строке -> `/reviews/<reviewId>/`

### /reviews/\<reviewId\>/ — Детали ревью

**Шапка:**
- title, description
- trafficLight (большой индикатор)
- author, sourceBranch -> targetBranch, commitHash (короткий)
- createdAt, durationMs (в секундах)
- modelInfo: model, inputTokens, outputTokens, costUsd

**Табы:** Architecture | Code | Security | Tests | Issues

Табы Architecture / Code / Security / Tests (reviewFile):
- trafficLight, summary
- issueStats: critical / high / medium / low (бейджи с цветами)
- content — отрендеренный markdown

Таб Issues (все issues ревью):

| Колонка | Данные |
|---------|--------|
| severity | бейдж critical/high/medium/low |
| title | заголовок issue |
| file | файл + lines |
| issueType | тип (architecture, concurrency, perf, ...) |
| reviewType | A / C / S / T |
| isFalsePositive | две кнопки: true / false (null = не обработан, true = false positive, false = confirmed) |

Фильтры: severity, issueType, reviewType
Сортировка: severity (по умолчанию), file, issueType

