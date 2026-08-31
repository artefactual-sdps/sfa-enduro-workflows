package persistence_test

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

func TestOpenSQLite(t *testing.T) {
	db, err := persistence.Open("sqlite3", "file:dip-open?mode=memory")
	assert.NilError(t, err)
	defer db.Close()
	assert.NilError(t, db.PingContext(t.Context()))
}

func TestOpenSQLiteConfiguresPragmas(t *testing.T) {
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "dips.sqlite")) + "?mode=rwc"
	db, err := persistence.Open("sqlite3", dsn)
	assert.NilError(t, err)
	defer db.Close()

	pragmas := map[string]int{
		"foreign_keys": 1,
		"busy_timeout": 1000,
		"synchronous":  0,
		"temp_store":   2,
	}
	for pragma, want := range pragmas {
		var got int
		err := db.QueryRowContext(t.Context(), "PRAGMA "+pragma).Scan(&got)
		assert.NilError(t, err)
		assert.Equal(t, got, want, "PRAGMA %s", pragma)
	}

	var journalMode string
	err = db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journalMode)
	assert.NilError(t, err)
	assert.Equal(t, journalMode, "wal")
}

func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	_, err := persistence.Open("postgres", "dsn")
	assert.ErrorContains(t, err, `database driver "postgres" not supported`)
}
