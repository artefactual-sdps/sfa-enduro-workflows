package activities_test

import (
	"fmt"
	"path/filepath"
	"testing"

	temporalsdk_activity "go.temporal.io/sdk/activity"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/activities"
)

const (
	digitizedAIPUpdatedArelda = `
<?xml version="1.0" encoding="UTF-8"?>
<paket
	xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xmlns:xip="http://www.tessella.com/XIP/v4"
	xmlns="http://bar.admin.ch/arelda/v4"
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:submissionTests="http://bar.admin.ch/submissionTestResult" xsi:type="paketAIP" schemaVersion="5.0">
	<paketTyp>AIP</paketTyp>
	<globaleAIPId>909c56e9-e334-4c0a-9736-f92c732149d9</globaleAIPId>
	<lokaleAIPId>fa5fb285-fa45-44e4-8d85-77ec1d774403</lokaleAIPId>
	<version>1</version>
	<inhaltsverzeichnis>
		<ordner>
			<name>header</name>
			<ordner>
				<name>old</name>
				<ordner>
					<name>SIP</name>
					<datei id="OLD_SIP">
						<name>metadata.xml</name>
						<originalName>metadata.xml</originalName>
						<pruefalgorithmus>MD5</pruefalgorithmus>
						<pruefsumme>2c5afa141670292c96c3c111c47b83b5</pruefsumme>
					</datei>
				</ordner>
			</ordner>
			<ordner>
				<name>xsd</name>
				<datei id="_xAlSBc3dYcypUMvN8HzeN5">
					<name>arelda.xsd</name>
					<originalName>arelda.xsd</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>661C2DF1B1E76D1446E90A54816D91AE</pruefsumme>
				</datei>
			</ordner>
		</ordner>
		<ordner>
			<name>content</name>
			<ordner>
				<name>d_0000001</name>
				<datei id="_SRpeVgb4xGImymb23OH1od">
					<name>00000001_PREMIS.xml</name>
					<originalName>00000001_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>e80b5017098950fc58aad83c8c14978e</pruefsumme>
				</datei>
				<datei id="_MKhAIC639MxzyOn8ji3tN5">
					<name>00000002_PREMIS.xml</name>
					<originalName>00000002_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>33f12195e0fc136bc17de332c6b92b0d</pruefsumme>
				</datei>
				<datei id="_fZzi3dX2jvrwakvY6jeJS8">
					<name>Prozess_Digitalisierung_PREMIS.xml</name>
					<originalName>Prozess_Digitalisierung_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>816cabd1c0334ed363555889d9f4dbe4</pruefsumme>
				</datei>
				<datei id="_miEf29GTkFR7ymi91IV4fO">
					<name>00000001.jp2</name>
					<originalName>00000001.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>827ccb0eea8a706c4c34a16891f84e7b</pruefsumme>
				</datei>
				<datei id="_mOXw3hINt3zY6WvKQOfYmk">
					<name>00000002.jp2</name>
					<originalName>00000002.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>1e01ba3e07ac48cbdab2d3284d1dd0fa</pruefsumme>
				</datei>
			</ordner>
		</ordner>
	</inhaltsverzeichnis>
</paket>
`

	bornDigitalAIPUpdatedArelda = `
<?xml version="1.0" encoding="UTF-8"?>
<paket
	xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	xmlns:xip="http://www.tessella.com/XIP/v4"
	xmlns="http://bar.admin.ch/arelda/v4"
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:submissionTests="http://bar.admin.ch/submissionTestResult" xsi:type="paketAIP" schemaVersion="5.0">
	<paketTyp>AIP</paketTyp>
	<globaleAIPId>909c56e9-e334-4c0a-9736-f92c732149d9</globaleAIPId>
	<lokaleAIPId>fa5fb285-fa45-44e4-8d85-77ec1d774403</lokaleAIPId>
	<version>1</version>
	<inhaltsverzeichnis>
		<ordner>
			<name>header</name>
			<ordner>
				<name>old</name>
				<ordner>
					<name>SIP</name>
					<datei id="OLD_SIP">
						<name>metadata.xml</name>
						<originalName>metadata.xml</originalName>
						<pruefalgorithmus>MD5</pruefalgorithmus>
						<pruefsumme>2c5afa141670292c96c3c111c47b83b5</pruefsumme>
					</datei>
				</ordner>
			</ordner>
			<ordner>
				<name>xsd</name>
				<datei id="_xAlSBc3dYcypUMvN8HzeN5">
					<name>arelda.xsd</name>
					<originalName>arelda.xsd</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>661c2df1b1e76d1446e90a54816d91ae</pruefsumme>
				</datei>
			</ordner>
		</ordner>
		<ordner>
			<name>content</name>
			<ordner>
				<name>d_0000001</name>
				<datei id="_SRpeVgb4xGImymb23OH1od">
					<name>00000001_PREMIS.xml</name>
					<originalName>00000001_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>e80b5017098950fc58aad83c8c14978e</pruefsumme>
				</datei>
				<datei id="_MKhAIC639MxzyOn8ji3tN5">
					<name>00000002_PREMIS.xml</name>
					<originalName>00000002_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>33f12195e0fc136bc17de332c6b92b0d</pruefsumme>
				</datei>
				<datei id="_miEf29GTkFR7ymi91IV4fO">
					<name>00000001.jp2</name>
					<originalName>00000001.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>827ccb0eea8a706c4c34a16891f84e7b</pruefsumme>
				</datei>
				<datei id="_mOXw3hINt3zY6WvKQOfYmk">
					<name>00000002.jp2</name>
					<originalName>00000002.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>1e01ba3e07ac48cbdab2d3284d1dd0fa</pruefsumme>
				</datei>
			</ordner>
		</ordner>
	</inhaltsverzeichnis>
</paket>
`

	digitizedAIPMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<paket xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://bar.admin.ch/arelda/v4" xsi:type="paketSIP" schemaVersion="4.0">
  <paketTyp>SIP</paketTyp>
  <inhaltsverzeichnis>
    <ordner>
      <name>header</name>
      <ordner>
        <name>xsd</name>
        <datei id="XSD3">
          <name>arelda.xsd</name>
          <originalName>arelda.xsd</originalName>
          <pruefalgorithmus>MD5</pruefalgorithmus>
          <pruefsumme>f8454632e1ebf97e0aa8d9527ce2641f</pruefsumme>
        </datei>
      </ordner>
    </ordner>
    <ordner>
      <name>content</name>
      <ordner>
        <name>d0001</name>
        <datei id="_SRpeVgb4xGImymb23OH1od">
			<name>00000001_PREMIS.xml</name>
			<originalName>00000001_PREMIS.xml</originalName>
			<pruefalgorithmus>MD5</pruefalgorithmus>
			<pruefsumme>e80b5017098950fc58aad83c8c14978e</pruefsumme>
		</datei>
		<datei id="_MKhAIC639MxzyOn8ji3tN5">
			<name>00000002_PREMIS.xml</name>
			<originalName>00000002_PREMIS.xml</originalName>
			<pruefalgorithmus>MD5</pruefalgorithmus>
			<pruefsumme>33f12195e0fc136bc17de332c6b92b0d</pruefsumme>
		</datei>
		<datei id="_miEf29GTkFR7ymi91IV4fO">
			<name>00000001.jp2</name>
			<originalName>00000001.jp2</originalName>
			<pruefalgorithmus>MD5</pruefalgorithmus>
			<pruefsumme>827ccb0eea8a706c4c34a16891f84e7b</pruefsumme>
		</datei>
		<datei id="_mOXw3hINt3zY6WvKQOfYmk">
			<name>00000002.jp2</name>
			<originalName>00000002.jp2</originalName>
			<pruefalgorithmus>MD5</pruefalgorithmus>
			<pruefsumme>1e01ba3e07ac48cbdab2d3284d1dd0fa</pruefsumme>
		</datei>
      </ordner>
    </ordner>
  </inhaltsverzeichnis>
</paket>
`

	digitizedSIPMetadata = `
<?xml version="1.0" encoding="UTF-8"?>
<paket
	xmlns="http://bar.admin.ch/arelda/v4"
	xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" schemaVersion="4.0" xsi:type="paketSIP">
	<paketTyp>SIP</paketTyp>
	<inhaltsverzeichnis>
		<ordner>
			<name>header</name>
			<originalName>header</originalName>
			<ordner>
				<name>xsd</name>
				<originalName>xsd</originalName>
				<datei id="_ZSANrSklQ9HGn99yjlUumz">
					<name>arelda.xsd</name>
					<originalName>arelda.xsd</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>661c2df1b1e76d1446e90a54816d91ae</pruefsumme>
				</datei>
			</ordner>
		</ordner>
		<ordner>
			<name>content</name>
			<originalName>content</originalName>
			<ordner>
				<name>d_0000001</name>
				<originalName>d_0000001</originalName>
				<datei id="_zodSTSD0nv05CpOp6JoV3X">
					<name>00000001.jp2</name>
					<originalName>00000001.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>827ccb0eea8a706c4c34a16891f84e7b</pruefsumme>
				</datei>
				<datei id="_rlPKJX9ZcAl4ooc4IfoIkM">
					<name>00000002.jp2</name>
					<originalName>00000002.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>1e01ba3e07ac48cbdab2d3284d1dd0fa</pruefsumme>
				</datei>
				<datei id="_WuDmXAs5UDwKTGVLsCcZxa">
					<name>00000001_PREMIS.xml</name>
					<originalName>00000001_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>e80b5017098950fc58aad83c8c14978e</pruefsumme>
				</datei>
				<datei id="_Ohk77y2DJa82RXqsWG4S90">
					<name>00000002_PREMIS.xml</name>
					<originalName>00000002_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>33f12195e0fc136bc17de332c6b92b0d</pruefsumme>
				</datei>
				<datei id="_cQ6sm5CChWVqtqmrWvne0W">
					<name>Prozess_Digitalisierung_PREMIS.xml</name>
					<originalName>Prozess_Digitalisierung_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>816cabd1c0334ed363555889d9f4dbe4</pruefsumme>
				</datei>
			</ordner>
		</ordner>
	</inhaltsverzeichnis>
</paket>
`

	bornDigitalSIPMetadataXML = `
<?xml version="1.0" encoding="UTF-8"?>
<paket
	xmlns="http://bar.admin.ch/arelda/v4"
	xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" schemaVersion="4.0" xsi:type="paketSIP">
	<paketTyp>SIP</paketTyp>
	<inhaltsverzeichnis>
		<ordner>
			<name>header</name>
			<originalName>header</originalName>
			<ordner>
				<name>xsd</name>
				<originalName>xsd</originalName>
				<datei id="_ZSANrSklQ9HGn99yjlUumz">
					<name>arelda.xsd</name>
					<originalName>arelda.xsd</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>661c2df1b1e76d1446e90a54816d91ae</pruefsumme>
				</datei>
			</ordner>
		</ordner>
		<ordner>
			<name>content</name>
			<originalName>content</originalName>
			<ordner>
				<name>d_0000001</name>
				<originalName>d_0000001</originalName>
				<datei id="_zodSTSD0nv05CpOp6JoV3X">
					<name>00000001.jp2</name>
					<originalName>00000001.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>827ccb0eea8a706c4c34a16891f84e7b</pruefsumme>
				</datei>
				<datei id="_rlPKJX9ZcAl4ooc4IfoIkM">
					<name>00000002.jp2</name>
					<originalName>00000002.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>1e01ba3e07ac48cbdab2d3284d1dd0fa</pruefsumme>
				</datei>
				<datei id="_WuDmXAs5UDwKTGVLsCcZxa">
					<name>00000001_PREMIS.xml</name>
					<originalName>00000001_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>e80b5017098950fc58aad83c8c14978e</pruefsumme>
				</datei>
				<datei id="_Ohk77y2DJa82RXqsWG4S90">
					<name>00000002_PREMIS.xml</name>
					<originalName>00000002_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>33f12195e0fc136bc17de332c6b92b0d</pruefsumme>
				</datei>
			</ordner>
		</ordner>
	</inhaltsverzeichnis>
</paket>
`

	unsupportedSchemaMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<paket xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://bar.admin.ch/arelda/v4" xsi:type="paketSIP" schemaVersion="5.1">
</paket>
`
)

func TestVerifyManifest(t *testing.T) {
	t.Parallel()

	missingFilesSIP := fs.NewDir(t, "Test_Missing_Files",
		fs.WithDir("additional",
			fs.WithFile("UpdatedAreldaMetadata.xml", digitizedAIPUpdatedArelda),
		),
		fs.WithDir("content",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					// fs.WithFile("00000001.jp2", "12345"),
					fs.WithFile("00000001_PREMIS.xml", "abcdef"),
					fs.WithFile("00000002.jp2", "67890"),
					fs.WithFile("00000002_PREMIS.xml", "ghijk"),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", "lmnop"),
				),
			),
			fs.WithDir("header",
				fs.WithDir("old",
					fs.WithDir("SIP",
						fs.WithFile("metadata.xml", digitizedAIPMetadata),
					),
				),
			),
		),
	)

	extraFilesSIP := fs.NewDir(t, "Test_Extra_Files",
		fs.WithDir("additional",
			fs.WithFile("UpdatedAreldaMetadata.xml", digitizedAIPUpdatedArelda),
		),
		fs.WithDir("content",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("extra_file.txt", "I'm an extra file."),
					fs.WithFile("00000001.jp2", "12345"),
					fs.WithFile("00000001_PREMIS.xml", "abcdef"),
					fs.WithFile("00000002.jp2", "67890"),
					fs.WithFile("00000002_PREMIS.xml", "ghijk"),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", "lmnop"),
				),
			),
			fs.WithDir("header",
				fs.WithDir("old",
					fs.WithDir("SIP",
						fs.WithFile("metadata.xml", digitizedAIPMetadata),
					),
				),
				fs.WithDir("xsd",
					fs.WithFile("arelda.xsd", "vwxyz"),
					fs.WithFile("extra.xsd", "I'm an extra XSD file."),
				),
			),
		),
	)

	tests := []struct {
		name    string
		params  activities.VerifyManifestParams
		want    activities.VerifyManifestResult
		wantErr string
	}{
		{
			name: "Verifies a digitized AIP manifest",
			params: activities.VerifyManifestParams{
				SIP: testSIP(t, "../testdata/little-Test-AIP-Digitization"),
			},
			want: activities.VerifyManifestResult{},
		},
		{
			name: "Verifies a digitized SIP manifest",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					fs.NewDir(t, "Test_Digitized_SIP",
						fs.WithDir("content",
							fs.WithDir("d_0000001",
								fs.WithFile("00000001.jp2", "12345"),
								fs.WithFile("00000001_PREMIS.xml", "abcdef"),
								fs.WithFile("00000002.jp2", "67890"),
								fs.WithFile("00000002_PREMIS.xml", "ghijk"),
								fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", "lmnop"),
							),
						),
						fs.WithDir("header",
							fs.WithDir("xsd",
								fs.WithFile("arelda.xsd", "vwxyz"),
							),
							fs.WithFile("metadata.xml", digitizedSIPMetadata),
						),
					).Path(),
				),
			},
			want: activities.VerifyManifestResult{},
		},
		{
			name: "Verifies a born digital AIP manifest",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					fs.NewDir(t, "Test_Born_Digital_AIP",
						fs.WithDir("additional",
							fs.WithFile("UpdatedAreldaMetadata.xml", bornDigitalAIPUpdatedArelda),
						),
						fs.WithDir("content",
							fs.WithDir("content",
								fs.WithDir("d_0000001",
									fs.WithFile("00000001.jp2", "12345"),
									fs.WithFile("00000001_PREMIS.xml", "abcdef"),
									fs.WithFile("00000002.jp2", "67890"),
									fs.WithFile("00000002_PREMIS.xml", "ghijk"),
								),
							),
							fs.WithDir("header",
								fs.WithDir("xsd",
									fs.WithFile("arelda.xsd", "vwxyz"),
								),
								fs.WithDir("old",
									fs.WithDir("SIP",
										fs.WithFile("metadata.xml", digitizedAIPMetadata),
									),
								),
							),
						),
					).Path(),
				),
			},
			want: activities.VerifyManifestResult{},
		},
		{
			name: "Verifies a born digital SIP manifest",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					fs.NewDir(t, "Test_Born_Digital_SIP",
						fs.WithDir("content",
							fs.WithDir("d_0000001",
								fs.WithFile("00000001.jp2", "12345"),
								fs.WithFile("00000001_PREMIS.xml", "abcdef"),
								fs.WithFile("00000002.jp2", "67890"),
								fs.WithFile("00000002_PREMIS.xml", "ghijk"),
							),
						),
						fs.WithDir("header",
							fs.WithDir("xsd",
								fs.WithFile("arelda.xsd", "vwxyz"),
							),
							fs.WithFile("metadata.xml", bornDigitalSIPMetadataXML),
						),
					).Path(),
				),
			},
			want: activities.VerifyManifestResult{},
		},
		{
			name: "Returns a list of missing files",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					missingFilesSIP.Path(),
				),
			},
			want: activities.VerifyManifestResult{
				MissingFiles: []string{
					fmt.Sprintf(
						"Missing file: %s/content/content/d_0000001/00000001.jp2",
						filepath.Base(missingFilesSIP.Path()),
					),
					fmt.Sprintf(
						"Missing file: %s/content/header/xsd/arelda.xsd",
						filepath.Base(missingFilesSIP.Path()),
					),
				},
			},
		},
		{
			name: "Returns a list of extra files",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					extraFilesSIP.Path(),
				),
			},
			want: activities.VerifyManifestResult{
				UnexpectedFiles: []string{
					fmt.Sprintf(
						"Unexpected file: %s/content/content/d_0000001/extra_file.txt",
						filepath.Base(extraFilesSIP.Path()),
					),
					fmt.Sprintf(
						"Unexpected file: %s/content/header/xsd/extra.xsd",
						filepath.Base(extraFilesSIP.Path()),
					),
				},
			},
		},
		{
			name: "Returns a list of mismatched checksums",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					fs.NewDir(t, "Test_Extra_Files",
						fs.WithDir("additional",
							fs.WithFile("UpdatedAreldaMetadata.xml", digitizedAIPUpdatedArelda),
						),
						fs.WithDir("content",
							fs.WithDir("content",
								fs.WithDir("d_0000001",
									fs.WithFile("00000001.jp2", "wrong checksum"),
									fs.WithFile("00000001_PREMIS.xml", "abcdef"),
									fs.WithFile("00000002.jp2", "67890"),
									fs.WithFile("00000002_PREMIS.xml", "ghijk"),
									fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", "lmnop"),
								),
							),
							fs.WithDir("header",
								fs.WithDir("old",
									fs.WithDir("SIP",
										fs.WithFile("metadata.xml", "also wrong checksum"),
									),
								),
								fs.WithDir("xsd",
									fs.WithFile("arelda.xsd", "vwxyz"),
								),
							),
						),
					).Path(),
				),
			},
			want: activities.VerifyManifestResult{
				ChecksumFailures: []string{
					`Checksum mismatch for "content/content/d_0000001/00000001.jp2" (expected: "827ccb0eea8a706c4c34a16891f84e7b", got: "2714364e3a0ac68e8bf9b898b31ff303")`,
					`Checksum mismatch for "content/header/old/SIP/metadata.xml" (expected: "2c5afa141670292c96c3c111c47b83b5", got: "dff24b6a34ff7ab645cb477e090bee5f")`,
				},
			},
		},
		{
			name: "Returns an unsupported schema version error",
			params: activities.VerifyManifestParams{
				SIP: testSIP(
					t,
					fs.NewDir(t, "Test_Extra_Files",
						fs.WithDir("additional",
							fs.WithFile("UpdatedAreldaMetadata.xml", unsupportedSchemaMetadata),
						),
						fs.WithDir("content"),
					).Path(),
				),
			},
			want: activities.VerifyManifestResult{
				ManifestFailures: []string{"Unsupported schema version: 5.1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ts := &temporalsdk_testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()
			env.RegisterActivityWithOptions(
				activities.NewVerifyManifest().Execute,
				temporalsdk_activity.RegisterOptions{Name: activities.VerifyManifestName},
			)

			future, err := env.ExecuteActivity(activities.VerifyManifestName, tt.params)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}
			assert.NilError(t, err)

			var res activities.VerifyManifestResult
			assert.NilError(t, future.Get(&res))
			assert.DeepEqual(t, res, tt.want)
		})
	}
}
