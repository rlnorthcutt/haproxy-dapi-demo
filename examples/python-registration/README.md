# Python Service Registration Example

`register.py` demonstrates how a service can register itself into HAProxy
via the Data Plane API v3, handling version conflicts gracefully.

## What it teaches

1. **Version pinning** — every transaction pins to the config version at the
   time it was opened. If someone else changes config concurrently, the commit
   returns 400.
2. **Retry on conflict** — the script re-fetches the version and retries,
   exactly as a real service mesh control plane would.
3. **No SDK required** — the standard library `urllib` is sufficient for all
   DAPI operations.

## Usage (inside dapi-client)

```sh
python3 /examples/register.py <address> <port> <server-name> <backend>
```

```sh
# Register backend-3:80 as web3 in be_app
python3 /examples/register.py backend-3 80 web3 be_app
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HAP` | `http://haproxy:5555/v3/services/haproxy` | Config API base |
| `AUTH` | `admin:adminpwd` | Basic auth credentials |

These are pre-exported by the dapi-demo runner, matching the standard env
used by all scenarios.

## Scenario 5 context

This script is the teaching artifact for Demo 5 in the HAProxy DAPI presentation.
The audience watches the retry logic execute in real time as the script handles
a simulated version conflict.
