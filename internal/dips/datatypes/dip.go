package datatypes

import (
	"time"

	"github.com/google/uuid"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db"
)

// DIP represents a DIP.
type DIP struct {
	DBID         int
	UUID         uuid.UUID
	DocKey       string
	Status       enums.DIPStatus
	ErrorMessage string
	CreatedAt    time.Time
	StartedAt    time.Time
	CompletedAt  time.Time
	ObjectKey    string
}

// NewDIP returns a new DIP instance from the given database DIP.
func NewDIP(d *db.DIP) *DIP {
	if d == nil {
		return nil
	}

	return &DIP{
		DBID:         d.ID,
		UUID:         d.UUID,
		DocKey:       d.DocKey,
		Status:       d.Status,
		ErrorMessage: d.ErrorMessage,
		CreatedAt:    d.CreatedAt,
		StartedAt:    d.StartedAt,
		CompletedAt:  d.CompletedAt,
		ObjectKey:    d.ObjectKey,
	}
}

// Goa returns the API representation of the DIP.
func (d *DIP) Goa() *goadips.ShowResult {
	if d == nil {
		return nil
	}

	re := &goadips.ShowResult{
		ID:        goadips.DIPID(d.UUID.String()),
		DocKey:    goadips.DocKey(d.DocKey),
		Status:    goadips.DIPStatus(d.Status.String()),
		CreatedAt: goadips.DateTime(d.CreatedAt.Format(time.RFC3339)),
	}

	if d.ErrorMessage != "" {
		re.ErrorMessage = new(d.ErrorMessage)
	}
	if !d.StartedAt.IsZero() {
		re.StartedAt = new(goadips.DateTime(d.StartedAt.Format(time.RFC3339)))
	}
	if !d.CompletedAt.IsZero() {
		re.CompletedAt = new(goadips.DateTime(d.CompletedAt.Format(time.RFC3339)))
	}
	if d.ObjectKey != "" {
		re.ObjectKey = new(goadips.ObjectKey(d.ObjectKey))
	}

	return re
}
