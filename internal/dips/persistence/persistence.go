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

type GetOrCreateRequest struct {
	UUID        uuid.UUID
	DocKey      string
	IgnoreCache bool
	Retention   time.Duration
}

type DIPUpdater func(*datatypes.DIP) (*datatypes.DIP, error)

type Service interface {
	GetOrCreateDIP(context.Context, GetOrCreateRequest) (*datatypes.DIP, error)
	ReadDIP(context.Context, uuid.UUID) (*datatypes.DIP, error)
	UpdateDIP(context.Context, uuid.UUID, DIPUpdater) (*datatypes.DIP, error)
	DeleteDIP(context.Context, uuid.UUID) error
}
