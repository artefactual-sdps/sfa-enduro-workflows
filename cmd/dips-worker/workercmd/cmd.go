package workercmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	_ "gocloud.dev/blob/fileblob"

	"github.com/artefactual-sdps/preprocessing-sfa/internal/dips"
	"github.com/artefactual-sdps/preprocessing-sfa/internal/dips/api"
	"github.com/artefactual-sdps/preprocessing-sfa/internal/dips/config"
)

const Name = "sfa-dips-worker"

type Main struct {
	logger     logr.Logger
	cfg        config.Config
	httpServer *http.Server
}

func NewMain(logger logr.Logger, cfg config.Config) *Main {
	return &Main{
		logger: logger,
		cfg:    cfg,
	}
}

func (m *Main) Run(ctx context.Context) error {
	svc := dips.NewService(m.logger.WithName("dips"))
	srv := api.HTTPServer(&m.cfg.API, svc)
	srv.BaseContext = func(net.Listener) context.Context {
		return ctx
	}

	m.httpServer = srv

	// Run follows the same start-and-return contract as cmd/worker; ListenAndServe blocks,
	// so run it in the background and let main call Close on shutdown.
	go func() {
		m.logger.Info("DIPs API HTTP server listening.", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.logger.Error(err, "DIPs API HTTP server failed.")
		}
	}()

	return nil
}

func (m *Main) Close() error {
	if m.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := m.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shut down DIPs API HTTP server: %w", err)
	}

	return nil
}
