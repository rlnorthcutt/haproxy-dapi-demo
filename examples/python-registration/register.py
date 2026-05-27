#!/usr/bin/env python3
"""
register.py — HAProxy Data Plane API v3 service registration example.

Registers a backend server using a transaction, with automatic retry on
HTTP 406 (outdated transaction / version conflict). This simulates a service
that races with concurrent config changes and handles the conflict gracefully.

Usage:
    python3 register.py <address> <port> <server-name> <backend>

Example (from inside dapi-client):
    python3 register.py backend-3 80 web3 be_app

Environment:
    HAP         — HAProxy config API base (default: http://haproxy:5555/v3/services/haproxy)
    AUTH        — user:password (default: admin:haproxypwd)
    DEMO_PAUSE  — seconds to hold an open transaction before committing (default: 0)
                  Set to a non-zero value to create a reliable window for concurrent
                  scripts to commit first, guaranteeing the 400 retry fires.
"""

import os
import sys
import json
import time
import urllib.request
import urllib.error
import base64

HAP = os.environ.get("HAP", "http://haproxy:5555/v3/services/haproxy")
AUTH = os.environ.get("AUTH", "admin:haproxypwd")
DEMO_PAUSE = float(os.environ.get("DEMO_PAUSE", "0"))

MAX_RETRIES = 5
RETRY_DELAY = 1.0  # seconds


def auth_header():
    """Return a Basic Auth header value."""
    encoded = base64.b64encode(AUTH.encode()).decode()
    return "Basic " + encoded


def request(method, url, data=None, content_type=None):
    """Make an HTTP request, returning (status_code, body_dict)."""
    body = json.dumps(data).encode() if data is not None else None
    headers = {"Authorization": auth_header()}
    if content_type:
        headers["Content-Type"] = content_type

    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        try:
            body = json.loads(e.read())
        except Exception:
            body = {}
        return e.code, body


def get_version():
    """Return the current config version."""
    status, body = request("GET", f"{HAP}/configuration/version")
    if status != 200:
        raise RuntimeError(f"failed to get version: {status} {body}")
    return body  # plain integer in response body


def open_transaction(version):
    """Open a transaction pinned to version. Returns transaction ID."""
    status, body = request("POST", f"{HAP}/transactions?version={version}")
    if status != 201:
        raise RuntimeError(f"failed to open transaction: {status} {body}")
    return body["id"]


def add_server(tx_id, backend, name, address, port):
    """Add a server to a backend within a transaction.

    If the server already exists (409) it is deleted within the same transaction
    and re-added — this handles re-registration after a service restart.
    """
    url = f"{HAP}/configuration/backends/{backend}/servers?transaction_id={tx_id}"
    payload = {"name": name, "address": address, "port": port, "check": "enabled"}
    status, body = request("POST", url, data=payload, content_type="application/json")

    if status == 409:
        print(f"  server already exists — replacing (re-registration)")
        del_url = f"{HAP}/configuration/backends/{backend}/servers/{name}?transaction_id={tx_id}"
        request("DELETE", del_url)
        status, body = request("POST", url, data=payload, content_type="application/json")

    return status, body


def commit_transaction(tx_id):
    """Commit a transaction, triggering a reload."""
    status, body = request("PUT", f"{HAP}/transactions/{tx_id}")
    return status, body


def register(address, port, server_name, backend):
    """
    Register a server, retrying on version conflicts (HTTP 400).

    A 400 means our pinned version is stale — another change landed between
    our GET /version and our POST /transactions. We fetch the new version
    and try again.
    """
    print(f"Registering {server_name} ({address}:{port}) into {backend}")

    for attempt in range(1, MAX_RETRIES + 1):
        version = get_version()
        print(f"  [attempt {attempt}] config version = {version}")

        tx_id = open_transaction(version)
        print(f"  opened transaction {tx_id}")

        status, body = add_server(tx_id, backend, server_name, address, port)
        if status not in (200, 201, 202):
            print(f"  add_server failed ({status}): {body}")
            # Abandon transaction — let it expire.
            time.sleep(RETRY_DELAY)
            continue

        print(f"  added server: {json.dumps(body, indent=2)}")

        if DEMO_PAUSE > 0:
            print(f"  [holding transaction open for {DEMO_PAUSE}s — concurrent commits can race now]")
            time.sleep(DEMO_PAUSE)

        commit_status, commit_body = commit_transaction(tx_id)
        if commit_status in (200, 202):
            print(f"  committed (reload triggered)")
            print(f"Registration complete: {server_name} is live in {backend}")
            return

        if commit_status in (400, 406):
            # 406: transaction is outdated — another commit landed first.
            # 400: version pinned to a stale version (older DPA behaviour).
            print(f"  commit returned {commit_status} (version conflict) — retrying in {RETRY_DELAY}s")
            time.sleep(RETRY_DELAY)
            continue

        raise RuntimeError(f"commit failed: {commit_status} {commit_body}")

    raise RuntimeError(f"registration failed after {MAX_RETRIES} attempts")


if __name__ == "__main__":
    if len(sys.argv) != 5:
        print(__doc__)
        sys.exit(1)

    address, port, server_name, backend = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
    try:
        register(address, port, server_name, backend)
    except RuntimeError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
