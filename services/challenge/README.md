# wau-challenge

Issues CAPTCHA challenges. `image_distort` is a 5-character code from `A-Z`, `a-z`, `0-9`
rendered with rotation, wave distortion, crossing lines, noise and a light blur.

## API

Contract: `common/proto/challenge/v1`.

| Transport | Endpoint | Notes |
|-----------|----------|-------|
| HTTP `:8000` | `GET /challenge/new?site_key=` | Public. Returns `challenge_id`, `challenge_type`, `image` (base64 PNG), `expires_in`. |
| gRPC `:9000` | `ChallengeService.CheckAnswer` | Internal. Consumes the challenge (one attempt) and checks the answer, ignoring case. |

## Run

Requires `infra` (PostgreSQL + Redis Cluster). Local settings live in this service's own
`.env` (git-ignored, loaded from the working directory; real environment variables win):

```dotenv
KRATOS_DATABASE_SOURCE=postgres://<user>:<password>@127.0.0.1:5432/<db>?sslmode=disable
KRATOS_REDIS_PASSWORD=<password>
```

```bash
go run ./cmd/challenge
```
