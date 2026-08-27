/*
Package api contains the API server.

HTTP is the only transport supported at the moment.

The design package is the Goa design package while the gen package contains all
the generated code produced with goa gen.
*/
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"go.artefactual.dev/tools/middleware"
	goahttp "goa.design/goa/v3/http"
	goamiddleware "goa.design/goa/v3/middleware"

	intdips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips"
	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	dipssvr "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/http/di_ps/server"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/version"
)

func HTTPServer(
	logger logr.Logger,
	apiLogger *slog.Logger,
	config *Config,
	dipssvc intdips.Service,
) *http.Server {
	dec := goahttp.RequestDecoder
	enc := goahttp.ResponseEncoder
	mux := goahttp.NewMuxer()
	mux.Use(middleware.Recover(logger))

	serverInterceptors := newServerInterceptors(logger)

	// DIPs service.
	dipsEndpoints := goadips.NewEndpoints(
		dipssvc,
		&dipsServerInterceptors{serverInterceptors: serverInterceptors},
	)
	dipsErrorHandler := transportErrorHandler(logger, "DIPs transport error.")
	dipsServer := dipssvr.New(dipsEndpoints, mux, dec, enc, dipsErrorHandler, nil)
	dipssvr.Mount(mux, dipsServer)

	// Global middlewares.
	var handler http.Handler = mux
	handler = middleware.VersionHeader("X-SFA-DIPs-Version", version.Short)(handler)

	// Add logging middleware if an API logger is configured. The log level is
	// set to the configured log level.
	if apiLogger != nil {
		handler = requestLogger(apiLogger, config.Log.Level)(handler)
	}

	return &http.Server{
		Addr:        config.Listen,
		Handler:     handler,
		ReadTimeout: time.Second * 5,
		// Keep this above defaultAPIOperationTimeout so normal handlers can
		// return a timeout response. Streaming handlers opt out above.
		WriteTimeout: time.Second * 7,
		IdleTimeout:  time.Second * 120,
	}
}

type errorMessage struct {
	RequestID string
	Error     error
}

// transportErrorHandler handles failures raised while the generated HTTP
// transport encodes a response. Endpoint errors are logged and sanitized by
// the Goa ServerErrorHandler interceptor before they reach this layer.
func transportErrorHandler(logger logr.Logger, msg string) func(context.Context, http.ResponseWriter, error) {
	return func(ctx context.Context, w http.ResponseWriter, err error) {
		reqID, ok := ctx.Value(goamiddleware.RequestIDKey).(string)
		if !ok {
			reqID = "unknown"
		}

		_ = json.NewEncoder(w).Encode(&errorMessage{RequestID: reqID})

		logger.Error(err, "HTTP transport error.", "reqID", reqID, "msg", msg)
	}
}
