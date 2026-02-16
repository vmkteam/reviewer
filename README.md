# reviewer

AI-powered code review platform using Claude. Collects, stores and displays code review results from CI pipelines.

## Features

- **Multi-project support** with configurable prompts per project
- **4 review types**: architecture, code, security, tests
- **Severity levels**: critical, high, medium, low with traffic light system (red/yellow/green)
- **GitLab CI integration** via generated CI component and Docker image
- **Slack notifications** for completed reviews
- **VT admin panel** for managing projects, prompts, users, and Slack channels
- **REST + JSON-RPC API** with auto-generated TypeScript clients and OpenRPC schema

## Architecture

```
GitLab CI (merge request)
  -> fetch prompt from reviewer (/v1/prompt/$PROJECT_KEY/)
  -> claude-code runs review with the prompt
  -> results uploaded to reviewer (/v1/upload/$PROJECT_KEY/)
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
| GET | `/v1/upload/upload.js` | Get the upload script for CI |

### JSON-RPC

| Path | Description |
|------|-------------|
| `/v1/rpc/` | Review API (projects, reviews, issues, feedback) |
| `/v1/rpc/doc/` | Review API documentation (SMDBox) |
| `/v1/vt/` | Admin API (users, projects, prompts, Slack channels, task trackers) |
| `/v1/vt/doc/` | Admin API documentation (SMDBox) |

TypeScript clients are auto-generated at `/v1/rpc/api.ts` and `/v1/vt/api.ts`.

## Review Types and Severity

**Review types:** `architecture`, `code`, `security`, `tests`

**Severity levels:** `critical`, `high`, `medium`, `low`

**Traffic light system:**
- Red: 1+ critical OR 2+ high issues
- Yellow: 1+ high OR 3+ medium issues
- Green: all other cases

## CI Integration

The admin panel generates a ready-to-use GitLab CI configuration for each project.

### Setup

1. Build the Docker image with Claude Code:

```dockerfile
FROM node:20-alpine
RUN apk add git bash curl
WORKDIR /app
RUN npm install -g @anthropic-ai/claude-code
RUN npm install -g marked

# Claude Code default settings
RUN mkdir -p /root/.claude && cat > /root/.claude/settings.json <<'EOF'
{
  "enabledPlugins": {
    "gopls-lsp@claude-plugins-official": true,
    "swift-lsp@claude-plugins-official": true
  },
  "attribution": {
    "commit": "",
    "pr": ""
  },
  "includeCoAuthoredBy": false,
  "permissions": {
    "deny": [
      "Read(**/.env)",
      "Bash(sudo:*)",
      "Bash(su:*)",
      "Bash(ssh:*)"
    ]
  },
  "language": "Russian",
  "autoUpdatesChannel": "latest",
  "gitAttribution": false,
  "env": {
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1,
    "DISABLE_TELEMETRY": 1,
    "DISABLE_ERROR_REPORTING": 1
  }
}
EOF

CMD ["claude-code"]
```

2. In the admin panel (`/vt/`), open the project list and click the **CI** button to get the generated `.gitlab-ci.yml` fragment.

3. Add the following CI/CD variables to your GitLab project:
   - `PROJECT_KEY` -- project key from reviewer
   - `ANTHROPIC_API_KEY` -- Claude API key

4. Paste the generated YAML into your repository's `.gitlab-ci.yml`.

The CI job runs on merge requests as a manual step. It fetches the prompt, runs Claude Code review, and uploads results back to the reviewer server.

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
make build            # Build binary
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
