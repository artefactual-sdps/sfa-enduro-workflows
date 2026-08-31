package client_test

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

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
