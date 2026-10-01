# goose with this directory's migrations (/migrations, the default) and demo seed (/seed).
#   docker build -t wau-migrate infra/postgres
#   Schema and catalog, the only thing production runs, once per release (e.g. a Kubernetes Job):
#     docker run --rm -e GOOSE_DBSTRING="host=... dbname=... sslmode=require" -e PGUSER=... -e PGPASSWORD=... wau-migrate up
#   Demo users and API keys, local only; applied once, tracked in their own table:
#     docker run --rm ... wau-migrate -dir /seed -table goose_seed_version up
# The builder runs on the build machine and cross-compiles goose for the target platform.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0
# A cross-compiled go install lands in bin/<os>_<arch>/ rather than bin/, hence the find.
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go install -trimpath -ldflags="-s -w" \
		-tags "no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
		github.com/pressly/goose/v3/cmd/goose@v3.28.0 \
	&& mkdir /out && mv "$(find /go/bin -name goose -type f)" /out/goose

FROM alpine:3

COPY --from=builder /out/goose /usr/local/bin/goose
COPY migrations /migrations
COPY seed /seed

ENV GOOSE_DRIVER=postgres \
	GOOSE_MIGRATION_DIR=/migrations

USER nobody
ENTRYPOINT ["goose"]
CMD ["up"]
