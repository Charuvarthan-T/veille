# Veille Engineering Documentation

## 1. Product purpose

Veille is a personal backend service that monitors Codeforces and CodeChef contests and sends a single email when a contest becomes live. It persists contest state in PostgreSQL and delivers notifications through Resend.

It deliberately does not provide a website, mobile app, authentication, multi-user accounts, or WhatsApp delivery. Its only job is reliable active-contest detection and one-time email notification.

## 2. System responsibilities

- `cmd/veille`: process entrypoint. Loads configuration, opens the database, applies migrations, wires dependencies, then either runs once (`-once`) or starts the local daemon scheduler.
- `internal/runner`: one-shot sync+notify pass used by GitHub Actions and manual testing.
- `internal/config`: reads environment variables and fails fast on missing or invalid values.
- `internal/domain`: shared contest and notification models, including lifecycle status derivation.
- `internal/source`: contest source interface plus Codeforces and CodeChef adapters.
- `internal/syncer`: pulls contests from sources, upserts them, refreshes statuses, and ensures active-notification rows exist.
- `internal/store`: persistence contracts; `internal/store/postgres` is the PostgreSQL implementation.
- `internal/notify`: active-contest eligibility, message formatting, notification orchestration, and channel abstractions.
- `internal/notify/resend`: Resend email sender.
- `internal/schedule`: periodic job runner with cooperative cancellation.
- `internal/clock`: clock abstraction for deterministic tests.
- `internal/migrate`: applies SQL migrations with goose.
- `migrations`: schema definitions owned by the database layer.

## 3. Contest collection

Each platform implements `source.ContestSource` with `Name`, `Platform`, and `FetchContests`.

- Codeforces uses the public `contest.list` API and keeps contests in `BEFORE` or `CODING` phase whose end time is still in the future.
- CodeChef uses the public `api/list/contests/all` endpoint and reads both `present_contests` and `future_contests`.

Contest lifecycle is derived from UTC timestamps:

```text
now < start_time               → upcoming
start_time <= now < end_time     → running
now >= end_time                  → finished
```

## 4. Database design

Two tables enforce the core invariants.

`contests`

- Unique constraint on `(platform, external_id)`
- Status check constraint for `upcoming`, `running`, `finished`, `cancelled`

`notifications`

- Unique constraint on `(contest_id, channel, kind)` so each contest has one active email
- Kind is `contest_started`
- Channel is `email` only

## 5. Synchronization

On each collect tick the syncer:

1. Fetches relevant contests from each source independently
2. Upserts every contest by `(platform, external_id)` with the current lifecycle status
3. Ensures a `contest_started` email notification exists with `due_at = start_time`
4. Refreshes stored statuses for contests transitioning between upcoming, running, and finished

If a contest is already running when first discovered, `due_at` is in the past and the notify job sends immediately.

## 6. Notification engineering

A notification is due when:

- `now >= due_at` (contest start time)
- `start_time <= now < end_time` (contest is running)
- the row is not already `sent`

Dispatch flow:

1. Release stale `sending` claims
2. Claim due rows with `FOR UPDATE SKIP LOCKED`
3. Re-check eligibility
4. Send through Resend
5. Persist `sent` or `failed`

The unique `(contest_id, channel, kind)` constraint prevents duplicate active emails for the same contest.

## 7. Configuration

Required environment variables:

- `DATABASE_URL`
- `RESEND_API_KEY`
- `EMAIL_FROM`
- `EMAIL_TO`

Optional with defaults:

- `TIMEZONE` (default `Asia/Kolkata`)
- `COLLECT_INTERVAL` (default `15m`)
- `NOTIFY_INTERVAL` (default `1m`)
- `HTTP_TIMEOUT` (default `30s`)
- `SHUTDOWN_TIMEOUT` (default `20s`)
- `NOTIFICATION_MAX_ATTEMPTS` (default `5`)

## 8. Running locally

Daemon mode (continuous scheduler):

```bash
cp .env.example .env
docker compose up -d db
go run ./cmd/veille
```

One-shot mode (same path as GitHub Actions):

```bash
go run ./cmd/veille -once
```

For host-run development against Compose Postgres:

```env
DATABASE_URL=postgres://veille:veille@127.0.0.1:5433/veille?sslmode=disable
```

## 9. Deployment notes

### GitHub Actions (production)

The workflow in `.github/workflows/veille.yml` runs every 5 minutes on `ubuntu-latest` and executes:

```bash
go test ./...
go build -o bin/veille ./cmd/veille
./bin/veille -once
```

Store secrets in the repository settings:

- `DATABASE_URL` (Neon PostgreSQL)
- `RESEND_API_KEY`
- `EMAIL_FROM`
- `EMAIL_TO`

Set `TIMEZONE=Asia/Kolkata` in the workflow env block. Never commit `.env` or secrets to the public repository.

### Docker (optional local/server)

The container entrypoint is `/app/veille`. Pass `-once` for a single pass or run without flags for daemon mode. Migrations run on startup.

Provide all required environment variables at runtime. Do not bake secrets into the image.

## 10. Testing

```bash
go test ./...
go build ./cmd/veille
docker build -t veille:local .
```

Unit tests use httptest for Codeforces, CodeChef, and Resend. They do not call live external APIs.
