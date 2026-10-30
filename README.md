# wau-services

The backend of WAU Captcha: four Go services built with Kratos that issue CAPTCHA challenges, check
the answers, redeem the one-time tokens, and run the accounts, API keys, usage and billing behind
the dashboard at https://waucaptcha.com. Postgres holds keys, plans and usage; a Redis cluster holds
short-lived state such as open challenges, tokens, rate limits and daily counters.

How a captcha is solved:

1. A website embeds the widget (`@waucaptcha-sdk/client` or `@waucaptcha-sdk/react`) with its public site key.
2. The widget calls `GET /challenge/new` (challenge service), then `POST /challenge/verify` with the
   answer (verify service, which asks challenge over gRPC whether it is right) and gets a one-time token.
3. The website's backend sends the token with its secret key to `POST /siteverify` (verify service)
   and gets the result.

The public API is `https://api.waucaptcha.com`.

## Services

| Service | Ports | What it does | Main HTTP paths |
| --- | --- | --- | --- |
| challenge | HTTP 8000, gRPC 9000 | Issues challenges of every type and keeps them in Redis; checks answers for verify over gRPC (`CheckAnswer`, gRPC only). | `GET /challenge/new` (public) |
| verify | HTTP 8001, gRPC 9001 | Turns right answers into tokens and redeems tokens for websites. | `POST /challenge/verify` (public), `POST /siteverify` (public) |
| tenant | HTTP 8002 | Google sign-in, accounts, API keys, usage and quota, the plan and type catalog, the admin console API. | `POST /v1/auth/google`, `POST /v1/auth/admin/google`, `GET /v1/me`, `/v1/api-keys/...`, `GET /v1/usage`, `GET /v1/quota`, `GET /v1/plans`, `GET /v1/challenge-types`, `/v1/admin/...` (all internal) |
| billing | HTTP 8003 | Plans and SePay payments. | `GET /v1/billing/plans`, `POST /v1/billing/checkout`, `GET /v1/billing/transactions`, `GET /v1/billing/transactions/{id}` (internal); `POST /v1/billing/sepay/ipn` (public, called by SePay) |

Every service also answers `GET /healthz` (internal).

Public means opened on `api.waucaptcha.com` by the Ingress `wau-api` in `deploy/k3s/ingress.yaml`,
with exact paths; every other path there gets a 404, `/v1/*` included. Internal paths are reachable
inside the cluster only: the dashboard calls tenant and billing there. The full list of each
service's routes is in its `openapi.yaml`.

The ports are not in `configs/config.yaml`: it reads the listen addresses from `KRATOS_HTTP_ADDR` and
`KRATOS_GRPC_ADDR`. The ports above are the ones `infra/caddy/Caddyfile` and `deploy/k3s` use.

Other ports:

| Component | Port | Where |
| --- | --- | --- |
| Caddy, the local gateway | 8080 | `infra/docker-compose.yml` |
| Postgres | 5432 | `infra/docker-compose.yml` |
| Redis cluster, six nodes | 7001-7006 (cluster bus 17001-17006) | `infra/docker-compose.yml` |
| wau-dashboard | 3000 | `deploy/k3s/dashboard.yaml` |

## Challenge types

| Type | What the user does | Added by |
| --- | --- | --- |
| `image_distort` | Types the five characters drawn in a distorted PNG. The default when no type is asked for. | `00002_catalog.sql` |
| `motion_noise` | Types the five characters hidden in an animated GIF of moving dots. | `00002_catalog.sql` |
| `slide_puzzle` | Drags a puzzle piece sideways into its hole; the drag itself is checked too. | `00003_slide_puzzle.sql` |
| `animal_jump` | Watches two rounds of 2 s in which five animal heads jump, and clicks the one that jumps highest in each. | `00007_animal_jump.sql` |
| `object_rotate` | In each of two rounds, turns an object a step at a time (eight directions) until it faces the way an arrow on the ground points, the arrow and the object in two panels so that neither covers the other, then presses Done. | `00008_object_rotate.sql` |
| `auto_check` | Nothing, for most visitors: the browser SDK sends probes of the browser and the behaviour in it. A visitor who does not pass gets a challenge of a fallback type instead. | `00009_auto_check.sql` |

The migrations are in `infra/postgres/migrations/`. Each adds the type to `challenge_types` and to
every tier in `tier_challenge_types`; a key may only use the types of its owner's tier. In the code,
the types are the constants in `services/challenge/internal/biz/challenge.go` and the `challengeKinds`
map in `services/challenge/internal/biz/kind.go`.

For `auto_check` the fallback is the type the site asked for when the key allows it, otherwise the
first allowed of `slide_puzzle`, `image_distort`, `animal_jump`, `motion_noise`, otherwise any other
allowed type (`fallbackOrder` in `services/challenge/internal/biz/autocheck.go`).

The probes of `auto_check` run in a browser a bot may control, so an automatic pass also rests on
what a bot cannot fake from there. The address must be outside the cloud and hosting ranges of
`services/challenge/internal/biz/hostingranges.txt`; refresh that list about once a month with
`go run ./cmd/hostingranges` in `services/challenge`. The browser needs a clearance, which it earns
by solving a puzzle of the key (7 days, 6 automatic passes an hour and 24 a day). In `balanced`
mode a browser without one may pass as a first visit, 5 an hour per address. siteverify reports
such passes with the reason `cleared` or `first_visit`. The settings are the `KRATOS_AUTO_CHECK_*`
rows under [challenge](#challenge).

## Repository layout

- `.github/workflows/`: `ci.yml` and `release.yml`.
- `deploy/`: production on k3s, described in [deploy/README.md](deploy/README.md) (in Vietnamese).
  - `deploy/argocd/`: the Argo CD Application `wau`, applied once by hand.
  - `deploy/cluster/`: one-time cluster settings (Traefik, the cert-manager ClusterIssuer).
  - `deploy/k3s/`: the Kustomize manifests Argo CD syncs: services, dashboard, Ingress, middlewares, migrate Job.
  - `deploy/scripts/`: `build-push.sh` (build and push every image by hand) and `ecr-login.sh` (refresh the image pull secret).
- `infra/`: local development.
  - `infra/docker-compose.yml`: Postgres, the Redis cluster, migrations and Caddy.
  - `infra/caddy/Caddyfile`: the local gateway on port 8080.
  - `infra/postgres/`: the `migrate` image (`Dockerfile`), `migrations/` and the demo `seed/`.
- `services/challenge/`: the challenge service; also `cmd/samples/`, which writes sample challenges for the dashboard.
- `services/verify/`: the verify service.
- `services/tenant/`: the tenant service.
- `services/billing/`: the billing service.

Each service is its own Go module with the same shape:

- `api/<name>/v1/`: the protobuf API and the Go, gRPC and HTTP code generated from it. verify also
  holds `api/challenge/v1/`, a copy of challenge's proto for its gRPC client.
- `cmd/<name>/`: `main.go`, `wire.go` and the generated `wire_gen.go`.
- `configs/config.yaml`: the config, with `${NAME}` placeholders filled from the environment.
- `internal/`: `biz` (rules), `data` (Postgres and Redis; `queries/` for sqlc, `db/` generated),
  `service` and `server` (transport), `conf` (the config proto), `pkg` (shared helpers such as
  `envconf`); tenant and billing also have `auth`.
- `openapi.yaml` (generated), `Makefile`, `buf.yaml`, `buf.gen.yaml`, `buf.gen.config.yaml`,
  `sqlc.yaml`, `Dockerfile`.

## Run locally

Prerequisites:

- Go 1.27 (`go 1.27.0` in each `services/*/go.mod`).
- Docker with Compose. The compose file uses `postgres:18-alpine`, `redis:8.10-alpine` and
  `caddy:2.11-alpine`, and builds the `migrate` image with goose v3.28.0.
- For code generation only: `make` and the tools `make init` installs (see below).

### 1. Start the infrastructure

The compose file reads four variables. Set them in your shell or in `infra/.env` (gitignored):

| Variable | Meaning |
| --- | --- |
| `REDIS_PASSWORD` | Password of every Redis node |
| `POSTGRES_USER` | Postgres user, also used by `migrate` |
| `POSTGRES_PASSWORD` | Its password |
| `POSTGRES_DB` | Database name |

```bash
cd infra
docker compose up -d
```

This starts six Redis nodes on ports 7001-7006 and forms them into a cluster, starts Postgres on
port 5432, runs `migrate` once (`goose up` for the schema, then the demo seed in its own table
`goose_seed_version`) and starts Caddy on port 8080. The demo seed adds sample users, an
administrator and API keys. To apply a migration added later:

```bash
cd infra
docker compose run --rm migrate
```

### 2. Configure each service

Each service reads `services/<name>/.env` at startup (gitignored); variables already set in the
environment win. All `${NAME}` placeholders of `configs/config.yaml` must be set and not blank, or
the service stops and names the missing ones. Local values:

| Variable | challenge | verify | tenant | billing |
| --- | --- | --- | --- | --- |
| `KRATOS_HTTP_ADDR` | `:8000` | `:8001` | `:8002` | `:8003` |
| `KRATOS_GRPC_ADDR` | `:9000` | `:9001` | | |
| `KRATOS_DATABASE_SOURCE` | `postgres://<user>:<password>@localhost:5432/<database>?sslmode=disable` | same | same | same |
| `KRATOS_REDIS_ADDRS` | `localhost:7001,localhost:7002,localhost:7003` | same | same | |
| `KRATOS_REDIS_PASSWORD` | `<your-value>` (the compose `REDIS_PASSWORD`) | same | same | |
| `KRATOS_REDIS_TLS` | `false` | `false` | `false` | |
| `KRATOS_TRUSTED_PROXIES` | `<cidr-list>` | `<cidr-list>` | | |
| `KRATOS_CHALLENGE_GRPC_ENDPOINT` | | `localhost:9000` | | |
| `KRATOS_TOKEN_SECRET` | the same value as verify | `<your-value>` | | |
| `KRATOS_AUTH_SECRET` | | | `<your-value>` | the same value as tenant |
| `KRATOS_GOOGLE_CLIENT_ID` | | | `<your-value>` | |
| `KRATOS_GOOGLE_JWKS_URL` | | | `https://www.googleapis.com/oauth2/v3/certs` | |
| `SEPAY_MERCHANT_ID` | | | | `<your-value>` |
| `SEPAY_SECRET_KEY` | | | | `<your-value>` |
| `SEPAY_CHECKOUT_URL` | | | | `https://pay-sandbox.sepay.vn/v1/checkout/init` |
| `KRATOS_BILLING_RETURN_URL` | | | | `http://localhost:3000/billing` |

`KRATOS_REDIS_TLS=false` is required locally: the placeholder has no default, and the compose Redis
nodes speak plain TCP. Production sets it to `true`.

`KRATOS_TRUSTED_PROXIES` is the comma-separated CIDR blocks whose `X-Forwarded-For` header is
believed. Locally that is the loopback range, where Caddy on Docker Desktop connects from (see
`trusted_proxies` in `internal/conf/conf.proto`). The meaning of every variable is in
[Configuration](#configuration).

### 3. Start the services

One terminal per service, from the repository root:

```bash
cd services/challenge && go run ./cmd/challenge
cd services/verify && go run ./cmd/verify
cd services/tenant && go run ./cmd/tenant
cd services/billing && go run ./cmd/billing
```

Each finds its `configs/` and `.env` from the service directory, also when started from
`cmd/<name>`. `-conf <path>` loads another config file or directory.

### 4. Use them through http://localhost:8080

Caddy gives SDKs, test apps and the dashboard one base URL:

| Path | Goes to |
| --- | --- |
| `/challenge/new*` | challenge (`:8000`) |
| `/challenge/verify*`, `/siteverify*` | verify (`:8001`) |
| `/v1/billing/*` | billing (`:8003`) |
| `/v1/*` | tenant (`:8002`) |
| anything else | a JSON 404, reason `ROUTE_NOT_FOUND` |

A service that is not running answers through Caddy as a JSON 502, reason `UPSTREAM_UNAVAILABLE`.
Unlike production, Caddy also serves `/v1/*`. `/healthz` is not routed: call each service's own port.

```bash
curl "http://localhost:8080/challenge/new?site_key=<your-site-key>&challenge_type=image_distort"
```

The SDKs accept `http://localhost:8080` as their base URL; outside localhost they require https.
To run the dashboard against these services, see the README of wau-dashboard.

## Configuration

Rules shared by every service (`internal/pkg/envconf`):

- `${NAME}` placeholders in `configs/config.yaml` are required: set and not blank. They take no
  inline default.
- Values come from the environment, or from `services/<name>/.env`; the environment wins.
- Lists are comma-separated. Booleans are `true` or `false`.
- Some sections hold tuning values with defaults in `config.yaml`. Each key can be replaced with
  `KRATOS_<SECTION>_<KEY>` in upper case, e.g. `KRATOS_RATE_LIMIT_NEW_PER_IP` for
  `rate_limit.new_per_ip`. These are optional.

Every service opens its Postgres pool the same way (`internal/pkg/pgpool`), so that a database
which pauses while nobody is connected, such as Aurora Serverless v2 with a minimum of 0 ACUs, can
pause:

- No connection is opened at start or kept idle: one closes within two minutes of its last use.
  The `pool_min_conns`, `pool_min_idle_conns` and `pool_max_conn_idle_time` of
  `KRATOS_DATABASE_SOURCE` are ignored.
- Connections carry the service's name as `application_name`, shown in `pg_stat_activity`.
- A connect waits at least 30 s, the time a paused Aurora instance may take to resume; a larger
  `connect_timeout` in the DSN is kept.
- A query that finds the pool empty retries a failed connect up to three times, after 1, 2 and
  4 s, as long as its own deadline allows. A wrong password, database or privilege is not retried.

In production the manifests in `deploy/k3s` set the listen addresses, trusted proxies, the challenge
endpoint, the Google JWKS URL and the billing return URL; everything else comes from the Secret
`wau-env`, created by hand (see `deploy/README.md`).

### challenge

| Name | Required | Default | Meaning |
| --- | --- | --- | --- |
| `KRATOS_HTTP_ADDR` | yes | | HTTP listen address, e.g. `:8000`. |
| `KRATOS_GRPC_ADDR` | yes | | gRPC listen address, e.g. `:9000`. |
| `KRATOS_TRUSTED_PROXIES` | yes | | Comma-separated CIDR blocks whose `X-Forwarded-For` is believed: the proxy in front (Traefik's pods on k3s, loopback behind the local Caddy). |
| `KRATOS_DATABASE_SOURCE` | yes | | Postgres DSN. |
| `KRATOS_REDIS_ADDRS` | yes | | Comma-separated Redis Cluster seed nodes, `host:port`. |
| `KRATOS_REDIS_PASSWORD` | yes | | Redis password. |
| `KRATOS_REDIS_TLS` | yes | | `true` to speak TLS to every node (production), `false` locally. |
| `KRATOS_TOKEN_SECRET` | yes | | verify's token secret, the same value: `auto_check` clearances are signed with a key derived from it. |
| `KRATOS_RATE_LIMIT_NEW_PER_IP` | no | `30` | `/challenge/new` calls per client address per minute. `0` turns a limit off. |
| `KRATOS_RATE_LIMIT_NEW_PER_SITE_KEY` | no | `600` | `/challenge/new` calls per API key per minute. |
| `KRATOS_RATE_LIMIT_NEW_PER_FLAGGED_SITE_KEY` | no | `60` | The per-key limit for a key the scraping check flagged today. |
| `KRATOS_SCRAPING_FLAG_MIN_ISSUED` | no | `500` | A key issued at least this many challenges today... |
| `KRATOS_SCRAPING_FLAG_MAX_ANSWERED_PERCENT` | no | `10` | ...of which under this percent were answered is flagged. |
| `KRATOS_SCRAPING_FLAG_MIN_ANSWERED` | no | `200` | A key that answered at least this many today... |
| `KRATOS_SCRAPING_FLAG_MAX_SOLVED_PERCENT` | no | `10` | ...of which under this percent were right is flagged... |
| `KRATOS_SCRAPING_FLAG_MAX_AUTOMATED_PERCENT` | no | `50` | ...or over this percent were judged automated. |
| `KRATOS_SCRAPING_SUSPEND_MIN_ISSUED` | no | `5000` | A key issued at least this many challenges today... |
| `KRATOS_SCRAPING_SUSPEND_MAX_ANSWERED_PERCENT` | no | `5` | ...of which under this percent were answered is suspended. |
| `KRATOS_IP_HISTORY_WRONG_PER_10_MINUTES` | no | `20` | Wrong answers per client address in a 10-minute bucket; past it the address gets `RATE_LIMITED` for the rest of the bucket. |
| `KRATOS_IP_HISTORY_AUTOMATED_PER_HOUR` | no | `5` | Answers judged automated per address in the last hour; past it later verdicts carry `ip_bad_history`. |
| `KRATOS_AUTO_CHECK_POW_DIFFICULTY` | no | `18` | Proof-of-work bits of `auto_check`. `0` asks for none. |
| `KRATOS_AUTO_CHECK_MIN_SCORE_PERCENT` | no | `80` | Least score, in percent, that passes `auto_check` (1-100). |
| `KRATOS_AUTO_CHECK_PASSES_PER_IP_HOUR` | no | `30` | `auto_check` passes one address may get per hour, then it gets the fallback. `0` turns the cap off. |
| `KRATOS_AUTO_CHECK_MODE` | no | `balanced` | `strict`: an automatic pass needs a clearance, earned by solving a puzzle of the key. `balanced`: a first visit may also pass on its signals alone, with the reason `first_visit`. |
| `KRATOS_AUTO_CHECK_FIRST_VISIT_PASSES_PER_IP_HOUR` | no | `5` | `balanced` mode: passes without a clearance one address may get per hour. `0` turns the cap off. |
| `KRATOS_AUTO_CHECK_CLEARANCE_TTL` | no | `604800s` | How long a clearance lasts, from `3600s` to `2592000s` (written in seconds, as every duration of `config.yaml`). |
| `KRATOS_AUTO_CHECK_CLEARANCE_PASSES_PER_HOUR` | no | `6` | Automatic passes one clearance may give per hour. `0` turns the cap off. |
| `KRATOS_AUTO_CHECK_CLEARANCE_PASSES_PER_DAY` | no | `24` | ...and per day of `quota.time_zone`. `0` turns the cap off. |
| `KRATOS_AUTO_CHECK_BLOCK_HOSTING_ADDRESSES` | no | `true` | An answer from a cloud or hosting range never passes automatically, and a puzzle solved from one earns no clearance. |
| `KRATOS_AUTO_CHECK_EXTRA_HOSTING_CIDRS` | no | | Comma-separated CIDR blocks to treat as hosting ranges too. |
| `KRATOS_AUTO_CHECK_ENFORCE_POINTER_SIGNALS` | no | `false` | Let the pointer evidence (`synthetic_pointer`, `step_pointer`) lower the score. It is logged either way, in the line `auto check judged`. |

Fixed in `config.yaml`: challenges live 120 s, the proof of work of the other types is 15 bits, and
the daily quota follows the calendar of `Asia/Ho_Chi_Minh`.

### verify

| Name | Required | Default | Meaning |
| --- | --- | --- | --- |
| `KRATOS_HTTP_ADDR` | yes | | HTTP listen address, e.g. `:8001`. |
| `KRATOS_GRPC_ADDR` | yes | | gRPC listen address, e.g. `:9001`. |
| `KRATOS_TRUSTED_PROXIES` | yes | | As for challenge; the rate limits count by this address. |
| `KRATOS_DATABASE_SOURCE` | yes | | Postgres DSN. |
| `KRATOS_REDIS_ADDRS` | yes | | Comma-separated Redis Cluster seed nodes. |
| `KRATOS_REDIS_PASSWORD` | yes | | Redis password. |
| `KRATOS_REDIS_TLS` | yes | | `true` or `false`, as for challenge. |
| `KRATOS_CHALLENGE_GRPC_ENDPOINT` | yes | | gRPC address of challenge: `localhost:9000` locally, `challenge:9000` in k3s. |
| `KRATOS_TOKEN_SECRET` | yes | | HMAC-SHA256 key that signs the tokens. |
| `KRATOS_RATE_LIMIT_VERIFY_PER_IP` | no | `60` | `/challenge/verify` calls per client address per minute. `0` turns a limit off. |
| `KRATOS_RATE_LIMIT_SITEVERIFY_PER_SECRET_KEY` | no | `1200` | `/siteverify` calls per secret key per minute. |

Fixed in `config.yaml`: tokens live 120 s.

### tenant

| Name | Required | Default | Meaning |
| --- | --- | --- | --- |
| `KRATOS_HTTP_ADDR` | yes | | HTTP listen address, e.g. `:8002`. |
| `KRATOS_DATABASE_SOURCE` | yes | | Postgres DSN. |
| `KRATOS_REDIS_ADDRS` | yes | | Redis Cluster seed nodes, read for the daily quota counters challenge and verify keep. |
| `KRATOS_REDIS_PASSWORD` | yes | | Redis password. |
| `KRATOS_REDIS_TLS` | yes | | `true` or `false`, as for challenge. |
| `KRATOS_AUTH_SECRET` | yes | | HMAC-SHA256 key for access tokens, at least 32 bytes. billing uses the same value. |
| `KRATOS_GOOGLE_CLIENT_ID` | yes | | OAuth client ID of the dashboard (its `GOOGLE_CLIENT_ID`); Google ID tokens must be issued for it. |
| `KRATOS_GOOGLE_JWKS_URL` | yes | | Google's signing keys in JWK format: `https://www.googleapis.com/oauth2/v3/certs`. |

Fixed in `config.yaml`: access tokens live 86400 s, a user holds at most 20 keys, a replaced secret
key keeps working for 86400 s.

### billing

| Name | Required | Default | Meaning |
| --- | --- | --- | --- |
| `KRATOS_HTTP_ADDR` | yes | | HTTP listen address, e.g. `:8003`. |
| `KRATOS_DATABASE_SOURCE` | yes | | Postgres DSN of the database tenant uses. billing uses no Redis. |
| `KRATOS_AUTH_SECRET` | yes | | tenant's access token secret: billing checks the user tokens tenant issues. |
| `SEPAY_MERCHANT_ID` | yes | | SePay merchant ID. |
| `SEPAY_SECRET_KEY` | yes | | Signs checkout forms; SePay sends it back in `X-Secret-Key` with every IPN. |
| `SEPAY_CHECKOUT_URL` | yes | | SePay checkout form: `https://pay-sandbox.sepay.vn/v1/checkout/init` for testing, `https://pay.sepay.vn/v1/checkout/init` in production. |
| `KRATOS_BILLING_RETURN_URL` | yes | | Dashboard page SePay sends the payer back to: `https://waucaptcha.com/billing` in production. |

Fixed in `config.yaml`: a checkout waits 900 s for its payment, sweeps run every 60 s, and plan
months follow `Asia/Ho_Chi_Minh`. The IPN URL to enter in SePay is
`https://api.waucaptcha.com/v1/billing/sepay/ipn`.

## Database migrations

- Tool: [goose](https://github.com/pressly/goose) v3.28.0, built into the `migrate` image from
  `infra/postgres/Dockerfile`.
- Files: `infra/postgres/migrations/NNNNN_<name>.sql`, numbered with five digits (`00009_auto_check.sql`),
  each with a `-- +goose Up` and a `-- +goose Down` section.
- Seed: `infra/postgres/seed/` holds demo data for local use only, tracked in the table
  `goose_seed_version`. Production never runs it.
- sqlc reads the same directory as its schema (`sqlc.yaml`), so a schema change is followed by
  `make sqlc` in every service whose queries use the changed tables.

To add one:

1. Create the next file, e.g. `infra/postgres/migrations/00010_<name>.sql`, with Up and Down.
2. Apply it locally with `docker compose run --rm migrate` in `infra/`.
3. Run `make sqlc` where needed and commit the migration together with the code that uses it.

In production, `release.yml` builds the `migrate` image from `infra/postgres`, and Argo CD runs it
as a PreSync Job (`deploy/k3s/migrate.yaml`, `goose up`) on every sync, before any app changes.
A failed migration stops the sync and the old version keeps running.

Migrations must be additive, because every production deploy runs `goose up` before the new code
starts: the old code keeps running against the new schema until the rollout ends, and a rollback
of the app does not undo a migration. Never drop or rename a column in the same release that stops
using it. Stop using it in one release, and drop it in a later one.

## Code generation and build

Run the targets in a service directory (`services/<name>`). `make` without a target prints the list.

| Target | What it does | When to run it |
| --- | --- | --- |
| `make init` | Installs `wire`, `buf` and `sqlc` with `go install`. | Once per machine. |
| `make api` | `buf generate --template buf.gen.yaml`: Go, gRPC and HTTP code into `api/`, and `openapi.yaml`. | After changing a proto under `api/`. |
| `make config` | `buf generate --template buf.gen.config.yaml`: Go code for `internal/conf`. | After changing `internal/conf/conf.proto`. |
| `make sqlc` | `sqlc generate`: `internal/data/db` from `internal/data/queries` and the migrations. | After changing a query or adding a migration. |
| `make generate` | `go generate ./...` (wire writes `cmd/<name>/wire_gen.go`), then `go mod tidy`. | After changing the providers or `cmd/<name>/wire.go`. |
| `make build` | `go build` of every command into `bin/`, with `main.Version` set from `git describe --tags --always`. | To get local binaries. |
| `make all` | `api`, `config`, `sqlc`, then `generate`. | To regenerate everything. |

The buf plugins run with `go run` at the versions pinned in `buf.gen.yaml`, so only `buf` itself
needs installing. Generated code is committed; CI builds it but does not regenerate it.

verify keeps a copy of challenge's proto in `services/verify/api/challenge/v1/challenge.proto`.
Keep it identical to `services/challenge/api/challenge/v1/challenge.proto` except for
`go_package`, and run `make api` in both services after a change.

Images are built from the repository root, e.g. `docker build -f services/challenge/Dockerfile .`.

## Tests

```bash
cd services/challenge
go test ./...
```

Run it in each service directory. CI also runs `gofmt -l .`, `go vet ./...` and `go build ./...`.
The tests use in-memory fakes: none needs Postgres, Redis or another service. `go test -short ./...`
makes 50 object_rotate challenges in the generator test instead of 1000.

| Service | Test files | What they cover |
| --- | --- | --- |
| challenge | 23 | The animal_jump and object_rotate generators and checkers, auto_check with its clearances and hosting ranges, answer checks, scraping and admission, domains, proof of work, browser signals, filters, rate limits, the HTTP server. The image_distort and motion_noise renderers and the slide_puzzle generator and checker have no tests of their own. |
| verify | 7 | Answer handling, the challenge client, filters, rate limits, origins, site binding, the HTTP server. |
| tenant | 5 | API keys and their events, appeals, admin actions, domains, quota counters, the HTTP server. |
| billing | 1 | Only that the SePay IPN refuses large bodies. Checkout, payment handling and sweeps have no tests. |

There are no tests against a real database or across services, and the Kubernetes manifests are
only checked to render (`kubectl kustomize deploy/k3s` in CI).

## CI/CD and deployment

- `.github/workflows/ci.yml` runs on every push and pull request. For each service, with the Go
  version from its `go.mod`: gofmt, `go vet ./...`, `go build ./...`, `go test ./...`. It also checks
  that `kubectl kustomize deploy/k3s` renders.
- `.github/workflows/release.yml` runs on a push to `main` that changes `services/**` or
  `infra/postgres/**`, or by hand. After CI passes it builds linux/arm64 images of the four services
  and `migrate`, pushes them to ECR tagged `sha-<first 7 characters of the commit>`, and sets that tag
  as `newTag` of `wau/challenge`, `wau/verify`, `wau/tenant`, `wau/billing` and `wau/migrate` in
  `deploy/k3s/kustomization.yaml`. `github-actions[bot]` commits that file to `main` as
  `chore(deploy): release sha-... [skip ci]`.
- wau-dashboard's release workflow does the same for the `wau/dashboard` entry, with the commit
  `chore(deploy): release dashboard sha-... [skip ci]` in this repository.
- Argo CD (Application `wau`, `deploy/argocd/wau.yaml`) syncs `deploy/k3s` from `main` into the
  namespace `wau`, with automatic prune and self-heal: the cluster follows `main`, and changes made
  by hand in the cluster are reverted. Each sync runs the `migrate` Job first.
- Rollback: on `main`, set `newTag` in `deploy/k3s/kustomization.yaml` back to an earlier `sha-...`
  tag (or revert the bot's release commit) and push. Argo CD deploys it like a release. Migrations
  are not rolled back.
- Pull after each release before you change anything: the bot commits to `main`.
- Never force-push `main`. Argo CD deploys whatever `main` holds, and the release commits live there.
- The namespace `wau`, the Secret `wau-env` and the image pull secret `ecr-pull` are created by hand
  and stay out of Git and Argo CD.

First install, updates, secrets and rollback in detail: [deploy/README.md](deploy/README.md).

## Adding a new challenge type

Found from how `object_rotate` and `auto_check` were added. Not every item applies to every type.

wau-services:

- [ ] `services/challenge/internal/biz/challenge.go`: a `Type...` constant, and new `Media` fields if the type needs them.
- [ ] `services/challenge/internal/biz/`: the generator and the checker (as `objectrotate.go` and `objectrotatecheck.go`), with tests, registered in `challengeKinds` in `kind.go`.
- [ ] `services/challenge/internal/biz/behaviorcheck.go`: the type's least solve time in `minSolveTime`.
- [ ] `services/challenge/internal/biz/autocheck.go`: whether it belongs in `fallbackOrder`.
- [ ] `services/challenge/internal/service/challenge.go`: new media fields in the `/challenge/new` reply.
- [ ] `services/challenge/api/challenge/v1/challenge.proto`: the type in the comment of `challenge_type` (a string, not an enum), new fields, and the answer format in `CheckAnswerRequest.answer`. Copy it to `services/verify/api/challenge/v1/challenge.proto` and run `make api` in both services.
- [ ] `services/verify/api/verify/v1/verify.proto`: the answer format in `VerifyChallengeRequest.answer`; `make api` in verify.
- [ ] `services/verify/internal/server/http.go`: the request body limit, if answers grow.
- [ ] Settings, if any: a section in `services/challenge/configs/config.yaml` and `internal/conf/conf.proto` (`make config`), passed through `cmd/challenge/main.go` (add it to the `envconf.Resolver` sections to allow overrides) and `cmd/challenge/wire.go` (`make generate`).
- [ ] `services/challenge/cmd/samples/main.go`: samples for the dashboard, if the type has media.
- [ ] `infra/postgres/migrations/NNNNN_<type>.sql`: add the type to `challenge_types` and `tier_challenge_types` (copy `00009_auto_check.sql`).

wau-sdk-js:

- [ ] `packages/client/src/constants.ts` (`ChallengeType`), `types.ts`, `client.ts` (new challenge fields and a `verify...` method), the widget (`widget.ts`, a new UI file, a tracker), `labels.ts`, `styles.ts`, `auto-render.ts`, `index.ts`, tests in `packages/client/test/`.
- [ ] `packages/react/src/`: a component for the type, `challenge-panel.tsx`, `use-captcha.ts`.
- [ ] `packages/server/src/constants.ts` (`ChallengeType`).
- [ ] The READMEs, `CHANGELOG.md`, versions in `packages/*/package.json` and `package-lock.json`.

wau-sdk-java:

- [ ] `src/main/java/com/waucaptcha/ChallengeType.java` (enum constant), `Challenge.java` (new fields), records for the answer (as `RotatePress.java`, `RotateRound.java`), a `verify...` method in `WauCaptchaClient.java`.
- [ ] Tests in `src/test/java/com/waucaptcha/`, `README.md`, the version in `pom.xml`.

wau-dashboard:

- [ ] `lib/captchas.ts` (`CAPTCHAS`, `CaptchaTypeId`, samples), `lib/i18n/messages/challenge-types.ts` (label and description, en and vi), `lib/usage-chart.ts` (chart colour).
- [ ] Texts in `lib/i18n/messages/` (`captchas.ts`, `captcha-guides.ts`, `docs.ts`, `docs-protection.ts`, `docs-reference.ts`, `site.ts`).
- [ ] Pages and parts: `app/(site)/page.tsx`, `app/(site)/_components/snippets.ts`, `app/(site)/captchas/samples/page.tsx`, `app/(site)/docs/_components/reference-sections.tsx`, `app/_components/captcha-illustration.tsx`, a preview in `components/ui/` with its texts in `components/ui/ui-text.ts`, and the `/demo` sections in `app/demo/_components/`.
- [ ] Media in `public/captchas/`, and the samples: `go run ./cmd/samples -out ../../../wau-dashboard/public/captchas/samples` in `services/challenge`.
- [ ] `lib/changelog.ts`.

Release order:

1. wau-services: merge to `main`. The release builds the images and Argo CD runs the migration, then
   the new code, so the service knows the type before any client asks for it.
2. The SDKs: publish `@waucaptcha-sdk/client`, `@waucaptcha-sdk/server` and `@waucaptcha-sdk/react` to npm
   (the `publish` workflow of wau-sdk-js), and the Java SDK to Maven Central.
3. wau-dashboard: point `lib/sdk-cdn.ts` at the new client release (`node scripts/sri.mjs <version>`
   in wau-sdk-js), then merge to `main`.
