package amss_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
)

func TestGetAIPPathActivity(t *testing.T) {
	t.Parallel()

	aipUUID := uuid.MustParse("9390594f-84c2-457d-bd6a-618f21f7c954")
	aipUUIDString := aipUUID.String()

	for _, tt := range []struct {
		name     string
		params   *amss.GetAIPPathActivityParams
		response string
		want     *amss.GetAIPPathActivityResult
		wantErr  string
	}{
		{
			name:     "success",
			params:   &amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
			response: fmt.Sprintf(`{"uuid":%q,"current_path":"test/path/METS.xml"}`, aipUUIDString),
			want:     &amss.GetAIPPathActivityResult{Path: "test/path/METS.xml"},
		},
		{
			name:     "missing current path",
			params:   &amss.GetAIPPathActivityParams{AIPUUID: aipUUID},
			response: fmt.Sprintf(`{"uuid":%q}`, aipUUIDString),
			wantErr:  "GetAIPPath: current_path not found in response",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			packages := newPackagesService(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, http.MethodGet)
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/file/%s/", aipUUIDString))
				assert.Equal(t, r.Header.Get("Authorization"), "ApiKey test:test")

				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.response)
			})

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				amss.NewGetAIPPathActivity(packages).Execute,
				temporalsdk_activity.RegisterOptions{Name: amss.GetAIPPathActivityName},
			)

			future, err := env.ExecuteActivity(amss.GetAIPPathActivityName, tt.params)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var got amss.GetAIPPathActivityResult
			err = future.Get(&got)
			assert.NilError(t, err)
			assert.DeepEqual(t, &got, tt.want)
		})
	}
}
