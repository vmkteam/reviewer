# AI Code Review: Object Model

## 0. Status

Reference table for soft-delete across all entities.

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | statusId | int4, PK | Status identifier |
| 2 | title | varchar(255) | Status name (e.g. `active`, `deleted`) |

## 1. Prompt

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | promptId | int4, PK, identity | |
| 2 | title | varchar(255) | Prompt name (e.g. "Go strict reviewer v2") |
| 3 | common | text | Common system prompt for LLM |
| 4 | architecture | text | Sub-prompt for architecture review |
| 5 | code | text | Sub-prompt for code review |
| 6 | security | text | Sub-prompt for security review |
| 7 | tests | text | Sub-prompt for tests review |
| 8 | createdAt | timestamptz | |
| 9 | statusId | int4, FK → statuses | Soft-delete |

## 2. Task Tracker

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | taskTrackerId | int4, PK, identity | |
| 2 | title | varchar(255) | Tracker name (e.g. "Jira", "YouTrack") |
| 3 | authToken | varchar(255) | API token |
| 4 | fetchPrompt | text | Prompt to extract task ID from branch/MR via API |
| 5 | createdAt | timestamptz | |
| 6 | statusId | int4, FK → statuses | Soft-delete |

## 3. Slack Channel

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | slackChannelId | int4, PK, identity | |
| 2 | title | varchar(255) | Display name |
| 3 | channel | varchar(255) | Slack channel name |
| 4 | webhookURL | varchar(1024) | Slack incoming webhook URL |
| 5 | statusId | int4, FK → statuses | Soft-delete |

## 4. Project

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | projectId | int4, PK, identity | |
| 2 | title | varchar(255) | Project name |
| 3 | vcsURL | varchar(255) | VCS HTTPS URL, e.g. `https://gitlab.company.com/group/project` |
| 4 | language | varchar(32) | Primary language (Go, TypeScript, Python, ...) |
| 5 | projectKey | uuid | API key for CI integration |
| 6 | promptId | int4, FK → prompts | |
| 7 | taskTrackerId | int4, FK → taskTrackers | |
| 8 | slackChannelId | int4, FK → slackChannels | |
| 9 | createdAt | timestamptz | |
| 10 | statusId | int4, FK → statuses | Soft-delete |

## 5. Review

One LLM call per review. Produces multiple review files + JSON with metadata.

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | reviewId | int4, PK, identity | |
| 2 | projectId | int4, FK → projects | |
| 3 | externalId | varchar(32) | External MR/PR identifier |
| 4 | title | varchar(255) | MR/PR title |
| 5 | description | varchar(2048) | MR/PR description or summary |
| 6 | commitHash | varchar(40) | Reviewed commit SHA |
| 7 | sourceBranch | varchar(255) | |
| 8 | targetBranch | varchar(255) | |
| 9 | author | varchar(255) | MR author |
| 10 | createdAt | timestamptz | Review creation time |
| 11 | durationMS | int4, DEFAULT 0 | Total LLM call duration in milliseconds |
| 12 | modelInfo | jsonb, DEFAULT '{}' | LLM model info: `{"model": "claude-opus-4-6", "inputTokens": 0, "outputTokens": 0, "costUsd": 0.00}` |
| 13 | trafficLight | varchar(32), DEFAULT none | `none` / `red` / `yellow` / `green` — calculated by server from review files |
| 14 | promptId | int4, FK → prompts | Snapshot of the prompt used |
| 15 | statusId | int4, FK → statuses | Soft-delete |

### Traffic Light Rules

| Color | Condition |
|-------|-----------|
| Red | Any CRITICAL or >= 2 HIGH |
| Yellow | Any HIGH or >= 3 MEDIUM |
| Green | Only LOW/MEDIUM (< 3) or no issues |

## 6. Review File

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | reviewFileId | int4, PK, identity | |
| 2 | reviewId | int4, FK → reviews | |
| 3 | reviewType | varchar(64) | `architecture` / `code` / `security` / `tests` |
| 4 | content | text | Full markdown review |
| 5 | issueStats | jsonb, DEFAULT '{}' | `{"critical": 0, "high": 1, "medium": 3, "low": 2}` — calculated by server from issues |
| 6 | trafficLight | varchar(32) | `red` / `yellow` / `green` — calculated by server from issues stats |
| 7 | summary | varchar(2048) | Short conclusion |
| 8 | isAccepted | bool, DEFAULT false | Accepted by developer |
| 9 | createdAt | timestamptz | |
| 10 | statusId | int4, FK → statuses | Soft-delete |

## 7. Issue

| # | Field | Type | Description |
|---|-------|------|-------------|
| 1 | issueId | int4, PK, identity | |
| 2 | reviewFileId | int4, FK → reviewFiles | |
| 3 | reviewId | int4, FK → reviews | Denormalized for fast queries |
| 4 | issueType | varchar(32) | Extensible: `nil-check`, `error-handling`, `tests`, `naming`, `duplication`, `security`, `perf`, `architecture`, `logging`, `concurrency` |
| 5 | severity | varchar(16) | `critical` / `high` / `medium` / `low` |
| 6 | title | varchar(255) | Issue headline |
| 7 | description | text | Short description (1-2 sentences) |
| 8 | content | text | Full description with markdown formatting from review file |
| 9 | file | varchar(255) | Affected file path |
| 10 | lines | varchar(255) | Line range (e.g. `121-156`) |
| 11 | isFalsePositive | bool, nullable | Developer marks issue as false positive |
| 12 | comment | varchar(255), nullable | Developer comment on the issue |
| 13 | processedAt | timestamptz, nullable | When developer processed the issue |
| 14 | createdAt | timestamptz | |
| 15 | statusId | int4, FK → statuses | Soft-delete |

## CI Pipeline Flow

TODO

## Slack Notification Format

```
🟡 AI Code Review: INT-3341 "Учитывать режим работы аптеки"
Project: reviewsrv | Author: @sergeyfast | Branch: INT-3341 → devel

📐 Architecture: 🟢 no issues
💻 Code: 🟡 1 HIGH, 3 MEDIUM, 2 LOW
🔒 Security: 🟢 no issues
🧪 Tests: 🟡 2 MEDIUM

Top issues:
  • HIGH: ClosesAt возвращает прошедшее время (working_hours.go:121)
  • MEDIUM: Игнорирование перерывов в isOpenAt (working_hours.go:252)

Cost: $0.12 | Tokens: 45K in / 8K out | Duration: 32s
```
