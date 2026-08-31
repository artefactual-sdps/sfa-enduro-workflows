package persistence

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Wait up to five minutes if another process is already holding the lock.
const lockTimeout = 5 * time.Minute

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrateLogger wraps a logr.Logger instance to implement the migrate.Logger
// interface.
type migrateLogger struct {
	logger logr.Logger
}

func (l *migrateLogger) Printf(format string, a ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, a...), "\r\n")
	l.logger.V(2).Info(msg)
}

func (l *migrateLogger) Verbose() bool {
	return false
}

func Migrate(logger logr.Logger, db *sql.DB) error {
	src, err := migrationSource()
	if err != nil {
		return fmt.Errorf("load migration sources: %w", err)
	}

	return up(logger, db, src)
}

func migrationSource() (source.Driver, error) {
	return iofs.New(migrationFS, "migrations")
}

// up migrates the database.
func up(logger logr.Logger, db *sql.DB, src source.Driver) error {
	m, err := newMigrate(logger, db, src)
	if err != nil {
		return fmt.Errorf("error creating golang-migrate object: %v", err)
	}

	if err := doMigrate(m); err != nil {
		return fmt.Errorf("error during database migration: %v", err)
	}

	return nil
}

// newMigrate builds the golang-migrate object.
func newMigrate(logger logr.Logger, db *sql.DB, src source.Driver) (*migrate.Migrate, error) {
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return nil, fmt.Errorf("error creating migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("source", src, "driver", driver)
	if err != nil {
		return nil, fmt.Errorf("error creating migrate instance: %w", err)
	}

	m.LockTimeout = lockTimeout

	// Log migration steps.
	m.Log = &migrateLogger{logger: logger}

	return m, nil
}

func doMigrate(m *migrate.Migrate) error {
	err := m.Up()
	if err == nil || err == migrate.ErrNoChange {
		return nil
	}

	if os.IsNotExist(err) {
		_, dirty, verr := m.Version()
		if verr != nil {
			return verr
		}
		if dirty {
			return err
		}
		return nil
	}

	return err
}
