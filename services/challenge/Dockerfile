# Build from the repository root: docker build -f services/challenge/Dockerfile .
# The builder runs on the build machine and cross-compiles for the target platform, so
# --platform linux/arm64 needs no emulation.
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder

WORKDIR /src
COPY services/challenge/go.mod services/challenge/go.sum ./
RUN go mod download
COPY services/challenge/ ./

ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o /out/challenge ./cmd/challenge

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/challenge /app/challenge
COPY services/challenge/configs/config.yaml /data/conf/config.yaml

WORKDIR /app

EXPOSE 8000 9000

CMD ["./challenge", "-conf", "/data/conf"]
