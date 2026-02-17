# Zenrpc — справочник по использованию

Ты — эксперт по zenrpc (https://github.com/vmkteam/zenrpc) в контексте проекта reviewsrv.

## Обзор

Zenrpc — JSON-RPC 2.0 сервер для Go, использующий `go generate` вместо рефлексии.
Поддерживает SMD (Service Mapping Description) и OpenRPC спецификации.

Зависимости проекта:
- `github.com/vmkteam/zenrpc/v2` v2.3.1 — основная библиотека
- `github.com/vmkteam/zenrpc-middleware` v1.3.2 — middleware
- `github.com/vmkteam/rpcgen/v2` v2.5.4 — генерация клиентов (Go/TypeScript/PHP)

Инструмент генерации подключен как Go tool:
```go
tool github.com/vmkteam/zenrpc/v2/zenrpc
```

## Архитектура проекта

В проекте **два отдельных RPC-сервера**:

### 1. Review API (`pkg/rpc/`) — публичный API для фронтенда `/reviews/`
- Endpoint: `/v1/rpc/`
- Документация: `/v1/rpc/doc/` (SMDBox)
- Namespace: `review`
- Один сервис: `ReviewService`

### 2. Admin API (`pkg/vt/`) — админка `/vt/`
- Endpoint: `/v1/vt/`
- Документация: `/v1/vt/doc/` (SMDBox)
- Namespace'ы: `auth`, `user`, `project`, `prompt`, `slackChannel`, `taskTracker`
- 6 сервисов: AuthService, UserService, ProjectService, PromptService, SlackChannelService, TaskTrackerService

### Подключение в приложении (`pkg/app/app.go`)

```go
type App struct {
    vtsrv *zenrpc.Server  // Admin API
    srv   *zenrpc.Server  // Review API
}

a.vtsrv = vt.New(a.db, a.Logger, a.cfg.Server.IsDevel, a.cfg.Server.BaseURL)
a.srv = rpc.New(a.db, a.Logger, a.cfg.Server.IsDevel)
```

Маршруты (`pkg/app/handlers.go`):
- `/v1/rpc/` → `a.srv` (Review API)
- `/v1/vt/` → `a.vtsrv` (Admin API)

## Генерация кода

### Директива в server.go

В каждом RPC-пакете в `server.go` указана директива:
```go
//go:generate go tool zenrpc
```

### Команда генерации

```bash
make generate
```

Запускает:
```bash
go generate ./pkg/rpc    # → генерирует pkg/rpc/rpc_zenrpc.go
go generate ./pkg/vt     # → генерирует pkg/vt/vt_zenrpc.go
```

**ВАЖНО:** После любых изменений в API (добавление/изменение методов, параметров, аннотаций) необходимо запускать `make generate`. Эта команда также обновляет файлы фронтенда.

### TypeScript-клиенты

```bash
make type-script-client
```

Генерирует:
- `frontend/src/api/factory.generated.ts` — клиент для VT API
- `frontend/src/api/vt.generated.ts` — клиент для Review API

Зависит от `make generate` (запускается автоматически).

## Определение сервиса

### Структура сервиса

Сервис — Go-структура, которая содержит RPC-методы. Должна встраивать `zenrpc.Service`:

```go
type ReviewService struct {
    rm *reviewer.ReviewManager
    pm *reviewer.ProjectManager
    zenrpc.Service
}
```

### Конструктор сервиса

```go
func NewReviewService(dbc db.DB) *ReviewService {
    return &ReviewService{
        rm: reviewer.NewReviewManager(dbc),
        pm: reviewer.NewProjectManager(dbc),
    }
}
```

### VT-сервисы (расширенный паттерн)

VT-сервисы дополнительно встраивают `embedlog.Logger`:

```go
type ProjectService struct {
    zenrpc.Service
    embedlog.Logger
    projectRepo db.ProjectRepo
    baseURL     string
}

func NewProjectService(dbo db.DB, logger embedlog.Logger, baseURL string) *ProjectService {
    return &ProjectService{
        Logger:      logger,
        projectRepo: db.NewProjectRepo(dbo),
        baseURL:     baseURL,
    }
}
```

## Сигнатуры методов

Zenrpc поддерживает 4 варианта сигнатур:

```go
func (Service) Method([args]) (<value>, <error>)
func (Service) Method([args]) <value>
func (Service) Method([args]) <error>
func (Service) Method([args])
```

- Первый аргумент может быть `context.Context` (рекомендуется)
- Возвращаемые значения могут быть указателями
- Ошибки: стандартный `error` или `*zenrpc.Error`

## Система аннотаций

Аннотации пишутся в комментариях к методу с префиксом `//zenrpc:`:

### Описание параметров
```
//zenrpc:<paramName>[=<defaultValue>] <Description>
```

### Описание кодов ошибок
```
//zenrpc:<httpCode> <Description>
```

### Описание возвращаемого значения
```
//zenrpc:return <Type или Description>
```

### Полный пример

```go
// GetByID returns full review details.
//
//zenrpc:reviewId Review ID
//zenrpc:return Review
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) GetByID(ctx context.Context, reviewId int) (*Review, error) {
    rv, err := s.rm.GetReview(ctx, reviewId)
    if err != nil {
        return nil, newInternalError(err)
    }
    if rv == nil {
        return nil, ErrNotFound
    }
    return newReview(rv), nil
}
```

### Параметр с значением по умолчанию

```go
//zenrpc:exp=2
func (as ArithService) Pow(base float64, exp float64) float64 {
    return math.Pow(base, exp)
}
```

## Обработка ошибок

### Определение ошибок (pkg/rpc/server.go)

```go
var (
    ErrNotImplemented = zenrpc.NewStringError(http.StatusInternalServerError, "not implemented")
    ErrInternal       = zenrpc.NewStringError(http.StatusInternalServerError, "internal error")
    ErrNotFound       = zenrpc.NewStringError(http.StatusNotFound, "not found")
    ErrBadRequest     = zenrpc.NewStringError(http.StatusBadRequest, "bad request")
)
```

### Определение ошибок (pkg/vt/server.go)

VT использует хелпер для создания стандартных HTTP-ошибок:

```go
func httpAsRPCError(code int) *zenrpc.Error {
    return zenrpc.NewStringError(code, http.StatusText(code))
}

var (
    ErrUnauthorized   = httpAsRPCError(http.StatusUnauthorized)
    ErrForbidden      = httpAsRPCError(http.StatusForbidden)
    ErrNotFound       = httpAsRPCError(http.StatusNotFound)
    ErrInternal       = httpAsRPCError(http.StatusInternalServerError)
    ErrNotImplemented = httpAsRPCError(http.StatusNotImplemented)
)
```

### Хелпер для внутренних ошибок

```go
// pkg/rpc/server.go
func newInternalError(err error) *zenrpc.Error {
    return zenrpc.NewError(http.StatusInternalServerError, err)
}

// pkg/vt/validator.go
func InternalError(err error) *zenrpc.Error {
    return zenrpc.NewError(http.StatusInternalServerError, err)
}
```

### Ошибки валидации (только VT)

```go
func ValidationError(fieldErrors []FieldError) *zenrpc.Error {
    return &zenrpc.Error{Code: http.StatusBadRequest, Data: fieldErrors, Message: "Validation err"}
}
```

## Настройка сервера (server.go)

### Review API (pkg/rpc/server.go)

```go
//go:generate go tool zenrpc

func New(dbo db.DB, logger embedlog.Logger, isDevel bool) *zenrpc.Server {
    rpc := zenrpc.NewServer(zenrpc.Options{
        ExposeSMD: true,
        AllowCORS: true,
    })

    rpc.Use(
        zm.WithDevel(isDevel),
        zm.WithHeaders(),
        zm.WithSentry(zm.DefaultServerName),
        zm.WithNoCancelContext(),
        zm.WithMetrics(zm.DefaultServerName),
        zm.WithTiming(isDevel, allowDebugFn()),
        zm.WithSQLLogger(dbo.DB, isDevel, allowDebugFn(), allowDebugFn()),
    )

    rpc.Use(
        zm.WithSLog(logger.Print, zm.DefaultServerName, nil),
        zm.WithErrorSLog(logger.Print, zm.DefaultServerName, nil),
    )

    rpc.RegisterAll(map[string]zenrpc.Invoker{
        "review": NewReviewService(dbo),
    })

    return rpc
}
```

### Admin API (pkg/vt/server.go)

```go
//go:generate go tool zenrpc

const (
    NSAuth         = "auth"
    NSUser         = "user"
    NSProject      = "project"
    NSPrompt       = "prompt"
    NSSlackChannel = "slackChannel"
    NSTaskTracker  = "taskTracker"
)

func New(dbo db.DB, logger embedlog.Logger, isDevel bool, baseURL string) *zenrpc.Server {
    rpc := zenrpc.NewServer(zenrpc.Options{
        ExposeSMD: true,
        AllowCORS: true,
    })

    commonRepo := db.NewCommonRepo(dbo)

    rpc.Use(
        zm.WithHeaders(),
        zm.WithDevel(isDevel),
        zm.WithNoCancelContext(),
        zm.WithMetrics("vt"),
        zm.WithSLog(logger.Print, zm.DefaultServerName, nil),
        zm.WithErrorSLog(logger.Error, zm.DefaultServerName, nil),
        zm.WithSQLLogger(dbo.DB, isDevel, allowDebugFn(), allowDebugFn()),
        zm.WithTiming(isDevel, allowDebugFn()),
        zm.WithSentry(zm.DefaultServerName),
        authMiddleware(&commonRepo, logger),  // аутентификация
    )

    rpc.RegisterAll(map[string]zenrpc.Invoker{
        NSAuth:         NewAuthService(dbo, logger),
        NSUser:         NewUserService(dbo, logger),
        NSProject:      NewProjectService(dbo, logger, baseURL),
        NSPrompt:       NewPromptService(dbo, logger),
        NSSlackChannel: NewSlackChannelService(dbo, logger),
        NSTaskTracker:  NewTaskTrackerService(dbo, logger),
    })

    return rpc
}
```

## Middleware (zenrpc-middleware)

Импорт: `zm "github.com/vmkteam/zenrpc-middleware"`

Доступные middleware:
- `zm.WithDevel(isDevel)` — режим разработки
- `zm.WithHeaders()` — заголовки
- `zm.WithSentry(serverName)` — Sentry error tracking
- `zm.WithNoCancelContext()` — не отменять контекст
- `zm.WithMetrics(serverName)` — метрики
- `zm.WithTiming(isDevel, allowDebugFn)` — тайминг запросов
- `zm.WithSQLLogger(db, isDevel, allow1, allow2)` — логирование SQL
- `zm.WithSLog(printFn, serverName, nil)` — structured logging
- `zm.WithErrorSLog(errorFn, serverName, nil)` — structured error logging

### Кастомный middleware (аутентификация, pkg/vt/middleware.go)

```go
func authMiddleware(commonRepo *db.CommonRepo, logger embedlog.Logger) zenrpc.MiddlewareFunc {
    return func(h zenrpc.InvokeFunc) zenrpc.InvokeFunc {
        return func(ctx context.Context, method string, params json.RawMessage) zenrpc.Response {
            req, ok := zenrpc.RequestFromContext(ctx)
            if !ok {
                return h(ctx, method, params)
            }

            ns := zenrpc.NamespaceFromContext(ctx)

            // пропускаем auth.Login
            if ns == NSAuth && method == RPC.AuthService.Login {
                return h(ctx, method, params)
            }

            authHeader := req.Header.Get(AuthKey)
            if authHeader == "" {
                return zenrpc.NewResponseError(...)
            }

            dbu, err := commonRepo.EnabledUserByAuthKey(ctx, authHeader)
            if err != nil || dbu == nil {
                return zenrpc.NewResponseError(...)
            }

            return h(context.WithValue(ctx, userKey, dbu), method, params)
        }
    }
}
```

### Доступ к контексту

```go
// Получить HTTP-запрос
req, ok := zenrpc.RequestFromContext(ctx)

// Получить namespace текущего вызова
ns := zenrpc.NamespaceFromContext(ctx)

// Получить ID запроса
id := zenrpc.IDFromContext(ctx)

// Создать ответ с ошибкой
zenrpc.NewResponseError(id, code, message, data)

// Получить пользователя из контекста (кастомный)
user := UserFromContext(ctx)
```

## Паттерны VT-сервисов (CRUD)

Каждый VT-сервис для сущности содержит стандартный набор методов:

### Count — подсчёт записей

```go
// Count returns count Projects according to conditions in search params.
//
//zenrpc:search ProjectSearch
//zenrpc:return int
//zenrpc:500 Internal Error
func (s ProjectService) Count(ctx context.Context, search *ProjectSearch) (int, error) {
    count, err := s.projectRepo.CountProjects(ctx, search.ToDB())
    if err != nil {
        return 0, InternalError(err)
    }
    return count, nil
}
```

### Get — список с пагинацией и сортировкой

```go
// Get returns а list of Projects according to conditions in search params.
//
//zenrpc:search ProjectSearch
//zenrpc:viewOps ViewOps
//zenrpc:return []ProjectSummary
//zenrpc:500 Internal Error
func (s ProjectService) Get(ctx context.Context, search *ProjectSearch, viewOps *ViewOps) ([]ProjectSummary, error) {
    list, err := s.projectRepo.ProjectsByFilters(ctx, search.ToDB(), viewOps.Pager(), s.dbSort(viewOps), s.projectRepo.FullProject())
    if err != nil {
        return nil, InternalError(err)
    }
    projects := make([]ProjectSummary, 0, len(list))
    for i := range list {
        if project := NewProjectSummary(&list[i]); project != nil {
            projects = append(projects, *project)
        }
    }
    return projects, nil
}
```

### GetByID — получение по ID

```go
// GetByID returns a Project by its ID.
//
//zenrpc:id int
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ProjectService) GetByID(ctx context.Context, id int) (*Project, error) {
    db, err := s.byID(ctx, id)
    if err != nil {
        return nil, err
    }
    return NewProject(db), nil
}
```

### byID — приватный хелпер

```go
func (s ProjectService) byID(ctx context.Context, id int) (*db.Project, error) {
    db, err := s.projectRepo.ProjectByID(ctx, id, s.projectRepo.FullProject())
    if err != nil {
        return nil, InternalError(err)
    } else if db == nil {
        return nil, ErrNotFound
    }
    return db, nil
}
```

### Add — создание

```go
// Add adds a Project from the query.
//
//zenrpc:project Project
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
func (s ProjectService) Add(ctx context.Context, project Project) (*Project, error) {
    if ve := s.isValid(ctx, project, false); ve.HasErrors() {
        return nil, ve.Error()
    }

    db, err := s.projectRepo.AddProject(ctx, project.ToDB())
    if err != nil {
        return nil, InternalError(err)
    }
    return NewProject(db), nil
}
```

### Update — обновление

```go
// Update updates the Project data identified by id from the query.
//
//zenrpc:projects Project
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s ProjectService) Update(ctx context.Context, project Project) (bool, error) {
    if _, err := s.byID(ctx, project.ID); err != nil {
        return false, err
    }

    if ve := s.isValid(ctx, project, true); ve.HasErrors() {
        return false, ve.Error()
    }

    ok, err := s.projectRepo.UpdateProject(ctx, project.ToDB())
    if err != nil {
        return false, InternalError(err)
    }
    return ok, nil
}
```

### Delete — удаление

```go
// Delete deletes the Project by its ID.
//
//zenrpc:id int
//zenrpc:return isDeleted
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s ProjectService) Delete(ctx context.Context, id int) (bool, error) {
    if _, err := s.byID(ctx, id); err != nil {
        return false, err
    }

    ok, err := s.projectRepo.DeleteProject(ctx, id)
    if err != nil {
        return false, InternalError(err)
    }
    return ok, err
}
```

### Validate — валидация без сохранения

```go
// Validate verifies that Project data is valid.
//
//zenrpc:project Project
//zenrpc:return []FieldError
//zenrpc:500 Internal Error
func (s ProjectService) Validate(ctx context.Context, project Project) ([]FieldError, error) {
    isUpdate := project.ID != 0
    if isUpdate {
        _, err := s.byID(ctx, project.ID)
        if err != nil {
            return nil, err
        }
    }

    ve := s.isValid(ctx, project, isUpdate)
    if ve.HasInternalError() {
        return nil, ve.Error()
    }

    return ve.Fields(), nil
}
```

### dbSort — хелпер сортировки

```go
func (s ProjectService) dbSort(ops *ViewOps) db.OpFunc {
    v := s.projectRepo.DefaultProjectSort()
    if ops == nil {
        return v
    }

    switch ops.SortColumn {
    case db.Columns.Project.ID, db.Columns.Project.Title, db.Columns.Project.VcsURL:
        v = db.WithSort(db.NewSortField(ops.SortColumn, ops.SortDesc))
    }

    return v
}
```

### isValid — хелпер валидации

```go
func (s ProjectService) isValid(ctx context.Context, project Project, isUpdate bool) Validator {
    _ = isUpdate

    var v Validator

    if v.CheckBasic(ctx, project); v.HasInternalError() {
        return v
    }

    // Проверка FK
    if project.PromptID != 0 {
        item, err := s.projectRepo.PromptByID(ctx, project.PromptID)
        if err != nil {
            v.SetInternalError(err)
        } else if item == nil {
            v.Append("promptId", FieldErrorIncorrect)
        }
    }

    // custom validation starts here
    return v
}
```

## Модели (паттерн)

### Три типа моделей для VT-сервисов

Для каждой сущности определяются три структуры:

#### 1. Entity — полная модель (для GetByID, Add, Update)

```go
type Project struct {
    ID             int    `json:"id"`
    Title          string `json:"title" validate:"required,max=255"`
    VcsURL         string `json:"vcsURL" validate:"required,max=255"`
    Language       string `json:"language" validate:"required,max=32"`
    ProjectKey     string `json:"projectKey"`
    PromptID       int    `json:"promptId" validate:"required"`
    TaskTrackerID  *int   `json:"taskTrackerId"`
    SlackChannelID *int   `json:"slackChannelId"`
    StatusID       int    `json:"statusId" validate:"required,status"`

    Prompt       *PromptSummary       `json:"prompt"`       // вложенные связи
    TaskTracker  *TaskTrackerSummary  `json:"taskTracker"`
    SlackChannel *SlackChannelSummary `json:"slackChannel"`
    Status       *Status              `json:"status"`
}

func (p *Project) ToDB() *db.Project { ... }
```

#### 2. EntitySearch — фильтры поиска

```go
type ProjectSearch struct {
    ID             *int    `json:"id"`
    Title          *string `json:"title"`
    VcsURL         *string `json:"vcsURL"`
    StatusID       *int    `json:"statusId"`
    IDs            []int   `json:"ids"`
}

func (ps *ProjectSearch) ToDB() *db.ProjectSearch { ... }
```

#### 3. EntitySummary — краткая модель для списков

```go
type ProjectSummary struct {
    ID             int    `json:"id"`
    Title          string `json:"title"`
    VcsURL         string `json:"vcsURL"`

    Status *Status `json:"status"`
}
```

### ViewOps — пагинация и сортировка (pkg/vt/vt.go)

```go
type ViewOps struct {
    Page       int    `json:"page"`       // номер страницы, по умолчанию 1
    PageSize   int    `json:"pageSize"`   // записей на странице, максимум 500
    SortColumn string `json:"sortColumn"` // имя колонки для сортировки
    SortDesc   bool   `json:"sortDesc"`   // обратный порядок
}

func (v *ViewOps) Pager() db.Pager { ... }
```

### Status — статус сущности (pkg/vt/vt.go)

```go
type Status struct {
    ID    int    `json:"id"`
    Alias string `json:"alias"`
    Title string `json:"title"`
}

func NewStatus(id int) *Status { ... }
// db.StatusEnabled (1) → "enabled" / "Опубликован"
// db.StatusDisabled (2) → "disabled" / "Не опубликован"
// db.StatusDeleted (3) → "deleted" / "Удален"
```

### Модели для RPC-слоя (pkg/rpc/model.go)

RPC-модели отличаются от VT: они специфичны для конкретного API и не обязаны следовать CRUD-паттерну:

```go
type ReviewSummary struct {
    ID           int                 `json:"reviewId"`
    Title        string              `json:"title"`
    TrafficLight string              `json:"trafficLight"`
    Author       string              `json:"author"`
    CreatedAt    time.Time           `json:"createdAt"`
    ReviewFiles  []ReviewFileSummary `json:"reviewFiles"`
}
```

## Конвертеры

### VT-конвертеры (pkg/vt/project_converter.go, vt_converter.go)

Для каждой модели — конвертер из db в VT:

```go
func NewProject(in *db.Project) *Project {
    if in == nil {
        return nil
    }
    return &Project{
        ID:    in.ID,
        Title: in.Title,
        // ... все поля
        Prompt: NewPromptSummary(in.Prompt),  // вложенные связи
        Status: NewStatus(in.StatusID),
    }
}

func NewProjectSummary(in *db.Project) *ProjectSummary { ... }
```

### Хелперы конвертации (pkg/vt/vt_converter.go)

```go
// generic-конвертер слайсов
func mapp[T, M any](a []T, f func(*T) *M) []M { ... }

// форматирование дат
func fmtDate(t time.Time) string { return t.Format(time.DateOnly) }
func fmtDatePtr(t *time.Time) *string { ... }
```

### RPC-конвертеры (pkg/rpc/model.go)

Используют colgen для генерации batch-конвертеров. Отдельные конвертеры определяются вручную:

```go
func newReview(in *reviewer.Review) *Review { ... }
func newReviewSummary(in *reviewer.Review) *ReviewSummary { ... }
```

## Валидация (только VT, pkg/vt/validator.go)

### Структура Validator

```go
type Validator struct {
    fields []FieldError
    err    error
}

func (v *Validator) CheckBasic(ctx context.Context, item interface{}) // go-playground/validator
func (v *Validator) Append(field string, err string)                  // добавить ошибку
func (v *Validator) SetInternalError(err error)                       // внутренняя ошибка
func (v *Validator) HasErrors() bool                                  // есть ли ошибки
func (v *Validator) HasInternalError() bool                           // есть ли внутренняя ошибка
func (v *Validator) Error() error                                     // преобразовать в *zenrpc.Error
func (v *Validator) Fields() []FieldError                             // список ошибок полей
```

### Теги валидации (go-playground/validator)

- `validate:"required"` — обязательное поле
- `validate:"max=255"` — максимальная длина
- `validate:"min=1"` — минимальное значение
- `validate:"required,max=255"` — комбинация
- `validate:"required,status"` — кастомный тег (проверяет через NewStatus)
- `validate:"required,alias"` — кастомный тег (regex `^([0-9a-z-])+$`)

### Структура FieldError

```go
type FieldError struct {
    Field      string                `json:"field"`
    Error      string                `json:"error"`
    Constraint *FieldErrorConstraint `json:"constraint,omitempty"`
}
```

Стандартные ошибки полей:
- `FieldErrorRequired = "required"`
- `FieldErrorMax = "max"`
- `FieldErrorMin = "min"`
- `FieldErrorIncorrect = "incorrect"` — FK не найден
- `FieldErrorUnique = "unique"` — не уникальное значение
- `FieldErrorFormat = "format"` — неверный формат

## Файловая структура

### RPC-пакет (pkg/rpc/)

| Файл | Назначение | Редактируемый |
|------|-----------|---------------|
| `server.go` | Настройка сервера, ошибки, `//go:generate` | Да |
| `review.go` | ReviewService — методы API | Да |
| `model.go` | RPC-модели, фильтры, конвертеры | Да |
| `collection.go` | Аннотации colgen | Да |
| `collection_colgen.go` | **Сгенерирован** colgen | Нет |
| `rpc_zenrpc.go` | **Сгенерирован** zenrpc | Нет |

### VT-пакет (pkg/vt/)

| Файл | Назначение | Редактируемый |
|------|-----------|---------------|
| `server.go` | Настройка сервера, namespace'ы, ошибки, `//go:generate` | Да |
| `vt.go` | ViewOps, Status, хелперы | Да |
| `vt_service.go` | AuthService, UserService | Да |
| `vt_model.go` | Модели User, UserSearch, UserSummary, UserProfile | Да |
| `vt_converter.go` | Конвертеры User, mapp, fmtDate | Да |
| `project.go` | ProjectService, PromptService, SlackChannelService, TaskTrackerService | Да |
| `project_model.go` | Модели Project, Prompt, SlackChannel, TaskTracker (+ Search, Summary) | Да |
| `project_converter.go` | Конвертеры Project, Prompt, SlackChannel, TaskTracker | Да |
| `validator.go` | Validator, FieldError, validate | Да |
| `middleware.go` | Auth middleware, UserFromContext | Да |
| `vt_zenrpc.go` | **Сгенерирован** zenrpc | Нет |

## Типичные сценарии

### 1. Добавить новый метод в существующий сервис

1. Добавить метод в файл сервиса (например `pkg/rpc/review.go`):
```go
// NewMethod description.
//
//zenrpc:param1 Description
//zenrpc:return Type
//zenrpc:500 Internal Error
func (s ReviewService) NewMethod(ctx context.Context, param1 int) (*Result, error) {
    // реализация
}
```

2. Если нужны новые модели — добавить в `model.go`
3. Запустить `make generate`

### 2. Добавить новый CRUD-сервис в VT

1. Создать namespace-константу в `pkg/vt/server.go`:
```go
const NSNewEntity = "newEntity"
```

2. Создать файл модели (например `pkg/vt/newentity_model.go`):
```go
type NewEntity struct {
    ID       int    `json:"id"`
    Title    string `json:"title" validate:"required,max=255"`
    StatusID int    `json:"statusId" validate:"required,status"`
    Status   *Status `json:"status"`
}

func (e *NewEntity) ToDB() *db.NewEntity { ... }

type NewEntitySearch struct { ... }
func (s *NewEntitySearch) ToDB() *db.NewEntitySearch { ... }

type NewEntitySummary struct { ... }
```

3. Создать файл конвертера (`pkg/vt/newentity_converter.go`):
```go
func NewNewEntity(in *db.NewEntity) *NewEntity { ... }
func NewNewEntitySummary(in *db.NewEntity) *NewEntitySummary { ... }
```

4. Создать файл сервиса (`pkg/vt/newentity.go`) со стандартными методами:
   - `Count`, `Get`, `GetByID`, `Add`, `Update`, `Delete`, `Validate`
   - `byID` (приватный), `dbSort`, `isValid`

5. Зарегистрировать сервис в `pkg/vt/server.go`:
```go
rpc.RegisterAll(map[string]zenrpc.Invoker{
    // ... существующие
    NSNewEntity: NewNewEntityService(dbo, logger),
})
```

6. Запустить `make generate`

### 3. Добавить новый метод в Review API

1. Добавить метод в `pkg/rpc/review.go` (или создать новый файл сервиса)
2. Добавить модели в `pkg/rpc/model.go`
3. При необходимости обновить colgen-аннотации в `pkg/rpc/collection.go`
4. Запустить `make generate`

### 4. Изменить существующий метод

1. Изменить сигнатуру и/или аннотации метода
2. Обновить модели при необходимости
3. Запустить `make generate`

### 5. Добавить middleware

Добавить вызов `rpc.Use(...)` в соответствующий `server.go` **до** `rpc.RegisterAll()`.

## Сгенерированные файлы

Файлы с суффиксом `_zenrpc.go` содержат:
- Методы `Invoke()` для маршрутизации JSON-RPC вызовов
- SMD-описания для каждого сервиса
- Константы имён методов (`RPC.ServiceName.MethodName`)
- Автоматический парсинг параметров из JSON

**НЕ РЕДАКТИРУЙ** файлы `*_zenrpc.go` вручную — они перезаписываются при каждой генерации.

## Доступ к контексту в методах

```go
// Получить HTTP-запрос (host, headers и т.д.)
req, ok := zenrpc.RequestFromContext(ctx)
if ok {
    host := req.Host
    header := req.Header.Get("X-Custom-Header")
}
```

## JSON-RPC 2.0

Поддерживаемые возможности:
- Одиночные и batch-запросы
- Нотификации (без ответа)
- Именованные и позиционные параметры
- Значения параметров по умолчанию
- SMD-схема (`ExposeSMD: true`)
