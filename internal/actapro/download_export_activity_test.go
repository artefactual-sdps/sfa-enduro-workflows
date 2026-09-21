package actapro_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

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

func TestDownloadExportActivity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		response     actaprogen.GetExportFileRes
		clientErr    error
		want         string
		wantErr      string
		nonRetryable bool
	}{
		{
			name: "downloads the export binary",
			response: &actaprogen.GetExportFileOKApplicationOctetStream{
				Data: strings.NewReader("<metadata>\x00\xff</metadata>\n"),
			},
			want: "<metadata>\x00\xff</metadata>\n",
		},
		{
			name: "downloads the XML export",
			response: &actaprogen.GetExportFileOKApplicationXML{
				Data: strings.NewReader("<metadata>Über &amp; export</metadata>\r\n"),
			},
			want: "<metadata>Über &amp; export</metadata>\r\n",
		},
		{
			name:         "rejects a missing export binary",
			response:     &actaprogen.GetExportFileOKApplicationOctetStream{},
			wantErr:      "download ACTApro export: missing export binary",
			nonRetryable: true,
		},
		{
			name:         "rejects a missing XML export",
			response:     &actaprogen.GetExportFileOKApplicationXML{},
			wantErr:      "download ACTApro export: missing export binary",
			nonRetryable: true,
		},
		{
			name: "returns bad request error",
			response: &actaprogen.GetExportFileBadRequest{
				Message: actaprogen.NewOptString("invalid export ID"),
			},
			wantErr:      "download ACTApro export: bad request: invalid export ID",
			nonRetryable: true,
		},
		{
			name: "returns forbidden error",
			response: &actaprogen.GetExportFileForbidden{
				Message: actaprogen.NewOptString("access denied"),
			},
			wantErr:      "download ACTApro export: forbidden: access denied",
			nonRetryable: true,
		},
		{
			name: "returns not found error",
			response: &actaprogen.GetExportFileNotFound{
				Message: actaprogen.NewOptString("unknown export"),
			},
			wantErr:      "download ACTApro export: not found: unknown export",
			nonRetryable: true,
		},
		{
			name: "returns retryable conflict error",
			response: &actaprogen.GetExportFileConflict{
				Message: actaprogen.NewOptString("export in progress"),
			},
			wantErr:      "download ACTApro export: conflict: export in progress",
			nonRetryable: false,
		},
		{
			name: "returns retryable locked error",
			response: &actaprogen.GetExportFileLocked{
				Message: actaprogen.NewOptString("export is locked"),
			},
			wantErr:      "download ACTApro export: locked: export is locked",
			nonRetryable: false,
		},
		{
			name: "returns retryable server error",
			response: &actaprogen.GetExportFileInternalServerError{
				Message: actaprogen.NewOptString("service unavailable"),
			},
			wantErr:      "download ACTApro export: server error: service unavailable",
			nonRetryable: false,
		},
		{
			name:         "returns retryable client error",
			clientErr:    errors.New("error from client"),
			wantErr:      "download ACTApro export: error from client",
			nonRetryable: false,
		},
		{
			name:         "returns retryable unexpected response error",
			wantErr:      "download ACTApro export: unexpected response",
			nonRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			client.EXPECT().GetExportFile(gomock.Any(), actaprogen.GetExportFileParams{
				ID: uuid.MustParse("1ef33301-83c4-407c-9895-18d16a1f10b9"),
			}).Return(tt.response, tt.clientErr)

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewDownloadExportActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.DownloadExportActivityName},
			)
			metadataPath := filepath.Join(t.TempDir(), "dip", "metadata.xml")
			future, err := env.ExecuteActivity(actapro.DownloadExportActivityName, &actapro.DownloadExportParams{
				ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
				MetadataPath: metadataPath,
			})
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				var applicationErr *temporalsdk_temporal.ApplicationError
				assert.Assert(t, errors.As(err, &applicationErr))
				assert.Equal(t, applicationErr.NonRetryable(), tt.nonRetryable)
				_, err := os.Stat(metadataPath)
				assert.Assert(t, errors.Is(err, os.ErrNotExist))
				return
			}
			assert.NilError(t, err)

			var result actapro.DownloadExportResult
			assert.NilError(t, future.Get(&result))
			data, err := os.ReadFile(metadataPath)
			assert.NilError(t, err)
			assert.Equal(t, string(data), tt.want)
		})
	}
}

func TestDownloadExportActivityInvalidParams(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		params  actapro.DownloadExportParams
		wantErr string
	}{
		{
			name:    "missing export ID",
			params:  actapro.DownloadExportParams{MetadataPath: filepath.Join(t.TempDir(), "metadata.xml")},
			wantErr: "download ACTApro export: invalid export ID",
		},
		{
			name: "invalid export ID",
			params: actapro.DownloadExportParams{
				ExportID:     "invalid",
				MetadataPath: filepath.Join(t.TempDir(), "metadata.xml"),
			},
			wantErr: "download ACTApro export: invalid export ID",
		},
		{
			name:    "missing metadata path",
			params:  actapro.DownloadExportParams{ExportID: "1ef33301-83c4-407c-9895-18d16a1f10b9"},
			wantErr: "download ACTApro export: missing metadata path",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewDownloadExportActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.DownloadExportActivityName},
			)
			_, err := env.ExecuteActivity(actapro.DownloadExportActivityName, &tt.params)
			assert.ErrorContains(t, err, tt.wantErr)
			var applicationErr *temporalsdk_temporal.ApplicationError
			assert.Assert(t, errors.As(err, &applicationErr))
			assert.Assert(t, applicationErr.NonRetryable())
		})
	}
}

func TestDownloadExportActivityFileErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		prepare func(*testing.T) string
		data    io.Reader
		wantErr string
	}{
		{
			name: "cannot create parent directory",
			prepare: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "file")
				assert.NilError(t, os.WriteFile(path, nil, 0o600))
				return filepath.Join(path, "metadata.xml")
			},
			data:    strings.NewReader("<metadata/>"),
			wantErr: "download ACTApro export: create directory:",
		},
		{
			name:    "cannot create metadata file",
			prepare: func(t *testing.T) string { return t.TempDir() },
			data:    strings.NewReader("<metadata/>"),
			wantErr: "download ACTApro export: create file:",
		},
		{
			name:    "cannot read export binary",
			prepare: func(t *testing.T) string { return filepath.Join(t.TempDir(), "metadata.xml") },
			data:    iotest.ErrReader(errors.New("read failed")),
			wantErr: "download ACTApro export: write file: read failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := fake_actapro.NewMockClient(gomock.NewController(t))
			client.EXPECT().GetExportFile(gomock.Any(), actaprogen.GetExportFileParams{
				ID: uuid.MustParse("1ef33301-83c4-407c-9895-18d16a1f10b9"),
			}).Return(&actaprogen.GetExportFileOKApplicationOctetStream{Data: tt.data}, nil)

			suite := temporalsdk_testsuite.WorkflowTestSuite{}
			env := suite.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				actapro.NewDownloadExportActivity(client).Execute,
				temporalsdk_activity.RegisterOptions{Name: actapro.DownloadExportActivityName},
			)
			_, err := env.ExecuteActivity(actapro.DownloadExportActivityName, &actapro.DownloadExportParams{
				ExportID:     "1ef33301-83c4-407c-9895-18d16a1f10b9",
				MetadataPath: tt.prepare(t),
			})
			assert.ErrorContains(t, err, tt.wantErr)
			var applicationErr *temporalsdk_temporal.ApplicationError
			assert.Assert(t, errors.As(err, &applicationErr))
			assert.Assert(t, !applicationErr.NonRetryable())
		})
	}
}
