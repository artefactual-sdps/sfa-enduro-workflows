package api

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	di_ps "github.com/artefactual-sdps/preprocessing-sfa/internal/dips/api/gen/di_ps"
	"github.com/artefactual-sdps/preprocessing-sfa/internal/dips/config"
	"goa.design/goa/v3/security"
)

func TestHTTPServerNormalizesGoaValidationErrors(t *testing.T) {
	handler := HTTPServer(&config.APIConfig{}, &testDIPsService{}).Handler

	tests := []struct {
		name             string
		method           string
		path             string
		accept           string
		authorization    string
		body             string
		wantStatus       int
		wantTitle        string
		wantDetail       string
		wantGoaError     string
		wantDetailPrefix bool
	}{
		{
			name:             "missing authorization header",
			method:           http.MethodPost,
			path:             "/dips",
			body:             `{"docKey":"CH-000001"}`,
			wantStatus:       http.StatusBadRequest,
			wantTitle:        "Bad Request",
			wantDetail:       `"token" is missing from header`,
			wantGoaError:     "missing_field",
			wantDetailPrefix: false,
		},
		{
			name:             "empty document key",
			method:           http.MethodPost,
			path:             "/dips",
			authorization:    "Bearer test",
			body:             `{"docKey":""}`,
			wantStatus:       http.StatusBadRequest,
			wantTitle:        "Bad Request",
			wantDetail:       "length of body.docKey must be greater or equal than 1",
			wantGoaError:     "invalid_length",
			wantDetailPrefix: true,
		},
		{
			name:             "invalid UUID",
			method:           http.MethodGet,
			path:             "/dips/invalid-uuid",
			authorization:    "Bearer test",
			wantStatus:       http.StatusBadRequest,
			wantTitle:        "Bad Request",
			wantDetail:       `id must be formatted as a uuid but got value "invalid-uuid"`,
			wantGoaError:     "invalid_format",
			wantDetailPrefix: true,
		},
		{
			name:             "invalid UUID with XML accept",
			method:           http.MethodGet,
			path:             "/dips/invalid-uuid",
			accept:           "application/xml",
			authorization:    "Bearer test",
			wantStatus:       http.StatusBadRequest,
			wantTitle:        "Bad Request",
			wantDetail:       `id must be formatted as a uuid but got value "invalid-uuid"`,
			wantGoaError:     "invalid_format",
			wantDetailPrefix: true,
		},
		{
			name:             "unauthorized token",
			method:           http.MethodPost,
			path:             "/dips",
			authorization:    "Bearer unauthorized",
			body:             `{"docKey":"CH-000001"}`,
			wantStatus:       http.StatusUnauthorized,
			wantTitle:        "Unauthorized",
			wantDetail:       "The bearer token is missing or invalid.",
			wantGoaError:     "unauthorized",
			wantDetailPrefix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
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
			if got, want := mediaType(t, resp.Header.Get("Content-Type")), problemDetailsContentType; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}
			if got, want := resp.Header.Get("Goa-Error"), tt.wantGoaError; got != want {
				t.Fatalf("Goa-Error = %q, want %q", got, want)
			}

			var problem problemDetails
			if err := json.Unmarshal(data, &problem); err != nil {
				t.Fatalf("json.Unmarshal() error = %v, body: %s", err, string(data))
			}
			if got, want := problem.Type, "about:blank"; got != want {
				t.Errorf("type = %q, want %q", got, want)
			}
			if got, want := problem.Title, tt.wantTitle; got != want {
				t.Errorf("title = %q, want %q", got, want)
			}
			if got, want := problem.Status, tt.wantStatus; got != want {
				t.Errorf("status = %d, want %d", got, want)
			}
			if tt.wantDetailPrefix {
				if !strings.HasPrefix(problem.Detail, tt.wantDetail) {
					t.Errorf("detail = %q, want prefix %q", problem.Detail, tt.wantDetail)
				}
			} else if got, want := problem.Detail, tt.wantDetail; got != want {
				t.Errorf("detail = %q, want %q", got, want)
			}

			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("json.Unmarshal() raw error = %v, body: %s", err, string(data))
			}
			if _, ok := raw["name"]; ok {
				t.Errorf("response unexpectedly includes Goa error field 'name': %s", string(data))
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

type testDIPsService struct{}

func (s *testDIPsService) BearerAuth(
	ctx context.Context,
	token string,
	_ *security.BearerScheme,
) (context.Context, error) {
	if token == "unauthorized" {
		return ctx, &di_ps.UnauthorizedProblem{
			Type:   "about:blank",
			Title:  "Unauthorized",
			Detail: "The bearer token is missing or invalid.",
			Status: http.StatusUnauthorized,
		}
	}

	return ctx, nil
}

func (s *testDIPsService) Create(context.Context, *di_ps.CreatePayload) (*di_ps.CreateResult, error) {
	return &di_ps.CreateResult{
		ID: "3f38d6f4-7b19-4db8-8d7d-693b84a9a2fb",
	}, nil
}

func (s *testDIPsService) Show(_ context.Context, p *di_ps.ShowPayload) (*di_ps.ShowResult, error) {
	return &di_ps.ShowResult{
		ID:        p.ID,
		DocKey:    "CH-000001",
		Status:    "queued",
		CreatedAt: "2026-05-27T15:04:05Z",
	}, nil
}
