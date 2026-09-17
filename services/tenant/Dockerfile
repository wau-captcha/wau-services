# Build from the repository root: docker build -f services/tenant/Dockerfile .
FROM golang:1.27 AS builder

COPY . /src
WORKDIR /src/services/tenant

RUN make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/services/tenant/bin /app

WORKDIR /app

EXPOSE 8002
VOLUME /data/conf

CMD ["./tenant", "-conf", "/data/conf"]
