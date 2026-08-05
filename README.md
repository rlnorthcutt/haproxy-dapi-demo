# haproxy-dapi-demo

A turnkey terminal demo environment for the **HAProxy Data Plane API v3**.

`dapi-demo` boots a Podman compose stack, walks you through pre-written API scenarios step by step, and shows live HAProxy and DPA log output alongside every command — all in a single terminal window. Every command you see is the exact string that runs; nothing is hidden or abstracted away.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ DEMO 2 · Add a server with a transaction    [CONFIG · RELOAD]   step 2 / 4   │
├─────────────────────────────────────┬────────────────────────────────────────┤
│ STEPS ────────────────────────────  │ • haproxy logs                         │
│ ✓ 1  Capture current config version │   [NOTICE] Loading success.            │
│ › 2  Open a transaction             │   [NOTICE] New worker forked (PID 42)  │
│ ○ 3  POST the new server            │   [NOTICE] Reload done.                │
│ ○ 4  Commit the transaction         │────────────────────────────────────────│
│                                     │ • dataplaneapi logs                    │
│ STEP 2  Open a transaction          │   level=info msg="transaction opened"  │
│                                     │   level=info msg="config version: 3"   │
│  A transaction lets us bundle       │                                        │
│  multiple changes atomically.       │                                        │
│                                     │                                        │
│  $ TX=$(curl -sX POST -u $AUTH \    │                                        │
│      "$HAP/transactions?version=    │                                        │
│      $VER" | jq -r .id)             │                                        │
│  $ echo "Transaction: $TX"          │                                        │
│                                     │                                        │
│  ── stdout ──────────────────────   │                                        │
│  Transaction: 7a2f-91bc-4e1d-...    │                                        │
├─────────────────────────────────────┴────────────────────────────────────────┤
│  n next  space run+next  r run  p prev  v verbose  R reset  l list  q quit   │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- **[Podman](https://podman.io/getting-started/installation)** with the Compose plugin  
  (on macOS/Windows: `podman machine start`)
- **Go 1.22+** — only if building from source

No other host tooling is required. `curl`, `jq`, and `python3` run inside the client container.

## Quick Start

```bash
# Clone and build
git clone https://github.com/rlnorthcutt/haproxy-dapi-demo
cd haproxy-dapi-demo
make build

# Launch (starts the compose stack automatically, then opens the TUI)
./dapi-demo
```

The first launch pulls container images and may take a minute. Subsequent starts are under 5 seconds.

### From source without make

```bash
go build -o dapi-demo ./cmd/dapi-demo
./dapi-demo
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `dapi-demo` | Launch the TUI (starts stack if needed) |
| `dapi-demo up` | Bring up the compose stack and wait for healthy |
| `dapi-demo down` | Tear down the stack |
| `dapi-demo reset` | Restore baseline HAProxy config and reload |
| `dapi-demo list` | List available scenarios |
| `dapi-demo show <id>` | Print a scenario's YAML |
| `dapi-demo run <id>` | Open a scenario directly in the TUI |
| `dapi-demo run <id> --auto` | Run a scenario headless (for CI) |
| `dapi-demo shell` | Drop into the client container with API env vars set |
| `dapi-demo version` | Print version |

## TUI Keybindings

| Key | Action |
|-----|--------|
| `space` | Execute current step, advance to next |
| `r` | Execute current step, stay on it |
| `n` | Advance to next step (without executing) |
| `p` | Go to previous step |
| `v` | Verbose mode — re-run current step with `curl -v` |
| `R` | Reset stack to baseline config |
| `l` | Open scenario picker |
| `j` / `k` | Scroll log panes down / up |
| `s` | Shell escape into `dapi-client` |
| `q` / `ctrl+c` | Quit |

## Scenarios

### Core demos

These five scenarios follow the HAProxy Data Plane API v3 presentation curriculum:

| ID | Title | Category | Reload | What it teaches |
|----|-------|----------|--------|-----------------|
| `01-get-it-running` | Minimum config + connect | config | — | Basic auth, `/info`, `/v3` endpoints |
| `02-add-server` | Add a server via transaction | config | ✓ | Version pinning, transactions, commit |
| `03-update-map` | Update a map — zero reload | runtime | — | Runtime endpoints; no transaction needed |
| `04-rotate-cert` | Rotate a certificate | storage | ✓ | Storage API, staged cert rotation |
| `05-python-registration` | Service registration loop | client | ✓ | Version drift handling, retry on 400 |

**Category colors** in the TUI banner tell you at a glance what kind of operation is happening:

| Color | Category | Meaning |
|-------|----------|---------|
| 🔵 Blue | `config` | Transactional — requires a version, triggers a reload on commit |
| 🟢 Green | `runtime` | Runtime-only — instant, no reload, no transaction |
| 🟠 Amber | `storage` | File storage (certificates, maps) |
| 🟣 Purple | `client` | Client-side automation (Python scripts, etc.) |

### Extras

Additional scenarios that round out the API surface area:

| ID | Title | Teaches |
|----|-------|---------|
| `drain-server` | Drain via runtime API | `set server state maint` without a reload |
| `add-acl` | ACL + use_backend rule | Nested config beyond server management |
| `stick-table-inspect` | Inspect stick tables | Runtime visibility into connection state |
| `discover-spec` | Browse `/info` and `/specification` | API discoverability |

## Container Stack

| Container | Image | Purpose |
|-----------|-------|---------|
| `haproxy` | `ghcr.io/haproxytech/haproxy-docker-alpine:s6-latest` | HAProxy CE + Data Plane API (s6 supervised) |
| `backend-1`, `backend-2`, `backend-3` | `nginxdemos/hello` | Backend targets with visible hostnames |
| `dapi-client` | `alpine` + curl + jq + python3 | Command execution environment |

### Port map

| Host port | Service | Purpose |
|-----------|---------|---------|
| `18080` | HAProxy `:80` | HTTP frontend |
| `18443` | HAProxy `:443` | HTTPS frontend (used in Demo 4) |
| `5555` | Data Plane API | REST API |
| `18404` | HAProxy stats | Stats page |

From your host you can hit the API directly:

```bash
curl -su admin:haproxypwd http://localhost:5555/v3/info
```

### Environment variables (inside client container)

All scenario steps run in a persistent shell with these pre-exported:

| Variable | Value |
|----------|-------|
| `API` | `http://haproxy:5555/v3` |
| `HAP` | `$API/services/haproxy` |
| `AUTH` | `admin:haproxypwd` |

## Writing a Custom Scenario

Drop a YAML file in `scenarios/` — no recompile needed. Filename must match the `id` field.

```yaml
id: my-scenario
title: My Custom Demo
category: runtime           # config | runtime | storage | client
reload: false
description: |
  What this scenario demonstrates.

steps:
  - title: Check current server count
    explain: The explain field appears above the command in the TUI.
    run: |
      curl -su $AUTH $HAP/configuration/backends/be_app/servers | jq '.items | length'

  - title: Check a runtime stat
    run: |
      curl -su $AUTH $HAP/runtime/servers?backend=be_app | jq '.[].runtime_settings.status'

verify:
  - run: curl -su $AUTH $HAP/runtime/servers?backend=be_app
    contains: '"status":"UP"'

# cleanup: [reset]   # uncomment if this scenario mutates config
```

**Rules to know:**
- `run` is the exact string shown in the TUI and the exact string that executes. Write it as you would type it.
- Shell state persists across steps within a run — `VER` and `TX` set in step 1 are available in step 2.
- Multi-line commands use YAML block scalars (`|`). The indentation is preserved.
- Use `curl -s` (silent). stderr stays out of the output region; `curl -v` is triggered by the `v` key in the TUI.
- Every mutating scenario should end with `cleanup: [reset]`.

## Development

```bash
make fmt      # gofmt all Go files
make lint     # go vet
make test     # unit tests (scenario parser, runner)
make smoke    # run every scenario in --auto mode against a live stack
make certs    # regenerate self-signed demo certs in haproxy/certs/
```

### How the runner works

Each scenario opens one persistent `sh` process inside `dapi-client` via `podman exec -i`. Commands are written to stdin one step at a time, followed by a unique sentinel:

```
<step command>
echo __DAPI_DONE_N__$?
```

The runner reads stdout until the sentinel appears, captures the exit code embedded in it, and returns the accumulated output. This is why shell variables like `$TX` survive across steps — it's all one shell session.

### Project layout

```
cmd/dapi-demo/       CLI entry points (one file per subcommand)
internal/
  scenario/          YAML parsing (no internal deps)
  compose/           Podman compose wrappers (no internal deps)
  logs/              Live log tailer using podman logs -f
  runner/            Persistent shell + marker protocol
  tui/               Bubble Tea TUI (model, update, view, styles)
scenarios/           Demo content — edit freely, no recompile
scenarios/extras/    Additional scenarios beyond the core five
haproxy/             HAProxy config, certs, maps (bind-mounted into container)
wireframe/           TUI layout reference images
```

## License

MIT
