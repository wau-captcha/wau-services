# Build from the repository root: docker build -f services/billing/Dockerfile .
FROM golang:1.27 AS builder

COPY . /src
WORKDIR /src/services/billing

RUN make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/services/billing/bin /app

WORKDIR /app

EXPOSE 8003
VOLUME /data/conf

CMD ["./billing", "-conf", "/data/conf"]
