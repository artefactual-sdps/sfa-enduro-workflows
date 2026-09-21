package actapro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

// TestClientExportFileMediaTypes verifies that export downloads preserve the
// response bytes for both the documented and observed media types.
func TestClientExportFileMediaTypes(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	const body = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\r\n<metadata>Über &amp; export</metadata>\r\n"
	for _, contentType := range []string{
		"application/octet-stream",
		"application/xml",
		"application/xml; charset=UTF-8",
	} {
		t.Run(contentType, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/massoperation/export/binary" ||
					r.URL.Query().Get("id") != id.String() {
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", contentType)
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(server.Close)

			client, err := NewClient(Config{URL: server.URL, Token: "test-token"}, server.Client(), nil)
			assert.NilError(t, err)
			response, err := client.GetExportFile(t.Context(), gen.GetExportFileParams{ID: id})
			assert.NilError(t, err)
			reader, ok := response.(io.Reader)
			assert.Assert(t, ok, "unexpected response type %T", response)
			data, err := io.ReadAll(reader)
			assert.NilError(t, err)
			assert.Equal(t, string(data), body)
		})
	}
}

func TestNewClientTimeout(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		timeout time.Duration
		wantErr string
	}{
		{
			name:    "negative timeout",
			timeout: -time.Second,
			wantErr: "ACTApro.Timeout: value -1s is less than 0",
		},
		{
			name: "zero timeout",
		},
		{
			name:    "positive timeout",
			timeout: time.Second,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(Config{URL: "http://localhost", Timeout: tt.timeout}, nil, nil)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				assert.Assert(t, client == nil)
				return
			}
			assert.NilError(t, err)
			assert.Assert(t, client != nil)
		})
	}
}

// TestClientJSONErrorResponses verifies the error response media-type correction
// from application/hal+json to application/json documented in README.md.
func TestClientJSONErrorResponses(t *testing.T) {
	t.Parallel()

	errorResponses := make(map[int32]*gen.ErrorResponse)
	for _, status := range []int32{
		http.StatusBadRequest,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusLocked,
		http.StatusInternalServerError,
	} {
		errorResponses[status] = &gen.ErrorResponse{
			Message: gen.NewOptString("request failed"),
			Status:  gen.NewOptInt32(status),
		}
	}

	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	for _, tt := range []struct {
		name      string
		method    string
		path      string
		invoke    func(context.Context, Client) (any, error)
		responses map[int]any
	}{
		{
			name:   "get document",
			method: http.MethodGet,
			path:   "/document/CH-000001",
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.GetDocument(ctx, gen.GetDocumentParams{Key: "CH-000001"})
			},
			responses: map[int]any{
				http.StatusBadRequest:          (*gen.GetDocumentBadRequest)(errorResponses[http.StatusBadRequest]),
				http.StatusForbidden:           (*gen.GetDocumentForbidden)(errorResponses[http.StatusForbidden]),
				http.StatusNotFound:            (*gen.GetDocumentNotFound)(errorResponses[http.StatusNotFound]),
				http.StatusConflict:            (*gen.GetDocumentConflict)(errorResponses[http.StatusConflict]),
				http.StatusLocked:              (*gen.GetDocumentLocked)(errorResponses[http.StatusLocked]),
				http.StatusInternalServerError: (*gen.GetDocumentInternalServerError)(errorResponses[http.StatusInternalServerError]),
			},
		},
		{
			name:   "create export",
			method: http.MethodPost,
			path:   "/massoperation/export",
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.CreateMassOperationExport(ctx, &gen.ExportParamsDTO{
					ScriptId:      "22222222-2222-4222-8222-222222222222",
					ExportDocKeys: []string{"CH-000001"},
				})
			},
			responses: map[int]any{
				http.StatusBadRequest:          (*gen.CreateMassOperationExportBadRequest)(errorResponses[http.StatusBadRequest]),
				http.StatusForbidden:           (*gen.CreateMassOperationExportForbidden)(errorResponses[http.StatusForbidden]),
				http.StatusNotFound:            (*gen.CreateMassOperationExportNotFound)(errorResponses[http.StatusNotFound]),
				http.StatusConflict:            (*gen.CreateMassOperationExportConflict)(errorResponses[http.StatusConflict]),
				http.StatusLocked:              (*gen.CreateMassOperationExportLocked)(errorResponses[http.StatusLocked]),
				http.StatusInternalServerError: (*gen.CreateMassOperationExportInternalServerError)(errorResponses[http.StatusInternalServerError]),
			},
		},
		{
			name:   "get export file",
			method: http.MethodGet,
			path:   "/massoperation/export/binary",
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.GetExportFile(ctx, gen.GetExportFileParams{ID: id})
			},
			responses: map[int]any{
				http.StatusBadRequest:          (*gen.GetExportFileBadRequest)(errorResponses[http.StatusBadRequest]),
				http.StatusForbidden:           (*gen.GetExportFileForbidden)(errorResponses[http.StatusForbidden]),
				http.StatusNotFound:            (*gen.GetExportFileNotFound)(errorResponses[http.StatusNotFound]),
				http.StatusConflict:            (*gen.GetExportFileConflict)(errorResponses[http.StatusConflict]),
				http.StatusLocked:              (*gen.GetExportFileLocked)(errorResponses[http.StatusLocked]),
				http.StatusInternalServerError: (*gen.GetExportFileInternalServerError)(errorResponses[http.StatusInternalServerError]),
			},
		},
		{
			name:   "get export info",
			method: http.MethodGet,
			path:   "/massoperation/" + id.String() + "/info",
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.GetMassOperationInfo(ctx, gen.GetMassOperationInfoParams{ID: id})
			},
			responses: map[int]any{
				http.StatusBadRequest:          (*gen.GetMassOperationInfoBadRequest)(errorResponses[http.StatusBadRequest]),
				http.StatusForbidden:           (*gen.GetMassOperationInfoForbidden)(errorResponses[http.StatusForbidden]),
				http.StatusNotFound:            (*gen.GetMassOperationInfoNotFound)(errorResponses[http.StatusNotFound]),
				http.StatusConflict:            (*gen.GetMassOperationInfoConflict)(errorResponses[http.StatusConflict]),
				http.StatusLocked:              (*gen.GetMassOperationInfoLocked)(errorResponses[http.StatusLocked]),
				http.StatusInternalServerError: (*gen.GetMassOperationInfoInternalServerError)(errorResponses[http.StatusInternalServerError]),
			},
		},
		{
			name:   "get export logs",
			method: http.MethodGet,
			path:   "/massoperation/" + id.String() + "/logs",
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.GetMassOperationLogs(ctx, gen.GetMassOperationLogsParams{ID: id})
			},
			responses: map[int]any{
				http.StatusBadRequest:          (*gen.GetMassOperationLogsBadRequest)(errorResponses[http.StatusBadRequest]),
				http.StatusForbidden:           (*gen.GetMassOperationLogsForbidden)(errorResponses[http.StatusForbidden]),
				http.StatusNotFound:            (*gen.GetMassOperationLogsNotFound)(errorResponses[http.StatusNotFound]),
				http.StatusConflict:            (*gen.GetMassOperationLogsConflict)(errorResponses[http.StatusConflict]),
				http.StatusLocked:              (*gen.GetMassOperationLogsLocked)(errorResponses[http.StatusLocked]),
				http.StatusInternalServerError: (*gen.GetMassOperationLogsInternalServerError)(errorResponses[http.StatusInternalServerError]),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for status, want := range tt.responses {
				t.Run(fmt.Sprint(status), func(t *testing.T) {
					t.Parallel()

					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != tt.method || r.URL.Path != tt.path {
							http.Error(w, "unexpected request", http.StatusBadRequest)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(status)
						_, _ = fmt.Fprintf(w, `{"message":"request failed","status":%d}`, status)
					}))
					t.Cleanup(server.Close)

					client, err := NewClient(Config{URL: server.URL, Token: "test-token"}, server.Client(), nil)
					assert.NilError(t, err)

					response, err := tt.invoke(t.Context(), client)
					assert.NilError(t, err)
					assert.DeepEqual(t, response, want)
				})
			}
		})
	}
}

// TestClientMassOperationCreationDates verifies the removal of the date-time
// format from crdate fields documented in README.md, preserving dates as strings.
func TestClientMassOperationCreationDates(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	for _, tt := range []struct {
		name   string
		method string
		path   string
		body   string
		invoke func(context.Context, Client) (any, error)
	}{
		{
			name:   "create export",
			method: http.MethodPost,
			path:   "/massoperation/export",
			body:   `{"id":"11111111-1111-4111-8111-111111111111","crdate":%q,"status":"REQUESTSTART"}`,
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.CreateMassOperationExport(ctx, &gen.ExportParamsDTO{
					ScriptId:      "22222222-2222-4222-8222-222222222222",
					ExportDocKeys: []string{"CH-000001"},
				})
			},
		},
		{
			name:   "get export info",
			method: http.MethodGet,
			path:   "/massoperation/" + id.String() + "/info",
			body:   `{"id":"11111111-1111-4111-8111-111111111111","crdate":%q,"status":"COMPLETED"}`,
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.GetMassOperationInfo(ctx, gen.GetMassOperationInfoParams{ID: id})
			},
		},
		{
			name:   "get export logs",
			method: http.MethodGet,
			path:   "/massoperation/" + id.String() + "/logs",
			body:   `[{"operationId":"11111111-1111-4111-8111-111111111111","crdate":%q,"status":"ERROR","message":"Export failed"}]`,
			invoke: func(ctx context.Context, client Client) (any, error) {
				return client.GetMassOperationLogs(ctx, gen.GetMassOperationLogsParams{ID: id})
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, crdate := range []string{
				"2026-09-15T09:15:10",
				"2026-09-15T09:15:10.948",
				"2026-09-15T07:15:10Z",
				"2026-09-15T09:15:10+02:00",
			} {
				t.Run(crdate, func(t *testing.T) {
					t.Parallel()

					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != tt.method || r.URL.Path != tt.path {
							http.Error(w, "unexpected request", http.StatusBadRequest)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						_, _ = fmt.Fprintf(w, tt.body, crdate)
					}))
					t.Cleanup(server.Close)

					client, err := NewClient(Config{URL: server.URL, Token: "test-token"}, server.Client(), nil)
					assert.NilError(t, err)
					response, err := tt.invoke(t.Context(), client)
					assert.NilError(t, err)

					switch got := response.(type) {
					case *gen.MassOperationInfoDTO:
						assert.Equal(t, got.ID.Value, id.String())
						assert.Equal(t, got.Crdate.Value, crdate)
					case *gen.GetMassOperationLogsOKApplicationJSON:
						assert.Equal(t, len(*got), 1)
						assert.Equal(t, (*got)[0].OperationId.Value, id.String())
						assert.Equal(t, (*got)[0].Crdate.Value, crdate)
						assert.Equal(t, (*got)[0].Message.Value, "Export failed")
					default:
						t.Fatalf("unexpected response type %T", response)
					}
				})
			}
		})
	}
}

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) AccessToken(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.token, nil
}

func TestSecuritySourceBearerAuth(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name      string
		source    securitySource
		wantToken string
		wantErr   string
	}

	for _, tc := range []testCase{
		{
			name: "static token overrides provider",
			source: securitySource{
				staticToken:   "mock-token",
				tokenProvider: stubTokenProvider{token: "provider-token"},
			},
			wantToken: "mock-token",
		},
		{
			name: "provider token is used when static token is absent",
			source: securitySource{
				tokenProvider: stubTokenProvider{token: "provider-token"},
			},
			wantToken: "provider-token",
		},
		{
			name:    "missing token source returns error",
			source:  securitySource{},
			wantErr: "missing ACTApro token provider",
		},
		{
			name: "provider error is returned",
			source: securitySource{
				tokenProvider: stubTokenProvider{err: errors.New("error")},
			},
			wantErr: "failed to get access token: error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.source.BearerAuth(t.Context(), gen.GetDocumentOperation)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, got.Token, tc.wantToken)
		})
	}
}
