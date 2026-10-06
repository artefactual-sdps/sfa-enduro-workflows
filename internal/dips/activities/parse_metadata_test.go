package activities_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/activities"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/datatypes"
)

const metadataExport = `<?xml version="1.0"?>
<paket xmlns="http://bar.admin.ch/arelda/v4">
  <inhaltsverzeichnis>
    <ordner>
      <name>header</name>
      <datei id="_header"><name>metadata.xml</name></datei>
    </ordner>
    <ordner>
      <name>content</name>
      <originalName>ignored-content-name</originalName>
      <datei id="_one">
        <name>one.jp2</name>
        <originalName>ignored-file-name.jp2</originalName>
        <pruefalgorithmus>MD5</pruefalgorithmus>
        <pruefsumme>09ac7d901a0dd16f1f95bbfe015336ae</pruefsumme>
      </datei>
      <ordner>
        <name>folder</name>
        <originalName>ignored-folder-name</originalName>
        <ordner>
          <name>nested</name>
          <datei id="_two">
            <name>two.pdf</name>
            <pruefalgorithmus>SHA-256</pruefalgorithmus>
            <pruefsumme>checksum-two</pruefsumme>
          </datei>
        </ordner>
      </ordner>
      <ordner>
        <name>sibling</name>
        <datei id="_three">
          <name>three.txt</name>
          <pruefalgorithmus>SHA-1</pruefalgorithmus>
          <pruefsumme>checksum-three</pruefsumme>
        </datei>
      </ordner>
    </ordner>
  </inhaltsverzeichnis>
</paket>`

func TestParseMetadata(t *testing.T) {
	t.Parallel()

	aipUUIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for _, tt := range []struct {
		name      string
		metadata  string
		mets      []string
		wantFiles []*datatypes.File
		wantErr   string
	}{
		{
			name:     "Matches content files across AIPs",
			metadata: metadataExport,
			mets: []string{
				metsDocument(t, metsObject(t, "_one", "%transferDirectory%data/objects/one.jp2")),
				metsDocument(t,
					// A file already matched in the first AIP keeps that match.
					metsObject(t, "_one", "data/objects/duplicate.jp2"),
					metsObject(t, "_two", "data/objects/two.pdf"),
					metsObject(t, "_three", "%transferDirectory%data/objects/three.txt"),
					"<malformed>", // Stop parsing this METS once all files are matched.
				),
				"", // All files are matched, so this missing METS is not opened.
			},
			wantFiles: []*datatypes.File{
				{
					DateiID: "_one", DIPPath: "content/one.jp2",
					Checksum: "09ac7d901a0dd16f1f95bbfe015336ae", ChecksumAlgorithm: "MD5",
					AIPUUID: aipUUIDs[0], AIPPath: "data/objects/one.jp2",
				},
				{
					DateiID: "_two", DIPPath: "content/folder/nested/two.pdf",
					Checksum: "checksum-two", ChecksumAlgorithm: "SHA-256",
					AIPUUID: aipUUIDs[1], AIPPath: "data/objects/two.pdf",
				},
				{
					DateiID: "_three", DIPPath: "content/sibling/three.txt",
					Checksum: "checksum-three", ChecksumAlgorithm: "SHA-1",
					AIPUUID: aipUUIDs[1], AIPPath: "data/objects/three.txt",
				},
			},
		},
		{
			name:     "Reports every unmatched file",
			metadata: metadataExport,
			mets: []string{
				metsDocument(t, metsObject(t, "_one", "data/objects/one.jp2")),
			},
			wantErr: "files not found in AIP METS:\n" +
				"_two (content/folder/nested/two.pdf)\n_three (content/sibling/three.txt)",
		},
		{
			name: "Empty content",
			metadata: `<paket><inhaltsverzeichnis>
  <ordner><name>content</name></ordner>
</inhaltsverzeichnis></paket>`,
		},
		{
			name:    "Missing export",
			wantErr: "open ACTApro export:",
		},
		{
			name:     "Malformed export",
			metadata: "<paket>",
			wantErr:  "parse ACTApro export:",
		},
		{
			name:     "Missing METS",
			metadata: metadataExport,
			mets:     []string{""},
			wantErr:  "open METS:",
		},
		{
			name:     "Malformed METS",
			metadata: metadataExport,
			mets:     []string{"<mets>"},
			wantErr:  "parse METS:",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			params := &activities.ParseMetadataParams{
				MetadataPath: filepath.Join(dir, "metadata.xml"),
			}
			// Empty fixture strings leave the corresponding files missing.
			if tt.metadata != "" {
				assert.NilError(t, os.WriteFile(params.MetadataPath, []byte(tt.metadata), 0o600))
			}
			for i, mets := range tt.mets {
				metsPath := filepath.Join(dir, fmt.Sprintf("mets-%d.xml", i))
				if mets != "" {
					assert.NilError(t, os.WriteFile(metsPath, []byte(mets), 0o600))
				}
				params.AIPs = append(params.AIPs, activities.AIPMETS{
					AIPUUID: aipUUIDs[i], METSPath: metsPath,
				})
			}

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewParseMetadata().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.ParseMetadataName},
			)
			enc, err := env.ExecuteActivity(activities.ParseMetadataName, params)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)

			var res activities.ParseMetadataResult
			assert.NilError(t, enc.Get(&res))
			assert.DeepEqual(t, res.Files, tt.wantFiles)
		})
	}
}

func metsDocument(t *testing.T, objects ...string) string {
	t.Helper()
	return `<mets:mets xmlns:mets="http://www.loc.gov/METS/" xmlns:p="http://www.loc.gov/premis/v3"
xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<mets:amdSec><mets:techMD><mets:mdWrap MDTYPE="PREMIS:OBJECT"><mets:xmlData>` +
		strings.Join(objects, "") + `</mets:xmlData></mets:mdWrap></mets:techMD></mets:amdSec></mets:mets>`
}

func metsObject(t *testing.T, id, originalName string) string {
	t.Helper()
	return fmt.Sprintf(`<p:object xsi:type="p:file">
  <p:objectIdentifier>
    <p:objectIdentifierType>local</p:objectIdentifierType>
    <p:objectIdentifierValue>%s</p:objectIdentifierValue>
  </p:objectIdentifier>
  <p:originalName>%s</p:originalName>
</p:object>`, id, originalName)
}
