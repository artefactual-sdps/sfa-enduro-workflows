/*
Package api contains the API server.

HTTP is the only transport supported at the moment.

The design package is the Goa design package while the gen package contains all
the generated code produced with goa gen.
*/
package api

import (
	"net/http"
	"os"
	"time"

	goahttp "goa.design/goa/v3/http"
	goahttpmwr "goa.design/goa/v3/http/middleware"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips"
	di_ps "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/http/di_ps/server"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/config"
)

func HTTPServer(config *config.APIConfig, svc dips.Service) *http.Server {
	mux := goahttp.NewMuxer()
	svr := server.New(di_ps.NewEndpoints(svc), mux, goahttp.RequestDecoder, goahttp.ResponseEncoder, nil, nil)
	server.Mount(mux, svr)

	var handler http.Handler = mux
	if config.Debug {
		handler = goahttpmwr.Debug(mux, os.Stdout)(handler)
	}

	return &http.Server{
		Addr:         config.Listen,
		Handler:      handler,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
		IdleTimeout:  time.Second * 120,
	}
}
