package persistence

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

func Open(driver, dsn string) (*sql.DB, error) {
	switch driver {
	case "mysql":
		return openMySQL(dsn)
	case "sqlite3":
		return openSQLite(dsn)
	default:
		return nil, fmt.Errorf("database driver %q not supported", driver)
	}
}

func openMySQL(dsn string) (*sql.DB, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse MySQL DSN: %w", err)
	}
	config.Collation = "utf8mb4_unicode_ci"
	config.Loc = time.UTC
	config.ParseTime = true
	config.MultiStatements = true
	config.Params = map[string]string{"time_zone": "'+00:00'"}

	connector, err := mysql.NewConnector(config)
	if err != nil {
		return nil, fmt.Errorf("create MySQL connector: %w", err)
	}

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func openSQLite(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	connections := runtime.NumCPU()
	db.SetMaxOpenConns(connections)
	db.SetMaxIdleConns(connections)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	pragmas := []string{
		"journal_mode=WAL",
		"synchronous=OFF",
		"foreign_keys=ON",
		"temp_store=MEMORY",
		"busy_timeout=1000", // Used with "_txlock=immediate" or "BEGIN IMMEDIATE".
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec("PRAGMA " + pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set SQLite pragma %q: %w", pragma, err)
		}
	}

	return db, nil
}
