package client_test

import (
	"context"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence"
	entclient "github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence/ent/client"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence/ent/db"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence/ent/db/enttest"
)

func setUpClient(t *testing.T) (*db.Client, persistence.Service) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name())
	entc := enttest.Open(t, "sqlite3", dsn)
	ps := entclient.New(entc)
	t.Cleanup(func() {
		_ = entc.Close()
	})

	return entc, ps
}

func TestCreateSIP(t *testing.T) {
	t.Parallel()

	name := "test.zip"
	checksum := "a58b0193fcd0b85b1c85ca07899e063d"

	type test struct {
		name        string
		sipName     string
		sipChecksum string
		initialData func(context.Context, *testing.T, *db.Client)
		wantErr     string
	}

	for _, tt := range []test{
		{
			name:        "Creates a SIP",
			sipName:     name,
			sipChecksum: checksum,
		},
		{
			name:    "Fails to create a SIP (missing checksum)",
			sipName: name,
			wantErr: "CreateSIP: checksum field is required",
		},
		{
			name:        "Fails to create a SIP (missing name)",
			sipChecksum: checksum,
			wantErr:     "CreateSIP: name field is required",
		},
		{
			name:        "Fails to create a SIP (duplicated checksum)",
			sipName:     name,
			sipChecksum: checksum,
			initialData: func(ctx context.Context, t *testing.T, entc *db.Client) {
				err := entc.SIP.Create().
					SetName("another.zip").
					SetChecksum(checksum).
					Exec(ctx)
				if err != nil {
					t.Fatalf("Couldn't create initial data: %v", err)
				}
			},
			wantErr: "CreateSIP: there is already a SIP with the same checksum",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			entc, ps := setUpClient(t)

			if tt.initialData != nil {
				tt.initialData(ctx, t, entc)
			}

			err := ps.CreateSIP(ctx, tt.sipName, tt.sipChecksum)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)
		})
	}
}
