package datatypes

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
)

func TestNewDIP(t *testing.T) {
	t.Parallel()

	dipUUID := uuid.MustParse("d61fe7ac-a9a9-42b8-a067-3f43d148f48e")
	createdAt := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, time.August, 31, 8, 30, 0, 0, time.UTC)
	completedAt := time.Date(2026, time.August, 31, 8, 35, 0, 0, time.UTC)

	for _, tt := range []struct {
		name string
		dip  *db.DIP
		want *DIP
	}{
		{
			name: "Converts nil database DIP to nil",
		},
		{
			name: "Converts database DIP with all fields",
			dip: &db.DIP{
				ID:           42,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusDone,
				ErrorMessage: "rendering recovered",
				CreatedAt:    createdAt,
				StartedAt:    startedAt,
				CompletedAt:  completedAt,
				ObjectKey:    "dips/CH-000001.zip",
			},
			want: &DIP{
				DBID:         42,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusDone,
				ErrorMessage: "rendering recovered",
				CreatedAt:    createdAt,
				StartedAt:    startedAt,
				CompletedAt:  completedAt,
				ObjectKey:    "dips/CH-000001.zip",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewDIP(tt.dip)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestDIPGoa(t *testing.T) {
	t.Parallel()

	dipUUID := uuid.MustParse("d61fe7ac-a9a9-42b8-a067-3f43d148f48e")
	createdAt := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, time.August, 31, 8, 30, 0, 0, time.UTC)
	completedAt := time.Date(2026, time.August, 31, 8, 35, 0, 0, time.UTC)

	for _, tt := range []struct {
		name string
		dip  *DIP
		want *goadips.ShowResult
	}{
		{
			name: "Converts nil DIP to nil Goa result",
		},
		{
			name: "Converts DIP with missing optional fields",
			dip: &DIP{
				UUID:      dipUUID,
				DocKey:    "CH-000001",
				Status:    enums.DIPStatusQueued,
				CreatedAt: createdAt,
			},
			want: &goadips.ShowResult{
				ID:        goadips.DIPID("d61fe7ac-a9a9-42b8-a067-3f43d148f48e"),
				DocKey:    goadips.DocKey("CH-000001"),
				Status:    goadips.DIPStatus("queued"),
				CreatedAt: goadips.DateTime("2026-08-31T08:00:00Z"),
			},
		},
		{
			name: "Converts DIP with all optional fields",
			dip: &DIP{
				DBID:         42,
				UUID:         dipUUID,
				DocKey:       "CH-000001",
				Status:       enums.DIPStatusFailed,
				ErrorMessage: "rendering failed",
				CreatedAt:    createdAt,
				StartedAt:    startedAt,
				CompletedAt:  completedAt,
				ObjectKey:    "dips/CH-000001.zip",
			},
			want: &goadips.ShowResult{
				ID:           goadips.DIPID("d61fe7ac-a9a9-42b8-a067-3f43d148f48e"),
				DocKey:       goadips.DocKey("CH-000001"),
				Status:       goadips.DIPStatus("failed"),
				ErrorMessage: new("rendering failed"),
				CreatedAt:    goadips.DateTime("2026-08-31T08:00:00Z"),
				StartedAt:    new(goadips.DateTime("2026-08-31T08:30:00Z")),
				CompletedAt:  new(goadips.DateTime("2026-08-31T08:35:00Z")),
				ObjectKey:    new(goadips.ObjectKey("dips/CH-000001.zip")),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.dip.Goa()
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
