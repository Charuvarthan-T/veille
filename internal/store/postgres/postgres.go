package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/store"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	sql *sql.DB
}

func Open(ctx context.Context, databaseURL string) (*DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{sql: db}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.sql.PingContext(ctx)
}

func (d *DB) Close() error {
	return d.sql.Close()
}

func (d *DB) SQL() *sql.DB {
	return d.sql
}

var _ store.Store = (*DB)(nil)

func (d *DB) UpsertContest(ctx context.Context, contest domain.Contest) (domain.Contest, bool, error) {
	const query = `
INSERT INTO contests (
    platform, external_id, name, url, start_time, end_time, duration_seconds,
    status, first_seen_at, last_seen_at, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, NOW(), NOW()
)
ON CONFLICT (platform, external_id) DO UPDATE SET
    name = EXCLUDED.name,
    url = EXCLUDED.url,
    start_time = EXCLUDED.start_time,
    end_time = EXCLUDED.end_time,
    duration_seconds = EXCLUDED.duration_seconds,
    status = EXCLUDED.status,
    last_seen_at = EXCLUDED.last_seen_at,
    updated_at = NOW()
RETURNING
    id, platform, external_id, name, url, start_time, end_time, duration_seconds,
    status, first_seen_at, last_seen_at, created_at, updated_at,
    (xmax = 0) AS inserted
`
	var (
		out      domain.Contest
		duration int64
		inserted bool
	)
	err := d.sql.QueryRowContext(
		ctx,
		query,
		contest.Platform,
		contest.ExternalID,
		contest.Name,
		contest.URL,
		contest.StartTime.UTC(),
		contest.EndTime.UTC(),
		int64(contest.Duration.Seconds()),
		contest.Status,
		contest.FirstSeenAt.UTC(),
		contest.LastSeenAt.UTC(),
	).Scan(
		&out.ID,
		&out.Platform,
		&out.ExternalID,
		&out.Name,
		&out.URL,
		&out.StartTime,
		&out.EndTime,
		&duration,
		&out.Status,
		&out.FirstSeenAt,
		&out.LastSeenAt,
		&out.CreatedAt,
		&out.UpdatedAt,
		&inserted,
	)
	if err != nil {
		return domain.Contest{}, false, fmt.Errorf("upsert contest: %w", err)
	}
	out.Duration = time.Duration(duration) * time.Second
	return out, inserted, nil
}

func (d *DB) GetContest(ctx context.Context, id int64) (domain.Contest, error) {
	const query = `
SELECT id, platform, external_id, name, url, start_time, end_time, duration_seconds,
       status, first_seen_at, last_seen_at, created_at, updated_at
FROM contests
WHERE id = $1
`
	var (
		out      domain.Contest
		duration int64
	)
	err := d.sql.QueryRowContext(ctx, query, id).Scan(
		&out.ID,
		&out.Platform,
		&out.ExternalID,
		&out.Name,
		&out.URL,
		&out.StartTime,
		&out.EndTime,
		&duration,
		&out.Status,
		&out.FirstSeenAt,
		&out.LastSeenAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Contest{}, fmt.Errorf("contest %d not found", id)
	}
	if err != nil {
		return domain.Contest{}, fmt.Errorf("get contest: %w", err)
	}
	out.Duration = time.Duration(duration) * time.Second
	return out, nil
}

func (d *DB) EnsureReminder(ctx context.Context, contestID int64, channel domain.Channel, dueAt time.Time) error {
	const query = `
INSERT INTO notifications (contest_id, channel, kind, status, due_at, created_at, updated_at)
VALUES ($1, $2, 'reminder_24h', 'pending', $3, NOW(), NOW())
ON CONFLICT (contest_id, channel, kind) DO UPDATE SET
    due_at = EXCLUDED.due_at,
    updated_at = NOW()
WHERE notifications.status <> 'sent'
`
	_, err := d.sql.ExecContext(ctx, query, contestID, channel, dueAt.UTC())
	if err != nil {
		return fmt.Errorf("ensure reminder: %w", err)
	}
	return nil
}

func (d *DB) ClaimDue(ctx context.Context, now time.Time, limit int, maxAttempts int) ([]domain.Notification, error) {
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const query = `
WITH due AS (
    SELECT n.id
    FROM notifications n
    JOIN contests c ON c.id = n.contest_id
    WHERE n.status IN ('pending', 'failed')
      AND n.attempt_count < $2
      AND n.due_at <= $1
      AND c.start_time > $1
      AND c.status = 'upcoming'
    ORDER BY n.due_at ASC
    FOR UPDATE OF n SKIP LOCKED
    LIMIT $3
)
UPDATE notifications n
SET status = 'sending',
    attempt_count = n.attempt_count + 1,
    updated_at = NOW()
FROM due
WHERE n.id = due.id
RETURNING n.id, n.contest_id, n.channel, n.kind, n.status, n.due_at, n.sent_at,
          n.attempt_count, n.last_error, n.created_at, n.updated_at
`
	rows, err := tx.QueryContext(ctx, query, now.UTC(), maxAttempts, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []domain.Notification
	for rows.Next() {
		var (
			n      domain.Notification
			sentAt sql.NullTime
		)
		if err := rows.Scan(
			&n.ID,
			&n.ContestID,
			&n.Channel,
			&n.Kind,
			&n.Status,
			&n.DueAt,
			&sentAt,
			&n.AttemptCount,
			&n.LastError,
			&n.CreatedAt,
			&n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		if sentAt.Valid {
			t := sentAt.Time.UTC()
			n.SentAt = &t
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim tx: %w", err)
	}
	return items, nil
}

func (d *DB) MarkSent(ctx context.Context, id int64, sentAt time.Time) error {
	const query = `
UPDATE notifications
SET status = 'sent',
    sent_at = $2,
    last_error = '',
    updated_at = NOW()
WHERE id = $1
`
	_, err := d.sql.ExecContext(ctx, query, id, sentAt.UTC())
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (d *DB) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	const query = `
UPDATE notifications
SET status = 'failed',
    last_error = $2,
    updated_at = NOW()
WHERE id = $1
`
	_, err := d.sql.ExecContext(ctx, query, id, errMsg)
	if err != nil {
		return fmt.Errorf("mark notification failed: %w", err)
	}
	return nil
}

func (d *DB) ReleaseStaleSending(ctx context.Context, olderThan time.Time) (int64, error) {
	const query = `
UPDATE notifications
SET status = 'failed',
    last_error = 'stale sending claim released',
    updated_at = NOW()
WHERE status = 'sending'
  AND updated_at < $1
`
	res, err := d.sql.ExecContext(ctx, query, olderThan.UTC())
	if err != nil {
		return 0, fmt.Errorf("release stale sending: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}
