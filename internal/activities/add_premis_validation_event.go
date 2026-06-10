package activities

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fformat"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fsutil"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fvalidate"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/sip"
)

const AddPREMISValidationEventName = "add-premis-validation-event"

type AddPREMISValidationEventParams struct {
	SIP            sip.SIP
	PREMISFilePath string
	Summary        premis.EventSummary
}

type AddPREMISValidationEventResult struct{}

type AddPREMISValidationEventActivity struct {
	// Clock for time-related operations, can be used to mock time in tests.
	clock clockwork.Clock

	// Random number generator for generating UUIDs. Can be set to a
	// deterministic generator for testing purposes.
	rng io.Reader

	validator fvalidate.Validator
}

func NewAddPREMISValidationEvent(
	clock clockwork.Clock,
	rng io.Reader,
	validator fvalidate.Validator,
) *AddPREMISValidationEventActivity {
	return &AddPREMISValidationEventActivity{
		clock:     clock,
		rng:       rng,
		validator: validator,
	}
}

func (a *AddPREMISValidationEventActivity) Execute(
	ctx context.Context,
	params *AddPREMISValidationEventParams,
) (*AddPREMISValidationEventResult, error) {
	var addAgent bool

	// Ensure the PREMIS file path exists.
	if !fsutil.FileExists(params.PREMISFilePath) {
		return nil, fmt.Errorf("PREMIS file path does not exist: %s", params.PREMISFilePath)
	}

	// Load or initialize PREMIS XML.
	doc, err := premis.ParseOrInitialize(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	PREMISEl := doc.FindElement("/premis:premis")
	if PREMISEl == nil {
		return nil, fmt.Errorf("no root premis element found in document")
	}

	// Determine formats of SIP files.
	fileformats, err := fformat.IdentifyFormats(ctx, fformat.NewSiegfriedEmbed(), params.SIP)
	if err != nil {
		return nil, fmt.Errorf("identifyFormats: %v", err)
	}

	agent := a.validator.PREMISAgent()

	// Determine which files should have been checked by the validator.
	allowedIds := a.validator.FormatIDs()

	for path, f := range fileformats {
		if slices.Contains(allowedIds, f.ID) {
			// Determine subpath.
			subpath, err := filepath.Rel(params.SIP.ContentPath, path)
			if err != nil {
				return nil, err
			}

			// Find PREMIS object element using original name.
			originalName := premis.OriginalNameForSubpath(params.SIP, subpath)
			objectOriginalNameEl := doc.FindElement(
				fmt.Sprintf("/premis:premis/premis:object/premis:originalName[text()='%s']", originalName),
			)
			if objectOriginalNameEl == nil {
				return nil, fmt.Errorf("element not found")
			}

			id, err := uuid.NewRandomFromReader(a.rng)
			if err != nil {
				return nil, fmt.Errorf("generate UUID: %v", err)
			}

			// Append PREMIS event linked to PREMIS object element.
			objectEl := objectOriginalNameEl.Parent()
			event := premis.Event{
				Summary:      params.Summary,
				IdType:       "UUID",
				IdValue:      id.String(),
				DateTime:     a.clock.Now().Format(time.RFC3339),
				AgentIdType:  agent.IdType,
				AgentIdValue: agent.IdValue,
			}

			premis.AddEventElement(PREMISEl, event)
			premis.LinkEventToObject(objectEl, event)

			// Add agent to PREMIS.
			addAgent = true
		}
	}

	// Add validator agents to PREMIS document.
	if addAgent {
		if e := premis.AppendAgentXML(doc, agent); e != nil {
			return nil, fmt.Errorf("addAgent: %v", e)
		}
	}

	// Write PREMIS.
	doc.Indent(2)
	err = doc.WriteToFile(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	return &AddPREMISValidationEventResult{}, nil
}
