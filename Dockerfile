# syntax=docker/dockerfile:1

# Build the server as a static binary. The SQLite driver is pure Go, so CGO is
# not needed and the binary runs in the small runtime image.
FROM golang:1.27.3-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Pin the Litestream release and copy its binary into the runtime image.
FROM litestream/litestream:0.5.18 AS litestream

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
	&& addgroup -S app \
	&& adduser -S -G app app

WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY deploy/litestream.yml /etc/litestream.yml
COPY deploy/entrypoint.sh /app/entrypoint.sh
COPY seed /app/seed
# Dev mode reads templates from disk. Prod serves the embedded copies.
COPY web /app/web

RUN chmod +x /app/entrypoint.sh \
	&& mkdir -p /data \
	&& chown app:app /data

USER app

ENV APP_ENV=prod \
	DB_PATH=/data/parlay.db \
	PORT=8080

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
