package amss_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"go.artefactual.dev/ssclient"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/amss"
)

func newPackagesService(t *testing.T, handler http.HandlerFunc) *ssclient.PackagesService {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := ssclient.New(ssclient.Config{
		BaseURL:  srv.URL,
		Username: "test",
		Key:      "test",
	})
	assert.NilError(t, err)

	return client.Packages()
}

func TestFetchActivity(t *testing.T) {
	t.Parallel()

	const (
		relativeAIP = "test-9390594f-84c2-457d-bd6a-618f21f7c954/data/METS.9390594f-84c2-457d-bd6a-618f21f7c954.xml"
		fileData    = "<mets/>"
	)
	aipUUID := uuid.MustParse("9390594f-84c2-457d-bd6a-618f21f7c954")
	aipUUIDString := aipUUID.String()

	for _, tt := range []struct {
		name              string
		checksum          string
		checksumAlgorithm string
		wantErr           string
	}{
		{
			name: "downloads without checksum validation",
		},
		{
			name:     "skips validation without an algorithm",
			checksum: "incorrect",
		},
		{
			name:              "skips validation without a hash",
			checksumAlgorithm: "unsupported",
		},
		{
			name:              "validates an MD5 checksum",
			checksum:          "cf8a7acb620a9ff4c77db811d5a548ed",
			checksumAlgorithm: "MD5",
		},
		{
			name:              "validates an uppercase checksum",
			checksum:          "CF8A7ACB620A9FF4C77DB811D5A548ED",
			checksumAlgorithm: "MD5",
		},
		{
			name:              "validates a SHA-256 checksum",
			checksum:          "42e902792b1f78b461f3a9b3a32416140edea5c73e19d63086053435219c2aba",
			checksumAlgorithm: "SHA-256",
		},
		{
			name:              "checksum mismatch",
			checksum:          "incorrect",
			checksumAlgorithm: "MD5",
			wantErr: `FetchActivity: checksum mismatch ` +
				`(expected: "incorrect", got: "cf8a7acb620a9ff4c77db811d5a548ed")`,
		},
		{
			name:              "unsupported checksum algorithm",
			checksum:          "cf8a7acb620a9ff4c77db811d5a548ed",
			checksumAlgorithm: "unsupported",
			wantErr:           `FetchActivity: validate checksum: hash algorithm "unsupported" is not supported`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			destination := filepath.Join(t.TempDir(), "nested", "METS.xml")
			packages := newPackagesService(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, http.MethodGet)
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/api/v2/file/%s/extract_file/", aipUUIDString))
				assert.Equal(t, r.URL.Query().Get("relative_path_to_file"), relativeAIP)
				assert.Equal(t, r.Header.Get("Authorization"), "ApiKey test:test")

				w.Header().Set("Content-Type", "application/xml")
				fmt.Fprint(w, fileData)
			})

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				amss.NewFetchActivity(packages).Execute,
				temporalsdk_activity.RegisterOptions{Name: amss.FetchActivityName},
			)

			future, err := env.ExecuteActivity(
				amss.FetchActivityName,
				&amss.FetchActivityParams{
					AIPUUID:           aipUUID,
					RelativePath:      relativeAIP,
					Destination:       destination,
					Checksum:          tt.checksum,
					ChecksumAlgorithm: tt.checksumAlgorithm,
				},
			)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var result amss.FetchActivityResult
			assert.NilError(t, future.Get(&result))

			got, err := os.ReadFile(destination)
			assert.NilError(t, err)
			assert.Equal(t, string(got), fileData)
		})
	}
}
