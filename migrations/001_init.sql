-- +goose Up
CREATE TABLE contests (
    id              BIGSERIAL PRIMARY KEY,
    platform        TEXT NOT NULL,
    external_id     TEXT NOT NULL,
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ NOT NULL,
    duration_seconds BIGINT NOT NULL,
    status          TEXT NOT NULL,
    first_seen_at   TIMESTAMPTZ NOT NULL,
    last_seen_at    TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT contests_platform_external_id_uidx UNIQUE (platform, external_id),
    CONSTRAINT contests_status_check CHECK (status IN ('upcoming', 'running', 'finished', 'cancelled')),
    CONSTRAINT contests_duration_nonneg CHECK (duration_seconds >= 0)
);

CREATE INDEX contests_status_start_time_idx ON contests (status, start_time);
CREATE INDEX contests_platform_start_time_idx ON contests (platform, start_time);

CREATE TABLE notifications (
    id              BIGSERIAL PRIMARY KEY,
    contest_id      BIGINT NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    channel         TEXT NOT NULL,
    kind            TEXT NOT NULL,
    status          TEXT NOT NULL,
    due_at          TIMESTAMPTZ NOT NULL,
    sent_at         TIMESTAMPTZ NULL,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notifications_contest_channel_kind_uidx UNIQUE (contest_id, channel, kind),
    CONSTRAINT notifications_channel_check CHECK (channel IN ('whatsapp', 'email')),
    CONSTRAINT notifications_kind_check CHECK (kind IN ('reminder_24h')),
    CONSTRAINT notifications_status_check CHECK (status IN ('pending', 'sending', 'sent', 'failed')),
    CONSTRAINT notifications_attempt_count_nonneg CHECK (attempt_count >= 0)
);

CREATE INDEX notifications_due_status_idx ON notifications (status, due_at);
CREATE INDEX notifications_contest_id_idx ON notifications (contest_id);

-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS contests;
