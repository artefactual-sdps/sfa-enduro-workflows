package pips_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/pips"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

func TestNew(t *testing.T) {
	t.Parallel()

	type params struct {
		path    string
		sipType enums.SIPType
	}
	type test struct {
		name string
		args params
		want pips.PIP
	}
	for _, tt := range []test{
		{
			name: "Returns a digitized SIP PIP",
			args: params{
				path:    "/path/to/SIP_20201201_Vecteur",
				sipType: enums.SIPTypeDigitizedSIP,
			},
			want: pips.PIP{
				Path:         "/path/to/SIP_20201201_Vecteur",
				Type:         enums.SIPTypeDigitizedSIP,
				ManifestPath: "/path/to/SIP_20201201_Vecteur/objects/SIP_20201201_Vecteur/header/metadata.xml",
			},
		},
		{
			name: "Returns a digitized AIP PIP",
			args: params{
				path:    "/path/to/digitized_AIP_012345",
				sipType: enums.SIPTypeDigitizedAIP,
			},
			want: pips.PIP{
				Path:         "/path/to/digitized_AIP_012345",
				Type:         enums.SIPTypeDigitizedAIP,
				ManifestPath: "/path/to/digitized_AIP_012345/metadata/UpdatedAreldaMetadata.xml",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := pips.New(tt.args.path, tt.args.sipType)
			assert.DeepEqual(t, p, tt.want)
		})
	}
}

func TestNewFromSIP(t *testing.T) {
	t.Parallel()

	s := sip.SIP{
		Path: "/path/to/born_digital_SIP_12345",
		Type: enums.SIPTypeBornDigitalSIP,
	}
	assert.DeepEqual(t, pips.NewFromSIP(s), pips.PIP{
		Path:         "/path/to/born_digital_SIP_12345",
		Type:         enums.SIPTypeBornDigitalSIP,
		ManifestPath: "/path/to/born_digital_SIP_12345/objects/born_digital_SIP_12345/header/metadata.xml",
	})
}

func TestName(t *testing.T) {
	t.Parallel()

	p := pips.New("/path/to/SIP_20201201_Vecteur", enums.SIPTypeDigitizedSIP)
	assert.Equal(t, p.Name(), "SIP_20201201_Vecteur")
}

func TestConvertSIPPath(t *testing.T) {
	t.Parallel()

	p := pips.New("/path/to/SIP_20201201_Vecteur", enums.SIPTypeDigitizedSIP)
	assert.Equal(t,
		p.ConvertSIPPath("content/d_0000001/Prozess_Digitalisierung_PREMIS.xml"),
		"metadata/Prozess_Digitalisierung_PREMIS.xml",
	)
	assert.Equal(t,
		p.ConvertSIPPath("header/metadata.xml"),
		"objects/SIP_20201201_Vecteur/header/metadata.xml",
	)
	assert.Equal(t,
		p.ConvertSIPPath("header/old/SIP/metadata.xml"),
		"objects/SIP_20201201_Vecteur/header/metadata.xml",
	)
	assert.Equal(t,
		p.ConvertSIPPath("additional/UpdatedAreldaMetadata.xml"),
		"metadata/UpdatedAreldaMetadata.xml",
	)
	assert.Equal(t,
		p.ConvertSIPPath("content/d_0000001/00000001.jp2"),
		"objects/SIP_20201201_Vecteur/content/d_0000001/00000001.jp2",
	)
	assert.Equal(t, p.ConvertSIPPath("header/xsd/arelda.xsd"), "")
}
