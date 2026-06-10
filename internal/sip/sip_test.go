package sip_test

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func TestNew(t *testing.T) {
	t.Parallel()

	digitizedAIP := fs.NewDir(t, "",
		fs.WithDir("Digitized-AIP",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			),
			fs.WithDir("additional"),
		),
	)
	digiAIPPath := digitizedAIP.Join("Digitized-AIP")

	digitizedSIPName := "SIP_20201201_Vecteur_someref"
	digitizedSIPPath := filepath.Join(digitizedSIPTempDir(t, digitizedSIPName), digitizedSIPName)

	digitizedSIPCaseName := "SIP_20201201_vecteur_someref"
	digitizedSIPCasePath := filepath.Join(digitizedSIPTempDir(t, digitizedSIPCaseName), digitizedSIPCaseName)

	bornDigitalAIP := fs.NewDir(t, "",
		fs.WithDir("Born-Digital-AIP",
			fs.WithDir("additional",
				fs.WithFile("UpdatedAreldaMetadata.xml", ""),
				fs.WithFile("Born-Digital-AIP-premis.xml", ""),
			),
			fs.WithDir("content",
				fs.WithDir("content"),
				fs.WithDir("header",
					fs.WithDir("old",
						fs.WithDir("SIP",
							fs.WithFile("metadata.xml", ""),
						),
					),
					fs.WithDir("xsd"),
				),
			),
		),
	)
	bdAIPPath := bornDigitalAIP.Join("Born-Digital-AIP")

	bornDigitalSIPName := "SIP_20201201_someoffice_someref"
	bornDigitalSIPTempDir := fs.NewDir(t, "",
		fs.WithDir(bornDigitalSIPName,
			fs.WithDir("content"),
			fs.WithDir("header"),
		),
	).Path()
	bornDigitalSIPPath := filepath.Join(bornDigitalSIPTempDir, bornDigitalSIPName)

	tests := []struct {
		name    string
		path    string
		wantSIP sip.SIP
		wantErr string
	}{
		{
			name: "Creates a new digitized AIP",
			path: digiAIPPath,
			wantSIP: sip.SIP{
				Type:                enums.SIPTypeDigitizedAIP,
				Path:                digiAIPPath,
				ContentPath:         filepath.Join(digiAIPPath, "content", "content"),
				LogicalMDPath:       filepath.Join(digiAIPPath, "additional", "Digitized-AIP-premis.xml"),
				ManifestPath:        filepath.Join(digiAIPPath, "additional", "UpdatedAreldaMetadata.xml"),
				MetadataPath:        filepath.Join(digiAIPPath, "content", "header", "old", "SIP", "metadata.xml"),
				UpdatedAreldaMDPath: filepath.Join(digiAIPPath, "additional", "UpdatedAreldaMetadata.xml"),
				XSDPath:             filepath.Join(digiAIPPath, "content", "header", "xsd", "arelda.xsd"),
				TopLevelPaths: []string{
					filepath.Join(digiAIPPath, "content"),
					filepath.Join(digiAIPPath, "additional"),
				},
			},
		},
		{
			name: "Creates a new born digital AIP",
			path: bdAIPPath,
			wantSIP: sip.SIP{
				Type:                enums.SIPTypeBornDigitalAIP,
				Path:                bdAIPPath,
				ContentPath:         filepath.Join(bdAIPPath, "content", "content"),
				LogicalMDPath:       filepath.Join(bdAIPPath, "additional", "Born-Digital-AIP-premis.xml"),
				ManifestPath:        filepath.Join(bdAIPPath, "additional", "UpdatedAreldaMetadata.xml"),
				MetadataPath:        filepath.Join(bdAIPPath, "content", "header", "old", "SIP", "metadata.xml"),
				UpdatedAreldaMDPath: filepath.Join(bdAIPPath, "additional", "UpdatedAreldaMetadata.xml"),
				XSDPath:             filepath.Join(bdAIPPath, "content", "header", "xsd", "arelda.xsd"),
				TopLevelPaths: []string{
					filepath.Join(bdAIPPath, "content"),
					filepath.Join(bdAIPPath, "additional"),
				},
			},
		},
		{
			name: "Creates a new digitized SIP",
			path: digitizedSIPPath,
			wantSIP: sip.SIP{
				Type:         enums.SIPTypeDigitizedSIP,
				Path:         digitizedSIPPath,
				ContentPath:  filepath.Join(digitizedSIPPath, "content"),
				ManifestPath: filepath.Join(digitizedSIPPath, "header", "metadata.xml"),
				MetadataPath: filepath.Join(digitizedSIPPath, "header", "metadata.xml"),
				XSDPath:      filepath.Join(digitizedSIPPath, "header", "xsd", "arelda.xsd"),
				TopLevelPaths: []string{
					filepath.Join(digitizedSIPPath, "content"),
					filepath.Join(digitizedSIPPath, "header"),
				},
			},
		},
		{
			name: "Creates a new digitized SIP (case-insensitive)",
			path: digitizedSIPCasePath,
			wantSIP: sip.SIP{
				Type:         enums.SIPTypeDigitizedSIP,
				Path:         digitizedSIPCasePath,
				ContentPath:  filepath.Join(digitizedSIPCasePath, "content"),
				ManifestPath: filepath.Join(digitizedSIPCasePath, "header", "metadata.xml"),
				MetadataPath: filepath.Join(digitizedSIPCasePath, "header", "metadata.xml"),
				XSDPath:      filepath.Join(digitizedSIPCasePath, "header", "xsd", "arelda.xsd"),
				TopLevelPaths: []string{
					filepath.Join(digitizedSIPCasePath, "content"),
					filepath.Join(digitizedSIPCasePath, "header"),
				},
			},
		},
		{
			name: "Creates a new born digital SIP",
			path: bornDigitalSIPPath,
			wantSIP: sip.SIP{
				Type:         enums.SIPTypeBornDigitalSIP,
				Path:         bornDigitalSIPPath,
				ContentPath:  filepath.Join(bornDigitalSIPPath, "content"),
				ManifestPath: filepath.Join(bornDigitalSIPPath, "header", "metadata.xml"),
				MetadataPath: filepath.Join(bornDigitalSIPPath, "header", "metadata.xml"),
				XSDPath:      filepath.Join(bornDigitalSIPPath, "header", "xsd", "arelda.xsd"),
				TopLevelPaths: []string{
					filepath.Join(bornDigitalSIPPath, "content"),
					filepath.Join(bornDigitalSIPPath, "header"),
				},
			},
		},
		{
			name:    "Fails with a non existing path",
			wantErr: "SIP: New: stat : no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := sip.New(tt.path)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("error is nil, expecting: %q", tt.wantErr)
				} else {
					assert.ErrorContains(t, err, tt.wantErr)
				}

				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, s, tt.wantSIP)
			assert.Equal(t, s.HasValidName(), true)
		})
	}
}

func TestName(t *testing.T) {
	t.Parallel()

	s := sip.SIP{
		Type: enums.SIPTypeDigitizedSIP,
		Path: "/path/to/SIP_20201201_Vecteur",
	}
	assert.Equal(t, s.Name(), "SIP_20201201_Vecteur")
}

func TestIsAIP(t *testing.T) {
	t.Parallel()

	s := sip.SIP{
		Type: enums.SIPTypeBornDigitalAIP,
		Path: "/path/to/AIP_20201201",
	}
	assert.Assert(t, s.IsAIP())

	s = sip.SIP{
		Type: enums.SIPTypeBornDigitalSIP,
		Path: "/path/to/SIP_20201201_Vecteur",
	}
	assert.Assert(t, !s.IsAIP())
}

func TestIsSIP(t *testing.T) {
	t.Parallel()

	s := sip.SIP{
		Type: enums.SIPTypeBornDigitalSIP,
		Path: "/path/to/SIP_20201201_Vecteur",
	}
	assert.Assert(t, s.IsSIP())

	s = sip.SIP{
		Type: enums.SIPTypeBornDigitalAIP,
		Path: "/path/to/AIP_20201201",
	}
	assert.Assert(t, !s.IsSIP())
}

func digitizedSIPTempDir(t *testing.T, sipName string) string {
	return fs.NewDir(t, "",
		fs.WithDir(sipName,
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			),
			fs.WithDir("header"),
		),
	).Path()
}
