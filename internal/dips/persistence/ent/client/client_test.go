package client_test

import (
	"fmt"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence"
	entclient "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/client"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db/enttest"
)

var dipUUID = uuid.MustParse("d61fe7ac-a9a9-42b8-a067-3f43d148f48e")

func setUpClient(t *testing.T, logger logr.Logger) (*db.Client, persistence.Service) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	entc := enttest.Open(t, "sqlite3", dsn)
	t.Cleanup(func() { entc.Close() })

	c := entclient.New(logger, entc)

	return entc, c
}

// setUpMockClient uses sqlmock because SQLite cannot execute FOR UPDATE.
func setUpMockClient(t *testing.T, logger logr.Logger) (persistence.Service, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	assert.NilError(t, err)
	entc := db.NewClient(db.Driver(entsql.OpenDB(dialect.MySQL, sqlDB)))
	t.Cleanup(func() { entc.Close() })

	c := entclient.New(logger, entc)

	return c, mock
}
