package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"goa.design/goa/v3/security"
	"gotest.tools/v3/assert"

	goadips "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
)

type testDIPsService struct {
	showResult *goadips.ShowResult
	showErr    error
}

func (s *testDIPsService) BearerAuth(
	ctx context.Context,
	token string,
	_ *security.BearerScheme,
) (context.Context, error) {
	return ctx, nil
}

func (s *testDIPsService) Livez(context.Context) error {
	return nil
}

func (s *testDIPsService) Create(context.Context, *goadips.CreatePayload) (*goadips.CreateResult, error) {
	return nil, nil
}

func (s *testDIPsService) Show(context.Context, *goadips.ShowPayload) (*goadips.ShowResult, error) {
	return s.showResult, s.showErr
}

type testAPI struct {
	dips    *testDIPsService
	handler http.Handler
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	t.Setenv("SFA_DIPS_API_CORS_ORIGIN", "http://example.com")

	dipsSvc := &testDIPsService{}

	server := HTTPServer(
		logr.Discard(),
		slog.New(slog.DiscardHandler),
		&Config{Listen: ":0"},
		dipsSvc,
	)

	return &testAPI{
		dips:    dipsSvc,
		handler: server.Handler,
	}
}

func TestHTTPServer(t *testing.T) {
	api := newTestAPI(t)

	dipID := "7fd0bb89-df4a-4aeb-a1bd-6db3907bb832"
	api.dips.showResult = &goadips.ShowResult{
		ID:        goadips.DIPID(dipID),
		DocKey:    "CH-000001",
		Status:    "queued",
		CreatedAt: "2025-01-01T00:00:00Z",
	}

	req := httptest.NewRequest(http.MethodGet, "/dips/"+dipID, nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	api.handler.ServeHTTP(rec, req)
	assert.Equal(t, rec.Code, http.StatusOK)

	var body map[string]any
	assert.NilError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.DeepEqual(t, body, map[string]any{
		"id":         dipID,
		"docKey":     "CH-000001",
		"status":     "queued",
		"created_at": "2025-01-01T00:00:00Z",
	})
}

func TestHTTPServerInternalError(t *testing.T) {
	api := newTestAPI(t)
	internalErr := goadips.MakeInternalError(errors.New("database password leaked"))
	api.dips.showErr = internalErr

	req := httptest.NewRequest(
		http.MethodGet,
		"/dips/7fd0bb89-df4a-4aeb-a1bd-6db3907bb832",
		nil,
	)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	api.handler.ServeHTTP(rec, req)

	assert.Equal(t, rec.Code, http.StatusInternalServerError)
	var body map[string]any
	assert.NilError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.DeepEqual(t, body, map[string]any{
		"name":      "internal_error",
		"id":        internalErr.ID,
		"message":   apiInternalErrorMsg,
		"temporary": false,
		"timeout":   false,
		"fault":     true,
	})
}
