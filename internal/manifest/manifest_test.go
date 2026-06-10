package manifest_test

import (
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/manifest"
)

var (
	AIPManifest = `
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
						<pruefsumme>43c533d499c572fca699e77e06295ba3</pruefsumme>
					</datei>
				</ordner>
			</ordner>
			<ordner>
				<name>xsd</name>
				<datei id="_xAlSBc3dYcypUMvN8HzeN5">
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
				<name>d_0000001</name>
				<datei id="_SRpeVgb4xGImymb23OH1od">
					<name>00000001_PREMIS.xml</name>
					<originalName>00000001_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>1428a269ff4e5b4894793b68646984b7</pruefsumme>
				</datei>
				<datei id="_fZzi3dX2jvrwakvY6jeJS8">
					<name>Prozess_Digitalisierung_PREMIS.xml</name>
					<originalName>Prozess_Digitalisierung_PREMIS.xml</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>8067daaa900eba6dace69572eea8f8f3</pruefsumme>
				</datei>
				<datei id="_miEf29GTkFR7ymi91IV4fO">
					<name>00000001.jp2</name>
					<originalName>00000001.jp2</originalName>
					<pruefalgorithmus>MD5</pruefalgorithmus>
					<pruefsumme>f7dc1f76a55cbdca0ae4a6dc8ae64644</pruefsumme>
				</datei>
			</ordner>
		</ordner>
	</inhaltsverzeichnis>
</paket>
`

	SIPManifest = `
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
						<pruefsumme>f8454632e1ebf97e0aa8d9527ce2641f</pruefsumme>
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
						<pruefsumme>dc29291d0e2a18363d0efd2ec2fe81c9</pruefsumme>
					</datei>
					<datei id="_WuDmXAs5UDwKTGVLsCcZxa">
						<name>00000001_PREMIS.xml</name>
						<originalName>00000001_PREMIS.xml</originalName>
						<pruefalgorithmus>MD5</pruefalgorithmus>
						<pruefsumme>1d310772d26138a42eb2d6bebb637457</pruefsumme>
					</datei>
					<datei id="_cQ6sm5CChWVqtqmrWvne0W">
						<name>Prozess_Digitalisierung_PREMIS.xml</name>
						<originalName>Prozess_Digitalisierung_PREMIS.xml</originalName>
						<pruefalgorithmus>MD5</pruefalgorithmus>
						<pruefsumme>21d8e90afdefd2c43386ca1d1658cab0</pruefsumme>
					</datei>
				</ordner>
			</ordner>
		</inhaltsverzeichnis>
	</paket>
`
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reader  io.Reader
		want    *manifest.Manifest
		wantErr string
	}{
		{
			name:   "Returns a digitized AIP manifest",
			reader: strings.NewReader(AIPManifest),
			want: &manifest.Manifest{
				SchemaVersion: "5.0",
				Files: map[string]*manifest.File{
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
				},
			},
		},
		{
			name:   "Returns a digitized SIP file list",
			reader: strings.NewReader(SIPManifest),
			want: &manifest.Manifest{
				SchemaVersion: "4.0",
				Files: map[string]*manifest.File{
					"content/d_0000001/00000001.jp2": {
						ID: "_zodSTSD0nv05CpOp6JoV3X",
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "dc29291d0e2a18363d0efd2ec2fe81c9",
						},
					},
					"content/d_0000001/00000001_PREMIS.xml": {
						ID: "_WuDmXAs5UDwKTGVLsCcZxa",
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "1d310772d26138a42eb2d6bebb637457",
						},
					},
					"content/d_0000001/Prozess_Digitalisierung_PREMIS.xml": {
						ID: "_cQ6sm5CChWVqtqmrWvne0W",
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "21d8e90afdefd2c43386ca1d1658cab0",
						},
					},
					"header/xsd/arelda.xsd": {
						ID: "_ZSANrSklQ9HGn99yjlUumz",
						Checksum: manifest.Checksum{
							Algorithm: "MD5",
							Hash:      "f8454632e1ebf97e0aa8d9527ce2641f",
						},
					},
				},
			},
		},
		{
			name:   "Returns an empty list from an empty manifest",
			reader: strings.NewReader(""),
			want:   &manifest.Manifest{Files: map[string]*manifest.File{}},
		},
		{
			name:    "Errors on a missing closing tag",
			reader:  strings.NewReader(`<datei id="_ZSANrSklQ9HGn99yjlUumz"><name>arelda.xsd</name>`),
			wantErr: "parse: XML syntax error on line 1: unexpected EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Parse(tt.reader)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}
