package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/google/go-cmp/cmp/cmpopts"
	goa "goa.design/goa/v3/pkg"
	"gotest.tools/v3/assert"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
)

func TestServerErrorHandlerLogsAndSanitizesInternalError(t *testing.T) {
	t.Parallel()

	cause := errors.New("database unavailable")
	returned := goadips.MakeInternalError(fmt.Errorf("load DIP data: %v", cause))
	var logged string
	logger := funcr.New(
		func(_, args string) { logged = args },
		funcr.Options{},
	)
	endpoint := goadips.WrapShowEndpoint(
		func(context.Context, any) (any, error) { return nil, returned },
		newDIPsServerInterceptors(logger),
	)

	_, err := endpoint(context.Background(), "payload")

	var serr *goa.ServiceError
	assert.Assert(t, errors.As(err, &serr))
	assert.DeepEqual(t, serr, &goa.ServiceError{
		Name:    "internal_error",
		ID:      returned.ID,
		Message: apiInternalErrorMsg,
		Fault:   true,
	}, cmpopts.IgnoreUnexported(goa.ServiceError{}))
	assert.Assert(t, strings.Contains(logged, "load DIP data: database unavailable"))
	assert.Assert(t, strings.Contains(logged, `"service"="DIPs"`))
	assert.Assert(t, strings.Contains(logged, `"method"="Show"`))
	assert.Assert(t, strings.Contains(logged, fmt.Sprintf(`"error_id"=%q`, serr.ID)))
}

func TestServerErrorHandlerClassifiesRawError(t *testing.T) {
	t.Parallel()

	cause := errors.New("unexpected failure")
	var logged string
	logger := funcr.New(
		func(_, args string) { logged = args },
		funcr.Options{},
	)
	endpoint := goadips.WrapLivezEndpoint(
		func(context.Context, any) (any, error) { return nil, cause },
		newDIPsServerInterceptors(logger),
	)

	_, err := endpoint(context.Background(), "payload")

	var serr *goa.ServiceError
	assert.Assert(t, errors.As(err, &serr))
	assert.DeepEqual(t, serr, &goa.ServiceError{
		Name:    "internal_error",
		Message: apiInternalErrorMsg,
		Fault:   true,
	},
		cmpopts.IgnoreFields(goa.ServiceError{}, "ID"),
		cmpopts.IgnoreUnexported(goa.ServiceError{}),
	)
	assert.Assert(t, strings.Contains(logged, cause.Error()))
}

func TestServerErrorHandlerPassesThroughOtherFault(t *testing.T) {
	t.Parallel()

	cause := errors.New("unexpected dependency fault")
	returned := goa.NewServiceError(cause, "dependency_fault", false, false, true)
	var logged string
	logger := funcr.New(
		func(_, args string) { logged = args },
		funcr.Options{},
	)
	endpoint := goadips.WrapLivezEndpoint(
		func(context.Context, any) (any, error) {
			return nil, returned
		},
		newDIPsServerInterceptors(logger),
	)

	_, err := endpoint(context.Background(), "payload")

	assert.DeepEqual(t, err, returned, cmpopts.IgnoreUnexported(goa.ServiceError{}))
	assert.Equal(t, logged, "")
}

func TestServerErrorHandlerLogsTimeoutOnce(t *testing.T) {
	t.Parallel()

	var logs []string
	logger := funcr.New(
		func(_, args string) { logs = append(logs, args) },
		funcr.Options{},
	)
	interceptors := newDIPsServerInterceptors(logger)
	interceptors.operationTimeout = time.Nanosecond
	endpoint := goadips.WrapLivezEndpoint(
		func(ctx context.Context, _ any) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		interceptors,
	)

	_, err := endpoint(context.Background(), "payload")

	var serr *goa.ServiceError
	assert.Assert(t, errors.As(err, &serr))
	assert.DeepEqual(t, serr, &goa.ServiceError{
		Name:    "internal_error",
		Message: apiInternalErrorMsg,
		Timeout: true,
		Fault:   true,
	},
		cmpopts.IgnoreFields(goa.ServiceError{}, "ID"),
		cmpopts.IgnoreUnexported(goa.ServiceError{}),
	)
	assert.Equal(t, len(logs), 1)
	assert.Assert(t, strings.Contains(logs[0], apiOperationTimeoutMsg))
	assert.Assert(t, strings.Contains(logs[0], fmt.Sprintf(`"error_id"=%q`, serr.ID)))
}

func TestOperationTimeoutMapsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	endpoint := goadips.WrapCreateEndpoint(
		func(ctx context.Context, _ any) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		newTestDIPsServerInterceptors(time.Nanosecond),
	)

	_, err := endpoint(context.Background(), "payload")
	var serr *goa.ServiceError
	assert.Assert(t, errors.As(err, &serr))
	assert.DeepEqual(t, serr, &goa.ServiceError{
		Name:    "internal_error",
		Message: apiInternalErrorMsg,
		Timeout: true,
		Fault:   true,
	},
		cmpopts.IgnoreFields(goa.ServiceError{}, "ID"),
		cmpopts.IgnoreUnexported(goa.ServiceError{}),
	)
}

func TestOperationTimeoutDIPsRules(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name         string
		wrap         func(goa.Endpoint, *dipsServerInterceptors) goa.Endpoint
		wantDeadline bool
	}{
		{
			name: "livez gets deadline",
			wrap: func(next goa.Endpoint, interceptors *dipsServerInterceptors) goa.Endpoint {
				return goadips.WrapLivezEndpoint(next, interceptors)
			},
			wantDeadline: true,
		},
		{
			name: "create gets deadline",
			wrap: func(next goa.Endpoint, interceptors *dipsServerInterceptors) goa.Endpoint {
				return goadips.WrapCreateEndpoint(next, interceptors)
			},
			wantDeadline: true,
		},
		{
			name: "show gets deadline",
			wrap: func(next goa.Endpoint, interceptors *dipsServerInterceptors) goa.Endpoint {
				return goadips.WrapShowEndpoint(next, interceptors)
			},
			wantDeadline: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			endpoint := tt.wrap(
				deadlineEndpoint(t, tt.wantDeadline),
				newDIPsServerInterceptors(logr.Discard()),
			)

			res, err := endpoint(context.Background(), "payload")
			assert.NilError(t, err)
			assert.Equal(t, res, "ok")
		})
	}
}

func deadlineEndpoint(t *testing.T, wantDeadline bool) goa.Endpoint {
	t.Helper()

	return func(ctx context.Context, _ any) (any, error) {
		deadline, ok := ctx.Deadline()
		assert.Equal(t, ok, wantDeadline)
		if wantDeadline {
			assert.Assert(t, time.Until(deadline) > 0)
			assert.Assert(t, time.Until(deadline) <= defaultAPIOperationTimeout)
		}
		return "ok", nil
	}
}

func newTestDIPsServerInterceptors(timeout time.Duration) *dipsServerInterceptors {
	i := newServerInterceptors(logr.Discard())
	i.operationTimeout = timeout

	return &dipsServerInterceptors{serverInterceptors: i}
}
