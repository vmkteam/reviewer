# MFD Generator — справочник по использованию

Ты — эксперт по mfd-generator (https://github.com/vmkteam/mfd-generator) в контексте проекта reviewsrv.

## Обзор проекта

- MFD-файл: `docs/model/reviewsrv.mfd`
- Пакеты (namespaces): `common`, `review`, `project`
- GoPGVer: 10
- TableMapping:
  - common: users
  - project: projects, taskTrackers, slackChannels, prompts
  - review: reviews, reviewFiles, issues
- XML-модели: `docs/model/common.xml`, `docs/model/review.xml`, `docs/model/project.xml`
- VT XML: `docs/model/common.vt.xml`

## Makefile-команды проекта

Все команды используют переменную `NAME=reviewsrv` из `Makefile.mk`. Некоторые требуют `NS=<namespace>`.

### Установка инструментов
```bash
make tools
```
Устанавливает: mfd-generator, pgmigrator, colgen, golangci-lint.

### Группа 1 — XML-генераторы (из БД в XML)

#### mfd-xml — обновить XML из схемы PostgreSQL
```bash
make mfd-xml
```
Команда: `mfd-generator xml -c "postgres://..." -m ./docs/model/reviewsrv.mfd`
Флаги:
- `-c, --conn string` — строка подключения к PostgreSQL
- `-m, --mfd string` — путь к MFD-файлу
- `-t, --tables strings` — таблицы (по умолчанию `public.*`)
- `-n, --namespaces string` — маппинг в формате `"ns1=table1,table2;ns2=table3"`
- `-p, --print` — только показать маппинг, не генерировать
- `-v, --verbose` — показывать SQL-запросы

Результат: обновляет XML-файлы в `docs/model/` на основе текущей схемы БД.
Сохраняет пользовательские изменения (удалённые поиски не восстанавливаются, существующие атрибуты не дублируются).

#### mfd-vt-xml — сгенерировать VT XML
```bash
make mfd-vt-xml
```
Команда: `mfd-generator xml-vt -m ./docs/model/reviewsrv.mfd`
Флаги:
- `-m, --mfd string` — путь к MFD-файлу
- `-n, --namespaces strings` — конкретные namespace'ы

Результат: создаёт/обновляет `[namespace].vt.xml` файлы. Режимы VT-сущностей: Full, ReadOnly, ReadOnlyWithTemplates, None.

#### mfd-xml-lang — сгенерировать переводы
```bash
make mfd-xml-lang
```
Команда: `mfd-generator xml-lang -m ./docs/model/reviewsrv.mfd`
Флаги:
- `-m, --mfd string` — путь к MFD-файлу
- `-l, --langs strings` — языки (ru, en, de)
- `-n, --namespaces strings` — namespace'ы
- `-e, --entities strings` — конкретные сущности

### Группа 2 — Go-генераторы кода

#### mfd-model — сгенерировать Go-модели
```bash
make mfd-model
```
Команда: `mfd-generator model -m ./docs/model/reviewsrv.mfd -p db -o ./pkg/db`
Флаги:
- `-m, --mfd string` — путь к MFD-файлу
- `-p, --package string` — имя пакета
- `-o, --output string` — директория вывода

Генерирует 4 файла в `pkg/db/`:
- `model.go` — структуры, Columns, Tables
- `model_search.go` — Searcher интерфейс, XxxSearch структуры с Apply()
- `model_validate.go` — Validate() для каждой сущности
- `model_params.go` — структуры для JSON/JSONB атрибутов (только дописывается, не перезаписывается)

#### mfd-repo — сгенерировать репозитории
```bash
make mfd-repo NS=review
make mfd-repo NS=project
make mfd-repo NS=common
```
Команда: `mfd-generator repo -m ./docs/model/reviewsrv.mfd -p db -o ./pkg/db -n <NS>`
Флаги:
- `-m, --mfd string` — путь к MFD-файлу
- `-p, --package string` — имя пакета
- `-o, --output string` — директория вывода
- `-n, --namespaces strings` — **обязательный**, namespace для генерации

Генерирует `pkg/db/<namespace>.go` с методами:
- `New<Entity>Repo(db)`, `WithTransaction(tx)`, `WithEnabledOnly()`
- `<Entity>ByID(ctx, id)`, `One<Entity>(ctx, search)`
- `<Entity>sByFilters(ctx, search, pager)`, `Count<Entity>s(ctx, search)`
- `Add<Entity>(ctx, obj)`, `Update<Entity>(ctx, obj)`, `Delete<Entity>(ctx, id)`

#### mfd-db-test — сгенерировать тестовые хелперы
```bash
make mfd-db-test
```
Команда: `mfd-generator dbtest -m docs/model/reviewsrv.mfd -o ./pkg/db/test -x reviewsrv/pkg/db`
Флаги:
- `-m, --mfd string` — путь к MFD-файлу
- `-o, --output string` — директория вывода
- `-x, --db-pkg string` — import path пакета db
- `-n, --namespaces strings` — конкретные namespace'ы
- `-e, --entities strings` — конкретные сущности
- `-f, --force` — принудительная перегенерация

Генерирует в `pkg/db/test/`:
- `test.go` — инфраструктура (подключение к БД, Cleaner, NextID)
- `common.go`, `project.go`, `review.go` — фабрики тестовых данных с gofakeit/v7
- Паттерн: `func <Entity>(t, dbo, in, ...OpFunc) (*db.<Entity>, Cleaner)`
- Хелперы: `WithFake<Entity>`, `With<Entity>Relations`

### Группа 3 — VT/UI генераторы

#### mfd-vt-rpc — сгенерировать VT RPC-код
```bash
make mfd-vt-rpc NS=common
```
Команда: `mfd-generator vt -m docs/model/reviewsrv.mfd -o pkg/vt -p vt -x reviewsrv/pkg/db -n <NS>`
Флаги:
- `-m, --mfd string` — путь к MFD-файлу
- `-o, --output string` — директория вывода
- `-p, --package string` — имя пакета
- `-x, --model string` — import path пакета с моделями
- `-n, --namespaces strings` — **обязательный**, namespace
- `-e, --entities strings` — конкретные сущности в одном namespace

Генерирует в `pkg/vt/`:
- `<ns>_model.go` — VT-модели с JSON-тегами, ToDB(), SearchFrom()
- `<ns>_converter.go` — конвертеры New<Entity>() из db в VT модели
- `<ns>.go` — сервис с методами Count, Get, GetByID, Add, Update, Delete

#### mfd-vt-template — сгенерировать JS-шаблоны
```bash
make mfd-vt-template NS=common
```
Команда: `mfd-generator template -m docs/model/reviewsrv.mfd -o ../gold-vt/ -n <NS>`

### Дополнительная генерация

#### go generate — zenrpc и colgen
```bash
make generate
```
Запускает `go generate ./pkg/rpc` и `go generate ./pkg/vt` — генерирует zenrpc-обвязку и colgen-коллекции.

## Типичные сценарии использования

### Добавлена новая таблица в БД
```bash
# 1. Обновить XML из схемы БД
make mfd-xml
# 2. Перегенерировать Go-модели
make mfd-model
# 3. Сгенерировать репозиторий для нужного namespace
make mfd-repo NS=<namespace>
# 4. Обновить тесты
make mfd-db-test
# 5. Если нужен VT
make mfd-vt-xml
make mfd-vt-rpc NS=<namespace>
make generate
```

### Изменилась структура существующей таблицы
```bash
make mfd-xml        # обновить XML
make mfd-model      # перегенерировать модели
make mfd-repo NS=<ns>  # перегенерировать репозиторий
make mfd-db-test    # обновить тесты
```

### Только обновить модели (без изменения БД)
```bash
make mfd-model
```

### Полная перегенерация всего
```bash
make mfd-xml
make mfd-model
make mfd-repo NS=common
make mfd-repo NS=review
make mfd-repo NS=project
make mfd-db-test
make mfd-vt-xml
make mfd-vt-rpc NS=common
make generate
```

## Структура XML и ручное редактирование

XML-файлы — это **источник истины** для всех генераторов. Их можно и нужно редактировать вручную для тонкой настройки модели. При повторном запуске `mfd-xml` пользовательские изменения сохраняются.

### MFD-файл (docs/model/reviewsrv.mfd)
```xml
<Project xmlns:xsi="" xmlns:xsd="">
    <Name>reviewsrv.mfd</Name>
    <PackageNames>  <!-- список активных namespace'ов, только они будут генерироваться -->
        <string>common</string>
        <string>review</string>
        <string>project</string>
    </PackageNames>
    <Languages><string>ru</string></Languages>  <!-- управляется xml-lang, используется template -->
    <GoPGVer>10</GoPGVer>  <!-- версия go-pg: 8, 9 или 10. Влияет на импорты и аннотации -->
    <TableMapping>  <!-- маппинг namespace → таблицы -->
        <common>users</common>
        <project>projects,taskTrackers,slackChannels,prompts</project>
        <review>reviews,reviewFiles,issues</review>
    </TableMapping>
</Project>
```

**Что можно менять в MFD-файле:**
- `PackageNames` — добавить/удалить namespace из генерации. Если namespace не в списке — его файл не генерируется, даже если XML есть
- `TableMapping` — переназначить таблицы между namespace'ами
- `GoPGVer` — влияет на все Go-генераторы (импорты: `go-pg/pg` vs `go-pg/pg/v10`, аннотации: `sql:"title"` vs `pg:"title"`, функции: `pg.F`/`pg.Q` vs `pg.Ident`/`pg.SafeQuery`)

### Namespace XML (docs/model/<namespace>.xml)

Файл содержит все сущности namespace'а:
```xml
<Package xmlns:xsi="" xmlns:xsd="">
    <Name>blog</Name>
    <Entities>
        <Entity Name="Post" Namespace="blog" Table="posts">
            <Attributes>
                <Attribute Name="ID" DBName="postId" DBType="int4" GoType="int" PK="true" Nullable="Yes" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
                <Attribute Name="Alias" DBName="alias" DBType="varchar" GoType="string" PK="false" Nullable="No" Addable="true" Updatable="true" Min="0" Max="255"></Attribute>
                <Attribute Name="Title" DBName="title" DBType="varchar" GoType="string" PK="false" Nullable="No" Addable="true" Updatable="true" Min="0" Max="255"></Attribute>
                <Attribute Name="Text" DBName="text" DBType="text" GoType="string" PK="false" Nullable="No" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
                <Attribute Name="Views" DBName="views" DBType="int4" GoType="int" PK="false" Nullable="No" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
                <Attribute Name="CreatedAt" DBName="createdAt" DBType="timestamp" GoType="time.Time" PK="false" Nullable="No" Addable="false" Updatable="false" Min="0" Max="0"></Attribute>
                <Attribute Name="UserID" DBName="userId" DBType="int4" GoType="int" PK="false" FK="User" Nullable="No" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
                <Attribute Name="TagIDs" DBName="tagIds" IsArray="true" DBType="int4" GoType="[]int" PK="false" FK="Tag" Nullable="Yes" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
                <Attribute Name="StatusID" DBName="statusId" DBType="int4" GoType="int" PK="false" Nullable="No" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
            </Attributes>
            <Searches>
                <Search Name="IDs" AttrName="ID" SearchType="SEARCHTYPE_ARRAY"></Search>
                <Search Name="NotID" AttrName="ID" SearchType="SEARCHTYPE_NOT_EQUALS"></Search>
                <Search Name="TitleILike" AttrName="Title" SearchType="SEARCHTYPE_ILIKE"></Search>
                <Search Name="TextILike" AttrName="Text" SearchType="SEARCHTYPE_ILIKE"></Search>
            </Searches>
        </Entity>
    </Entities>
</Package>
```

**Entity** — описание сущности:
- `Name` — имя сущности (капитализированное единственное число от имени таблицы)
- `Namespace` — namespace, к которому принадлежит
- `Table` — имя таблицы в БД (если без схемы — используется `public`)

### Свойства атрибутов (Attribute)

Каждый атрибут описывает колонку таблицы:

| Свойство | Описание | Значения |
|----------|----------|----------|
| `Name` | Имя Go-поля. PK автоматически переименовывается в "ID". Уникально для сущности | `ID`, `Title`, `UserID` |
| `DBName` | Имя колонки в БД | `postId`, `title`, `userId` |
| `DBType` | Тип PostgreSQL | `int4`, `varchar`, `text`, `timestamptz`, `jsonb`, `bool` |
| `GoType` | Тип Go | `int`, `string`, `time.Time`, `*string`, `*int`, `[]int` |
| `IsArray` | Флаг массива | `true` (для колонок-массивов) |
| `PK` | Primary key | `true`/`false` |
| `FK` | Foreign key — ссылка на Entity | `User`, `Project`, `Review` |
| `Nullable` | Может быть NULL | `Yes`/`No` |
| `Addable` | Можно задать при INSERT | `true`/`false` |
| `Updatable` | Можно задать при UPDATE | `true`/`false` |
| `DisablePointer` | Отключить указатель в поиске | `true` |
| `Min` | Минимальное значение/длина строки | `0` |
| `Max` | Максимальное значение/длина строки | `255` |
| `HasDefault` | Есть DEFAULT в БД | `true` |

**Правила при редактировании атрибутов:**
- На `Name` ссылаются поиски (`<Searches>`) и VT-атрибуты — при переименовании обновить все ссылки
- `FK` указывается как имя Entity (не таблицы): `FK="User"`, не `FK="users"`. Для массивов FK: поле `EntityIDs` → `FK="Entity"`, если Entity существует
- Поля `createdAt` и `modifiedAt` автоматически получают `Addable="false"`, `Updatable="false"`
- Для `Nullable="Yes"` к GoType добавляется `*` (указатель). Можно убрать указатель вручную если нужно
- `json`/`jsonb` типы генерируют именованный тип: `EntityFieldName` (например `ReviewFileIssueStats`)
- Неизвестные типы БД генерируют `interface{}`

### Маппинг типов PostgreSQL → Go (GoType)

| PostgreSQL | Go |
|-----------|-----|
| integer, serial | int |
| bigint | int64 |
| real | float32 |
| double, numeric | float64 |
| text, varchar, uuid, point | string |
| boolean | bool |
| timestamp, date, time | time.Time |
| interval | time.Duration |
| hstore | map[string]string |
| inet | net.IP |
| cidr | net.IPNet |
| json, jsonb | `EntityFieldName` (именованный тип) |

Массивы: к типу добавляется `[]` (hstore и json(b) не могут быть массивами).
Nullable: к типу добавляется `*`.

### Поиски (Searches)

Каждый поиск описывает условие фильтрации по сущности.

| Свойство | Описание |
|----------|----------|
| `Name` | Имя поиска в структуре Search. Уникально для сущности (включая имена атрибутов) |
| `AttrName` | Ссылка на атрибут. Может ссылаться на другую сущность: `User.ID`, `Category.ShowOnMain` |
| `SearchType` | Тип SQL-условия |
| `GoType` | Опционально, только для JSON/JSONB полей: `int`, `string`, `bool`, `[]int`, `[]string` и т.д. |

**Автогенерация поисков при добавлении нового атрибута:**
- Строковые поля (кроме `Alias`): `SEARCHTYPE_ILIKE` с суффиксом `ILike` (например `TitleILike`)
- `ID` поля: `SEARCHTYPE_ARRAY` с именем `IDs`
- Если есть поле `Alias`: добавляется `NotID` с `SEARCHTYPE_NOT_EQUALS` (для проверки уникальности в VT)

**Важно:** Если атрибут уже существует — новые поиски для него НЕ генерируются. Удалённый вручную поиск НЕ восстановится при повторной генерации.

### Типы поиска (SearchType) — полный список

| Тип | SQL-условие |
|-----|-------------|
| `SEARCHTYPE_EQUALS` | `f = v` |
| `SEARCHTYPE_NOT_EQUALS` | `f != v` |
| `SEARCHTYPE_NULL` | `f IS NULL` |
| `SEARCHTYPE_NOT_NULL` | `f IS NOT NULL` |
| `SEARCHTYPE_GE` | `f >= v` |
| `SEARCHTYPE_LE` | `f <= v` |
| `SEARCHTYPE_G` | `f > v` |
| `SEARCHTYPE_L` | `f < v` |
| `SEARCHTYPE_LEFT_LIKE` | `f LIKE '%v'` |
| `SEARCHTYPE_LEFT_ILIKE` | `f ILIKE '%v'` |
| `SEARCHTYPE_RIGHT_LIKE` | `f LIKE 'v%'` |
| `SEARCHTYPE_RIGHT_ILIKE` | `f ILIKE 'v%'` |
| `SEARCHTYPE_LIKE` | `f LIKE '%v%'` |
| `SEARCHTYPE_ILIKE` | `f ILIKE '%v%'` |
| `SEARCHTYPE_ARRAY` | `f IN (v, v1, v2)` |
| `SEARCHTYPE_NOT_INARRAY` | `f NOT IN (v1, v2)` |
| `SEARCHTYPE_ARRAY_CONTAINS` | `v = ANY(f)` |
| `SEARCHTYPE_ARRAY_NOT_CONTAINS` | `v != ALL(f)` |
| `SEARCHTYPE_ARRAY_CONTAINED` | `ARRAY[v] <@ f` |
| `SEARCHTYPE_ARRAY_INTERSECT` | `ARRAY[v] && f` |
| `SEARCHTYPE_JSONB_PATH` | `f @> v` |

**Влияние SearchType на GoType в сгенерированном Search:**
- `SEARCHTYPE_ARRAY`, `SEARCHTYPE_NOT_INARRAY` — всегда слайс (`[]int`, `[]string`)
- `SEARCHTYPE_NULL`, `SEARCHTYPE_NOT_NULL` — всегда `bool`
- Все остальные — GoType из атрибута с обязательным указателем (`*int`, `*string`)

### Поиск по JSON/JSONB полям

Формат ссылки: `AttrName="JsonField->keyName"`. Поддерживаются вложенные пути: `Params->parent->subValue`.

**SQL для JSON-поисков:**

| SearchType | SQL |
|-----------|-----|
| `SEARCHTYPE_EQUALS` | `f->>'k' IN (v)` |
| `SEARCHTYPE_NOT_EQUALS` | `f->>'k' NOT IN (v)` |
| `SEARCHTYPE_NULL` | `f->>'k' IS NULL` |
| `SEARCHTYPE_NOT_NULL` | `f->>'k' IS NOT NULL` |
| `SEARCHTYPE_ARRAY` | `f->>'k' IN (v, v1, v2)` |
| `SEARCHTYPE_NOT_INARRAY` | `f->>'k' NOT IN (v1, v2)` |
| `SEARCHTYPE_ARRAY_CONTAINS` | `f @> '{"k": [v]}'` (только JSONB) |
| `SEARCHTYPE_ARRAY_NOT_CONTAINS` | `NOT f @> '{"k": [v]}'` (только JSONB) |
| `SEARCHTYPE_JSONB_PATH` | `f @> v` |

**Ограничения:**
- Нельзя ссылаться на JSON-поля других сущностей
- `ARRAY_CONTAINS`/`ARRAY_NOT_CONTAINS` — только для JSONB (не JSON), ищет одно значение в массиве, рекомендуется GIN-индекс с `jsonb_path_ops`
- Для JSON-поисков обязательно указывать `GoType`, иначе будет `interface{}`

**Примеры поисков (для добавления в `<Searches>`):**
```xml
<!-- Поиск по значению в связанной сущности -->
<Search Name="IsMain" AttrName="Rubric.IsMain" SearchType="SEARCHTYPE_EQUALS"></Search>

<!-- Поиск int-значения в JSON-ключе smsCount поля Params -->
<Search Name="SmsCount" AttrName="Params->smsCount" SearchType="SEARCHTYPE_EQUALS" GoType="int"></Search>

<!-- Поиск string-значения в JSON-ключе addressHome поля Params (NOT EQUALS) -->
<Search Name="NotAddressHome" AttrName="Params->addressHome" SearchType="SEARCHTYPE_NOT_EQUALS" GoType="string"></Search>

<!-- Поиск bool-значения в JSON-ключе isPasswordSent -->
<Search Name="IsPasswordSent" AttrName="Params->isPasswordSent" SearchType="SEARCHTYPE_EQUALS" GoType="bool"></Search>

<!-- Проверка наличия/null ключа token в JSON -->
<Search Name="TokenNotExists" AttrName="Params->token" SearchType="SEARCHTYPE_NULL" GoType="string"></Search>

<!-- Вложенный JSON-ключ: parent->subValue -->
<Search Name="YandexSubValue" AttrName="Params->parent->subValue" SearchType="SEARCHTYPE_EQUALS" GoType="int"></Search>

<!-- Поиск значения в JSON-массиве (ARRAY_CONTAINS, только JSONB) -->
<Search Name="FavoriteProduct" AttrName="Params->favoriteProducts" SearchType="SEARCHTYPE_ARRAY_CONTAINS" GoType="int"></Search>

<!-- Исключение значения из JSON-массива -->
<Search Name="NotFavoriteProduct" AttrName="Params->favoriteProducts" SearchType="SEARCHTYPE_ARRAY_NOT_CONTAINS" GoType="int"></Search>

<!-- Поиск по нескольким значениям JSON-ключа (слайс) -->
<Search Name="SmsCounts" AttrName="Params->smsCount" SearchType="SEARCHTYPE_ARRAY" GoType="[]int"></Search>
<Search Name="AddressHomes" AttrName="Params->addressHome" SearchType="SEARCHTYPE_ARRAY" GoType="[]string"></Search>

<!-- Поиск по JSONB-пути -->
<Search Name="ParamsPath" AttrName="Params" SearchType="SEARCHTYPE_JSONB_PATH"></Search>
```

### Консистентность и валидация

При загрузке проекта проверяется:
- Каждый поиск в `<Searches>` ссылается на существующие в XML сущность и атрибут
- Каждый FK-атрибут ссылается на существующие сущность и атрибут

Если проверки не пройдены — проект НЕ загрузится с ошибкой. При ручном редактировании XML обязательно проверять ссылочную целостность.

### Поведение при повторной генерации (mfd-xml)

- Существующая сущность дополняется новыми атрибутами (определяются по паре `DBName` + `DBType`)
- Если поменять тип колонки в БД — она добавится как новый атрибут
- Для существующих атрибутов новые поиски НЕ генерируются
- Удалённые вручную поиски НЕ восстанавливаются
- Все пользовательские изменения в атрибутах (Name, GoType, Addable, Min/Max и т.д.) сохраняются

### Типичные операции ручного редактирования XML

**1. Добавить новый поиск к сущности:**
```xml
<!-- Добавить в секцию <Searches> нужной Entity -->
<Search Name="StatusIDs" AttrName="StatusID" SearchType="SEARCHTYPE_ARRAY"></Search>
<Search Name="CreatedAtFrom" AttrName="CreatedAt" SearchType="SEARCHTYPE_GE"></Search>
<Search Name="CreatedAtTo" AttrName="CreatedAt" SearchType="SEARCHTYPE_LE"></Search>
```
После: `make mfd-model` для перегенерации Go-кода.

**2. Изменить ограничения поля:**
```xml
<!-- Увеличить максимальную длину Title до 512 -->
<Attribute Name="Title" ... Max="512"></Attribute>
```
После: `make mfd-model` (обновит валидатор).

**3. Сделать поле неизменяемым:**
```xml
<Attribute Name="ExternalID" ... Addable="true" Updatable="false"></Attribute>
```
После: `make mfd-model` и `make mfd-repo NS=<ns>`.

**4. Добавить FK на другую сущность:**
```xml
<Attribute Name="CategoryID" DBName="categoryId" DBType="int4" GoType="int" PK="false" FK="Category" Nullable="No" Addable="true" Updatable="true" Min="0" Max="0"></Attribute>
```
Сущность `Category` должна существовать в проекте.

**5. Добавить поиск по связанной сущности:**
```xml
<Search Name="ProjectTitle" AttrName="Project.Title" SearchType="SEARCHTYPE_ILIKE"></Search>
```

**6. Убрать указатель у nullable-поля:**
Изменить `GoType="*string"` на `GoType="string"` — поле останется nullable в БД, но в Go будет без указателя.

## Маркеры сгенерированного кода

Все сгенерированные файлы содержат заголовок:
```go
// Code generated by mfd-generator v0.6.1; DO NOT EDIT.
```

**НЕ РЕДАКТИРУЙ** файлы с таким заголовком вручную — изменения будут потеряны при следующей генерации.

Файлы, которые безопасно редактировать:
- `pkg/db/model_params.go` — только дописывается (append), не перезаписывается
- `pkg/db/db.go` — ручной код подключения к БД
- `pkg/app/` — логика приложения
- `pkg/reviewer/` — доменная логика (кроме `*_colgen.go`)
- `pkg/rpc/server.go`, `pkg/rpc/collection.go`, `pkg/rpc/review.go` — ручной RPC-код

## Web UI

```bash
mfd-generator server
```
Запускает веб-интерфейс для визуального редактирования XML-файлов и сущностей.

## Dictionary (пользовательские переводы)

В MFD-файле можно задать секцию `<Dictionary>` для кастомных переводов:
```xml
<Dictionary>
    <user>Пользователь (кастомный)</user>
    <myCustomButton>Моя кнопка</myCustomButton>
</Dictionary>
```

Используется через `Translate(RuLang, "user")`.
