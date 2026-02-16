# Colgen — справочник по использованию

Ты — эксперт по colgen (https://github.com/vmkteam/colgen) в контексте проекта reviewsrv.
Документация: https://vmkteam.pages.dev/colgen/

## Обзор

Colgen — генератор коллекционных методов для Go. Устраняет утилитарный boilerplate-код:
извлечение ID из слайсов, индексирование, группировка, конвертация между слоями (db → domain → rpc).

Работает через AST-парсинг: **код должен компилироваться перед генерацией**.

## Установка

```bash
go install github.com/vmkteam/colgen/cmd/colgen@latest
# или через Makefile проекта:
make tools
```

## Формат аннотаций

Аннотации пишутся в файле `collection.go` перед `go:generate`:

```go
//go:generate colgen [flags]
//colgen:Struct1,Struct2,Struct3           // базовые генераторы (IDs, Index, тип коллекции)
//colgen:Struct1:TagIDs,Group(FieldName)   // расширенные генераторы для конкретной структуры
```

### Флаги CLI

| Флаг | Описание | Пример |
|------|----------|--------|
| `-imports` | Пути импортов через запятую | `-imports=reviewsrv/pkg/db` |
| `-funcpkg` | Пакет для функций Map/MapP | `-funcpkg=reviewer` |
| `-list` | Суффикс "List" вместо "s" для коллекций | `-list` |

## Режимы генерации

### Базовый — тип коллекции + IDs + Index

```go
//colgen:Review,Issue
```

Для каждой структуры генерирует:
- `type Reviews []Review` — тип коллекции
- `func (ll Reviews) IDs() []int` — извлечение всех ID
- `func (ll Reviews) Index() map[int]Review` — индекс по ID

**Требование:** структура должна иметь поле `ID` с типом. Тип ID определяется автоматически.

### Расширенные генераторы

Указываются после `:` для конкретной структуры:

| Генератор | Результат | Пример |
|-----------|-----------|--------|
| `Group(Field)` | `GroupByField() map[T]Structs` | `//colgen:Issue:Group(ReviewFileID)` |
| `Index(Field)` | `IndexByField() map[T]Struct` | `//colgen:Episode:Index(MovieID)` |
| `<Field>` | `FieldValues() []T` — все значения поля | `//colgen:News:TagIDs` |
| `Unique<Field>` | `UniqueFieldValues() []T` — уникальные значения | `//colgen:News:UniqueTagIDs` |
| `MapP(pkg)` | `NewStructs([]pkg.Struct) Structs` — конвертер (public) | `//colgen:Review:MapP(db)` |
| `Map(pkg)` | `NewStructs([]pkg.Struct) Structs` — конвертер (public) | `//colgen:Review:Map(db)` |
| `mapp(pkg)` | `newStructs([]pkg.Struct) []Struct` — конвертер (private) | `//colgen:Review:mapp(reviewer)` |
| `map(pkg)` | `newStructs([]pkg.Struct) []Struct` — конвертер (private) | `//colgen:Review:map(reviewer)` |
| `mapp(pkg.Type)` | конвертер из другого типа | `//colgen:ReviewSummary:mapp(reviewer.Review)` |

**MapP vs Map:**
- `MapP` — конвертер принимает указатель: `func NewStruct(*pkg.Struct) *Struct`
- `Map` — конвертер принимает значение: `func NewStruct(pkg.Struct) Struct`
- Строчная `mapp`/`map` — генерирует private-функцию (без экспорта)

### Комбинирование

Генераторы комбинируются через запятую:

```go
//colgen:Issue:MapP(db),Group(ReviewFileID)
//colgen:Episode:ShowIDs,MapP(db.SiteUser),Index(MovieID),Group(ShowID)
```

### Inline-режим (конструкторы)

```go
//colgen@NewCall(db)
//colgen@newUserSummary(newsportal.User,full,json)
```

Генерирует конструкторы прямо в исходном файле.

## Вспомогательные функции

Colgen использует generic-хелперы, которые должны быть определены в пакете. В проекте они в `pkg/reviewer/collection.go`:

```go
// MapP converts slice of type T to slice of type M with given converter with pointers.
func MapP[T, M any](a []T, f func(*T) *M) []M {
    n := make([]M, len(a))
    for i := range a {
        n[i] = *f(&a[i])
    }
    return n
}

// Map converts slice of type T to slice of type M with given converter.
func Map[T, M any](a []T, f func(T) M) []M {
    n := make([]M, len(a))
    for i := range a {
        n[i] = f(a[i])
    }
    return n
}
```

При использовании `-funcpkg`, colgen ссылается на эти функции из указанного пакета: `reviewer.MapP(in, newStruct)`.

## Использование в проекте reviewsrv

### Архитектура конвертации

```
db (сгенерирован mfd)  →  reviewer (domain)  →  rpc (API)
    db.Review                Review                Review
    db.Issue                 Issue                  Issue
    []db.Review         →    Reviews           →    []Review
```

Каждый переход слоя использует colgen-конвертеры.

### pkg/reviewer/collection.go — domain-слой

```go
//go:generate colgen -imports=reviewsrv/pkg/db
//colgen:Review,ReviewFile,Issue,Project
//colgen:Project:MapP(db)
//colgen:Issue:MapP(db),Group(ReviewFileID)
//colgen:ReviewFile:MapP(db),Group(ReviewID)
//colgen:Review:MapP(db)
```

Генерирует `pkg/reviewer/collection_colgen.go`:
- Типы: `Issues`, `Projects`, `Reviews`, `ReviewFiles`
- Для каждого: `IDs()`, `Index()`
- Конвертеры: `NewIssues([]db.Issue)`, `NewReviews([]db.Review)` и т.д.
- Группировки: `Issues.GroupByReviewFileID()`, `ReviewFiles.GroupByReviewID()`

### pkg/rpc/collection.go — RPC-слой

```go
//go:generate colgen -funcpkg=reviewer -imports=reviewsrv/pkg/reviewer
//colgen:Project:mapp(reviewer)
//colgen:Issue:mapp(reviewer)
//colgen:ReviewFile:mapp(reviewer)
//colgen:Review:mapp(reviewer)
//colgen:ReviewSummary:mapp(reviewer.Review)
```

Генерирует `pkg/rpc/collection_colgen.go`:
- Private-конвертеры: `newIssues([]reviewer.Issue) []Issue` и т.д.
- Использует `reviewer.MapP` через `-funcpkg=reviewer`
- `ReviewSummary:mapp(reviewer.Review)` — конвертер из другого типа (Review → ReviewSummary)

### Запуск генерации

```bash
make generate
# запускает: go generate ./pkg/rpc && go generate ./pkg/vt
```

Или напрямую:
```bash
go generate ./pkg/reviewer
go generate ./pkg/rpc
```

## Типичные сценарии

### Добавить новую domain-сущность с коллекцией

1. Создать структуру-обёртку в `pkg/reviewer/model.go`:
```go
type Prompt struct {
    db.Prompt
}

func NewPrompt(in *db.Prompt) *Prompt {
    if in == nil { return nil }
    return &Prompt{Prompt: *in}
}
```

2. Добавить в `pkg/reviewer/collection.go`:
```go
//colgen:Review,ReviewFile,Issue,Project,Prompt          // добавить в базовый список
//colgen:Prompt:MapP(db)                                  // конвертер из db
```

3. Запустить: `go generate ./pkg/reviewer`

Результат — в `collection_colgen.go` появятся:
- `type Prompts []Prompt`
- `func (ll Prompts) IDs() []int`
- `func (ll Prompts) Index() map[int]Prompt`
- `func NewPrompts(in []db.Prompt) Prompts`

### Добавить группировку по полю

```go
//colgen:Issue:MapP(db),Group(ReviewFileID),Group(ReviewID)
```

Добавит `func (ll Issues) GroupByReviewID() map[int]Issues`.

### Добавить RPC-конвертер для нового типа

1. Создать структуру и конвертер в `pkg/rpc/model.go`:
```go
type Prompt struct { ... }
func newPrompt(in *reviewer.Prompt) *Prompt { ... }
```

2. Добавить в `pkg/rpc/collection.go`:
```go
//colgen:Prompt:mapp(reviewer)
```

3. Запустить: `go generate ./pkg/rpc`

### Заменить ручной `mapp` в vt-пакете

В `pkg/vt/vt_converter.go` есть дублирующийся `mapp` и ручной вызов `mapp(list, NewUserSummary)`.
Можно заменить на colgen:

1. Создать `pkg/vt/collection.go`:
```go
package vt

//go:generate colgen -funcpkg=vt -imports=reviewsrv/pkg/db
//colgen:UserSummary:mapp(db.User)
```

2. Удалить ручную функцию `mapp` из `vt_converter.go`
3. Запустить: `go generate ./pkg/vt`

### Извлечь значения конкретного поля

```go
//colgen:News:TagIDs                    // func (ll Newss) TagIDs() []int
//colgen:News:UniqueTagIDs              // func (ll Newss) UniqueTagIDs() []int — без дубликатов
```

## Маркеры сгенерированного кода

```go
// Code generated by colgen v0.1.2; DO NOT EDIT.
```

**НЕ РЕДАКТИРУЙ** файлы `*_colgen.go` вручную — они перезаписываются при каждой генерации.

Файлы, которые безопасно редактировать:
- `collection.go` — аннотации и хелперы (MapP, Map, Ptr)
- Файлы с конвертерами (`NewXxx` функции) — они вызываются из сгенерированного кода

## Требования

- Код **должен компилироваться** перед запуском colgen (AST-парсинг)
- Структура должна иметь поле `ID` для базовых генераторов (IDs, Index)
- Конвертер `New<Struct>(*pkg.Struct) *Struct` должен существовать для MapP
- Хелперы `MapP`/`Map` должны быть определены в пакете (или доступны через `-funcpkg`)

## AI-режимы (опционально)

```go
//colgen@ai:readme            // генерация README через DeepSeek
//colgen@ai:tests(claude)     // генерация тестов через Claude
//colgen@ai:review(deepseek)  // код-ревью через DeepSeek
```

Требует настройки ключа: `colgen -write-key=<key> -ai=claude`
