package persistence_test

import (
	"testing"

	"github.com/go-logr/logr"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
)

func TestMigrateRejectsSQLite(t *testing.T) {
	db, err := persistence.Open("sqlite3", "file:dip-migrate?mode=memory&_fk=1")
	assert.NilError(t, err)
	defer db.Close()

	err = persistence.Migrate(logr.Discard(), db)
	assert.ErrorContains(t, err, "migrate driver")
}
