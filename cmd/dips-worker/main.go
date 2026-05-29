package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/pflag"
	"go.artefactual.dev/tools/log"

	"github.com/artefactual-sdps/preprocessing-sfa/cmd/dips-worker/workercmd"
	"github.com/artefactual-sdps/preprocessing-sfa/internal/dips/config"
	"github.com/artefactual-sdps/preprocessing-sfa/internal/version"
)

func main() {
	p := pflag.NewFlagSet(workercmd.Name, pflag.ExitOnError)
	p.String("config", "", "Configuration file")
	p.Bool("version", false, "Show version information")
	if err := p.Parse(os.Args[1:]); err == flag.ErrHelp {
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if v, _ := p.GetBool("version"); v {
		fmt.Println(version.Info(workercmd.Name))
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
		log.WithName(workercmd.Name),
		log.WithDebug(cfg.Debug),
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

	// Cancel the root context when the process receives a shutdown signal.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	m := workercmd.NewMain(logger, cfg)

	if err := m.Run(ctx); err != nil {
		_ = m.Close()
		os.Exit(1)
	}

	<-ctx.Done()
	logger.Info("Quitting...")

	if err := m.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		logger.Error(err, "Failed to close the application.")
		os.Exit(1)
	}

	logger.Info("Bye!")
}
