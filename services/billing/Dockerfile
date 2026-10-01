# Build from the repository root: docker build -f services/billing/Dockerfile .
# The builder runs on the build machine and cross-compiles for the target platform, so
# --platform linux/arm64 needs no emulation.
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder

WORKDIR /src
COPY services/billing/go.mod services/billing/go.sum ./
RUN go mod download
COPY services/billing/ ./

ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -ldflags "-s -w -X main.Version=$VERSION" -o /out/billing ./cmd/billing

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/billing /app/billing
COPY services/billing/configs/config.yaml /data/conf/config.yaml

WORKDIR /app

EXPOSE 8003

CMD ["./billing", "-conf", "/data/conf"]
