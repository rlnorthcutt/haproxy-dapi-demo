# Manual Demo — HAProxy Data Plane API v3

Step-by-step commands for running each demo scenario from your local shell.
No TUI required — just a running stack and a terminal.

---

## Prerequisites

**Start the stack** (if it isn't already running):

```bash
dapi-demo up
# or: podman compose up -d
```

**All commands below assume you run them from the repo root.**

---

## Environment variables

Set these once in your shell session. Every command below uses them.

```bash
export API=http://localhost:5555/v3
export HAP=$API/services/haproxy
export AUTH=admin:haproxypwd
```

---

## Reset to baseline

Wipes any config changes and reloads HAProxy from `haproxy/baseline.cfg`.
Run this between demos to get back to a clean slate.

```bash
cp haproxy/baseline.cfg haproxy/haproxy.cfg
podman exec haproxy /package/admin/s6/command/s6-svc -2 /run/s6-rc/servicedirs/haproxy
sleep 1  # allow reload to complete
```

---

## Demo 1 · API Discovery

> Verify the Data Plane API is reachable, learn the v3 URL structure, and
> explore what `/info` and `/specification` expose. No config changes.

### Step 1 — Check API version info

```bash
curl -su $AUTH $API/info | python3 -m json.tool
```

### Step 2 — List configured backends

```bash
curl -su $AUTH $HAP/configuration/backends | python3 -m json.tool
```

### Step 3 — List servers in be_app

```bash
curl -su $AUTH $HAP/configuration/backends/be_app/servers | python3 -m json.tool
```

### Step 4 — Show current config version

```bash
curl -su $AUTH $HAP/configuration/version
```

### Step 5 — Fetch raw HAProxy config

```bash
curl -su $AUTH $HAP/configuration/raw
```

---

## Demo 2 · Add a Server (Transaction + Reload)

> Open a version-pinned transaction, add `web3` to `be_app`, then commit —
> which triggers a live HAProxy reload. Requires `reset` first.

**Reset first:**
```bash
cp haproxy/baseline.cfg haproxy/haproxy.cfg
podman exec haproxy /package/admin/s6/command/s6-svc -2 /run/s6-rc/servicedirs/haproxy && sleep 1
```

### Step 1 — Open a transaction

```bash
VER=$(curl -su $AUTH $HAP/configuration/version)
echo "Version: $VER"
TX=$(curl -sX POST -u $AUTH "$HAP/transactions?version=$VER" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Transaction: $TX"
```

### Step 2 — Add web3 to be_app

```bash
curl -sX POST -u $AUTH \
  -H 'Content-Type: application/json' \
  "$HAP/configuration/backends/be_app/servers?transaction_id=$TX" \
  -d '{"name":"web3","address":"backend-3","port":80,"check":"enabled"}' \
  | python3 -m json.tool
```

### Step 3 — Commit (triggers reload)

```bash
curl -sX PUT -u $AUTH "$HAP/transactions/$TX" | python3 -m json.tool
```

### Step 4 — Verify web3 takes traffic

```bash
for i in 1 2 3 4 5 6; do
  curl -s http://localhost:18080/ | grep -o 'backend-[0-9]'
done
```

---

## Demo 3 · Update a Routing Map (Zero Reload)

> Add an entry to `routing.map` at runtime — no transaction, no reload.
> The change is live the instant the POST returns.

**Reset first:**
```bash
cp haproxy/baseline.cfg haproxy/haproxy.cfg
podman exec haproxy /package/admin/s6/command/s6-svc -2 /run/s6-rc/servicedirs/haproxy && sleep 1
```

### Step 1 — Show current map entries

```bash
curl -su $AUTH "$API/services/haproxy/runtime/maps/routing.map/entries"
```

### Step 2 — Add /canary route to be_canary

```bash
curl -sX POST -u $AUTH \
  -H 'Content-Type: application/json' \
  "$API/services/haproxy/runtime/maps/routing.map/entries" \
  -d '{"key":"/canary","value":"be_canary"}'
```

### Step 3 — Verify the entry is live

```bash
curl -su $AUTH "$API/services/haproxy/runtime/maps/routing.map/entries"
```

### Step 4 — Test the routing rule

```bash
echo "→ /canary (should reach backend-3):"
curl -s http://localhost:18080/canary | grep -o 'backend-[0-9]'
echo "→ / (should reach backend-1 or backend-2):"
curl -s http://localhost:18080/ | grep -o 'backend-[0-9]'
```

---

## Demo 4 · Rotate a Certificate (Staged)

> Upload a new cert via the storage API, then swap the frontend binding in a
> transaction. One reload switches the certificate — no SSH, no file copy.

**Reset first:**
```bash
cp haproxy/baseline.cfg haproxy/haproxy.cfg
podman exec haproxy /package/admin/s6/command/s6-svc -2 /run/s6-rc/servicedirs/haproxy && sleep 1
```

### Step 1 — List current certificates

```bash
curl -su $AUTH "$API/services/haproxy/storage/ssl_certificates" | python3 -m json.tool
```

### Step 2 — Upload api-2026.pem

```bash
curl -sX DELETE -u $AUTH \
  "$API/services/haproxy/storage/ssl_certificates/api-2026.pem" > /dev/null 2>&1 || true

curl -sX POST -u $AUTH \
  -F 'file_upload=@haproxy/staging/api-2026.pem' \
  "$API/services/haproxy/storage/ssl_certificates" | python3 -m json.tool
```

### Step 3 — Open a transaction and update the HTTPS bind

```bash
VER=$(curl -su $AUTH $HAP/configuration/version)
TX=$(curl -sX POST -u $AUTH "$HAP/transactions?version=$VER" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Transaction: $TX"

curl -sX PUT -u $AUTH \
  -H 'Content-Type: application/json' \
  "$HAP/configuration/frontends/fe_https/binds/:443?transaction_id=$TX" \
  -d '{"name":":443","port":443,"ssl":true,"ssl_certificate":"/usr/local/etc/haproxy/certs/api-2026.pem"}' \
  | python3 -m json.tool
```

### Step 4 — Commit (triggers reload)

```bash
curl -sX PUT -u $AUTH "$HAP/transactions/$TX" | python3 -m json.tool
```

### Step 5 — Verify both certs are in storage

```bash
curl -su $AUTH "$API/services/haproxy/storage/ssl_certificates" | python3 -m json.tool
```

### Step 6 — Test the new cert (optional)

```bash
curl -sk https://localhost:18443/ | grep -o 'backend-[0-9]'
```

---

## Demo 5 · Python Service Registration

> A Python script registers itself as a backend server, handling version
> conflicts (HTTP 406) with a retry loop — the optimistic concurrency pattern
> used by real service meshes.

**Reset first:**
```bash
cp haproxy/baseline.cfg haproxy/haproxy.cfg
podman exec haproxy /package/admin/s6/command/s6-svc -2 /run/s6-rc/servicedirs/haproxy && sleep 1
```

### Step 1 — Read the registration script

```bash
cat examples/python-registration/register.py
```

### Step 2 — Register two services concurrently

Both scripts open a transaction at the same version. `web3` holds its
transaction open (`DEMO_PAUSE=2`) while `web4` races ahead and commits.
`web3` then gets a 406 version conflict and retries.

```bash
DEMO_PAUSE=2 python3 examples/python-registration/register.py backend-3 80 web3 be_app &
sleep 0.3
python3 examples/python-registration/register.py backend-3 80 web4 be_app &
wait
```

### Step 3 — Verify both servers are registered

```bash
curl -su $AUTH $HAP/configuration/backends/be_app/servers | python3 -m json.tool
```

### Step 4 — Test load distribution

```bash
for i in 1 2 3 4 5 6 7 8; do
  curl -s http://localhost:18080/ | grep -o 'backend-[0-9]'
done
```

---

## Extra · Drain a Server (Runtime API)

> Set a server to maintenance mode via the runtime API. No transaction, no
> reload. Existing connections drain; new requests skip the server.

### Step 1 — Show current server states

```bash
curl -su $AUTH "$API/services/haproxy/runtime/backends/be_app/servers" | python3 -m json.tool
```

### Step 2 — Set web2 to maintenance mode

```bash
curl -sX PUT -u $AUTH \
  -H 'Content-Type: application/json' \
  "$API/services/haproxy/runtime/backends/be_app/servers/web2" \
  -d '{"admin_state":"maint"}' | python3 -m json.tool
```

### Step 3 — Verify only web1 gets traffic

```bash
for i in 1 2 3 4; do
  curl -s http://localhost:18080/ | grep -o 'backend-[0-9]'
done
```

### Step 4 — Re-enable web2

```bash
curl -sX PUT -u $AUTH \
  -H 'Content-Type: application/json' \
  "$API/services/haproxy/runtime/backends/be_app/servers/web2" \
  -d '{"admin_state":"ready"}' | python3 -m json.tool
```

---

## Extra · Add ACL + use_backend Rule

> Add a Host-header ACL and a `use_backend` rule to route mobile clients to
> `be_canary`. Shows nested config objects beyond server management.
> Requires `reset` first.

**Reset first:**
```bash
cp haproxy/baseline.cfg haproxy/haproxy.cfg
podman exec haproxy /package/admin/s6/command/s6-svc -2 /run/s6-rc/servicedirs/haproxy && sleep 1
```

### Step 1 — Open a transaction

```bash
VER=$(curl -su $AUTH $HAP/configuration/version)
TX=$(curl -sX POST -u $AUTH "$HAP/transactions?version=$VER" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Transaction: $TX"
```

### Step 2 — Add the mobile ACL to fe_main

The ACL endpoint is PUT-only (replaces the full list). The list is empty after
reset, so this single-item PUT is safe.

```bash
curl -sX PUT -u $AUTH \
  -H 'Content-Type: application/json' \
  "$HAP/configuration/frontends/fe_main/acls?transaction_id=$TX" \
  -d '[{"index":0,"acl_name":"is_mobile","criterion":"hdr_sub(User-Agent)","value":"Mobile"}]' \
  | python3 -m json.tool
```

### Step 3 — Add use_backend rule

PUT replaces the whole list — include the existing map-based rule at index 1
to preserve it.

```bash
curl -sX PUT -u $AUTH \
  -H 'Content-Type: application/json' \
  "$HAP/configuration/frontends/fe_main/backend_switching_rules?transaction_id=$TX" \
  -d '[{"index":0,"name":"be_canary","cond":"if","cond_test":"is_mobile"},{"index":1,"name":"%[path,map_beg(/usr/local/etc/haproxy/maps/routing.map,be_app)]"}]' \
  | python3 -m json.tool
```

### Step 4 — Commit

```bash
curl -sX PUT -u $AUTH "$HAP/transactions/$TX" | python3 -m json.tool
```

### Step 5 — Test mobile routing

```bash
echo "Mobile UA (should reach backend-3 / be_canary):"
curl -s -A "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148" \
  http://localhost:18080/ | grep -o 'backend-[0-9]'

echo "Desktop UA (should reach backend-1 or backend-2):"
curl -s http://localhost:18080/ | grep -o 'backend-[0-9]'
```

---

## Extra · API Discovery via /specification

> The DPA is self-documenting. `/specification` returns the full OpenAPI spec —
> useful for exploring capabilities and generating client SDKs.

### Step 1 — Fetch /info

```bash
curl -su $AUTH $API/info | python3 -m json.tool
```

### Step 2 — Count available API operations

```bash
curl -su $AUTH $API/specification | python3 -c "
import sys, json
spec = json.load(sys.stdin)
paths = spec.get('paths', {})
ops = sum(len(v) for v in paths.values())
print(f'Paths: {len(paths)}')
print(f'Operations: {ops}')
"
```

### Step 3 — List resource categories (tags)

```bash
curl -su $AUTH $API/specification | python3 -c "
import sys, json
spec = json.load(sys.stdin)
tags = sorted({t for path in spec.get('paths', {}).values()
               for op in path.values() for t in op.get('tags', [])})
for t in tags:
    print(' ', t)
"
```

---

## Extra · Inspect Stick Tables

> Stick tables store per-client state for rate limiting and session affinity.
> The runtime API exposes live table contents in real time.

### Step 1 — List stick tables

```bash
curl -su $AUTH "$API/services/haproxy/runtime/stick_tables" | python3 -m json.tool
```

### Step 2 — Generate some traffic

```bash
for i in 1 2 3 4 5; do
  curl -s http://localhost:18080/ > /dev/null
done
echo "Sent 5 requests"
```

### Step 3 — Inspect stick table entries

```bash
curl -su $AUTH "$API/services/haproxy/runtime/stick_table_entries?stick_table=be_app" \
  | python3 -m json.tool
```

---

## Port reference

| Service           | Host port | Container port |
|-------------------|-----------|----------------|
| HAProxy HTTP      | 18080     | 80             |
| HAProxy HTTPS     | 18443     | 443            |
| Data Plane API    | 5555      | 5555           |
| HAProxy stats     | 18404     | 8404           |

Stats page: <http://localhost:18404/stats>
