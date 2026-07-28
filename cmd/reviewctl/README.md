# reviewctl

Go CLI orchestrator for AI code review. Single binary for the full review cycle: prompt → Claude → upload → MR comments → HTML.

## Subcommands

| Command | Description |
|---------|-------------|
| `reviewctl review` | Full cycle: fetch prompt → Claude → parse → upload → MR comment → HTML |
| `reviewctl upload` | Upload local `review.json` + `R*.md` to server |
| `reviewctl comment` | Post MR comments for an existing review |
| `reviewctl version` | Print version |

## Runner configuration

The runner (runner type, model, effort, provider, optional fallback token) comes
from the project's **runner profile**, fetched from the server at run time — CI
passes only the project key, server URL and credentials, so it stays thin. The
runner flags below are **local overrides**: an explicit flag always wins over the
profile. Multi-review is profile-driven too (see below).

## Flags & Environment Variables

Most flags have an environment-variable default for CI.

### Identity, server & operation

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `--key` | `$PROJECT_KEY` | *required* | Project key (UUID) |
| `--url` | `$REVIEWSRV_URL` | *required* | Reviewer server URL used for API calls from CI |
| `--public-url` | `$REVIEWSRV_PUBLIC_URL` | *falls back to `--url`* | Browser-facing base URL used in MR comment links |
| `--dir` | `$REVIEW_DIR` | `.` | Working directory with review files |
| `--verbose` | `$REVIEW_VERBOSE` | `false` | Verbose output |
| `--session` | — | — | Claude session ID for `--resume` (reuses prompt cache) |
| `--continue` | — | `false` | Continue the last Claude session instead of `--resume` |
| `--debug-upload` | `$REVIEW_DEBUG_UPLOAD` | `false` | Always upload artifacts to `/v1/upload/debug/` (failures upload regardless) |
| `--timeout` | `$REVIEW_TIMEOUT` | `30m` | Per-run timeout (and per panel member/judge); `0` = no timeout |

### Runner overrides (otherwise taken from the runner profile)

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `--runner` | `$REVIEW_RUNNER` | `claude` | Runner: `claude` \| `opencode` \| `codex` \| `direct` |
| `--model` | `$REVIEW_MODEL` | *runner default* | Model name |
| `--effort` | `$REVIEW_EFFORT` | — | Reasoning effort for the `direct` Anthropic runner: `low`..`max` |
| `--api-provider` | `$REVIEW_API_PROVIDER` | `deepseek` | `direct` runner provider: `deepseek` \| `openai-compat` \| `anthropic` |
| `--api-base-url` | `$REVIEW_API_BASE_URL` | *provider default* | `direct` runner API base URL |
| `--allow-dangerous-permissions` | `$REVIEW_ALLOW_DANGEROUS_PERMISSIONS` | `true` | Pass `--dangerously-skip-permissions` to opencode (needed for unattended CI) |

### MR & CI metadata (from GitLab CI)

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `--gitlab-url` | `$CI_API_V4_URL` | — | GitLab API URL |
| `--gitlab-token` | `$REVIEWER_GITLAB_TOKEN` | — | GitLab API token for MR comments |
| `--mr-iid` | `$CI_MERGE_REQUEST_IID` | — | Merge Request IID |
| `--project-id` | `$CI_PROJECT_ID` | — | GitLab project ID |
| `--source-branch` | `$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME` | — | Source branch |
| `--target-branch` | `$CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | — | Target branch |
| `--commit` | `$CI_COMMIT_SHA` | — | Commit SHA |
| `--author` | `$CI_COMMIT_AUTHOR` | *falls back to `$GITLAB_USER_LOGIN`* | MR author (email stripped) |
| `--mr-title` | `$CI_MERGE_REQUEST_TITLE` | — | MR title |
| `--external-id` | `$CI_MERGE_REQUEST_IID` | — | External ID |
| `--diff-base-sha` | `$CI_MERGE_REQUEST_DIFF_BASE_SHA` | — | Diff base SHA for inline comments |
| `--review-id` | — | — | Existing review ID (for `comment` subcommand) |

## Multi-review (panel + fusion)

When a project's runner profile config attaches **panel members** and a **judge**,
`reviewctl review` fans out: it runs each member in its own detached git worktree
(in parallel, fault-tolerant — one survivor is enough), then a judge runner fuses
the member reviews into one, keeping the individual member reviews linked. The
panel and judge are configured per project in the admin panel; CI runs the same
`reviewctl review` either way. Direct credentials for each member come from its
profile token (or the ambient env). The cost is roughly the sum of the members
plus the judge, so it is opt-in per project.

## Usage

### CI (GitLab)

```yaml
review:
  stage: review
  image: your-registry/reviewer-ci:latest  # vmkteam/claude-ci + reviewctl
  script:
    - reviewctl review
  artifacts:
    paths:
      - review.html
    expire_in: 30 days
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

Required CI variables: `PROJECT_KEY`, `REVIEWSRV_URL`, `REVIEWER_GITLAB_TOKEN`. The LLM
API key is optional — the runner profile supplies the runner, model and an optional token;
set `REVIEW_API_KEY` (or `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `DEEPSEEK_API_KEY`) only
when the profile has no token.

### Local Run

```bash
export PROJECT_KEY="your-project-uuid"
export REVIEWSRV_URL="https://reviewer.example.com"

# Full review
reviewctl review

# Resume previous session (reuses prompt cache, ~90% cheaper)
reviewctl review --session <session-id>

# Upload only (after manual Claude run)
reviewctl upload

# Post MR comments only
reviewctl comment --review-id 42
```

## Output Files

| File | Description |
|------|-------------|
| `review.json` | Structured review data (created by Claude) |
| `R1.*.md` — `R5.*.md` | Review files: architecture, code, security, tests, operability |
| `review.html` | HTML artifact with syntax highlighting and mermaid diagrams |
| `claude-output.json` | Raw Claude CLI output for diagnostics |

## GitLab MR Comments

When `$REVIEWER_GITLAB_TOKEN` is set, reviewctl posts:

1. **Summary comment** — traffic light, cost, duration, per-type stats, link to full review
2. **Inline comments** — critical issues as discussions on specific lines with suggested fixes (falls back to plain notes if line is outside diff)

### Token Setup

#### Phase 1: Read-only review (current)

Create a **Project Access Token** (recommended) or Group Access Token:

- **Role:** Developer (minimum for MR comments)
- **Scope:** `api`
- **Path:** Settings → Access Tokens in the GitLab project

Add as CI/CD variable:

- **Key:** `REVIEWER_GITLAB_TOKEN`
- **Flags:** Protected, Masked

#### Phase 2: Interactive auto-fix (future)

When reviewctl gains the ability to commit suggested fixes:

1. Create a dedicated GitLab user (e.g. `reviewer-bot`)
2. **Personal Access Token** of this bot user with scopes: `api`, `write_repository`
3. **Role:** Developer on the project (pushes to MR source branch, never to protected branches)
4. Commits appear as `reviewer-bot` in git blame — clearly distinguishable from human commits

## Build

```bash
make build-reviewctl          # builds bin/reviewctl
go test ./pkg/reviewer/ctl/... # run tests
```

## Docker CI Image

```dockerfile
FROM vmkteam/reviewer:latest AS source

FROM node:20-alpine
RUN apk add --no-cache git bash curl
RUN npm install -g @anthropic-ai/claude-code
COPY --from=source /reviewctl /usr/local/bin/reviewctl
WORKDIR /workspace
```
