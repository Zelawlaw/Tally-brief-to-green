# DISCOVERY.md — Tally

Gate 0 deliverable. Answers with evidence, not assumption.

## 1. Toolchain

| Tool | Version | Path | Notes |
|---|---|---|---|
| Go | 1.26.3 | `go` | `go version go1.26.3 darwin/arm64` |
| Docker | 29.3.0 | `docker` | `Docker version 29.3.0` |
| Docker Compose | v5.1.2 | `docker compose` | Plugin, not standalone binary |
| golangci-lint | 2.12.2 | `golangci-lint` | Installed via Homebrew (was absent initially, now present) |
| sonar-scanner | 8.1.0.6389 | `/opt/homebrew/bin/sonar-scanner` | Already present via Homebrew |

All required tools are available. `golangci-lint` was the only missing piece and was installed via `brew install golangci-lint` before any application code was written.

## 2. Gate Harness

### Makefile targets (topological order)

| Target | What it does | Depends on |
|---|---|---|
| `build` | `go vet ./...` then `go build -o tally ./cmd/tally` | Go source |
| `unit` | `go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic` | Go source |
| `functional` | `go test ./... -tags=functional -count=1` | App running via `docker compose up` |
| `lint` | `golangci-lint run ./...` | `.golangci.yml` config |
| `sonar` | Runs `sonar-scanner` against `localhost:9000`, then polls quality gate | SonarQube running, `coverage.out` from `unit` |
| `gates` | `build → unit → functional → lint → sonar` (sequential, stops at first failure) | All of the above |
| `dev-up` | Starts SonarQube container, waits for it, bootstraps the Standard Gate | Docker |
| `dev-down` | `docker compose down` | Docker |

### SonarQube setup

- **Image:** `sonarqube:community` in docker-compose.yml
- **URL:** `http://localhost:9000`
- **Credentials:** `admin:admin` (default, used by scanner and bootstrap scripts)
- **Bootstrap:** `sonar/bootstrap.sh` creates:
  - Quality gate named "Standard Gate" with conditions:
    - `coverage < 100%` → ERROR
    - `code_smells > 0` → ERROR
    - `duplicated_lines_density > 0` → ERROR
    - `bugs > 0` → ERROR
    - `vulnerabilities > 0` → ERROR
    - `security_hotspots_reviewed < 100%` → ERROR
  - Project named `tally`
  - Sets the Standard Gate as the default quality gate
- **Scanner:** `sonar-scanner` CLI (installed locally, not the Docker image) pointed at `localhost:9000`
- **Sanctioned exclusion:** `cmd/tally/main.go` excluded from coverage via `sonar.coverage.exclusions` in `sonar-project.properties`

### Gate check: `sonar/check-gate.sh`

Polls `GET /api/qualitygates/project_status?projectKey=tally` every 3 seconds (up to 60 attempts). Exits 0 on OK, 1 on ERROR (with failing conditions printed), 1 on timeout.

### Docker Compose services

- **app:** Go binary on `:8080`, SQLite volume at `/data/tally.db`, `restart: unless-stopped`
- **sonarqube:** Community edition on `:9000`, H2 embedded (no external DB)
- **sonar-scanner:** One-shot service (profile `scanner`), mounts the project and runs `sonar-scanner` against the SonarQube container

## 3. Data Model

### SQLite schema (sketch)

```sql
CREATE TABLE members (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE contributions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id   INTEGER NOT NULL REFERENCES members(id),
    amount      REAL NOT NULL CHECK (amount > 0),
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### Key decisions

- **`amount` as REAL:** SQLite has no DECIMAL. Using `CHECK (amount > 0)` to enforce positive values. The app rounds all currency math to 2 decimal places before display.
- **`created_at` as TEXT (ISO 8601):** SQLite has no native datetime type; TEXT in ISO 8601 gives human readability and sortability.
- **CGO-free driver:** `modernc.org/sqlite` — pure Go SQLite driver, no CGO needed, works in `CGO_ENABLED=0` Docker builds.
- **Persistence:** SQLite file lives on a Docker named volume (`tally-data`). Container restart preserves all data.

## 4. API Contract

| Method | Path | Request Body | Response | Status |
|---|---|---|---|---|
| `POST` | `/members` | `{"name": "Alice"}` | `{"id": 1, "name": "Alice", "created_at": "..."}` | 201 |
| `GET` | `/members` | — | `[{"id": 1, "name": "Alice", "created_at": "..."}]` | 200 |
| `POST` | `/contributions` | `{"member_id": 1, "amount": 50.00, "description": "Jan rent"}` | `{"id": 1, "member_id": 1, "amount": 50.00, ...}` | 201 |
| `GET` | `/summary` | — | `{"members": [{"id": 1, "name": "Alice", "total": 150.00}], "group_total": 150.00}` | 200 |

### Error responses

All errors return `{"error": "message"}` with appropriate status codes:
- 400: bad input (missing fields, invalid types, negative amount, unknown member_id)
- 404: resource not found
- 405: wrong method
- 500: internal error (DB failure)

### Validation rules

- `POST /members`: `name` required, non-empty, trimmed
- `POST /contributions`: `member_id` required (must exist), `amount` required (positive number), `description` optional (defaults to empty string)

## 5. Frontend Screens

Three screens, all server-rendered HTML with HTMX for interactivity:

### Screen 1: Members (`/`)
- List all members in a table (name, join date)
- Form to add a new member (name input + submit)
- HTMX: form submits via `hx-post`, list refreshes without page reload

### Screen 2: Add Contribution (`/contributions`)
- Form: member dropdown, amount input, description input
- HTMX: submit via `hx-post`, display success/error inline

### Screen 3: Summary (`/summary`)
- Table: member name → total contributions
- Group total displayed prominently
- Auto-refresh via HTMX polling or manual refresh button

### Shared layout
- Navigation bar linking all three screens
- Minimal CSS (crude aesthetic)
- All templates in `web/templates/`, static assets in `web/static/`

## 6. Test Strategy

### Unit tests (fast, no external dependencies)
Run with: `make unit` → `go test ./... -coverprofile=coverage.out`

| Package | What's tested | Technique |
|---|---|---|
| `internal/store` | SQL operations (CRUD for members, contributions, summary query) | In-memory SQLite (`:memory:`) via `modernc.org/sqlite`; table-driven tests |
| `internal/handler` | HTTP handlers: request parsing, validation, response formatting, error codes | `httptest.NewRecorder` + table-driven cases; store dependency injected as interface |

### Functional tests (real stack)
Run with: `make functional` → `go test ./... -tags=functional`

| What's tested | How |
|---|---|
| Full HTTP flow: create member → add contribution → check summary math | Real HTTP client against `localhost:8080` (compose stack must be up) |
| Error handling: bad input, missing fields, unknown references | Black-box HTTP, verify status codes and error bodies |
| Data persistence across restart | `docker compose restart app`, then verify data is still queryable |

### Coverage target: 100%
With `cmd/tally/main.go` excluded (per sanctioned exclusion in `sonar-project.properties`), all other packages must reach 100% coverage. The `main.go` exclusion is valid per R6 — it contains only wiring (create store, create handler, start server).

### Test conventions
- `internal/store/*_test.go` — unit tests for store
- `internal/handler/*_test.go` — unit tests for handlers
- `internal/handler/*_functional_test.go` — functional tests (gated by `//go:build functional` tag)
