# Server image. Primarily used by the e2e test harness (test/e2e) so
# testcontainers can build and run a real wirety-server; also suitable as a
# generic container image. The production release path builds with ko; this
# Dockerfile is the plain multi-stage equivalent (single-platform).
FROM golang:1.26-alpine AS builder

ARG VERSION=dev

WORKDIR /build

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=${VERSION}" -o wirety-server ./cmd/main.go

FROM alpine:3.24

RUN apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app

COPY --from=builder /build/wirety-server /app/wirety-server
# Migrations are read at startup when DB_ENABLED=true. The server resolves them
# at "${KO_DATA_PATH:-kodata}/migrations", so KO_DATA_PATH points at the parent.
COPY --from=builder /build/cmd/kodata/migrations /app/cmd/kodata/migrations

ENV HTTP_PORT="8080" \
    KO_DATA_PATH="/app/cmd/kodata" \
    LOG_LEVEL="info"

EXPOSE 8080

ENTRYPOINT ["/app/wirety-server"]
