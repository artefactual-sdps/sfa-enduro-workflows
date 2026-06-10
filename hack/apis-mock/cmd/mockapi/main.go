package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/artefactual-sdps/sfa-enduro-workflows/hack/apis-mock/internal/gen"
	"github.com/artefactual-sdps/sfa-enduro-workflows/hack/apis-mock/internal/mock"
)

const (
	envMockAnalysisResult = "MOCK_ANALYSIS_RESULT"
	envMockImportResult   = "MOCK_IMPORT_RESULT"
)

func main() {
	addr := ":" + envOrDefault("PORT", "8080")

	analysisResult, importResult, err := handlerResultsFromEnv()
	if err != nil {
		log.Fatalf("read mock configuration: %v", err)
	}

	handler := mock.NewHandler(analysisResult, importResult)
	security := mock.NewSecurity(envOrDefault("MOCK_AUTH_TOKEN", "mock-token"))
	server, err := gen.NewServer(handler, security)
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	log.Printf("mock API configured with analysisResult=%s importResult=%s", analysisResult, importResult)
	log.Printf("mock API listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func handlerResultsFromEnv() (gen.AnalysisResult, gen.ImportResult, error) {
	analysisResult, err := mock.ParseAnalysisResult(
		envOrDefault(envMockAnalysisResult, string(mock.DefaultAnalysisResult)),
	)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", envMockAnalysisResult, err)
	}

	importResult, err := mock.ParseImportResult(
		envOrDefault(envMockImportResult, string(mock.DefaultImportResult)),
	)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", envMockImportResult, err)
	}

	return analysisResult, importResult, nil
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
