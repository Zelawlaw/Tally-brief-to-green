# brief-to-green

**Tally — a gated-development demo.** Hand an AI coding agent a brief with hard quality gates, and watch it discover, build, and iterate a small Go application until every gate is green.

> **You are on the `start` branch** — it contains exactly two files: this README (for you) and [`BRIEF.md`](./BRIEF.md) (for the agent). Everything else — the app, the tests, the Docker setup, even the quality-gate tooling — gets built by Claude Code executing the brief. The `master` branch holds one finished run, every gate green, so you can compare your result with mine.

## The idea

Most people try to get better output from AI agents by prompting harder. This repo demonstrates a different lever: **change the definition of done.** The brief gives the agent a discovery phase it must complete before writing code, then a loop of gates — build, unit tests, functional tests against the running stack, lint, and a SonarQube quality gate (100% coverage, 0 maintainability issues, 0 security issues, 0 duplication, triple-A ratings). The agent iterates until all of it passes. The rule that makes it honest: **the agent may not weaken a gate to pass it.**

You're not reading about the method — you're going to run it.

## Prerequisites

- **Docker** with Compose v2 (Docker Desktop on macOS/Windows, docker-ce on Linux)
- **Go** 1.22+ (`go version`)
- **Git**
- ~4 GB free RAM (SonarQube is hungry)
- **Linux only:** SonarQube needs `sudo sysctl -w vm.max_map_count=262144` (Docker Desktop handles this on macOS/Windows)

## Install Claude Code

```bash
curl -fsSL https://claude.ai/install.sh | bash
claude --version
```

**Authenticate** (pick one):

- Simplest: run `claude` once and log in via the browser prompt (Pro/Max subscription or Console account).
- Or use an API key kept out of your shell history and dotfile repos:

```bash
# create a private env file
cat > ~/.claude-env.sh << 'EOF'
export ANTHROPIC_API_KEY=sk-ant-...
EOF
chmod 600 ~/.claude-env.sh

# source it from your shell profile
echo 'source ~/.claude-env.sh' >> ~/.bashrc    # Linux
echo 'source ~/.claude-env.sh' >> ~/.zshrc     # macOS
```

Open a new terminal and confirm `claude` starts.

## Run the experiment

```bash
git clone https://github.com/<you>/brief-to-green.git
cd brief-to-green
git checkout start
claude
```

Then give it one instruction:

> **Read BRIEF.md and execute it.**

That's it. Now your job is mostly to **not help**.

## What you'll see (and your role in it)

1. **Discovery first.** The agent must produce `DISCOVERY.md` before any code — including designing the gate harness itself (compose file, Makefile, SonarQube bootstrap that creates the "Standard Gate" via API).
2. **Build, then the loop.** `make gates` runs everything in order and stops at the first failure. The agent reads the actual failure and fixes the specific finding. Expect multiple iterations — that's the point, not a flaw.
3. **Your rules as the human:**
   - Approve tool/permission prompts; answer genuine environment questions (ports in use, missing tools).
   - **Do not** suggest fixes, and **do not** accept any proposal to lower a threshold, exclude files from coverage, or disable a lint rule. The brief forbids it (rule R3); if the agent proposes it anyway, say no and point it back at the brief.
4. **Done means:** `make gates` green three consecutive runs, and the SonarQube dashboard at [http://localhost:9000](http://localhost:9000) (default first login `admin`/`admin`, you'll be asked to change it) showing the Standard Gate **PASSED**.

Budget expectation: a run takes roughly an hour of wall time and a few dollars of API usage (or a slice of your Pro/Max quota), most of it in the iteration loop. SonarQube's first boot alone takes a couple of minutes — patience before assuming failure.

## Verify and compare

```bash
make gates          # the whole loop, one command
```

Then look at what your run produced versus mine:

```bash
git diff start master --stat        # how far the journey was
git checkout master && make gates   # my end state passes the same gates
```

Your `master`-equivalent won't match mine file-for-file — same brief, same gates, different valid solutions. That's the interesting part. Compare structure, test style, how the agent handled 100% coverage.

## Troubleshooting

| Symptom | Fix |
|---|---|
| SonarQube exits immediately (Linux) | `sudo sysctl -w vm.max_map_count=262144`, then `docker compose up -d` again |
| Port 9000 or 8080 already in use | stop the squatter, or let the agent remap ports in its compose (allowed — that's environment, not a gate) |
| Sonar analysis "pending" forever | first analysis on a cold instance is slow; check `docker compose logs sonarqube` |
| Agent stalls asking for permissions | that's Claude Code being polite — approve file/bash access for the repo |
| Coverage stuck below 100% | don't hint; the brief's R6 (thin `main`) is the intended path — let it find it |

## Share your run

If you try this, I'd genuinely like to see it: how many loop iterations, where it got stuck, what your diff looked like. Open an issue with your `make gates` output and a link to your fork, or tag me on the LinkedIn post.

---

*Two files started this. Everything else was built through the gates.*
