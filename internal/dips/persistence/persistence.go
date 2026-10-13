package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
)

var (
	// ErrNotFound is returned when a resource cannot be found.
	ErrNotFound = errors.New("not found error")

	// ErrNotValid is returned when the provided data is invalid.
	ErrNotValid = errors.New("invalid data error")

	// ErrInternal is returned when an internal error occurs.
	ErrInternal = errors.New("internal error")
)

type RequestDIPParams struct {
	// UUID identifies the new DIP if no existing DIP is reused.
	UUID uuid.UUID
	// DocKey identifies the ACTApro document to generate a DIP for.
	DocKey string
	// IgnoreCache skips cached DIPs. Queued or in-progress DIPs are still reused.
	IgnoreCache bool
	// Retention only applies when reusing a cached DIP. It can extend the expiry
	// or remove it when zero, but never shorten it.
	Retention time.Duration
}

type DIPUpdater func(*datatypes.DIP) (*datatypes.DIP, error)

type Service interface {
	RequestDIP(context.Context, RequestDIPParams) (*datatypes.DIP, error)
	ReadDIP(context.Context, uuid.UUID) (*datatypes.DIP, error)
	UpdateDIP(context.Context, uuid.UUID, DIPUpdater) (*datatypes.DIP, error)
	DeleteDIP(context.Context, uuid.UUID) error
}
