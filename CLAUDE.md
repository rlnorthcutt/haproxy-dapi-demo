# CLAUDE.md

Working agreement for AI-assisted development on `haproxy-dapi-demo`. Read this before touching code or scenarios. `SPEC.md` is the source of truth for what we are building; this file is about how to build it. Visual references for the TUI live in `wireframe/`; consult them before changing layout or styling.

## Project context in one paragraph

`haproxy-dapi-demo` is a turnkey demo environment for the HAProxy Data Plane API v3. A Go binary boots a Podman compose stack (HAProxy + DPA + three backends + a client container), parses YAML scenarios, and executes their shell commands inside the client container while rendering a Bubble Tea TUI that shows the scenario, the live command and output, and tailed container logs side by side. The primary use case is live presentation; a headless `--auto` mode supports self-serve testing and CI. Cross-platform via Podman on any host OS.

## Ground rules

### Command transparency is non-negotiable

The string a user sees in the TUI must be byte-identical to what executes. The `podman exec dapi-client sh -c '...'` wrapping is implementation detail and never appears in the visible command region. If a refactor would break this invariant, the refactor is wrong, not the invariant.

### One container, one image, one process tree

HAProxy and Data Plane API live in the same container, managed by s6, sourced from `ghcr.io/haproxytech/haproxy-docker-alpine:s6-latest`. Do not propose splitting them, do not propose a sidecar pattern, do not propose rebuilding the image. If a scenario needs a capability the image does not provide, raise it as a question; do not invent a Dockerfile to fix it.

### Scenarios are content, not code

Adding or modifying a demo should never require a recompile. If a scenario needs a feature the runner does not support, propose the runner change first, get sign-off, then write the scenario against it. Do not bake scenario-specific logic into Go code.

### Reset is destructive and that is fine

`reset` overwrites `haproxy/haproxy.cfg` with `haproxy/baseline.cfg`. Do not add prompts, confirmations, or backups. The whole point is a fast known baseline between demos. Users who want to preserve changes can commit them to git themselves.

## Go conventions

### Module and packages

- Module path: `github.com/rlnorthcutt/haproxy-dapi-demo`.
- Go 1.22+.
- Standard project layout: `cmd/` for binaries, `internal/` for implementation, no `pkg/`.

### Dependencies (pinned set, do not expand without reason)

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI |
| `github.com/charmbracelet/bubbletea` | TUI runtime |
| `github.com/charmbracelet/bubbles` | List, viewport, spinner |
| `github.com/charmbracelet/lipgloss` | Styling |
| `gopkg.in/yaml.v3` | Scenario parsing |

If a problem seems to need a new dependency, the answer is usually "use the standard library." Ask before adding.

### Style

- `gofmt` on save, no exceptions.
- Errors wrapped with `fmt.Errorf("doing X: %w", err)`. No bare returns of underlying errors from internal packages.
- Exported identifiers get doc comments. Internal helpers do not need them unless non-obvious.
- No `panic()` outside `main.go` initialization. Return errors.
- Context propagation: every long-running operation (compose up, log tail, scenario run) takes a `context.Context` as its first arg.

### Package layout discipline

| Package | May import | May not import |
|---------|-----------|----------------|
| `scenario` | stdlib, yaml.v3 | anything else internal |
| `runner` | `scenario`, `compose` | `tui`, `logs` |
| `logs` | stdlib | anything else internal |
| `compose` | stdlib | anything else internal |
| `tui` | `scenario`, `runner`, `logs`, `compose` | nothing it should not |

The TUI is the only package that knows about all the others. Keep it that way; it makes the headless path possible without the TUI in the link.

## Bubble Tea conventions

### Model-Update-View, strictly

- Model holds state. No side effects in struct fields.
- Update returns `(Model, tea.Cmd)`. The `tea.Cmd` performs side effects asynchronously.
- View is pure: same model, same output, every time.

### Side effects are commands

Running a scenario step is a `tea.Cmd`. Tailing logs is a `tea.Cmd` that posts messages back. Resetting the stack is a `tea.Cmd`. If you find yourself doing IO inside `Update`, stop and refactor.

### Messages

- One message type per category of async result: `stepFinishedMsg`, `logLineMsg`, `composeReadyMsg`, `resetCompleteMsg`.
- Errors come back as their own typed messages, not as `error` fields on success messages.

### Layout

- Use `lipgloss.JoinHorizontal` and `JoinVertical`, never manual string padding.
- All colors, borders, and spacing live in `internal/tui/styles.go`. No magic numbers or hex codes outside that file.

## Runner protocol

The marker protocol is the only non-obvious piece of the runner. If you forget how it works, this is the contract:

```
Open():    podman exec -i dapi-client sh   (long-running)
Run(cmd):  write to stdin:
             cmd
             echo __DAPI_DONE_<N>__$?
           read stdout until line starts with __DAPI_DONE_<N>__
           parse exit code from that line, return accumulated output
Close():   write `exit`, wait for process to terminate
```

`<N>` is a monotonic counter per shell instance. Do not reuse markers across runs; if the marker collides with stdout content from a step (extremely unlikely but possible), the runner hangs.

Pre-export env vars on `Open()`:

```sh
export API=http://haproxy:5555/v3
export HAP=$API/services/haproxy
export AUTH=admin:adminpwd
```

These are the contract for scenario authors. If they change, every scenario file must be updated in the same commit.

## Scenario authoring rules

- One scenario per file. Filename matches `id` field. Filename matches the leading number convention (`NN-kebab-case.yaml`).
- The `run` field is the visible command. Write it as you would type it. No `set -e`, no `if` blocks unless they teach something; use `verify` for assertions instead.
- Multi-line commands use YAML block scalars (`|`). Indentation matters.
- Prefer environment variables (`$VER`, `$TX`) over inline subshells when the value is reused. Audiences can read variable names; they cannot easily read 80-char curl chains.
- Every mutation scenario ends with `cleanup: [reset]`. No exceptions.
- `explain` blocks are 1–3 sentences max. The TUI pane is not infinite.
- `verify.contains` is a literal substring check, not a regex. Choose strings that are stable across HAProxy versions.

## Compose conventions

- One `compose.yaml`. No environment-specific overrides; if config needs to vary, ask first.
- Service names are stable identifiers used by the runner and the scenarios: `haproxy`, `dapi-client`, `backend-1`, `backend-2`, `backend-3`. Do not rename.
- Health checks on every service. The runner waits for `haproxy` healthy before allowing scenarios to run.
- Volumes are host bind mounts from `./haproxy/`. No named volumes for demo state; the host filesystem is the demo.

## Common pitfalls

### Do not shell out to `bash`

The `dapi-client` container is alpine. Default shell is `sh` (ash). Avoid bash-isms: no `[[ ]]`, no `${var,,}`, no process substitution.

### Do not capture stderr into the command output

The TUI's output region shows what the curl actually returned. If you merge stderr into stdout for "completeness," the JSON response gets prefixed with progress meters. Use `curl -s` for silent mode and let stderr go to the runner's log buffer, not the display.

### Do not assume jq is available on the host

It is in the client container, not the host. The runner shells out to Podman, never to local tools.

### Do not let the log tailer leak goroutines

`podman logs -f` is a subprocess. If the TUI exits without sending `cancel()` to the tailer, the subprocess persists. Always pair `Tail()` with a deferred cancel.

### Do not let scenario state leak between runs

A fresh `runner.Open()` per scenario run. New shell, new marker counter, new env. If two scenarios share state, that is a scenario design problem, not a runner optimization opportunity.

### Do not "improve" the demo PDF

Scenarios 1–5 mirror the source presentation deliberately. The argument flow (config vs runtime, version pinning, retry on 400, staged cert rotation) is the curriculum. Polish the wording, not the structure.

## Workflow

### Phase discipline

`SPEC.md` defines seven phases with checkpoints. Do not start phase N+1 until phase N's checkpoint passes. If a phase reveals a flaw in the spec, fix the spec in the same commit as the discovery.

### Commits

- One logical change per commit.
- Subject line: imperative, under 72 chars, no period.
- Body explains the why, not the what. The diff shows the what.
- Reference the phase in the subject when relevant: `phase-3: add transaction handling to scenario runner`.

### Testing

- `internal/scenario` and `internal/runner` get unit tests. Table-driven, no testify.
- TUI does not get unit tests; integration via real scenario runs is the test.
- A `scenarios-smoke-test` Makefile target runs all scenarios in `--auto` mode against a clean stack. This is the canary.

### What I will not auto-decide

If you (Claude) are unsure about any of the following, ask before acting:

- Adding a new top-level dependency.
- Changing the marker protocol.
- Changing the scenario YAML schema in a backwards-incompatible way.
- Adding new pre-exported env vars to the runner.
- Renaming a service in `compose.yaml`.
- Changing the keybindings in the TUI.

Everything else: use judgment and commit.

## Quick reference

- Image: `ghcr.io/haproxytech/haproxy-docker-alpine:s6-latest`
- Default credentials: `admin:adminpwd`
- API base: `http://haproxy:5555/v3` (inside the network)
- API base from host: `http://localhost:5555/v3`
- Client container: `dapi-client` (alpine + curl + jq + python3)
- Reload signal: `podman kill -s USR2 haproxy`
- Reset: copy `haproxy/baseline.cfg` over `haproxy/haproxy.cfg`, send USR2.
