# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o wirety-agent ./cmd/agent

# Runtime
FROM alpine:3.20
# wireguard-tools provides wg / wg-quick (may require kernel module on host)
RUN apk add --no-cache wireguard-tools ca-certificates
WORKDIR /app
COPY --from=build /app/wirety-agent /usr/local/bin/wirety-agent
ENTRYPOINT ["wirety-agent"]
