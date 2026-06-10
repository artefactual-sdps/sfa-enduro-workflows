package localact_test

import (
	"errors"
	"testing"

	"go.artefactual.dev/tools/mockutil"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/localact"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/persistence/fake"
)

func TestSavePreprocessingTasksActivity(t *testing.T) {
	t.Parallel()

	name := "test.zip"
	checksum := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	type test struct {
		name      string
		params    localact.CheckDuplicateParams
		mockCalls func(m *fake.MockServiceMockRecorder)
		want      localact.CheckDuplicateResult
		wantErr   string
	}
	for _, tt := range []test{
		{
			name: "Checks duplicate (none found)",
			params: localact.CheckDuplicateParams{
				Name:     name,
				Checksum: checksum,
			},
			mockCalls: func(m *fake.MockServiceMockRecorder) {
				m.CreateSIP(mockutil.Context(), name, checksum).Return(nil)
			},
			want: localact.CheckDuplicateResult{},
		},
		{
			name: "Checks duplicate (found)",
			params: localact.CheckDuplicateParams{
				Name:     name,
				Checksum: checksum,
			},
			mockCalls: func(m *fake.MockServiceMockRecorder) {
				m.CreateSIP(mockutil.Context(), name, checksum).Return(persistence.ErrDuplicatedSIP)
			},
			want: localact.CheckDuplicateResult{IsDuplicate: true},
		},
		{
			name: "Checks duplicate (error)",
			params: localact.CheckDuplicateParams{
				Name:     name,
				Checksum: checksum,
			},
			mockCalls: func(m *fake.MockServiceMockRecorder) {
				m.CreateSIP(mockutil.Context(), name, checksum).Return(errors.New("fake error"))
			},
			wantErr: "CheckDuplicate: fake error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			svc := fake.NewMockService(gomock.NewController(t))
			if tt.mockCalls != nil {
				tt.mockCalls(svc.EXPECT())
			}

			enc, err := env.ExecuteLocalActivity(
				localact.CheckDuplicate,
				svc,
				&tt.params,
			)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var res localact.CheckDuplicateResult
			_ = enc.Get(&res)
			assert.DeepEqual(t, res, tt.want)
		})
	}
}
