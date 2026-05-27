# haproxy-dapi-demo · Specification

A turnkey demo environment and TUI runner for the HAProxy Data Plane API.

## Purpose

`dapi-demo` exists for two audiences and one tool:

1. **Presenter.** Ron, walking an audience through Data Plane API v3 capabilities live, with full command transparency and no chance of a fumbled curl ruining a beat.
2. **Self-serve learner.** Anyone who picks up the repo, installs Podman, and wants to poke at the API without spinning up their own stack.

The same binary, scenarios, and container stack serve both. The TUI is the primary interface; a non-interactive run mode exists as an escape hatch for CI and automation.

## Goals

- Single binary, no host dependencies beyond Podman.
- Cross-platform: Linux, macOS, Windows (via `podman machine`).
- Every command shown to the audience is the exact string that runs. No abstraction, no shell-wrapping artifacts visible.
- YAML scenarios are the unit of content. Adding a new demo is one file, no recompile.
- Stack comes up in under 15 seconds and resets to a known baseline in under 5.

## Non-Goals

- Not a Data Plane API SDK. Curl is the teaching tool.
- Not a config management product. The reset path is destructive by design.
- Not multi-node. One HAProxy container, three backends. Production topologies are out of scope.
- No k3s, no Kubernetes. The audience for these demos is learning the API, not kube primitives.
- No web UI. Terminal only.

---

## Architecture

Three pieces:

1. **Container stack** (`compose.yaml`) — HAProxy + Data Plane API in one container, three backend containers, one client container.
2. **Scenario library** (`scenarios/*.yaml`) — declarative demo content.
3. **Go binary** (`dapi-demo`) — boots the stack, parses scenarios, runs them in a Bubble Tea TUI or in headless mode.

### Why a client container

Scenario commands are executed via `podman exec dapi-client sh -c '<command>'`. This buys us:

- **Cross-platform without shims.** Windows PowerShell does not need to grok bash quoting. The host runs Podman; the command runs in Linux.
- **No host tooling required.** Audience members do not need curl, jq, or bash installed.
- **Command transparency.** The string displayed in the TUI is identical to what executes inside the container. The `podman exec` wrapper is invisible.

### Why one HAProxy container

The `ghcr.io/haproxytech/haproxy-docker-alpine:s6-latest` image runs HAProxy and the Data Plane API as separate s6 services inside a single container. This matches the most common production deployment pattern and keeps the compose file simple. Splitting them is more "Fusion-shaped" but adds config surface that distracts from API demos.

---

## Container Stack

### Image choices

| Container      | Image                                                  | Purpose                                |
|----------------|--------------------------------------------------------|----------------------------------------|
| `haproxy`      | `ghcr.io/haproxytech/haproxy-docker-alpine:s6-latest`  | HAProxy CE + Data Plane API           |
| `backend-1..3` | `nginxdemos/hello`                                     | Visible per-container hostname response |
| `dapi-client`  | `alpine:latest` (with curl, jq, python3)               | Command execution target               |

### Port map

| Host  | Container       | Purpose                          |
|-------|-----------------|----------------------------------|
| 8080  | haproxy:80      | HAProxy frontend (HTTP traffic)  |
| 8443  | haproxy:443     | HAProxy frontend (HTTPS, Demo 4) |
| 5555  | haproxy:5555    | Data Plane API                   |
| 8404  | haproxy:8404    | HAProxy stats page (optional, useful for talks) |

Backend containers are not exposed to the host. They are only reachable through HAProxy or from inside the compose network.

### Network

One user-defined network (`dapi-net`). All containers join it. Service discovery by container name (`backend-1`, `haproxy`, etc.).

### Volumes

Bind mounts from the host repo:

- `./haproxy/haproxy.cfg` → `/usr/local/etc/haproxy/haproxy.cfg`
- `./haproxy/dataplaneapi.yml` → `/etc/haproxy/dataplaneapi.yml` *(path to verify against the s6 image during Phase 1)*
- `./haproxy/maps/` → `/etc/haproxy/maps/`
- `./haproxy/certs/` → `/etc/haproxy/certs/`
- `./haproxy/transactions/` → `/etc/haproxy/transactions/`

Host paths are visible during demos. Editing `haproxy.cfg` directly from the host is intentional; it lets the presenter answer "what did the API actually change?" by showing the file diff.

### Baseline HAProxy config

A minimal but interesting starting point:

- Frontend `fe_main` on `:80` with default backend `be_app`.
- Frontend `fe_https` on `:443` with a self-signed cert (for Demo 4).
- Backend `be_app` with two servers: `web1` → `backend-1:80`, `web2` → `backend-2:80`. The third server is added in Demo 2.
- Backend `be_canary` with one server (`backend-3`), reachable via a map lookup in Demo 3.
- `routing.map` referenced by a `use_backend` rule keyed off path prefix.
- Stats socket enabled for runtime API access.

The full baseline lives in `haproxy/baseline.cfg`. `haproxy/haproxy.cfg` is a copy at startup, modified during demos, restored by `reset`.

---

## Scenario Format

YAML, one file per scenario. Schema:

```yaml
id: 02-add-server                       # filename minus .yaml; used in CLI args
title: Add a server with a transaction
category: config                        # config | runtime | storage | client
reload: true                            # surfaces a banner in the TUI
description: |
  Most config changes need a transaction. Capture version, open a
  transaction, make changes, commit. The reload happens on commit.
prereqs:                                # optional; runs silently before the scenario
  - reset                               # special: restores baseline.cfg
steps:
  - title: Capture current config version
    explain: Every config change is versioned. We pin to this version when opening the transaction.
    run: |
      VER=$(curl -su $AUTH $HAP/configuration/version)
      echo "Version: $VER"
  - title: Open a transaction
    explain: A transaction lets us bundle multiple changes atomically.
    run: |
      TX=$(curl -sX POST -u $AUTH "$HAP/transactions?version=$VER" | jq -r .id)
      echo "Transaction: $TX"
  - title: POST the new server
    run: |
      curl -sX POST -u $AUTH -H 'Content-Type: application/json' \
        "$HAP/configuration/backends/be_app/servers?transaction_id=$TX" \
        -d '{"name":"web3","address":"backend-3","port":80}'
  - title: Commit the transaction
    explain: Commit triggers a reload. The change is now live.
    run: |
      curl -sX PUT -u $AUTH "$HAP/transactions/$TX"
verify:
  - run: curl -su $AUTH $HAP/configuration/backends/be_app/servers
    contains: '"name":"web3"'
cleanup:                                # optional; runs after verify
  - reset                               # convention: cleanup via reset
```

### Scenario fields

| Field | Required | Notes |
|-------|----------|-------|
| `id` | yes | Must match filename. Used in `dapi-demo run <id>`. |
| `title` | yes | Shown in TUI header and picker. |
| `category` | yes | Drives the colored banner. One of: `config`, `runtime`, `storage`, `client`. |
| `reload` | yes | Boolean. Tells the audience whether this scenario triggers an HAProxy reload. |
| `description` | yes | Markdown allowed (rendered as plain text in the TUI). |
| `prereqs` | no | List of either scenario IDs to run silently, or the special string `reset`. |
| `steps` | yes | Ordered list. See step fields below. |
| `verify` | no | Assertions run at the end. Failure does not roll back; it reports red in the TUI. |
| `cleanup` | no | Same shape as `prereqs`. Convention: end with `reset` if the scenario mutated config. |

### Step fields

| Field | Required | Notes |
|-------|----------|-------|
| `title` | yes | Shown as the step header. |
| `explain` | no | Optional paragraph rendered above the command. Use for teaching context. |
| `run` | yes | The shell command. Multi-line allowed. Environment variables persist across steps in the same scenario run. |

### Environment available to every step

Pre-exported in the persistent shell when the scenario starts:

| Var | Value |
|-----|-------|
| `API` | `http://haproxy:5555/v3` |
| `HAP` | `$API/services/haproxy` |
| `AUTH` | `admin:haproxypwd` |

Steps may define and reuse their own variables (`VER`, `TX`, etc.). State persists across steps within a single scenario run; it is reset between runs.

---

## Runner Architecture

### Persistent shell with marker protocol

Each scenario run opens one long-running shell process inside the client container:

```
podman exec -i dapi-client sh
```

Commands are fed to its stdin one step at a time, followed by a unique sentinel:

```
<step command>
echo __DAPI_DONE_<step_index>__$?
```

The runner reads stdout until it sees the marker, captures the exit code from the marker line, and returns the accumulated output. Shell-local state (env vars, working directory) persists because it is all one shell process.

This is the only mildly clever piece of the runtime. It exists because:

- The PDF's Demo 2 chains `$VER` → `$TX` across four steps. Without a persistent shell, each `podman exec` invocation gets a fresh environment.
- Running the entire scenario as one giant `sh -c` blob would defeat step-mode presentation.

### Verify block

After all steps, `verify` entries run in the same shell. Each entry has a `run` command and a `contains` substring. Pass/fail is reported but does not abort.

### Reset

`reset` copies `haproxy/baseline.cfg` over `haproxy/haproxy.cfg` on the host, then triggers a reload by sending `SIGUSR2` to the container:

```
podman kill -s USR2 haproxy
```

Faster than restarting the container. Also exercises the same reload path that the API uses on commit, which is good for parity.

---

## TUI Design

Bubble Tea with Lipgloss. Three rendered regions plus header and footer. Visual mockups live in `wireframe/` and are the authoritative reference for layout, spacing, and color decisions; this section describes intent, the wireframes describe pixels.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ DEMO 2 · Add a server                       [CONFIG · RELOAD]   2/4      │
├─────────────────────────────────┬────────────────────────────────────────┤
│                                 │                                        │
│  STEP 2: Open a transaction     │  $ podman logs -f haproxy              │
│                                 │                                        │
│  A transaction lets us bundle   │  [NOTICE] Loading success.             │
│  multiple changes atomically.   │  [NOTICE] New worker (12) forked       │
│                                 │  ...                                   │
│  ─── Command ───────────────    │                                        │
│  $ TX=$(curl -sX POST -u $AUTH ├────────────────────────────────────────┤
│    "$HAP/transactions?version= │                                        │
│    $VER" | jq -r .id)          │  $ podman logs -f dataplaneapi         │
│  $ echo "Transaction: $TX"      │                                        │
│                                 │  time="..." level=info msg="started"   │
│  ─── Output ────────────────    │  time="..." level=info msg="trans..."  │
│  Transaction:                   │  ...                                   │
│  7a2f-91bc-...                  │                                        │
│                                 │                                        │
├─────────────────────────────────┴────────────────────────────────────────┤
│  n/space=next  p=prev  r=run step  R=reset  s=shell  l=list  q=quit      │
└──────────────────────────────────────────────────────────────────────────┘
```

### Layout

- **Header** — scenario title, category banner (color-coded), reload flag, step counter.
- **Left pane (60% width)** — top half is current step title and explain. Bottom half is the command (with `$` prompt) and captured output. Same pane because the cognitive link "ran X, got Y" is the entire teaching point.
- **Right pane top (40% width × 50% height)** — live HAProxy container log tail.
- **Right pane bottom (40% width × 50% height)** — live Data Plane API log tail.
- **Footer** — keybinding hint bar.

### Category color scheme

| Category | Color  | Meaning                                  |
|----------|--------|------------------------------------------|
| config   | blue   | Transactional. Requires version. Reloads. |
| runtime  | green  | No transaction. No reload. Instant.       |
| storage  | amber  | File upload to storage endpoints.         |
| client   | purple | Client-side scripts (Python, etc.).       |

The banner reinforces the runtime-vs-config distinction (the Demo 3 callout in the source PDF) on every screen, automatically.

### Keybindings

| Key       | Action                                                    |
|-----------|-----------------------------------------------------------|
| `n`       | Advance to next step (do not execute)                     |
| `space`   | Execute current step, then advance                        |
| `r`       | Execute current step, do not advance                      |
| `p`       | Previous step                                             |
| `v`       | Verbose mode: re-run current step with `curl -v`          |
| `R`       | Reset stack to baseline                                   |
| `s`       | Shell escape (drop into `podman exec -it dapi-client sh`) |
| `l`       | Show scenario picker                                      |
| `j` / `k` | Scroll log panes                                          |
| `q`       | Quit                                                      |

### Modes

- **Step mode** (default) — user advances manually with `n`/`space`. The presentation mode.
- **Auto mode** (`--auto` flag) — runs all steps with a short delay. For self-serve testing and CI.

---

## CLI Surface

```
dapi-demo                       # launch TUI (default; runs `up` if needed)
dapi-demo up                    # bring up the compose stack
dapi-demo down                  # tear down
dapi-demo reset                 # restore baseline config, reload HAProxy
dapi-demo list                  # list scenarios
dapi-demo show <id>             # print scenario YAML
dapi-demo run <id>              # run scenario interactively in TUI
dapi-demo run <id> --auto       # run scenario headless, exit when done
dapi-demo shell                 # exec into dapi-client with $HAP, $AUTH set
dapi-demo version
```

Built with Cobra. Each subcommand is its own file in `cmd/dapi-demo/`.

---

## Repository Layout

```
dapi-demo/
├── README.md
├── SPEC.md                      # this file
├── CLAUDE.md                    # working agreement for AI assistance
├── compose.yaml
├── go.mod
├── go.sum
├── Makefile                     # build, release, test shortcuts
│
├── wireframe/                   # TUI mockup images, authoritative for layout
│   └── *.png                    # referenced from SPEC and CLAUDE
│
├── haproxy/
│   ├── haproxy.cfg              # live config (modified during demos)
│   ├── baseline.cfg             # reset target
│   ├── dataplaneapi.yml         # DPA config
│   ├── maps/
│   │   └── routing.map          # empty at baseline; populated in Demo 3
│   ├── certs/
│   │   ├── api-2025.pem         # current cert (Demo 4)
│   │   └── api-2026.pem         # rotation target (Demo 4)
│   └── transactions/            # DPA scratch dir; .gitignored
│
├── scenarios/
│   ├── 01-get-it-running.yaml
│   ├── 02-add-server.yaml
│   ├── 03-update-map.yaml
│   ├── 04-rotate-cert.yaml
│   ├── 05-python-registration.yaml
│   └── extras/
│       ├── drain-server.yaml
│       ├── add-acl.yaml
│       ├── stick-table-inspect.yaml
│       └── discover-spec.yaml
│
├── examples/
│   └── python-registration/
│       ├── register.py          # the script demoed in Scenario 5
│       └── README.md
│
├── cmd/dapi-demo/
│   ├── main.go
│   ├── root.go
│   ├── up.go
│   ├── down.go
│   ├── reset.go
│   ├── list.go
│   ├── show.go
│   ├── run.go
│   ├── shell.go
│   └── version.go
│
└── internal/
    ├── scenario/                # YAML parsing
    │   ├── types.go
    │   └── load.go
    ├── runner/                  # step execution
    │   ├── shell.go             # persistent shell with marker protocol
    │   ├── runner.go
    │   └── verify.go
    ├── compose/                 # podman compose wrappers
    │   └── compose.go
    ├── logs/                    # log tailers
    │   ├── tailer.go
    │   └── manager.go
    └── tui/                     # Bubble Tea
        ├── model.go
        ├── update.go
        ├── view.go
        ├── commands.go
        ├── picker.go
        └── styles.go
```

---

## Scenario Inventory

### Core five (from the source PDF)

| ID                          | Title                       | Category | Reload | Teaches                                  |
|-----------------------------|-----------------------------|----------|--------|------------------------------------------|
| 01-get-it-running           | Minimum config + connect    | config   | no     | Basic auth, /info, /v3, v2→v3 migration  |
| 02-add-server               | Add a server                | config   | yes    | Transactions, version pinning, commit    |
| 03-update-map               | Update a map · zero reload  | runtime  | no     | Runtime endpoints, no transaction needed |
| 04-rotate-cert              | Rotate a certificate        | storage  | yes    | Storage upload, staged rotation          |
| 05-python-registration      | Service registration loop   | client   | yes    | Version drift, retry on 400              |

### Extras (round out the surface area)

| ID                          | Title                       | Category | Reload | Teaches                                  |
|-----------------------------|-----------------------------|----------|--------|------------------------------------------|
| drain-server                | Drain via runtime           | runtime  | no     | `set server state maint` over runtime    |
| add-acl                     | ACL + use_backend rule      | config   | yes    | Nested config beyond servers             |
| stick-table-inspect         | Inspect stick tables        | runtime  | no     | Runtime visibility unique to DPA         |
| discover-spec               | Hit /info and /specification| config   | no     | API discoverability                      |

---

## Build Phases

Phases are sequential. Each ends with a checkpoint that must pass before the next begins.

### Phase 1 — Container stack

**Deliverable:** `podman compose up` brings up HAProxy + DPA + 3 backends + client, all healthy.

**Checkpoint:** `podman exec dapi-client curl -su admin:haproxypwd http://haproxy:5555/v3/info` returns JSON.

Files: `compose.yaml`, `haproxy/haproxy.cfg`, `haproxy/baseline.cfg`, `haproxy/dataplaneapi.yml`, `haproxy/maps/routing.map`, `haproxy/certs/*`.

### Phase 2 — Go skeleton + scenario schema

**Deliverable:** `go run ./cmd/dapi-demo list` prints scenarios from `scenarios/*.yaml`.

**Checkpoint:** YAML schema feels right after writing the first real scenario (`01-get-it-running`).

Files: `go.mod`, `internal/scenario/types.go`, `internal/scenario/load.go`, `cmd/dapi-demo/main.go`, `cmd/dapi-demo/list.go`, `scenarios/01-get-it-running.yaml`.

### Phase 3 — Runner

**Deliverable:** `dapi-demo run 02-add-server --auto` walks through the PDF's Demo 2 commands and `web3` appears in the server list.

**Checkpoint:** Scenarios 1–5 all run end-to-end in `--auto` mode. No TUI yet.

Files: `internal/runner/shell.go`, `internal/runner/runner.go`, `internal/runner/verify.go`, `internal/compose/compose.go`, plus the up/down/reset/run subcommands.

### Phase 4 — Log tailer (standalone)

**Deliverable:** A test harness can subscribe to live HAProxy and DPA logs and receive line-by-line.

**Checkpoint:** Standalone smoke test prints both streams for 10 seconds with clean cancellation.

Files: `internal/logs/tailer.go`, `internal/logs/manager.go`.

### Phase 5 — TUI

**Deliverable:** `dapi-demo` (no args) opens the TUI, shows the picker, lets the user step through Demo 2, and the right pane shows the reload happening in real time.

**Checkpoint:** A full presenter walkthrough of all five core scenarios feels good.

Files: everything under `internal/tui/`, plus `cmd/dapi-demo/tui.go` (or fold into `root.go` as the default action).

### Phase 6 — Remaining scenarios

**Deliverable:** All scenarios in the inventory exist and pass.

**Checkpoint:** `dapi-demo run <id> --auto` succeeds for every scenario in `scenarios/` and `scenarios/extras/`.

### Phase 7 — Polish

**Deliverable:** Verbose mode, shell escape, release artifacts, README quickstart.

**Checkpoint:** A fresh machine with only Podman installed can: clone repo, download binary, `dapi-demo`, step through Demo 2 in under 3 minutes.

Files: GoReleaser config, Makefile targets, README.

---

## Open Questions

These do not block Phase 1 but should be resolved by Phase 5:

1. **Exact mount path for `dataplaneapi.yml` in the s6 image.** Verify in Phase 1 when the stack first boots. Adjust SPEC if wrong.
2. **Default credentials in the s6 image.** The image ships with `admin/adminpwd` by default per the project README. Confirm and decide whether to override.
3. **Whether to embed the Python script source in the Scenario 5 explanation or treat it as a black box.** Lean toward black box; the script is the teaching artifact and lives in `examples/`.
