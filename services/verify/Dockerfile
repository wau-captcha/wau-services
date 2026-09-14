# Build from the repository root: docker build -f services/verify/Dockerfile .
FROM golang:1.27 AS builder

COPY . /src
WORKDIR /src/services/verify

RUN make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/services/verify/bin /app

WORKDIR /app

EXPOSE 8001
EXPOSE 9001
VOLUME /data/conf

CMD ["./verify", "-conf", "/data/conf"]
