package premis_test

import (
	"errors"
	"testing"

	"github.com/beevik/etree"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/enums"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const premisObjectAddContent = `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>uuid</premis:objectIdentifierType>
      <premis:objectIdentifierValue>c74a85b7-919b-409e-8209-9c7ebe0e7945</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/test_transfer/content/cat.jpg</premis:originalName>
  </premis:object>
</premis:premis>
`

const premisObjectAndEventAddContent = `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:object xsi:type="premis:file">
    <premis:objectIdentifier>
      <premis:objectIdentifierType>uuid</premis:objectIdentifierType>
      <premis:objectIdentifierValue>c74a85b7-919b-409e-8209-9c7ebe0e7945</premis:objectIdentifierValue>
    </premis:objectIdentifier>
    <premis:objectCharacteristics>
      <premis:format>
        <premis:formatDesignation>
          <premis:formatName/>
        </premis:formatDesignation>
      </premis:format>
    </premis:objectCharacteristics>
    <premis:originalName>data/objects/test_transfer/content/cat.jpg</premis:originalName>
    <premis:linkingEventIdentifier>
      <premis:linkingEventIdentifierType>UUID</premis:linkingEventIdentifierType>
      <premis:linkingEventIdentifierValue/>
    </premis:linkingEventIdentifier>
  </premis:object>
  <premis:event>
    <premis:eventIdentifier>
      <premis:eventIdentifierType>UUID</premis:eventIdentifierType>
      <premis:eventIdentifierValue/>
    </premis:eventIdentifier>
    <premis:eventType>validation</premis:eventType>
    <premis:eventDateTime/>
    <premis:eventDetailInformation>
      <premis:eventDetail>name=&quot;Validate SIP metadata&quot;</premis:eventDetail>
    </premis:eventDetailInformation>
    <premis:eventOutcomeInformation>
      <premis:eventOutcome>invalid</premis:eventOutcome>
      <premis:eventOutcomeDetail>
        <premis:eventOutcomeDetailNote>Metadata validation successful</premis:eventOutcomeDetailNote>
      </premis:eventOutcomeDetail>
    </premis:eventOutcomeInformation>
    <premis:linkingAgentIdentifier>
      <premis:linkingAgentIdentifierType valueURI="http://id.loc.gov/vocabulary/identifiers/local">url</premis:linkingAgentIdentifierType>
      <premis:linkingAgentIdentifierValue>https://github.com/artefactual-sdps/sfa-enduro-workflows</premis:linkingAgentIdentifierValue>
    </premis:linkingAgentIdentifier>
  </premis:event>
  <premis:agent>
    <premis:agentIdentifier>
      <premis:agentIdentifierType valueURI="http://id.loc.gov/vocabulary/identifiers/local">url</premis:agentIdentifierType>
      <premis:agentIdentifierValue>https://github.com/artefactual-sdps/sfa-enduro-workflows</premis:agentIdentifierValue>
    </premis:agentIdentifier>
    <premis:agentName>Enduro</premis:agentName>
    <premis:agentType>software</premis:agentType>
  </premis:agent>
</premis:premis>
`

const premisAgentAddContent = `<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0">
  <premis:agent>
    <premis:agentIdentifier>
      <premis:agentIdentifierType valueURI="http://id.loc.gov/vocabulary/identifiers/local">url</premis:agentIdentifierType>
      <premis:agentIdentifierValue>https://github.com/artefactual-sdps/sfa-enduro-workflows</premis:agentIdentifierValue>
    </premis:agentIdentifier>
    <premis:agentName>Enduro</premis:agentName>
    <premis:agentType>software</premis:agentType>
  </premis:agent>
</premis:premis>
`

func blankElementText(doc *etree.Document, xpath string) error {
	el := doc.FindElement(xpath)
	if el == nil {
		return errors.New("element not found")
	}

	el.SetText("")

	return nil
}

func TestParseFile(t *testing.T) {
	t.Parallel()

	td := fs.NewDir(t, "", fs.WithFile(
		"agent.xml",
		premisAgentAddContent,
	))

	doc, err := premis.ParseFile(td.Join("agent.xml"))
	assert.NilError(t, err)

	got, err := doc.WriteToString()
	assert.NilError(t, err)
	assert.Equal(t, got, premisAgentAddContent)
}

func TestParseOrInitialize(t *testing.T) {
	t.Parallel()

	t.Run("Parses an existing XML file", func(t *testing.T) {
		t.Parallel()

		td := fs.NewDir(t, "", fs.WithFile(
			"agent.xml",
			premisAgentAddContent,
		))

		doc, err := premis.ParseOrInitialize(td.Join("agent.xml"))
		assert.NilError(t, err)

		got, err := doc.WriteToString()
		assert.NilError(t, err)
		assert.Equal(t, got, premisAgentAddContent)
	})

	t.Run("Creates an empty PREMIS XML file", func(t *testing.T) {
		t.Parallel()

		td := fs.NewDir(t, "")

		doc, err := premis.ParseOrInitialize(td.Join("test.xml"))
		assert.NilError(t, err)

		got, err := doc.WriteToString()
		assert.NilError(t, err)
		assert.Equal(
			t,
			got,
			`<?xml version="1.0" encoding="UTF-8"?>
<premis:premis xmlns:premis="http://www.loc.gov/premis/v3" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd" version="3.0"/>
`)
	})
}

func TestAppendPREMISObjectXML(t *testing.T) {
	t.Parallel()

	// Test with PREMIS object.
	uuid := "c74a85b7-919b-409e-8209-9c7ebe0e7945"
	originalName := "data/objects/test_transfer/content/cat.jpg"

	doc, err := premis.NewDoc()
	assert.NilError(t, err)

	err = premis.AppendObjectXML(doc, premis.Object{
		IdType:       "uuid",
		IdValue:      uuid,
		OriginalName: originalName,
	})
	assert.NilError(t, err)

	// Get resulting XML string.
	doc.Indent(2)
	xml, err := doc.WriteToString()
	assert.NilError(t, err)

	// Compare XML to constant.
	assert.Equal(t, xml, premisObjectAddContent)
}

func TestAppendPREMISEventXML(t *testing.T) {
	t.Parallel()

	// Add test PREMIS object.
	uuid := "c74a85b7-919b-409e-8209-9c7ebe0e7945"
	originalName := "data/objects/test_transfer/content/cat.jpg"

	doc, err := premis.NewDoc()
	assert.NilError(t, err)

	err = premis.AppendObjectXML(doc, premis.Object{
		IdType:       "uuid",
		IdValue:      uuid,
		OriginalName: originalName,
	})
	assert.NilError(t, err)

	err = premis.AppendAgentXML(doc, premis.AgentDefault())
	assert.NilError(t, err)

	// Test adding PREMIS event.
	err = premis.AppendEventXMLForEachObject(doc, premis.EventSummary{
		Type:          "validation",
		Detail:        "name=\"Validate SIP metadata\"",
		Outcome:       "invalid",
		OutcomeDetail: "Metadata validation successful",
	}, premis.AgentDefault())
	assert.NilError(t, err)

	// Check length then blank the event identifier value.
	idValueEl := doc.FindElement("/premis:premis/premis:event/premis:eventIdentifier/premis:eventIdentifierValue")
	assert.Assert(t, idValueEl != nil)
	assert.Assert(t, len(idValueEl.Text()) == 36)
	idValueEl.SetText("")

	// Blank text for other random/time elements.
	err = blankElementText(
		doc,
		"/premis:premis/premis:object/premis:linkingEventIdentifier/premis:linkingEventIdentifierValue",
	)
	assert.NilError(t, err)

	err = blankElementText(doc, "/premis:premis/premis:event/premis:eventDateTime")
	assert.NilError(t, err)

	// Get resulting XML string.
	doc.Indent(2)
	xml, err := doc.WriteToString()
	assert.NilError(t, err)

	// Compare XML to constant.
	assert.Equal(t, xml, premisObjectAndEventAddContent)
}

func TestAppendPREMISAgentXML(t *testing.T) {
	t.Parallel()

	// Test with PREMIS agent.
	doc, err := premis.NewDoc()
	assert.NilError(t, err)

	err = premis.AppendAgentXML(doc, premis.AgentDefault())
	assert.NilError(t, err)

	// Get resulting XML string.
	doc.Indent(2)
	xml, err := doc.WriteToString()
	assert.NilError(t, err)

	// Compare XML to constant.
	assert.Equal(t, xml, premisAgentAddContent)

	// Try to add another PREMIS agent to existing XML document.
	doc = etree.NewDocument()
	doc.ReadFromString(xml)
	doc.Indent(2)

	err = premis.AppendAgentXML(doc, premis.AgentDefault())
	assert.NilError(t, err)

	// Get resulting XML string.
	doc.Indent(2)
	xml, err = doc.WriteToString()
	assert.NilError(t, err)

	// Compare XML to constant to make sure a duplicate agent wasn't added.
	assert.Equal(t, xml, premisAgentAddContent)
}

func TestFilesWithinDirectory(t *testing.T) {
	t.Parallel()

	contentPath := fs.NewDir(t, "",
		fs.WithDir("content",
			fs.WithDir("content",
				fs.WithDir("d_0000001",
					fs.WithFile("00000001.jp2", ""),
					fs.WithFile("00000001_PREMIS.xml", ""),
					fs.WithFile("Prozess_Digitalisierung_PREMIS.xml", ""),
				),
			),
		),
	).Path()

	expectedFiles := []string{
		"content/content/d_0000001/00000001.jp2",
		"content/content/d_0000001/00000001_PREMIS.xml",
		"content/content/d_0000001/Prozess_Digitalisierung_PREMIS.xml",
	}

	files, err := premis.FilesWithinDirectory(contentPath)
	assert.NilError(t, err)

	assert.DeepEqual(t, files, expectedFiles)
}

func TestOriginalNameForSubpath(t *testing.T) {
	t.Parallel()

	// Check for correct adjustment of digitized AIP file path in PREMIS.
	aipSIP := sip.SIP{
		Type:        enums.SIPTypeDigitizedAIP,
		Path:        "test_transfer",
		ContentPath: "test_transfer/content/content",
	}

	aipOriginalName := premis.OriginalNameForSubpath(
		aipSIP,
		"d_0000001/00000001.jp2",
	)

	assert.Equal(t, aipOriginalName,
		"data/objects/test_transfer/content/d_0000001/00000001.jp2")

	// Check for correct adjustment of digitized SIP file path in PREMIS.
	digitizedSIP := sip.SIP{
		Type:        enums.SIPTypeDigitizedSIP,
		Path:        "test_transfer",
		ContentPath: "test_transfer/content",
	}

	digitizedSIPOriginalName := premis.OriginalNameForSubpath(
		digitizedSIP,
		"d_0000001/00000001.jp2",
	)

	assert.Equal(t, digitizedSIPOriginalName,
		"data/objects/test_transfer/content/d_0000001/00000001.jp2")

	// Check for correct adjustment of born digital SIP file path in PREMIS.
	bornDigitalSIP := sip.SIP{
		Type:        enums.SIPTypeBornDigitalSIP,
		Path:        "test_transfer",
		ContentPath: "test_transfer/content",
	}

	bornDigitalSIPOriginalName := premis.OriginalNameForSubpath(
		bornDigitalSIP,
		"d_0000001/00000001.jp2",
	)

	assert.Equal(t, bornDigitalSIPOriginalName,
		"data/objects/test_transfer/content/d_0000001/00000001.jp2")

	// Check for special handling of this specific file's path in PREMIS.
	metadataOriginalName := premis.OriginalNameForSubpath(
		aipSIP,
		"content/content/d_0000001/Prozess_Digitalisierung_PREMIS.xml",
	)

	assert.Equal(t, metadataOriginalName,
		"data/metadata/Prozess_Digitalisierung_PREMIS.xml")
}
