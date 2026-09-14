# Build from the repository root: docker build -f services/challenge/Dockerfile .
FROM golang:1.27 AS builder

COPY . /src
WORKDIR /src/services/challenge

RUN make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/services/challenge/bin /app

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./challenge", "-conf", "/data/conf"]
