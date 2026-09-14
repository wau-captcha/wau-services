# wau-verify

Turns solved challenges into one-time tokens and validates them for site backends.
Answers are checked by wau-challenge over gRPC.

## API

Contract: `common/proto/verify/v1`.

| HTTP `:8001` | Caller | Notes |
|--------------|--------|-------|
| `POST /challenge/verify` `{"challenge_id", "answer"}` | Site frontend | Returns `success`, `token`, `expires_in`. One attempt per challenge. |
| `POST /siteverify?token=` + `Authorization: Bearer <secret_key>` | Site backend | Returns `success`, `challenge_type`, `challenge_ts`. Each token is valid once. |

gRPC `:9001` serves the same methods.

## Run

Requires `infra` and a running wau-challenge. Local settings live in this service's own
`.env` (git-ignored, loaded from the working directory; real environment variables win):

```dotenv
KRATOS_DATABASE_SOURCE=postgres://<user>:<password>@127.0.0.1:5432/<db>?sslmode=disable
KRATOS_REDIS_PASSWORD=<password>
KRATOS_TOKEN_SECRET=<random secret, 32+ bytes>
```

```bash
go run ./cmd/verify
```
