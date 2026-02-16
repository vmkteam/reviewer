
# reviewsrv

Создать хорошую интеграцию в CI для трекинга ревью в разных проектах с разными промтами и хорошей нотификацией в слак, светофором ревью и разработчиков со статистикой

# Концепция

1. запускаем в claude-code c промтом. На выходе ревью файлы и данные json.
2. В JSON уже есть данные по issues из MD файлов (генерит LLM)
3. Данные отправляются на сервер

## Соглашения

- JSON: `lowerCamelCase` для всех ключей (`trafficLight`, `issuesStats`, `fileType`, ...)

## Файлы
* [reviewsrv.sql](../reviewsrv.sql) — БД-схема (источник истины)
* ObjectModel.md — объектная модель (RU)
* Model.md — детальная объектная модель (EN)
* Prompt.md — промт для LLM
* [CI.md](CI.md) — план CI-интеграции (GitLab CI кнопка в VT)

## TypeScript API-клиенты

Оба фронтенда используют сгенерированные API-клиенты с враппером.

### Схема

```
*.generated.ts  ← curl с сервера (не редактируется)
*.ts            ← враппер: адаптирует сигнатуры и реэкспортирует типы
```

### Обновление

```bash
# Review API (основной фронтенд)
{ echo '// @ts-nocheck'; curl -sf http://localhost:8075/v1/rpc/api.ts; } > frontend/src/api/factory.generated.ts

# VT API (админка)
{ echo '// @ts-nocheck'; curl -sf http://localhost:8075/v1/vt/api.ts; } > frontend/src/api/vt.generated.ts
```

### Файлы

| Сгенерированный | Враппер | RPC-клиент | Эндпоинт |
|---|---|---|---|
| `factory.generated.ts` | `factory.ts` | `client.ts` (`/v1/rpc/`) | `/v1/rpc/api.ts` |
| `vt.generated.ts` | `vt.ts` | `vtClient.ts` (`/v1/vt/`) | `/v1/vt/api.ts` |

Враппер адаптирует:
- Сигнатуры методов: `getByID({id})` → `getByID(id)` (плоские аргументы для composables)
- Неймспейсы: `slackchannel` → `slackChannel`, `tasktracker` → `taskTracker`
- Типы: реэкспорт `IProject` → `Project`, `IFieldError` → `FieldError` и т.д.


# Загрузка
* review.json загружаем через curl по адресу /v1/upload/<projectKey>/
  * возвращается reviewId в сыром виде (подставляется дальше)
  * 200 – успешно
  * 404 - project key не найден
  * 400 - ошибка входных данных
  * 500 - ошибка сервера
* все остальные файлы загружаем через curl как /v1/upload/<projectKey>/<reviewId>/<reviewType>/
  * 200 – успешно
  * 404 - project key не найден
  * 400 - ошибка входных данных
  * 500 - ошибка сервера
* получить prompt /v1/prompt/<projectKey>/
  * 200 – успешно
  * 404 - project key не найден
  * 500 - ошибка сервера

### Примеры

#### 1. Загрузка review.json

```bash
PROJECT_KEY="93b90214-3b5d-4fa6-b497-f064ff7bf8a9"
REVIEW_ID=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -d @review.json \
  http://localhost:8075/v1/upload/${PROJECT_KEY}/)
```

#### 2. Загрузка файлов ревью

```bash
curl -s -X POST \
  -H "Content-Type: application/octet-stream" \
  --data-binary @architecture.md \
  http://localhost:8075/v1/upload/${PROJECT_KEY}/${REVIEW_ID}/architecture/

curl -s -X POST \
  -H "Content-Type: application/octet-stream" \
  --data-binary @code.md \
  http://localhost:8075/v1/upload/${PROJECT_KEY}/${REVIEW_ID}/code/

curl -s -X POST \
  -H "Content-Type: application/octet-stream" \
  --data-binary @security.md \
  http://localhost:8075/v1/upload/${PROJECT_KEY}/${REVIEW_ID}/security/

curl -s -X POST \
  -H "Content-Type: application/octet-stream" \
  --data-binary @tests.md \
  http://localhost:8075/v1/upload/${PROJECT_KEY}/${REVIEW_ID}/tests/
```

#### 3. CI скрипт (Node.js)

```js
const fs = require("fs");
const path = require("path");

const BASE_URL = process.env.REVIEWSRV_URL || "http://localhost:8075";
const PROJECT_KEY = process.env.PROJECT_KEY;
const DIR = process.env.REVIEW_DIR || ".";

const TYPES = { R1: "architecture", R2: "code", R3: "security", R4: "tests" };

async function upload(url, body, contentType = "application/octet-stream") {
  const res = await fetch(url, { method: "POST", body, headers: { "Content-Type": contentType } });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${text}`);
  }
  return text;
}

async function main() {
  const reviewJSON = fs.readFileSync(path.join(DIR, "review.json"));
  const reviewId = await upload(`${BASE_URL}/v1/upload/${PROJECT_KEY}/`, reviewJSON, "application/json");
  console.log(`reviewId=${reviewId}`);

  const files = fs.readdirSync(DIR);
  for (const [prefix, type] of Object.entries(TYPES)) {
    const file = files.find((f) => f.startsWith(prefix + ".") && f.endsWith(".md"));
    if (!file) continue;
    const content = fs.readFileSync(path.join(DIR, file));
    await upload(`${BASE_URL}/v1/upload/${PROJECT_KEY}/${reviewId}/${type}/`, content);
    console.log(`uploaded ${file}`);
  }
}

main().catch((e) => { console.error(e.message); process.exit(1); });
```