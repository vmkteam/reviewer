# reviewer

AI-powered code review platform using Claude. Collects, stores and displays code review results from CI pipelines.

## Features

- **Multi-project support** with configurable prompts per project
- **5 review types**: architecture, code, security, tests, operability
- **Severity levels**: critical, high, medium, low with traffic light system (red/yellow/green)
- **reviewctl CLI** — single binary for the full review cycle: prompt fetch, Claude Code, upload, GitLab MR comments, HTML report
- **GitLab MR inline comments** — critical and high issues posted directly in the diff with cleanup on re-runs
- **Session caching** — `--session`/`--continue` flags to reuse Claude prompt cache (~90% token savings)
- **Auto-migrations** — pgmigrator integrated as Go library, runs SQL patches on server startup
- **GitLab CI integration** via generated CI component and Docker image
- **Slack notifications** for completed reviews
- **VT admin panel** for managing projects, prompts, users, and Slack channels
- **REST + JSON-RPC API** with auto-generated TypeScript clients and OpenRPC schema

## Architecture

```
GitLab CI (merge request)
  -> reviewctl review
       -> fetch prompt from reviewer (/v1/prompt/$PROJECT_KEY/)
       -> claude --print --output-format json -p "$PROMPT"
       -> parse review.json + R*.md files
       -> upload to reviewer (/v1/upload/$PROJECT_KEY/)
       -> post GitLab MR comments (summary + inline issues)
       -> generate HTML report
  -> reviewer server stores results in PostgreSQL
  -> frontend displays reviews / Slack notification sent
```

## Prerequisites

- Go 1.25+
- PostgreSQL
- Node.js 20+ (for frontend build)

## Quick Start

```bash
# 1. Initialize config files
make init

# 2. Edit configuration
#    Set database credentials in Makefile.mk and cfg/local.toml

# 3. Create and seed database
make db

# 4. Install frontend dependencies and build
make frontend-install
make frontend-build

# 5. Run the server
make run
```

Default admin credentials: `admin` / `12345`

The server starts at `http://localhost:8075`. The review UI is available at `/reviews/`, the admin panel at `/vt/`.

## Docker Compose

Run locally with Docker Compose — builds the image from source and initializes the database automatically:

```bash
docker compose up -d
```

This starts PostgreSQL (exposed on port `6432`) and the reviewer app on `http://localhost:8080`. The database schema and seed data (`docs/reviewsrv.sql`, `docs/init.sql`) are applied on first run.

To rebuild the image after code changes:

```bash
docker compose up -d --build
```

To stop and remove containers (add `-v` to also remove the database volume):

```bash
docker compose down
```

## Configuration

Configuration file: `cfg/local.toml`

```toml
[Server]
Host    = "localhost"
Port    = 8075
IsDevel = true
BaseURL = "http://localhost:8075"

[Database]
Addr     = "localhost:5432"
User     = "postgres"
Database = "reviewsrv"
Password = ""
PoolSize = 5

[Sentry]
DSN         = ""
Environment = ""
```

## API Endpoints

### REST

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/prompt/:projectKey/` | Get review prompt for a project |
| POST | `/v1/upload/:projectKey/` | Create a new review |
| POST | `/v1/upload/:projectKey/:reviewId/:reviewType/` | Upload a review file |

### JSON-RPC

| Path | Description |
|------|-------------|
| `/v1/rpc/` | Review API (projects, reviews, issues, feedback) |
| `/v1/rpc/doc/` | Review API documentation (SMDBox) |
| `/v1/vt/` | Admin API (users, projects, prompts, Slack channels, task trackers) |
| `/v1/vt/doc/` | Admin API documentation (SMDBox) |

TypeScript clients are auto-generated at `/v1/rpc/api.ts` and `/v1/vt/api.ts`.

## Review Types and Severity

**Review types:** `architecture`, `code`, `security`, `tests`, `operability`

**Severity levels:** `critical`, `high`, `medium`, `low`

**Traffic light system:**
- Red: 1+ critical OR 2+ high issues
- Yellow: 1+ high OR 3+ medium issues
- Green: all other cases

## reviewctl

`reviewctl` is a Go CLI that replaces the old bash + Node.js CI scripts with a single binary.

```bash
reviewctl review    # Full cycle: prompt -> Claude -> upload -> GitLab comments -> HTML
reviewctl upload    # Upload local review.json + R*.md to server
reviewctl comment   # Post MR comments for an existing review
reviewctl version   # Print version
```

Key flags: `--key`, `--url`, `--model`, `--session` (prompt cache reuse), `--continue` (resume last session). All flags have env variable equivalents for CI. See `reviewctl --help` for details.

```bash
make build-reviewctl   # Build reviewctl binary
```

## Auto-migrations

The server can apply SQL patches automatically on startup using pgmigrator (integrated as Go library):

```bash
reviewsrv -config config.toml -patches /patches
```

Patches are stored in `docs/patches/*.sql` with `YYYY-MM-DD-description.sql` naming. The Docker image includes patches at `/patches/`. Docker Compose runs with `--patches` by default.

## CI Integration

The admin panel (`/vt/`) provides ready-to-use CI configuration:

1. Open **Projects** and click the **CI** button in the page header — it shows the Dockerfile and GitLab CI YAML.
2. Build the Docker image from the provided Dockerfile.
3. Add CI/CD variables to your GitLab project:
   - `PROJECT_KEY` — project key from reviewer
   - `ANTHROPIC_API_KEY` — Claude API key
   - `REVIEWER_GITLAB_TOKEN` — GitLab token for MR comments (optional)
4. Paste the generated YAML into your repository's `.gitlab-ci.yml`.

The CI job runs `reviewctl review` on merge requests. It fetches the prompt, runs Claude Code review, uploads results, and posts inline comments to the MR.

For local runs, click the **Run** button on a specific project row to get a ready-to-use bash script.

## Administration

### URL Access Control

When deploying behind a reverse proxy, URLs should be split by access level:

**Public (available within the closed network):**

| Path | Description |
|------|-------------|
| `/reviews/` | Review results UI |
| `/vt/` | Admin panel |
| `/v1/rpc/` | Review JSON-RPC API |
| `/v1/vt/` | Admin JSON-RPC API |

**Internal (CI only, must not be exposed externally):**

| Path | Description |
|------|-------------|
| `/v1/upload/` | Review upload endpoint |
| `/v1/prompt/` | Prompt fetch endpoint |

Example nginx configuration:

```nginx
# Public URLs — accessible within the closed network
location /reviews/ { proxy_pass http://reviewer:8075; }
location /vt/       { proxy_pass http://reviewer:8075; }
location /v1/rpc/   { proxy_pass http://reviewer:8075; }
location /v1/vt/    { proxy_pass http://reviewer:8075; }

# Internal URLs — accessible only from CI runners
location /v1/upload/ { deny all; }
location /v1/prompt/ { deny all; }
```

## Development

```bash
make run              # Run server in dev mode
make build            # Build server binary
make build-reviewctl  # Build reviewctl CLI
make frontend-dev     # Run frontend dev server (Vite)
make frontend-build   # Build frontend (main + admin)
make generate         # Generate RPC/VT code
make lint             # Run golangci-lint
make test             # Run tests with coverage
make tools            # Install dev tools
make mod              # Tidy and vendor Go modules
```

## Tech Stack

**Backend:** Go, Echo, zenrpc, go-pg, Prometheus, Sentry

**Frontend:** Vue 3, TypeScript, Tailwind CSS, Vite, Headless UI

## License

MIT
