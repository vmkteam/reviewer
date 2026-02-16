# Slack-уведомления о новых ревью

## Контекст

При создании нового ревью через REST API (`POST /v1/upload/:projectKey/`) нужно отправлять уведомление в Slack-канал проекта.

### Текущая схема данных

В БД уже есть таблица `slackChannels`:

| Поле         | Тип    | Описание                                           |
|--------------|--------|----------------------------------------------------|
| slackChannelId | int  | PK                                                 |
| title        | string | Название канала (для отображения в UI)              |
| channel      | string | Slack channel name (например `#code-reviews`)       |
| webhookURL   | string | Slack Incoming Webhook URL                          |
| statusId     | int    | 1=enabled, 2=deleted                               |

Связь с проектом: `projects.slackChannelId` -> `slackChannels.slackChannelId` (nullable FK).

## Получение Webhook URL

Slack Incoming Webhooks создаются через Slack App:

1. Перейти на https://api.slack.com/apps
2. Создать новое приложение (или выбрать существующее)
3. В разделе **Incoming Webhooks** — включить функцию
4. Нажать **Add New Webhook to Workspace**, выбрать канал
5. Скопировать URL вида `https://hooks.slack.com/services/T.../B.../xxx`
6. Сохранить URL в таблицу `slackChannels` (поле `webhookURL`)

Webhook URL привязывается к каналу через VT-админку (CRUD для `slackChannels` + выбор в настройках проекта).

## Алгоритм отправки уведомлений

### Точка вызова

`rest.Handler.CreateReview` — после успешного `rm.CreateReview()`.

### Последовательность

1. `rest.Handler.CreateReview` уже вызывает `h.projectByKey()` → `pm.GetByKey()`. Добавить `FullProject()` join в существующий `GetByKey`, чтобы подтянуть `SlackChannel`
2. Проверить: `project.SlackChannel != nil && project.SlackChannel.WebhookURL != ""`
3. Если да — отправить уведомление асинхронно (в горутине, чтобы не блокировать ответ API)

### Формат сообщения

Минимальный формат — простой `text` (mrkdwn), одна-две строки:

```json
{
  "text": ":red_circle: *<reviewURL|Review Title>* by author (`source` → `target`) — 2 critical, 1 high, 3 medium, 0 low"
}
```

Примеры:

```
:red_circle: *<https://reviewsrv.example.com/reviews/5/|CHT-47: Add payment processing>* by john (`feature/pay` → `main`) — 2 critical, 1 high, 3 medium, 0 low
```

```
:large_green_circle: *<https://reviewsrv.example.com/reviews/5/|CHT-52: Fix typo in readme>* by anna (`fix/typo` → `main`) — 0 critical, 0 high, 1 medium, 2 low
```

Маппинг `trafficLight` → emoji:
- `red` → `:red_circle:`
- `yellow` → `:large_yellow_circle:`
- `green` → `:large_green_circle:`

### URL ревью

Базовый URL фронтенда — через конфиг `Config.Server.BaseURL`.

URL ревью: `<BaseURL>/reviews/<reviewId>/`

## Изменения по файлам

### 1. `pkg/slack/slack.go` — новый пакет

```go
package slack

// Notifier отправляет уведомления в Slack через Incoming Webhook.
type Notifier struct {
    httpClient *http.Client
    logger     embedlog.Logger
}

// ReviewNotification — данные для отправки уведомления.
type ReviewNotification struct {
    WebhookURL   string
    ProjectTitle string
    ReviewID     int
    ProjectID    int
    Title        string
    Author       string
    SourceBranch string
    TargetBranch string
    TrafficLight string
    IssueStats   IssueStats
    ReviewURL    string
}

type IssueStats struct {
    Critical int
    High     int
    Medium   int
    Low      int
}

// Send отправляет уведомление в Slack. Логирует ошибку при неуспешном HTTP-статусе.
func (n *Notifier) Send(ctx context.Context, notif ReviewNotification) error
```

- HTTP POST на `notif.WebhookURL` с JSON-телом `{"text": "..."}`
- Таймаут: 10 секунд
- При ошибке — логируем, retry нет

### 2. `pkg/rest/rest.go` — добавить Notifier

```go
type Handler struct {
    pm       *reviewer.ProjectManager
    rm       *reviewer.ReviewManager
    notifier *slack.Notifier  // может быть nil
    baseURL  string
}
```

### 3. `pkg/rest/rest.go` — Handler.CreateReview

После `rm.CreateReview`:

```go
// Отправить Slack-уведомление асинхронно
if h.notifier != nil && project.SlackChannel != nil && project.SlackChannel.WebhookURL != "" {
    notif := slack.ReviewNotification{
        WebhookURL:   project.SlackChannel.WebhookURL,
        ProjectTitle: project.Title,
        ReviewID:     reviewID,
        ProjectID:    project.ID,
        // ... заполнить из rv и project
    }
    go h.notifier.Send(context.Background(), notif)
}
```

### 4. `pkg/reviewer/project.go` — GetByKey с FullProject join

Добавить `pm.repo.FullProject()` в существующий `GetByKey`:

```go
func (pm *ProjectManager) GetByKey(ctx context.Context, projectKey string) (*Project, error) {
    p, err := pm.repo.OneProject(ctx, &db.ProjectSearch{ProjectKey: &projectKey}, pm.repo.FullProject())
    return NewProject(p), err
}
```

### 5. `pkg/app/app.go` — инициализация

```go
type Config struct {
    // ...
    Server struct {
        // ...
        BaseURL string
    }
}
```

Передать `slack.Notifier` и `baseURL` в `rest.NewHandler`.
