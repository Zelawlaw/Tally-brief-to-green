# CLAUDE.md — Tally

A gated-development demo: Go + HTMX + SQLite group-contribution tracker with SonarQube quality gates.

## Rules of engagement

1. **Discovery before code.** No application code before `DISCOVERY.md` is committed (done).
2. **Gates are the definition of done.** `make gates` must pass end-to-end, three consecutive times.
3. **Never weaken a gate.** Fix the code, not the thresholds. One sanctioned exclusion: `cmd/tally/main.go` from coverage (wiring only, per `sonar-project.properties`).
4. **Small commits,** one gate-relevant change each, with messages naming the gate.
5. **Read the actual failure** before fixing — no shotgun rewrites.
6. **Keep `main()` thin.** All logic in testable packages.

## Tech stack

- **Go 1.26** (stdlib `net/http`), **HTMX** (server-rendered HTML), **SQLite** (`modernc.org/sqlite`, CGO-free)
- **Docker Compose** for local dev: app on `:8080`, SonarQube community on `:9000`
- **SonarQube** quality gate: 100% coverage, 0 smells, 0 duplication, 0 bugs, 0 vulnerabilities, 100% hotspots reviewed
- **golangci-lint** with errcheck, staticcheck, gocyclo, dupl, gosec

## Workflow

```
make gates   # build → unit → functional → lint → sonar (stops at first failure)
```

Iterate until three consecutive green runs, then execute the improvement round (member statement endpoint).

## Gates

| Gate | Command | What |
|---|---|---|
| G1 | `make build` | `go vet` + binary builds + `docker compose build` |
| G2 | `make unit` | `go test ./... -coverprofile=coverage.out` |
| G3 | `make functional` | Black-box HTTP tests against compose stack |
| G4 | `make lint` | `golangci-lint run ./...` zero findings |
| G5 | `make sonar` | SonarQube scan + quality gate poll → PASSED |

## Project structure

```
cmd/tally/main.go     — wiring only (excluded from coverage)
internal/store/       — SQLite CRUD (in-memory for unit tests)
internal/handler/     — HTTP handlers (httptest for unit tests)
web/templates/        — Go html/template files
web/static/           — CSS, HTMX
```