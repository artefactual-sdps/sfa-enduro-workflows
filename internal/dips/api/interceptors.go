package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	goa "goa.design/goa/v3/pkg"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
)

const (
	defaultAPIOperationTimeout = 5 * time.Second
	apiOperationSlowThreshold  = 2 * time.Second
	apiOperationTimeoutMsg     = "API operation timed out"
	apiInternalErrorMsg        = "internal error"
)

type interceptorInfo interface {
	Service() string
	Method() string
	CallType() goa.InterceptorCallType
	RawPayload() any
}

type operationKey struct {
	service string
	method  string
}

type operationRule struct {
	timeout       time.Duration
	slowThreshold time.Duration
	skipTimeout   bool
	skipLogging   bool
}

// serverInterceptors is shared by all DIPs methods so internal-error handling,
// operation budgets, and logging stay consistent. The rule map keeps
// service/method exceptions explicit because Goa reports some body-returning
// endpoints as unary.
type serverInterceptors struct {
	logger           logr.Logger
	operationTimeout time.Duration
	slowThreshold    time.Duration
	rules            map[operationKey]operationRule
}

func newServerInterceptors(logger logr.Logger) *serverInterceptors {
	return &serverInterceptors{
		logger:           logger,
		operationTimeout: defaultAPIOperationTimeout,
		slowThreshold:    apiOperationSlowThreshold,
		rules:            map[operationKey]operationRule{},
	}
}

type dipsServerInterceptors struct {
	*serverInterceptors
}

func newDIPsServerInterceptors(logger logr.Logger) *dipsServerInterceptors {
	return &dipsServerInterceptors{serverInterceptors: newServerInterceptors(logger)}
}

func (i *dipsServerInterceptors) OperationTimeout(
	ctx context.Context,
	info *goadips.OperationTimeoutInfo,
	next goa.Endpoint,
) (any, error) {
	return i.handleOperationTimeout(ctx, info, next)
}

func (i *dipsServerInterceptors) ServerErrorHandler(
	ctx context.Context,
	info *goadips.ServerErrorHandlerInfo,
	next goa.Endpoint,
) (any, error) {
	return i.handleServerError(ctx, info, next)
}

func (i *serverInterceptors) handleServerError(
	ctx context.Context,
	info interceptorInfo,
	next goa.Endpoint,
) (any, error) {
	res, err := next(ctx, info.RawPayload())
	if err == nil {
		return res, nil
	}

	// Check ServiceError first because it also implements GoaErrorNamer. Only
	// the declared internal_error belongs to this handler; other service errors
	// already have their own Goa classification and must pass through unchanged.
	var serviceErr *goa.ServiceError
	if errors.As(err, &serviceErr) {
		if serviceErr.Name != "internal_error" {
			return res, err
		}
	} else {
		// Generated domain errors implement GoaErrorNamer without being
		// ServiceErrors. Their generated transport mappings are authoritative,
		// so preserve them instead of converting them to internal_error.
		var namedErr goa.GoaErrorNamer
		if errors.As(err, &namedErr) {
			return res, err
		}

		// An unnamed error is an unexpected implementation failure. Classify it
		// here so Goa consistently encodes it as the declared HTTP 500 response.
		serviceErr = goa.NewServiceError(
			err,
			"internal_error",
			false,
			false,
			true,
		)
	}

	i.logger.Error(
		err,
		"API internal error.",
		"service", info.Service(),
		"method", info.Method(),
		"error_id", serviceErr.ID,
	)

	// Preserve the Goa error ID and flags for correlation while ensuring that
	// the transport exposes no implementation details.
	sanitized := *serviceErr
	sanitized.Message = apiInternalErrorMsg

	return res, &sanitized
}

func (i *serverInterceptors) handleOperationTimeout(
	ctx context.Context,
	info interceptorInfo,
	next goa.Endpoint,
) (any, error) {
	rule := i.rule(info)
	if info.CallType() != goa.InterceptorUnary || rule.skipLogging {
		return next(ctx, info.RawPayload())
	}

	parentCtx := ctx
	timeout, hasTimeout := rule.timeout, !rule.skipTimeout && rule.timeout > 0
	if hasTimeout {
		// Apply a default budget to normal API methods. Goa reports
		// body-returning endpoints as unary, so methods that continue work
		// after the endpoint returns must be excluded by rule.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	start := time.Now()
	res, err := next(ctx, info.RawPayload())
	elapsed := time.Since(start)

	if hasTimeout &&
		errors.Is(ctx.Err(), context.DeadlineExceeded) &&
		!errors.Is(parentCtx.Err(), context.DeadlineExceeded) {
		// Only translate deadlines introduced here. Parent request cancellation
		// should keep its original error semantics.
		logErr := err
		if logErr == nil {
			logErr = ctx.Err()
		}
		return nil, goa.NewServiceError(
			fmt.Errorf("%s: %v", apiOperationTimeoutMsg, logErr),
			"internal_error",
			true,
			false,
			true,
		)
	}

	if elapsed >= rule.slowThreshold {
		i.logger.V(1).Info(
			"API operation completed slowly.",
			"service", info.Service(),
			"method", info.Method(),
			"duration", elapsed,
			"threshold", rule.slowThreshold,
		)
	}

	return res, err
}

func (i *serverInterceptors) rule(info interceptorInfo) operationRule {
	rule := operationRule{
		timeout:       i.operationTimeout,
		slowThreshold: i.slowThreshold,
	}

	if override, ok := i.rules[operationKey{
		service: info.Service(),
		method:  info.Method(),
	}]; ok {
		if override.timeout != 0 {
			rule.timeout = override.timeout
		}
		if override.slowThreshold != 0 {
			rule.slowThreshold = override.slowThreshold
		}
		rule.skipTimeout = override.skipTimeout
		rule.skipLogging = override.skipLogging
	}

	return rule
}
