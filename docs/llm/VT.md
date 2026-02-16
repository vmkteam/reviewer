# VT — Административная панель

Административная часть для управления сущностями: Projects, Prompts, Task Trackers, Slack Channels, Users.

## Стек

Vue 3, TypeScript, Composition API, Tailwind CSS, JSON-RPC 2.0

## API

- Endpoint: `POST /v1/vt/` (JSON-RPC 2.0)
- Авторизация: заголовок `Authorization2: <authKey>`
- Контракт: `http://localhost:8075/v1/vt/api.ts` (генерируется из zenrpc)
- Неймспейсы: `auth`, `project`, `prompt`, `slackChannel` (в RPC — `slackchannel`), `taskTracker` (в RPC — `tasktracker`), `user`

### Общие типы

```typescript
IViewOps { page, pageSize, sortColumn, sortDesc }  // пагинация + сортировка
IFieldError { field, error, constraint? }           // ошибка валидации
IStatus { id, alias, title }                        // statusId: 1=Опубликован (зеленый), 2=Не опубликован (серый)
```

### CRUD-контракт (одинаков для всех сущностей)

Каждый неймспейс (`project`, `prompt`, `slackchannel`, `tasktracker`, `user`) имеет методы:

| Метод | Params | Return | Описание |
|-------|--------|--------|----------|
| `Count` | `{ search? }` | `number` | Количество записей по фильтрам |
| `Get` | `{ search?, viewOps? }` | `Summary[]` | Список с пагинацией и сортировкой |
| `GetByID` | `{ id }` | `Entity` | Одна запись (полная) |
| `Add` | `{ entity }` | `Entity` | Создание |
| `Update` | `{ entity }` | `boolean` | Обновление |
| `Delete` | `{ id }` | `boolean` | Удаление (soft delete, statusId=3) |
| `Validate` | `{ entity }` | `FieldError[]` | Валидация без сохранения |

### auth

| Метод | Params | Return |
|-------|--------|--------|
| `auth.Login` | `{ login, password, remember }` | `string` (authKey) |
| `auth.Logout` | — | `boolean` |
| `auth.Profile` | — | `IUserProfile` |
| `auth.ChangePassword` | `{ password }` | `string` |

## Роутинг

baseURL: `/vt/`

| URL | Страница | Описание |
|-----|----------|----------|
| `/vt/login` | LoginPage | Авторизация |
| `/vt/` | — | Редирект на `/vt/projects` |
| `/vt/projects` | ProjectsPage | Список проектов |
| `/vt/projects/:id` | ProjectFormPage | Форма проекта (создание/редактирование) |
| `/vt/prompts` | PromptsPage | Список промптов |
| `/vt/prompts/:id` | PromptFormPage | Форма промпта |
| `/vt/task-trackers` | TaskTrackersPage | Список трекеров задач |
| `/vt/task-trackers/:id` | TaskTrackerFormPage | Форма трекера |
| `/vt/slack-channels` | SlackChannelsPage | Список Slack-каналов |
| `/vt/slack-channels/:id` | SlackChannelFormPage | Форма канала |
| `/vt/users` | UsersPage | Список пользователей |
| `/vt/users/:id` | UserFormPage | Форма пользователя |
| `/vt/profile` | ProfilePage | Профиль + смена пароля |

`:id` = `new` для создания, числовой ID для редактирования.

## Layout

Согласован с основным фронтендом (`/reviews/`): верхний header, `max-w-7xl` контейнер, `bg-gray-50` фон.

### Общий layout (авторизованная зона)

- **Header** (`sticky top-0`, `bg-white border-b border-gray-200`):
  - Слева: лого "ReviewSrv" (ссылка на `/vt/projects`) + навигация (табы/ссылки)
  - Справа: имя пользователя, Logout
- **Content** (`max-w-7xl mx-auto`, `py-8`): текущая страница

### Навигация header

Горизонтальные ссылки в header, active-состояние — `border-b-2 border-blue-500 text-blue-600`.

| Пункт | URL |
|-------|-----|
| Projects | `/vt/projects` |
| Prompts | `/vt/prompts` |
| Task Trackers | `/vt/task-trackers` |
| Slack Channels | `/vt/slack-channels` |
| Users | `/vt/users` |

## Страницы-списки (List Pages)

Все списки имеют единообразную структуру:

### Заголовок
- Название сущности (h1)
- Кнопка "Add" (ведет на `/vt/<entity>/new`)

### Фильтры (панель поиска)
- Текстовый поиск (по title / login)
- Фильтр по statusId (select: Все / Опубликован / Не опубликован)
- Кнопка "Reset"

### Таблица
- Сортировка по колонкам (клик по заголовку, sortColumn + sortDesc)
- Пагинация (page, pageSize=25, показываем total из Count)
- Клик по строке — переход на форму редактирования

### По сущностям

#### /vt/projects

Фильтры: title, language, statusId

| Колонка | Поле | Сортировка |
|---------|------|------------|
| ID | id | да |
| Title | title | да |
| Language | language | да |
| VCS URL | vcsURL | да |
| Project Key | projectKey | да |
| Prompt | prompt.title (FK) | нет |
| Task Tracker | taskTracker.title (FK) | нет |
| Slack Channel | slackChannel.title (FK) | нет |
| Status | status.title | да |

#### /vt/prompts

Фильтры: title, statusId

| Колонка | Поле | Сортировка |
|---------|------|------------|
| ID | id | да |
| Title | title | да |
| Status | status.title | да |

#### /vt/task-trackers

Фильтры: title, statusId

| Колонка | Поле | Сортировка |
|---------|------|------------|
| ID | id | да |
| Title | title | да |
| Auth Token | authToken (masked) | да |
| Status | status.title | да |

#### /vt/slack-channels

Фильтры: title, channel, statusId

| Колонка | Поле | Сортировка |
|---------|------|------------|
| ID | id | да |
| Title | title | да |
| Channel | channel | да |
| Webhook URL | webhookURL (masked) | да |
| Status | status.title | да |

#### /vt/users

Фильтры: login, statusId

| Колонка | Поле | Сортировка |
|---------|------|------------|
| ID | id | да |
| Login | login | да |
| Last Activity | lastActivityAt | да |
| Status | status.title | да |

## Страницы-формы (Form Pages)

Единообразная структура:

### Заголовок
- "New <Entity>" или "Edit <Entity> #ID"
- Breadcrumb: `<Entity List> / <Current>`

### Форма
- Поля в соответствии с моделью (см. ниже)
- Валидация по кнопке "Save": сначала `Validate`, если ошибок нет — `Add`/`Update`
- Ошибки валидации отображаются под соответствующими полями (по `field` из `IFieldError`)
- Ошибки сбрасываются при следующем нажатии "Save"
- Кнопки: "Save" (Validate → Add/Update), "Cancel" (возврат к списку)
- Для существующей записи: кнопка "Delete" с confirm-диалогом

### Поля форм

#### Project

| Поле | Тип | Обязательное | Валидация |
|------|-----|-------------|-----------|
| title | input text | да | max 255 |
| vcsURL | input text | да | max 255 |
| language | input text | да | max 32 |
| promptId | select (из prompt.Get) | да | FK prompt |
| taskTrackerId | select (из tasktracker.Get) | нет | FK taskTracker |
| slackChannelId | select (из slackchannel.Get) | нет | FK slackChannel |
| statusId | radio (Опубликован / Не опубликован) | да | — |

projectKey — readonly, генерируется на бэкенде.

#### Prompt

| Поле | Тип | Обязательное | Валидация |
|------|-----|-------------|-----------|
| title | input text | да | max 255 |
| common | textarea | да | — |
| architecture | textarea | да | — |
| code | textarea | да | — |
| security | textarea | да | — |
| tests | textarea | да | — |
| statusId | radio (Опубликован / Не опубликован) | да | — |

#### Task Tracker

| Поле | Тип | Обязательное | Валидация |
|------|-----|-------------|-----------|
| title | input text | да | max 255 |
| authToken | input text | да | max 255 |
| fetchPrompt | textarea | да | — |
| statusId | radio (Опубликован / Не опубликован) | да | — |

#### Slack Channel

| Поле | Тип | Обязательное | Валидация |
|------|-----|-------------|-----------|
| title | input text | да | max 255 |
| channel | input text | да | max 255 |
| webhookURL | input text | да | max 1024 |
| statusId | radio (Опубликован / Не опубликован) | да | — |

#### User

| Поле | Тип | Обязательное | Валидация |
|------|-----|-------------|-----------|
| login | input text | да | max 255 |
| password | input password | да (при создании) | — |
| statusId | radio (Опубликован / Не опубликован) | да | — |

## Авторизация

### Login Page (`/vt/login`)
- Форма: login, password, remember (checkbox)
- `auth.Login` -> сохраняем authKey в localStorage
- Редирект на `/vt/projects`

### Хранение сессии
- authKey в `localStorage`
- При каждом запросе: заголовок `Authorization2: <authKey>`
- При 401 — редирект на `/vt/login`, очистка authKey

### Profile Page (`/vt/profile`)
- Показать login, lastActivityAt
- Форма смены пароля: старый пароль, новый пароль (+ подтверждение)

## Структура файлов

```
frontend/src/
├── api/
│   ├── client.ts               # существующий JSON-RPC клиент
│   ├── factory.ts              # существующий API (review)
│   └── vt.ts                   # новый API-клиент для VT
├── vt/
│   ├── App.vue                 # layout с sidebar
│   ├── router.ts               # vue-router для /vt/
│   ├── composables/
│   │   ├── useAuth.ts          # авторизация, authKey, logout
│   │   ├── useCrud.ts          # общий composable для CRUD-таблиц (count, get, sort, page)
│   │   └── useForm.ts          # общий composable для форм (load, save, validate, delete)
│   ├── components/
│   │   ├── NavHeader.vue       # верхняя навигация + user/logout
│   │   ├── DataTable.vue       # таблица с сортировкой
│   │   ├── Pagination.vue      # пагинация
│   │   ├── SearchBar.vue       # панель фильтров
│   │   ├── FormField.vue       # обертка для полей формы с ошибками
│   │   ├── StatusRadio.vue     # radio для statusId (Опубликован / Не опубликован)
│   │   ├── FKSelect.vue        # select для FK (загружает список через API)
│   │   └── ConfirmDialog.vue   # диалог подтверждения удаления
│   └── pages/
│       ├── LoginPage.vue
│       ├── ProfilePage.vue
│       ├── projects/
│       │   ├── ProjectsPage.vue
│       │   └── ProjectFormPage.vue
│       ├── prompts/
│       │   ├── PromptsPage.vue
│       │   └── PromptFormPage.vue
│       ├── task-trackers/
│       │   ├── TaskTrackersPage.vue
│       │   └── TaskTrackerFormPage.vue
│       ├── slack-channels/
│       │   ├── SlackChannelsPage.vue
│       │   └── SlackChannelFormPage.vue
│       └── users/
│           ├── UsersPage.vue
│           └── UserFormPage.vue
```

## API-клиент (vt.ts)

Создается на основе `api.ts` контракта. JSON-RPC клиент аналогичен существующему `client.ts`, но:
- baseURL: `/v1/vt/` (вместо `/v1/rpc/`)
- заголовок `Authorization2` добавляется из localStorage
- при ошибке 401 — редирект на login

```typescript
// frontend/src/api/vt.ts
import { factory } from './vt-api.generated'  // из api.ts контракта

const vtApi = factory(send)
export default vtApi
```

## Composables

### useCrud(namespace)
Общий composable для всех страниц-списков:

```typescript
// Состояние
const items = ref([])
const total = ref(0)
const viewOps = reactive({ page: 1, pageSize: 25, sortColumn: '', sortDesc: false })
const search = reactive({})
const loading = ref(false)

// Методы
async function load()      // вызывает Count + Get параллельно
function setSort(column)   // переключает sortColumn/sortDesc, перезагружает
function setPage(page)     // меняет page, перезагружает
function setSearch(s)      // обновляет search, сбрасывает page=1, перезагружает
```

### useForm(namespace, id)
Общий composable для всех форм:

```typescript
const entity = ref(null)
const errors = ref<IFieldError[]>([])
const loading = ref(false)
const isNew = computed(() => id === 'new')

async function load()       // GetByID если !isNew
async function save()       // Validate → если ошибок нет → Add/Update
async function remove()     // Delete
```

## Порядок реализации

1. API-клиент (`api/vt.ts`) + скопировать типы из `api.ts`
2. Авторизация (`useAuth`, `LoginPage`)
3. Layout (`App.vue`, `Sidebar`, router)
4. Общие компоненты (`DataTable`, `Pagination`, `SearchBar`, `FormField`, `StatusRadio`, `FKSelect`, `ConfirmDialog`)
5. CRUD-страницы в порядке зависимостей:
   - Prompts (нет FK)
   - Task Trackers (нет FK)
   - Slack Channels (нет FK)
   - Projects (FK на Prompt, TaskTracker, SlackChannel)
   - Users
6. Profile + смена пароля
