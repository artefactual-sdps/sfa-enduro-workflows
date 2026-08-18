package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goa.design/goa/v3/security"

	di_ps "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/config"
)

func TestHTTPServerUsesGoaErrorResponses(t *testing.T) {
	handler := HTTPServer(&config.APIConfig{}, &testGoaErrorService{}).Handler

	tests := []struct {
		name          string
		method        string
		path          string
		authorization string
		body          string
		wantStatus    int
		wantName      string
		wantMessage   string
		wantGoaError  string
		wantFault     bool
	}{
		{
			name:        "missing authorization header",
			method:      http.MethodPost,
			path:        "/dips",
			body:        `{"docKey":"CH-000001"}`,
			wantStatus:  http.StatusBadRequest,
			wantName:    "missing_field",
			wantMessage: `"token" is missing from header`,
		},
		{
			name:          "empty document key",
			method:        http.MethodPost,
			path:          "/dips",
			authorization: "Bearer test",
			body:          `{"docKey":""}`,
			wantStatus:    http.StatusBadRequest,
			wantName:      "invalid_length",
			wantMessage:   `length of body.docKey must be greater or equal than 1 but got value "" (len=0)`,
		},
		{
			name:          "unauthorized token",
			method:        http.MethodPost,
			path:          "/dips",
			authorization: "Bearer unauthorized",
			body:          `{"docKey":"CH-000001"}`,
			wantStatus:    http.StatusUnauthorized,
			wantName:      "unauthorized",
			wantMessage:   "the bearer token is missing or invalid",
			wantGoaError:  "unauthorized",
		},
		{
			name:          "internal server error",
			method:        http.MethodPost,
			path:          "/dips",
			authorization: "Bearer test",
			body:          `{"docKey":"error-doc-key"}`,
			wantStatus:    http.StatusInternalServerError,
			wantName:      "internal_server_error",
			wantMessage:   "failed to create DIP in the database",
			wantGoaError:  "internal_server_error",
			wantFault:     true,
		},
		{
			name:          "invalid UUID",
			method:        http.MethodGet,
			path:          "/dips/invalid-uuid",
			authorization: "Bearer test",
			wantStatus:    http.StatusBadRequest,
			wantName:      "invalid_format",
			wantMessage:   `id must be formatted as a uuid but got value "invalid-uuid", uuid: invalid-uuid: invalid UUID length: 12`,
		},
		{
			name:          "not found",
			method:        http.MethodGet,
			path:          "/dips/e8d32bd5-faa4-4ce1-bb50-55d9c28b306d",
			authorization: "Bearer test",
			wantStatus:    http.StatusNotFound,
			wantName:      "not_found",
			wantMessage:   "the requested DIP was not found",
			wantGoaError:  "not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}

			if got, want := resp.StatusCode, tt.wantStatus; got != want {
				t.Fatalf("status = %d, want %d, body: %s", got, want, string(data))
			}
			if got, want := mediaType(t, resp.Header.Get("Content-Type")), "application/json"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
			if got, want := resp.Header.Get("Goa-Error"), tt.wantGoaError; got != want {
				t.Fatalf("Goa-Error = %q, want %q", got, want)
			}

			var body goaErrorBody
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v, body: %s", err, string(data))
			}
			if got, want := body.Name, tt.wantName; got != want {
				t.Errorf("name = %q, want %q", got, want)
			}
			if got, want := body.Message, tt.wantMessage; got != want {
				t.Errorf("message = %q, want %q", got, want)
			}
			if body.ID == "" {
				t.Error("id is empty")
			}
			if body.Temporary {
				t.Error("temporary = true, want false")
			}
			if body.Timeout {
				t.Error("timeout = true, want false")
			}
			if got, want := body.Fault, tt.wantFault; got != want {
				t.Errorf("fault = %t, want %t", got, want)
			}
		})
	}
}

func mediaType(t *testing.T, contentType string) string {
	t.Helper()

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("mime.ParseMediaType(%q) error = %v", contentType, err)
	}

	return mediaType
}

type goaErrorBody struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	Message   string `json:"message"`
	Temporary bool   `json:"temporary"`
	Timeout   bool   `json:"timeout"`
	Fault     bool   `json:"fault"`
}

type testGoaErrorService struct{}

func (s *testGoaErrorService) BearerAuth(
	ctx context.Context,
	token string,
	_ *security.BearerScheme,
) (context.Context, error) {
	if token == "unauthorized" {
		return ctx, di_ps.MakeUnauthorized(errors.New("the bearer token is missing or invalid"))
	}

	return ctx, nil
}

func (s *testGoaErrorService) Create(_ context.Context, p *di_ps.CreatePayload) (*di_ps.CreateResult, error) {
	if p.DocKey == "error-doc-key" {
		return nil, di_ps.MakeInternalServerError(errors.New("failed to create DIP in the database"))
	}

	return &di_ps.CreateResult{
		ID: "3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb",
	}, nil
}

func (s *testGoaErrorService) Show(_ context.Context, p *di_ps.ShowPayload) (*di_ps.ShowResult, error) {
	if p.ID == "e8d32bd5-faa4-4ce1-bb50-55d9c28b306d" {
		return nil, di_ps.MakeNotFound(errors.New("the requested DIP was not found"))
	}

	return &di_ps.ShowResult{
		ID:        p.ID,
		DocKey:    "CH-000001",
		Status:    "queued",
		CreatedAt: "2026-05-27T15:04:05Z",
	}, nil
}
