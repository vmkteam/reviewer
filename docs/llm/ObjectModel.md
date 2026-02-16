# AI Code Review: Объектная модель

## 0. Статус

Справочник для soft-delete во всех сущностях.

1. statusId (PK)
2. Название

## 1. Промт

1. promptId (PK, identity)
2. Название (title)
3. Системный промт (common, текст)
4. Промт Архитектура (architecture, текст)
5. Промт Код (code, текст)
6. Промт Безопасность (security, текст)
7. Промт Тесты (tests, текст)
8. Дата создания (createdAt)
9. Статус (statusId, FK → statuses)

## 2. Таск трекер

1. taskTrackerId (PK, identity)
2. Название (title)
3. Токен (authToken)
4. Промт для получения номера задачи по API (fetchPrompt)
5. Дата создания (createdAt)
6. Статус (statusId, FK → statuses)

## 3. Slack канал

1. slackChannelId (PK, identity)
2. Название (title)
3. Канал (channel)
4. Webhook URL (webhookURL)
5. Статус (statusId, FK → statuses)

## 4. Проект

1. projectId (PK, identity)
2. Название (title)
3. VCS URL (vcsURL) — HTTPS URL до репозитория
4. Язык (language: Go, TypeScript, Python, ...)
5. Ключ проекта (projectKey, uuid) — API-ключ для CI
6. Промт (promptId, FK → prompts)
7. Таск трекер (taskTrackerId, FK → taskTrackers)
8. Slack канал (slackChannelId, FK → slackChannels)
9. Дата создания (createdAt)
10. Статус (statusId, FK → statuses)

## 5. Ревью

Один вызов LLM на ревью. На выходе — несколько md-файлов + review.json с метаданными.

1. reviewId (PK, identity)
2. Проект (projectId, FK → projects)
3. Внешний ID (externalId, varchar 32) — ID MR/PR
4. Заголовок (title)
5. Описание (description, varchar 2048) — описание/summary MR
6. Хеш коммита (commitHash)
7. Source Branch (sourceBranch)
8. Target Branch (targetBranch)
9. Автор (author)
10. Дата создания (createdAt)
11. Длительность (durationMS, int4) — в миллисекундах
12. Информация о модели (modelInfo, jsonb) — модель, токены, стоимость
13. Светофор (trafficLight, varchar 32, default 'none') — вычисляется на сервере
14. Промт (promptId, FK → prompts) — снимок промта
15. Статус (statusId, FK → statuses)

## 6. Ревью файл

1. reviewFileId (PK, identity)
2. Ревью (reviewId, FK → reviews)
3. Тип ревью (reviewType, varchar 64: architecture / code / security / tests)
4. Контент (content, markdown)
5. Статистика issues (issueStats, jsonb: critical / high / medium / low) — вычисляется на сервере
6. Светофор (trafficLight, varchar 32) — вычисляется на сервере
7. Краткий вывод (summary)
8. Принято (isAccepted, bool, default false)
9. Дата создания (createdAt)
10. Статус (statusId, FK → statuses)

## 7. Замечание

1. issueId (PK, identity)
2. Ревью файл (reviewFileId, FK → reviewFiles)
3. Ревью (reviewId, FK → reviews) — денормализация
4. Тип замечания (issueType, varchar 32: nil-check, error-handling, tests, naming, duplication, security, perf, architecture, logging, concurrency)
5. Критичность (severity: critical / high / medium / low)
6. Заголовок (title)
7. Краткое описание (description, 1-2 предложения)
8. Полное описание (content, markdown из ревью файла)
9. Файл (file)
10. Строки (lines, например 121-156)
11. Ложное срабатывание (isFalsePositive, bool, nullable)
12. Комментарий разработчика (comment, varchar 255, nullable)
13. Дата обработки (processedAt, nullable)
14. Дата создания (createdAt)
15. Статус (statusId, FK → statuses)
