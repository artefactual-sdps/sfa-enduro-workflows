package identifiers_test

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/identifiers"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/manifest"
)

func TestSortFunc(t *testing.T) {
	l := []identifiers.File{
		{Path: "dir/a.txt"},
		{Path: "dir/C.txt"},
		{Path: "b.txt"},
		{Path: "b.txt"},
		{Path: "e.txt"},
	}
	slices.SortFunc(l, identifiers.Compare)
	assert.DeepEqual(t, l, []identifiers.File{
		{Path: "b.txt"},
		{Path: "b.txt"},
		{Path: "dir/C.txt"}, // Uppercase letters sort before lowercase.
		{Path: "dir/a.txt"},
		{Path: "e.txt"},
	})
}

func TestFromManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *manifest.Manifest
		want     []identifiers.File
		wantErr  string
	}{
		{
			name: "Returns a digitized AIP identifier list",
			manifest: &manifest.Manifest{Files: map[string]*manifest.File{
				"content/d_0000001/00000001.jp2": {
					ID: "_miEf29GTkFR7ymi91IV4fO",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "f7dc1f76a55cbdca0ae4a6dc8ae64644",
					},
				},
				"content/d_0000001/00000001_PREMIS.xml": {
					ID: "_SRpeVgb4xGImymb23OH1od",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "1428a269ff4e5b4894793b68646984b7",
					},
				},
				"content/d_0000001/Prozess_Digitalisierung_PREMIS.xml": {
					ID: "_fZzi3dX2jvrwakvY6jeJS8",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "8067daaa900eba6dace69572eea8f8f3",
					},
				},
				"header/old/SIP/metadata.xml": {
					ID: "OLD_SIP",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "43c533d499c572fca699e77e06295ba3",
					},
				},
				"header/xsd/arelda.xsd": {
					ID: "_xAlSBc3dYcypUMvN8HzeN5",
					Checksum: manifest.Checksum{
						Algorithm: "MD5",
						Hash:      "f8454632e1ebf97e0aa8d9527ce2641f",
					},
				},
			}},
			want: []identifiers.File{
				{
					Path: "content/d_0000001/00000001.jp2",
					Identifiers: []identifiers.Identifier{
						{
							Value: "_miEf29GTkFR7ymi91IV4fO",
							Type:  "local",
						},
					},
				},
				{
					Path: "content/d_0000001/00000001_PREMIS.xml",
					Identifiers: []identifiers.Identifier{
						{
							Value: "_SRpeVgb4xGImymb23OH1od",
							Type:  "local",
						},
					},
				},
				{
					Path: "content/d_0000001/Prozess_Digitalisierung_PREMIS.xml",
					Identifiers: []identifiers.Identifier{
						{
							Value: "_fZzi3dX2jvrwakvY6jeJS8",
							Type:  "local",
						},
					},
				},
				{
					Path: "header/old/SIP/metadata.xml",
					Identifiers: []identifiers.Identifier{
						{
							Value: "OLD_SIP",
							Type:  "local",
						},
					},
				},
				{
					Path: "header/xsd/arelda.xsd",
					Identifiers: []identifiers.Identifier{
						{
							Value: "_xAlSBc3dYcypUMvN8HzeN5",
							Type:  "local",
						},
					},
				},
			},
		},
		{
			name:     "Errors when manifest is empty",
			manifest: &manifest.Manifest{},
			wantErr:  "no files in manifest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := identifiers.FromManifest(tt.manifest)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
