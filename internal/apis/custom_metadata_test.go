package apis_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis"
	apisgen "github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

func TestCustomMetadataMarshal(t *testing.T) {
	t.Parallel()

	t.Run("marshals APIS metadata", func(t *testing.T) {
		t.Parallel()

		data, err := apis.CustomMetadata{
			ImportTaskID: "task-000001",
			Decision:     "Continue and append",
		}.Marshal()
		assert.NilError(t, err)
		assert.Equal(t, string(data), `{"importTaskId":"task-000001","decision":"Continue and append"}`)
	})

	t.Run("rejects missing task ID", func(t *testing.T) {
		t.Parallel()

		_, err := apis.CustomMetadata{}.Marshal()
		assert.ErrorContains(t, err, "requires import task ID")
	})
}

func TestCustomMetadataUnmarshal(t *testing.T) {
	t.Parallel()

	t.Run("unmarshals APIS metadata", func(t *testing.T) {
		t.Parallel()

		var metadata apis.CustomMetadata
		err := metadata.Unmarshal([]byte(`{"importTaskId":"task-000001","decision":"Continue and append"}`))
		assert.NilError(t, err)
		assert.DeepEqual(t, metadata, apis.CustomMetadata{
			ImportTaskID: "task-000001",
			Decision:     "Continue and append",
		})
	})

	t.Run("rejects missing task ID", func(t *testing.T) {
		t.Parallel()

		var metadata apis.CustomMetadata
		err := metadata.Unmarshal([]byte(`{"decision":"Continue and append"}`))
		assert.ErrorContains(t, err, "requires import task ID")
	})

	t.Run("rejects nil destination", func(t *testing.T) {
		t.Parallel()

		var metadata *apis.CustomMetadata
		err := metadata.Unmarshal([]byte(`{"importTaskId":"task-000001","decision":"Continue and append"}`))
		assert.ErrorContains(t, err, "destination is nil")
	})
}

func TestCustomMetadataImportBehaviour(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata apis.CustomMetadata
		want     apisgen.ImportBehaviourType
		wantErr  string
	}{
		{
			name:     "empty decision maps to append only",
			metadata: apis.CustomMetadata{ImportTaskID: "task-000001"},
			want:     apisgen.ImportBehaviourTypeAppendOnly,
		},
		{
			name: "append decision maps to append only",
			metadata: apis.CustomMetadata{
				ImportTaskID: "task-000002",
				Decision:     apis.DecisionOptionContinueAppend,
			},
			want: apisgen.ImportBehaviourTypeAppendOnly,
		},
		{
			name: "overwrite decision maps to overwrite and append",
			metadata: apis.CustomMetadata{
				ImportTaskID: "task-000003",
				Decision:     apis.DecisionOptionContinueOverwrite,
			},
			want: apisgen.ImportBehaviourTypeOverwriteAndAppend,
		},
		{
			name: "cancel decision returns error",
			metadata: apis.CustomMetadata{
				ImportTaskID: "task-000004",
				Decision:     apis.DecisionOptionCancelIngest,
			},
			wantErr: "was canceled during preprocessing",
		},
		{
			name: "unsupported decision returns error",
			metadata: apis.CustomMetadata{
				ImportTaskID: "task-000005",
				Decision:     "Something else",
			},
			wantErr: `unsupported APIS decision "Something else"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.metadata.ImportBehaviour()
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
