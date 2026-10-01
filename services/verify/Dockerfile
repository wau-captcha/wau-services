# Build from the repository root: docker build -f services/verify/Dockerfile .
# The builder runs on the build machine and cross-compiles for the target platform, so
# --platform linux/arm64 needs no emulation.
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder

WORKDIR /src
COPY services/verify/go.mod services/verify/go.sum ./
RUN go mod download
COPY services/verify/ ./

ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o /out/verify ./cmd/verify

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/verify /app/verify
COPY services/verify/configs/config.yaml /data/conf/config.yaml

WORKDIR /app

EXPOSE 8001 9001

CMD ["./verify", "-conf", "/data/conf"]
