#!/bin/sh
# Restore a read-only production Litestream snapshot to a temp directory and run cmd/eval.
set -eu

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

if ! command -v litestream >/dev/null 2>&1; then
	echo "eval-prod: litestream is required on PATH" >&2
	exit 1
fi

: "${LITESTREAM_BUCKET:=}"
if [ -z "${LITESTREAM_REPLICA_URL:-}" ]; then
	if [ -z "$LITESTREAM_BUCKET" ]; then
		echo "eval-prod: set LITESTREAM_BUCKET or LITESTREAM_REPLICA_URL" >&2
		echo "usage: LITESTREAM_BUCKET=<bucket> ./scripts/eval-prod.sh [eval flags...]" >&2
		exit 1
	fi
	LITESTREAM_REPLICA_URL="gs://${LITESTREAM_BUCKET}/parlay.db"
fi

mkdir -p ./data
tmpdir=$(mktemp -d ./data/eval-prod.XXXXXX)
trap 'rm -rf "$tmpdir"' EXIT INT TERM

tmpdb="${tmpdir}/parlay.db"
DB_PATH="$tmpdb"
export DB_PATH LITESTREAM_REPLICA_URL

litestream restore -config deploy/litestream.yml "$tmpdb"

if [ "$#" -eq 0 ]; then
	set -- --all
fi

go run ./cmd/eval --db "$tmpdb" "$@"
