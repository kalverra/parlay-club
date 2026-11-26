#!/bin/sh
# Restore the database from its replica, then run the server under Litestream.
set -eu

: "${DB_PATH:=/data/parlay.db}"
: "${LITESTREAM_BUCKET:=}"

if [ -z "${LITESTREAM_REPLICA_URL:-}" ]; then
	if [ -z "$LITESTREAM_BUCKET" ]; then
		echo "entrypoint: set LITESTREAM_BUCKET or LITESTREAM_REPLICA_URL" >&2
		exit 1
	fi
	LITESTREAM_REPLICA_URL="gs://${LITESTREAM_BUCKET}/parlay.db"
fi

export DB_PATH LITESTREAM_REPLICA_URL

# -if-db-not-exists keeps a warm instance's database. -if-replica-exists lets a
# first boot start with an empty replica. -integrity-check makes a corrupt
# restore fail the boot instead of serving a database that answers 500s.
litestream restore \
	-if-db-not-exists \
	-if-replica-exists \
	-integrity-check full \
	-config /etc/litestream.yml \
	"$DB_PATH"

exec litestream replicate -config /etc/litestream.yml -exec /app/server
