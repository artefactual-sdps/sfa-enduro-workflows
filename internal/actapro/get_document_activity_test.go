package actapro_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro"
	fake_actapro "github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/fake"
	actaprogen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/actapro/gen"
)

func TestGetDocumentActivity(t *testing.T) {
	t.Parallel()

	uuid1 := uuid.MustParse("28c1a3e2-abd3-4b9b-9214-ae851c87b1a6")
	uuid2 := uuid.MustParse("1231e569-a94e-4ac1-873e-65e1f524b1c8")
	uuid3 := uuid.MustParse("a38c3e43-c6c3-42f6-a7a0-8227c378deab")
	uuid4 := uuid.MustParse("d4c779f8-b6e2-4b7b-9eab-7fd36c21de23")

	tests := []struct {
		name         string
		response     actaprogen.GetDocumentRes
		clientErr    error
		want         actapro.GetDocumentResult
		wantErr      string
		nonRetryable bool
	}{
		{
			name: "returns no AIP UUIDs for a document without AIP groups",
			response: &actaprogen.Document{
				DocKey:   actaprogen.NewOptString("CH-000001"),
				DocTitle: actaprogen.NewOptString("Document title"),
				Object:   "document",
				Block: actaprogen.DocumentBlock{
					Type: "document",
					Fields: []actaprogen.DocumentField{
						{
							Type:  "title",
							Value: actaprogen.NewOptString("Document title"),
							Fields: []actaprogen.DocumentField{
								{Type: "text", PlainValue: actaprogen.NewOptString("Nested value")},
							},
						},
					},
				},
			},
			want: actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{}},
		},
		{
			name: "returns AIP UUIDs from multiple groups",
			response: &actaprogen.Document{
				Block: actaprogen.DocumentBlock{
					Fields: []actaprogen.DocumentField{
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.String())},
							},
						},
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid2.String())},
							},
						},
					},
				},
			},
			want: actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{uuid1, uuid2}},
		},
		{
			name: "returns multiple AIP UUIDs from each group",
			response: &actaprogen.Document{
				Block: actaprogen.DocumentBlock{
					Fields: []actaprogen.DocumentField{
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.String())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid2.String())},
							},
						},
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid3.String())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid4.String())},
							},
						},
					},
				},
			},
			want: actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{uuid1, uuid2, uuid3, uuid4}},
		},
		{
			name: "deduplicates AIP UUIDs within and across groups in response order",
			response: &actaprogen.Document{
				Block: actaprogen.DocumentBlock{
					Fields: []actaprogen.DocumentField{
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid2.String())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid2.String())},
								{
									Type:  "AIP_ID",
									Value: actaprogen.NewOptString("1231E569-A94E-4AC1-873E-65E1F524B1C8"),
								},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.String())},
							},
						},
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.String())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.URN())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid3.String())},
							},
						},
					},
				},
			},
			want: actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{uuid2, uuid1, uuid3}},
		},
		{
			name: "ignores unrelated fields and missing or empty IDs",
			response: &actaprogen.Document{
				Block: actaprogen.DocumentBlock{
					Fields: []actaprogen.DocumentField{
						{Type: "AIP_ID", Value: actaprogen.NewOptString("top-level ID")},
						{
							Type: "other group",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString("unrelated ID")},
							},
						},
						{Type: "AIP_ID_Gp"},
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "other field", Value: actaprogen.NewOptString("unrelated value")},
								{Type: "AIP_ID"},
								{Type: "AIP_ID", Value: actaprogen.NewOptString("")},
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.String())},
							},
						},
					},
				},
			},
			want: actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{uuid1}},
		},
		{
			name:     "returns no AIP UUIDs for an empty document",
			response: &actaprogen.Document{},
			want:     actapro.GetDocumentResult{AIPUUIDs: []uuid.UUID{}},
		},
		{
			name: "returns an error for an invalid AIP UUID",
			response: &actaprogen.Document{
				Block: actaprogen.DocumentBlock{
					Fields: []actaprogen.DocumentField{
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString("invalid")},
							},
						},
					},
				},
			},
			wantErr:      `get ACTApro document: invalid AIP UUID "invalid": invalid UUID length: 7`,
			nonRetryable: true,
		},
		{
			name: "collects all UUID parsing errors",
			response: &actaprogen.Document{
				Block: actaprogen.DocumentBlock{
					Fields: []actaprogen.DocumentField{
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid1.String())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString("invalid")},
								{
									Type:  "AIP_ID",
									Value: actaprogen.NewOptString("1231e569-a94e-4ac1-873e-65e1f524b1cg"),
								},
							},
						},
						{
							Type: "AIP_ID_Gp",
							Fields: []actaprogen.DocumentField{
								{Type: "AIP_ID", Value: actaprogen.NewOptString(uuid3.String())},
								{Type: "AIP_ID", Value: actaprogen.NewOptString("invalid")},
							},
						},
					},
				},
			},
			wantErr: "get ACTApro document: " +
				"invalid AIP UUID \"invalid\": invalid UUID length: 7\n" +
				"invalid AIP UUID \"1231e569-a94e-4ac1-873e-65e1f524b1cg\": invalid UUID format\n" +
				"invalid AIP UUID \"invalid\": invalid UUID length: 7",
			nonRetryable: true,
		},
		{
			name: "returns bad request error",
			response: &actaprogen.GetDocumentBadRequest{
				Message: actaprogen.NewOptString("invalid document key"),
			},
			wantErr:      "get ACTApro document: bad request: invalid document key",
			nonRetryable: true,
		},
		{
			name: "returns forbidden error",
			response: &actaprogen.GetDocumentForbidden{
				Message: actaprogen.NewOptString("access denied"),
			},
			wantErr:      "get ACTApro document: forbidden: access denied",
			nonRetryable: true,
		},
		{
			name: "returns not found error",
			response: &actaprogen.GetDocumentNotFound{
				Message: actaprogen.NewOptString("unknown document"),
			},
			wantErr:      "get ACTApro document: not found: unknown document",
			nonRetryable: true,
		},
		{
			name: "returns conflict error",
			response: &actaprogen.GetDocumentConflict{
				Message: actaprogen.NewOptString("document is being updated"),
			},
			wantErr:      "get ACTApro document: conflict: document is being updated",
			nonRetryable: false,
		},
		{
			name: "returns locked error",
			response: &actaprogen.GetDocumentLocked{
				Message: actaprogen.NewOptString("document is locked"),
			},
			wantErr:      "get ACTApro document: locked: document is locked",
			nonRetryable: false,
		},
		{
			name: "returns server error",
			response: &actaprogen.GetDocumentInternalServerError{
				Message: actaprogen.NewOptString("service unavailable"),
			},
			wantErr:      "get ACTApro document: server error: service unavailable",
			nonRetryable: false,
		},
		{
			name:         "returns client error",
			clientErr:    errors.New("error from client"),
			wantErr:      "get ACTApro document: error from client",
			nonRetryable: false,
		},
		{
			name:         "returns retryable unexpected response error",
			wantErr:      "get ACTApro document: unexpected response",
			nonRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := fake_actapro.NewMockClient(ctrl)
			client.EXPECT().GetDocument(gomock.Any(), actaprogen.GetDocumentParams{
				Key:    "CH-000001",
				Format: actaprogen.NewOptString("json"),
			}).Return(tt.response, tt.clientErr)

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewGetDocumentActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.GetDocumentActivityName},
			)

			future, err := env.ExecuteActivity(
				actapro.GetDocumentActivityName,
				&actapro.GetDocumentParams{DocKey: "CH-000001"},
			)
			if tt.wantErr != "" {
				assert.Assert(t, future == nil)
				assert.ErrorContains(t, err, tt.wantErr)
				var applicationErr *temporalsdk_temporal.ApplicationError
				assert.Assert(t, errors.As(err, &applicationErr))
				assert.Equal(t, applicationErr.Message(), tt.wantErr)
				assert.Equal(t, applicationErr.NonRetryable(), tt.nonRetryable)
				return
			}
			assert.NilError(t, err)

			var result actapro.GetDocumentResult
			assert.NilError(t, future.Get(&result))
			assert.DeepEqual(t, result, tt.want)
		})
	}
}
