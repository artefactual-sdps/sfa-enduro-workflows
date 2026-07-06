package localact_test

import (
	"testing"

	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/localact"
)

func TestIsBag(t *testing.T) {
	t.Parallel()

	bagPath := fs.NewDir(t, "ppsfa-test",
		fs.WithFile("bagit.txt", ""),
	).Path()
	emptyPath := fs.NewDir(t, "ppsfa-test").Path()

	type test struct {
		name   string
		params localact.IsBagParams
		want   localact.IsBagResult
	}
	for _, tt := range []test{
		{
			name:   "Is a bag",
			params: localact.IsBagParams{Path: bagPath},
			want:   localact.IsBagResult{IsBag: true},
		},
		{
			name:   "Is not a bag",
			params: localact.IsBagParams{Path: emptyPath},
			want:   localact.IsBagResult{IsBag: false},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()

			var res localact.IsBagResult
			enc, err := env.ExecuteLocalActivity(localact.IsBag, &tt.params)
			assert.NilError(t, err)

			assert.NilError(t, enc.Get(&res))
			assert.DeepEqual(t, res, tt.want)
		})
	}
}
