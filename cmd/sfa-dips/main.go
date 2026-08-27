package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/oklog/run"
	"github.com/spf13/pflag"
	"go.artefactual.dev/tools/log"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/config"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/version"
)

const appName = "sfa-dips"

func main() {
	p := pflag.NewFlagSet(appName, pflag.ExitOnError)
	p.String("config", "", "Configuration file")
	p.Bool("version", false, "Show version information")
	if err := p.Parse(os.Args[1:]); err == flag.ErrHelp {
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if v, _ := p.GetBool("version"); v {
		fmt.Println(version.Info(appName))
		os.Exit(0)
	}

	var cfg config.Config
	configFile, _ := p.GetString("config")
	configFileFound, configFileUsed, err := config.Read(&cfg, configFile)
	if err != nil {
		fmt.Printf("Failed to read configuration: %v\n", err)
		os.Exit(1)
	}

	logger := log.New(os.Stderr,
		log.WithName(appName),
		log.WithFormat(cfg.LogFormat.LoggerFormat()),
		log.WithLevel(cfg.Verbosity),
	)
	defer log.Sync(logger)

	keys := []any{
		"version", version.Long,
		"pid", os.Getpid(),
		"go", runtime.Version(),
	}
	if version.GitCommit != "" {
		keys = append(keys, "commit", version.GitCommit)
	}
	logger.Info("Starting...", keys...)

	if configFileFound {
		logger.Info("Configuration file loaded.", "path", configFileUsed)
	} else {
		logger.Info("Configuration file not found.")
	}

	// Set up the API logger for logging HTTP requests and responses.
	apiLog, err := api.NewFileLogger(cfg.API.Log)
	if err != nil {
		logger.Error(err, "Failed to open API log file")
		os.Exit(1)
	}
	apiLog = apiLog.WithName(appName + ".api")
	defer apiLog.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var g run.Group

	// API server.
	{
		var srv *http.Server

		g.Add(
			func() error {
				srv = api.HTTPServer(logger, apiLog.Logger, &cfg.API, dips.NewService())
				logger.Info("DIPs API HTTP server listening.", "addr", srv.Addr)
				return srv.ListenAndServe()
			},
			func(err error) {
				ctx, cancel := context.WithTimeout(ctx, time.Second*5)
				defer cancel()
				_ = srv.Shutdown(ctx)
			},
		)
	}

	// TODO: Add a Temporal worker to the run group when it is implemented.

	// Signal handler.
	{
		var (
			cancelInterrupt = make(chan struct{})
			ch              = make(chan os.Signal, 2)
		)
		defer close(ch)

		g.Add(
			func() error {
				signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

				select {
				case <-ch:
				case <-cancelInterrupt:
				}

				return nil
			}, func(err error) {
				logger.Info("Quitting...")
				close(cancelInterrupt)
				cancel()
				signal.Stop(ch)
			},
		)
	}

	err = g.Run()
	if err != nil {
		logger.Error(err, "Application failure.")
		log.Sync(logger)
		os.Exit(1)
	}
	logger.Info("Bye!")
}
