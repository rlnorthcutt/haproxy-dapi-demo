#!/usr/bin/env python3
"""
register.py — HAProxy Data Plane API v3 service registration example.

Demonstrates how a service can register itself as a backend server at runtime
using the Data Plane API's transaction model, without restarting HAProxy or
touching config files directly.

The key concept is the VERSION-PINNED TRANSACTION:
  1. GET the current config version number
  2. Open a transaction pinned to that version
  3. Make config changes inside the transaction
  4. Commit — HAProxy validates, applies, and hot-reloads in one step

If another client commits between steps 1 and 4, the version number advances
and our transaction becomes "outdated". The commit returns HTTP 406, and we
retry from the top with the new version. This is the same conflict-resolution
pattern used by optimistic concurrency control in databases.

Usage:
    python3 register.py <address> <port> <server-name> <backend>

Example (from inside dapi-client):
    python3 register.py backend-3 80 web3 be_app

Environment:
    HAP         — HAProxy config API base URL
                  (default: http://haproxy:5555/v3/services/haproxy)
    AUTH        — user:password for Basic Auth
                  (default: admin:haproxypwd)
    DEMO_PAUSE  — seconds to hold an open transaction before committing
                  (default: 0). Set to a positive value to create a reliable
                  race window: the paused script holds its transaction open
                  while a concurrent script commits first, guaranteeing the
                  406 version-conflict retry fires on the next attempt.
"""

import os
import sys
import json
import time
import random
import base64
import urllib.request
import urllib.error

# ── Configuration ─────────────────────────────────────────────────────────────
# All settings come from environment variables so the script works both
# inside the demo container and against a remote HAProxy instance.

HAP = os.environ.get("HAP", "http://haproxy:5555/v3/services/haproxy")
AUTH = os.environ.get("AUTH", "admin:haproxypwd")
DEMO_PAUSE = float(os.environ.get("DEMO_PAUSE", "0"))

MAX_RETRIES = 5
BASE_RETRY_DELAY = 1.0  # seconds between retry attempts


# ── HTTP helpers ──────────────────────────────────────────────────────────────

def auth_header():
    """Return a Basic Auth header value for the configured AUTH credential.

    Basic Auth encodes "user:password" as base64 and prepends "Basic ".
    This is the authentication scheme expected by the Data Plane API.
    """
    encoded = base64.b64encode(AUTH.encode()).decode()
    return "Basic " + encoded


def request(method, url, data=None, content_type=None):
    """Make an authenticated HTTP request and return (status_code, body_dict).

    All Data Plane API calls go through this single function so that auth,
    serialisation, and error handling are consistent.

    Args:
        method:       HTTP verb — "GET", "POST", "PUT", "DELETE"
        url:          Full URL including query parameters
        data:         Python dict to send as JSON body (optional)
        content_type: Override Content-Type header (defaults to
                      "application/json" when data is provided)

    Returns:
        (status_code: int, body: dict)
        On network failure, returns (0, {"error": reason}).
    """
    body = json.dumps(data).encode() if data is not None else None
    headers = {"Authorization": auth_header()}

    # Default to JSON when we are sending a body.
    if data is not None and not content_type:
        content_type = "application/json"
    if content_type:
        headers["Content-Type"] = content_type

    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read()
            # Some successful responses (e.g. 204 No Content) have no body.
            return resp.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        # HTTPError carries the non-2xx status code and response body.
        try:
            body = json.loads(e.read())
        except Exception:
            body = {}
        return e.code, body
    except urllib.error.URLError as e:
        # URLError covers DNS failures, refused connections, timeouts, etc.
        return 0, {"error": str(e.reason)}


# ── Data Plane API operations ─────────────────────────────────────────────────

def get_version():
    """Return the current HAProxy configuration version number.

    The Data Plane API tracks a monotonically increasing version counter.
    Every committed transaction increments it by one. We read it here so we
    can open a transaction pinned to this exact version — the API rejects
    any commit whose pinned version no longer matches the current version,
    which is how concurrent writes are detected.
    """
    status, body = request("GET", f"{HAP}/configuration/version")
    if status != 200:
        raise RuntimeError(f"failed to get version: {status} {body}")
    return body  # The response body is a plain integer, not a JSON object.


def open_transaction(version):
    """Open a new transaction pinned to the given config version.

    A transaction is a staging area for config changes. Multiple changes
    can be batched inside one transaction and then applied atomically on
    commit. Pinning to a version means: "I am basing my changes on the
    config as it was at version N — reject my commit if anything else has
    changed since."

    Returns the transaction ID string, which must be passed to every
    subsequent API call that belongs to this transaction.
    """
    status, body = request("POST", f"{HAP}/transactions?version={version}")
    if status != 201:
        raise RuntimeError(f"failed to open transaction: {status} {body}")
    return body["id"]


def delete_transaction(tx_id):
    """Abort and delete an open transaction.

    Transactions that are never committed are eventually garbage-collected
    by the Data Plane API, but explicitly deleting them is good practice —
    it frees resources immediately and keeps the transaction list clean.
    Call this whenever a transaction cannot be committed.
    """
    status, _ = request("DELETE", f"{HAP}/transactions/{tx_id}")
    return status


def add_server(tx_id, backend, name, address, port):
    """Stage the addition of a backend server inside an open transaction.

    The server is not live until the transaction is committed. Fields:
      name    — HAProxy server label (must be unique within the backend)
      address — hostname or IP that HAProxy will resolve/connect to
      port    — TCP port on the backend host
      check   — enables active health checking; HAProxy will periodically
                 probe the server and remove it from rotation if it fails

    If the server name already exists (HTTP 409), it means the service is
    re-registering after a restart. We delete the old entry within the
    same transaction and add the new one, so the replacement is atomic.
    """
    url = f"{HAP}/configuration/backends/{backend}/servers?transaction_id={tx_id}"
    payload = {"name": name, "address": address, "port": port, "check": "enabled"}
    status, body = request("POST", url, data=payload)

    if status == 409:
        # Server already exists — replace it within this transaction.
        print(f"  server already exists — replacing (re-registration)")
        del_url = f"{HAP}/configuration/backends/{backend}/servers/{name}?transaction_id={tx_id}"
        del_status, del_body = request("DELETE", del_url)

        if del_status not in (200, 202, 204):
            print(f"  failed to delete existing server: {del_status} {del_body}")
            return del_status, del_body

        # Re-add the server with the updated parameters.
        status, body = request("POST", url, data=payload)

    return status, body


def commit_transaction(tx_id):
    """Commit the transaction, applying all staged changes.

    On success (200/202), HAProxy:
      - Validates the new configuration
      - Writes it to haproxy.cfg
      - Performs a live reload (no dropped connections)
      - Increments the config version number

    On HTTP 406 ("transaction is outdated"), another client committed
    between the time we called get_version() and now. Our pinned version
    is stale and the commit is rejected — we must retry from scratch.
    """
    status, body = request("PUT", f"{HAP}/transactions/{tx_id}")
    return status, body


# ── Registration logic ────────────────────────────────────────────────────────

def register(address, port, server_name, backend):
    """Register a backend server, retrying automatically on version conflicts.

    This is the core of the demo. The retry loop illustrates the optimistic
    concurrency pattern that real service-mesh clients use when registering
    with HAProxy under concurrent load:

      - Attempt to open a transaction and commit a server addition.
      - On HTTP 406 (version conflict), back off briefly and retry.
      - Use random jitter on the delay so that competing scripts don't all
        retry at the same instant — the "thundering herd" problem.
    """
    print(f"Registering {server_name} ({address}:{port}) into {backend}")

    for attempt in range(1, MAX_RETRIES + 1):
        # Add random jitter to spread out retries from concurrent scripts.
        # Without jitter, all scripts would retry at the same time and
        # immediately conflict again, potentially looping forever.
        jitter = random.uniform(0.1, 0.5)
        retry_delay = BASE_RETRY_DELAY + jitter

        # Step 1: read the current config version.
        version = get_version()
        print(f"  [attempt {attempt}] config version = {version}")

        # Step 2: open a transaction pinned to that version.
        tx_id = open_transaction(version)
        print(f"  opened transaction {tx_id}")

        # Step 3: stage the server addition inside the transaction.
        status, body = add_server(tx_id, backend, server_name, address, port)
        if status not in (200, 201, 202):
            print(f"  add_server failed ({status}): {body}")
            print(f"  aborting transaction {tx_id}")
            delete_transaction(tx_id)
            time.sleep(retry_delay)
            continue

        print(f"  added server: {json.dumps(body, indent=2)}")

        # DEMO_PAUSE: hold the transaction open so a concurrent script can
        # commit first, making the version conflict deterministic for the demo.
        if DEMO_PAUSE > 0:
            print(f"  [holding transaction open for {DEMO_PAUSE}s — concurrent commits can race now]")
            time.sleep(DEMO_PAUSE)

        # Step 4: commit — this is the point of potential conflict.
        commit_status, commit_body = commit_transaction(tx_id)

        if commit_status in (200, 202):
            # Success: HAProxy has reloaded with the new server live.
            print(f"  committed (reload triggered)")
            print(f"Registration complete: {server_name} is live in {backend}")
            return

        if commit_status in (400, 406):
            # 406 = "transaction is outdated" — another client committed
            # while our transaction was open, advancing the version past
            # what we pinned. The Data Plane API rejects our commit.
            # We retry from step 1 with the new version on the next loop.
            print()
            print(f"  ⚡ HTTP {commit_status} — VERSION CONFLICT")
            print(f"     Another client committed first; our pinned version is stale.")
            print(f"  ↺  Fetching new version and retrying"
                  f" (attempt {attempt + 1}/{MAX_RETRIES}) in {retry_delay:.2f}s…")
            print()
            time.sleep(retry_delay)
            continue

        # Any other failure (5xx, auth error, etc.) is not retryable.
        # Clean up the dangling transaction before raising.
        delete_transaction(tx_id)
        raise RuntimeError(f"commit failed: {commit_status} {commit_body}")

    raise RuntimeError(f"registration failed after {MAX_RETRIES} attempts")


# ── Entry point ───────────────────────────────────────────────────────────────

if __name__ == "__main__":
    if len(sys.argv) != 5:
        print(__doc__)
        sys.exit(1)

    try:
        address = sys.argv[1]
        port = int(sys.argv[2])  # raises ValueError if not a valid integer
        server_name = sys.argv[3]
        backend = sys.argv[4]

        register(address, port, server_name, backend)
    except ValueError:
        print("ERROR: port must be a valid integer.", file=sys.stderr)
        sys.exit(1)
    except RuntimeError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
