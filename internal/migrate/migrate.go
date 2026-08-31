package migrate

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
)

func Up(db *sql.DB, dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("migrations directory: %w", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
