# reviewctl

Go CLI orchestrator for AI code review. Replaces bash + Node.js (`upload.cjs`) pipeline with a single binary.

## Subcommands

| Command | Description |
|---------|-------------|
| `reviewctl review` | Full cycle: fetch prompt → Claude → parse → upload → MR comment → HTML |
| `reviewctl upload` | Upload local `review.json` + `R*.md` to server |
| `reviewctl comment` | Post MR comments for an existing review |
| `reviewctl version` | Print version |

## Flags & Environment Variables

All flags have environment variable defaults for backward compatibility with CI.

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `--key` | `$PROJECT_KEY` | *required* | Project key (UUID) |
| `--url` | `$REVIEWSRV_URL` | *required* | Reviewer server URL |
| `--model` | `$REVIEW_MODEL` | `opus` | Claude model |
| `--dir` | `$REVIEW_DIR` | `.` | Working directory with review files |
| `--verbose` | `$REVIEW_VERBOSE` | `false` | Verbose output |
| `--session` | — | — | Claude session ID for `--resume` (reuses prompt cache) |
| `--gitlab-url` | `$CI_API_V4_URL` | — | GitLab API URL |
| `--gitlab-token` | `$REVIEWER_GITLAB_TOKEN` | — | GitLab API token for MR comments |
| `--mr-iid` | `$CI_MERGE_REQUEST_IID` | — | Merge Request IID |
| `--project-id` | `$CI_PROJECT_ID` | — | GitLab project ID |
| `--source-branch` | `$CI_MERGE_REQUEST_SOURCE_BRANCH_NAME` | — | Source branch |
| `--target-branch` | `$CI_MERGE_REQUEST_TARGET_BRANCH_NAME` | — | Target branch |
| `--commit` | `$CI_COMMIT_SHA` | — | Commit SHA |
| `--author` | `$GITLAB_USER_LOGIN` | — | MR author |
| `--mr-title` | `$CI_MERGE_REQUEST_TITLE` | — | MR title |
| `--external-id` | `$CI_MERGE_REQUEST_IID` | — | External ID |
| `--diff-base-sha` | `$CI_MERGE_REQUEST_DIFF_BASE_SHA` | — | Diff base SHA for inline comments |
| `--review-id` | — | — | Existing review ID (for `comment` subcommand) |

## Usage

### CI (GitLab)

```yaml
review:
  stage: review
  image: vmkteam/claude-ci:latest
  script:
    - reviewctl review
  artifacts:
    paths:
      - review.html
    expire_in: 30 days
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

Required CI variables: `PROJECT_KEY`, `REVIEWSRV_URL`, `ANTHROPIC_API_KEY`, `REVIEWER_GITLAB_TOKEN`.

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
