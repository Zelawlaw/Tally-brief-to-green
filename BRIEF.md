# BRIEF.md — Tally: a Gated-Development Demo (Go + HTMX + SQLite + SonarQube)

**For:** Claude Code, running locally on the reader's machine.
**You are on the `start` branch.** It contains this brief, the gate harness (Makefile, docker-compose, SonarQube bootstrap), and empty application directories. Your job: **discover, build, and iterate the application until every gate below is green.** The `master` branch holds one finished result — do not peek; the point of this repo is that the loop, not the destination, produces the quality.

**The application — deliberately crude:** *Tally*, a tiny group-contribution tracker: members can be created, contributions recorded against members, and a dashboard shows each member's total plus the group total.
- **Backend:** Go (stdlib `net/http` or chi), JSON API: `POST /members`, `GET /members`, `POST /contributions`, `GET /summary`.
- **Frontend:** server-rendered HTML + HTMX served by the same Go binary (one language, one container — crude is the aesthetic).
- **Database:** SQLite via a CGO-free driver (`modernc.org/sqlite`), file on a compose volume.
- **Everything runs with `docker compose up`:** the app and SonarQube. Nothing deploys anywhere; localhost is the production environment.

---

## Rules of engagement

- **R1 — Discovery before code.** Produce `DISCOVERY.md` first (Gate 0). No application code before it is committed.
- **R2 — The gates are the definition of done.** Not "it works," not "tests mostly pass" — all gates green in a single run of `make gates`.
- **R3 — Never weaken a gate to pass it.** Do not lower the coverage threshold, exclude files to dodge smells, disable lint rules, or mark code `//nolint` without a stated justification in the commit message. Fix the code, not the referee. (One sanctioned exclusion exists: see G5 note on `main.go`.)
- **R4 — Small commits, one gate-relevant change each,** with messages that say which gate the change serves.
- **R5 — When a gate fails, read the actual failure** (test output, Sonar issue list via its API) and fix the specific finding — no shotgun rewrites.
- **R6 — Keep `main()` thin.** All logic lives in testable packages; `main` only wires and starts. This is what makes G5's 100% coverage honest rather than heroic.

---

## Gate 0 — Discovery → `DISCOVERY.md`

Answer with evidence, not assumption:
1. Toolchain present: Go version, Docker + compose version, `golangci-lint` availability (install locally if absent), SonarQube scanner approach (the compose file runs `sonar-scanner-cli` as a one-shot service — confirm).
2. The gate harness: read the `Makefile` targets (`build`, `unit`, `functional`, `lint`, `sonar`, `gates`) and `sonar/bootstrap.sh` (creates the **Standard Gate** in SonarQube via its API: coverage = 100%, code smells = 0, duplicated lines = 0, bugs = 0, vulnerabilities = 0, security hotspots reviewed = 100%). Confirm SonarQube boots at `localhost:9000` and the bootstrap ran.
3. Data model sketch (members, contributions), API contract, and the frontend's three screens (members, add contribution, summary).
4. Test strategy: what will be unit-tested (handlers via `httptest`, store logic) vs functionally tested (real HTTP against the composed stack, real SQLite file).
**Gate 0 passes when `DISCOVERY.md` is committed and the SonarQube Standard Gate exists.**

---

## Functional gates

**G1 — Build:** `make build` — `go vet ./...` clean, binary builds, `docker compose build` succeeds.
**G2 — Unit tests:** `make unit` — `go test ./... -coverprofile=coverage.out` all green. Table-driven tests expected; the coverage profile feeds G5.
**G3 — Functional tests:** `make functional` — compose stack up, then black-box HTTP tests against the running app (Go tests tagged `functional`): create member → contribute → summary math correct to the cent; bad input rejected with proper status codes; data survives a container restart (the SQLite volume proves persistence).

## Quality gates

**G4 — Lint:** `make lint` — `golangci-lint run` zero findings with the committed config (errcheck, staticcheck, gocyclo, dupl, gosec enabled).
**G5 — SonarQube Standard Gate:** `make sonar` — scanner pushes analysis + `coverage.out`; poll the quality-gate API until it reports **PASSED** against: 100% coverage, 0 smells, 0 duplication, 0 bugs, 0 vulnerabilities, 100% hotspots reviewed.
*Sanctioned exclusion:* `main.go` (wiring only, per R6) via `sonar.coverage.exclusions` — declared in `sonar-project.properties` on the `start` branch already; do not extend it.

## The loop

```
make gates   # runs G1→G5 in order, stops at first failure
```
Iterate: fix → commit → `make gates` → repeat until one clean end-to-end green run. Then run it **twice more** — flaky green is red.

---

## Improvement round (the "continuously improve" part)

After the first all-green run, execute one mini-brief *through the same loop*: **add a member statement endpoint + page** (`GET /members/{id}/statement` — chronological contributions with running balance). Discovery addendum → code → all gates green again, including back to 100% coverage. This demonstrates that the gates make change cheap, not expensive.

---

## Exit checklist
- [ ] `DISCOVERY.md` committed before any code
- [ ] `make gates` green three consecutive runs
- [ ] SonarQube dashboard: Standard Gate PASSED (screenshot saved to `docs/`)
- [ ] Functional persistence test proves SQLite survives restart
- [ ] Improvement round shipped through the full loop
- [ ] No gate was weakened (R3) — `git log` tells the story honestly
