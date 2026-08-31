# Veille Engineering Documentation

## 1. Product purpose

Veille is a personal backend service that continuously watches competitive programming contest calendars and reminds you 24 hours before each contest starts. It currently covers Codeforces and CodeChef, persists contest state in PostgreSQL, and sends reminders through WhatsApp (Twilio) and email (Resend).

It deliberately does not provide a website, mobile app, authentication, multi-user accounts, analytics, recommendations, or any AI features. Its only job is reliable contest discovery and reminder delivery.

## 2. System responsibilities

- `cmd/veille`: process entrypoint. Loads configuration, opens the database, applies migrations, wires dependencies, starts schedulers, and handles graceful shutdown.
- `internal/config`: reads environment variables and fails fast on missing or invalid values.
- `internal/domain`: shared contest and notification models used across packages.
- `internal/source`: contest source interface plus Codeforces and CodeChef adapters.
- `internal/syncer`: pulls contests from sources, upserts them, and ensures reminder rows exist.
- `internal/store`: persistence contracts; `internal/store/postgres` is the PostgreSQL implementation.
- `internal/notify`: due-window rules, message formatting, notification orchestration, and channel abstractions.
- `internal/notify/twilio` and `internal/notify/resend`: provider-specific senders.
- `internal/schedule`: periodic job runner with cooperative cancellation.
- `internal/clock`: clock abstraction for deterministic tests.
- `internal/migrate`: applies SQL migrations with goose.
- `migrations`: schema definitions owned by the database layer.

## 3. Contest collection

Each platform implements `source.ContestSource` with `Name`, `Platform`, and `FetchUpcoming`.

- Codeforces uses the public `contest.list` API and keeps contests whose phase is `BEFORE` and whose start time is still in the future. Contests are normalized into the shared `domain.Contest` shape with a stable external ID, URL, UTC start/end times, duration, and status.
- CodeChef uses the public `api/list/contests/all` endpoint and reads `future_contests`. Start and end times are parsed from ISO fields when present, with legacy formats as fallback, then converted to UTC.

Source-specific JSON and HTTP details stay inside the adapter packages. The syncer and domain layers only see `domain.Contest` values. New platforms can be added by implementing the same interface and registering the adapter in `main`.

## 4. Database design

Two tables enforce the core invariants.

`contests`

- Primary key `id`
- Unique constraint on `(platform, external_id)` to prevent duplicate contests
- Status check constraint for `upcoming`, `running`, `finished`, `cancelled`
- Timestamps stored as `TIMESTAMPTZ`
- Indexes on `(status, start_time)` and `(platform, start_time)` for reminder eligibility and listing patterns

`notifications`

- Primary key `id`
- Foreign key to `contests(id)` with cascade delete
- Unique constraint on `(contest_id, channel, kind)` so each contest/channel reminder exists once
- Status check constraint for `pending`, `sending`, `sent`, `failed`
- Index on `(status, due_at)` for due claiming

The unique notification constraint is the durable idempotency key. Concurrent workers cannot create duplicate reminder rows for the same contest and channel.

## 5. Synchronization

On each collect tick the syncer:

1. Fetches upcoming contests from each source independently
2. Upserts every contest by `(platform, external_id)`
3. Updates mutable fields such as name, URL, start time, end time, duration, status, and `last_seen_at`
4. Ensures a `reminder_24h` notification row exists for WhatsApp and email with `due_at = start_time - REMINDER_LEAD`

Behavior notes:

- Newly discovered contests are inserted and reminders are created
- Already-known contests are updated in place
- Start-time or duration changes refresh `due_at` for reminders that are not yet `sent`
- A temporary source failure fails only that source; other sources continue
- Duplicate source payloads collapse through the unique contest key
- A contest disappearing from one scrape is not treated as cancellation

## 6. Notification engineering

Reminders are due when `now >= start_time - 24h` and `now < start_time`. The due-window strategy intentionally avoids exact equality checks. If the scheduler is delayed, any time inside the configured window still qualifies.

Dispatch flow:

1. Release stale `sending` claims left by crashed workers
2. Claim due rows with `FOR UPDATE SKIP LOCKED`, moving them to `sending` and incrementing `attempt_count`
3. Re-check contest eligibility and window rules
4. Send through the channel sender
5. Persist `sent` or `failed`

Because claiming mutates durable state before send, overlapping scheduler ticks do not double-send the same contest/channel reminder once it reaches `sent`. Failed sends remain retryable until `NOTIFICATION_MAX_ATTEMPTS` is exhausted.

## 7. Reliability

- Source and provider calls use HTTP timeouts and return explicit errors
- Malformed responses fail the fetch/send for that dependency without crashing the process
- Notification claims use transactions and row locks to survive concurrent ticks
- Successful delivery is recorded in PostgreSQL, so process restarts do not resend completed reminders
- Scheduler delays are absorbed by the due window
- Provider failures mark the notification `failed` and leave it eligible for later retry
- Stale `sending` rows are released back to a retryable state after a timeout

## 8. WhatsApp

WhatsApp delivery is implemented behind `notify.ChannelSender` in `internal/notify/twilio`.

Required configuration:

- `TWILIO_ACCOUNT_SID`: Twilio Account SID
- `TWILIO_AUTH_TOKEN`: Twilio Auth Token
- `TWILIO_WHATSAPP_FROM`: Twilio WhatsApp-enabled from address, typically `whatsapp:+...`
- `WHATSAPP_TO`: destination WhatsApp address, typically `whatsapp:+...`

The rest of the application depends only on the channel interface, not Twilio HTTP details.

## 9. Email

Email delivery is implemented behind `notify.ChannelSender` in `internal/notify/resend`.

Required configuration:

- `RESEND_API_KEY`: Resend API key
- `EMAIL_FROM`: verified sender address or domain identity in Resend
- `EMAIL_TO`: destination mailbox

Provider details remain isolated in the Resend package.

## 10. Time handling

All persisted contest and notification timestamps are UTC (`TIMESTAMPTZ`). The application clock also normalizes to UTC.

Presentation and reminder copy convert start/end times into the configured timezone, defaulting to `Asia/Kolkata`. Timezone conversion lives in message formatting rather than being scattered through collectors or repositories.

## 11. Configuration

There is exactly one runtime env file for local secrets: `.env`.

Committed template: `.env.example`

Required and important variables:

- `DATABASE_URL`
- `TIMEZONE`
- `COLLECT_INTERVAL`
- `NOTIFY_INTERVAL`
- `REMINDER_LEAD`
- `REMINDER_WINDOW`
- `HTTP_TIMEOUT`
- `SHUTDOWN_TIMEOUT`
- `NOTIFICATION_MAX_ATTEMPTS`
- `TWILIO_ACCOUNT_SID`
- `TWILIO_AUTH_TOKEN`
- `TWILIO_WHATSAPP_FROM`
- `WHATSAPP_TO`
- `RESEND_API_KEY`
- `EMAIL_FROM`
- `EMAIL_TO`

Startup validation rejects missing required secrets and invalid timezone values.

## 12. Testing

Tests focus on behavior:

- Contest identity and normalization
- Source adapters with local HTTP test servers
- Sync insert/update behavior and isolated source failure handling
- Due-window and duplicate-send prevention
- Timezone formatting for Asia/Kolkata
- Provider success and failure paths
- Fail-fast configuration validation

Ordinary unit tests do not call real Codeforces, CodeChef, Twilio, or Resend APIs.

## 13. Graceful shutdown

`main` listens for `SIGINT` and `SIGTERM`. On signal:

1. The root context cancels
2. Scheduler loops stop accepting new ticks
3. In-flight job goroutines are waited on
4. A shutdown timeout bounds the wait
5. The database connection is closed

This keeps ownership of goroutines and resources explicit.

## 14. CI

GitHub Actions runs on pushes and pull requests to `main` and verifies:

- `gofmt`
- `go vet`
- `golangci-lint`
- `go test`
- `go build`

A failing check means the project is not considered healthy.

## 15. Engineering principles

- Single Responsibility Principle: each package owns one concern such as config, sync, persistence, or a single provider
- Separation of Concerns: collection, scheduling, and notification dispatch are independent jobs
- Dependency Inversion: syncer and orchestrator depend on store and source interfaces, not PostgreSQL or Twilio concretions
- Interface Segregation: channel senders expose only `Channel` and `Send`
- Encapsulation: Twilio and Resend credentials and HTTP shapes stay inside provider packages
- Composition: sources and senders are composed in `main` rather than inherited hierarchies
- DRY: reminder due calculation and message formatting are centralized
- KISS: one process, one database, no message broker
- YAGNI: no Redis, Kafka, Kubernetes, or multi-tenant features
- High cohesion / low coupling: adapters are swappable without touching contest domain logic
- Explicit dependencies: constructors take collaborators directly; no global mutable service locator
- Fail-fast configuration: invalid env stops startup before side effects
- Idempotency: unique notification keys plus claim/sent state prevent duplicate reminders
- Deterministic behavior: clock abstraction makes time-based tests stable
- Testability: fakes and httptest isolate external systems
- Proper resource ownership and graceful shutdown: context cancellation, wait groups, and DB close
- Observability: structured `slog` events for sync, dispatch, failures, and lifecycle
- Secure secret handling: secrets come from env, `.env` is ignored, logs avoid credentials

## 16. Running locally

Configure environment:

```bash
cp .env.example .env
```

Fill in Twilio, WhatsApp, Resend, and email values in `.env`.

Start PostgreSQL only:

```bash
docker compose up -d db
```

Run the service (applies migrations on startup):

```bash
go run ./cmd/veille
```

Or run app and database together:

```bash
docker compose up --build
```

Tests:

```bash
go test ./...
```

Formatting, vet, lint, and build:

```bash
gofmt -l .
go vet ./...
golangci-lint run ./...
go build -o bin/veille ./cmd/veille
```

## 17. External credentials checklist

Obtain and place these values in `.env` before real notifications will work:

- PostgreSQL connection string (`DATABASE_URL`), or use the Compose defaults
- Twilio Account SID
- Twilio Auth Token
- Twilio WhatsApp sender address (`TWILIO_WHATSAPP_FROM`)
- Your WhatsApp destination address (`WHATSAPP_TO`), enabled for the Twilio sandbox or production sender
- Resend API key
- Verified Resend from address (`EMAIL_FROM`)
- Destination email address (`EMAIL_TO`)
- Confirm `TIMEZONE=Asia/Kolkata` unless you intentionally change it

Real WhatsApp and email delivery have not been verified in this repository without your live credentials.
