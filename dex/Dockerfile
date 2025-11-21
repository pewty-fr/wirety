# Dex server Dockerfile (development/testing)
FROM ghcr.io/dexidp/dex:v2.41.1

USER root
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY config.yaml /app/config.yaml
RUN mkdir -p /data && chown -R 1001:1001 /data
VOLUME ["/data"]
EXPOSE 5556 5558
USER 1001
ENTRYPOINT ["dex", "serve", "/app/config.yaml"]
