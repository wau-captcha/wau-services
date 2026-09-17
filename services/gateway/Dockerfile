# Build from the repository root: docker build -f services/gateway/Dockerfile .
FROM golang:1.27 AS builder

COPY . /src
WORKDIR /src/services/gateway

RUN make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/services/gateway/bin /app

WORKDIR /app

EXPOSE 8080
VOLUME /data/conf

CMD ["./gateway", "-conf", "/data/conf"]
