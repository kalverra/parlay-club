#!/bin/sh
# Restore a Litestream replica and verify it with a full SQLite integrity check.
#
# Usage: deploy/verify_replica.sh REPLICA_URL [OUTPUT_DB]
#
# With OUTPUT_DB the restored, verified database is kept there. Without it the
# script uses a temporary file and removes it. An empty replica exits 0 so the
# first deploy can create one.
#
# Requires gcloud, and litestream either on PATH or in LITESTREAM_BIN.
set -eu

replica="${1:-}"
out="${2:-}"

if [ -z "$replica" ]; then
	echo "usage: $0 REPLICA_URL [OUTPUT_DB]" >&2
	exit 2
fi

if [ -z "$out" ]; then
	out=$(mktemp "${TMPDIR:-/tmp}/parlay-verify.XXXXXX")
	trap 'rm -f "$out" "$out-wal" "$out-shm"' EXIT
fi

objects=$(gcloud storage ls -r "$replica" 2>/dev/null || true)
if [ -z "$objects" ]; then
	echo "no replica objects at $replica; skipping restore verification"
	exit 0
fi

litestream="${LITESTREAM_BIN:-litestream}"
"$litestream" restore -integrity-check full -o "$out" "$replica"
echo "replica verified: $replica"
