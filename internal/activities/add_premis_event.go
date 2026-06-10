package activities

import (
	"context"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
)

const AddPREMISEventName = "add-premis-event"

type AddPREMISEventParams struct {
	PREMISFilePath string
	Agent          premis.Agent
	Type           string
	Detail         string
	OutcomeDetail  string
	Failures       []string
}

type AddPREMISEventResult struct{}

type AddPREMISEventActivity struct{}

func NewAddPREMISEvent() *AddPREMISEventActivity {
	return &AddPREMISEventActivity{}
}

func (md *AddPREMISEventActivity) Execute(
	ctx context.Context,
	params *AddPREMISEventParams,
) (*AddPREMISEventResult, error) {
	doc, err := premis.ParseOrInitialize(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	outcome := "valid"
	if params.Failures != nil {
		outcome = "invalid"
	}

	eventSummary := premis.EventSummary{
		Type:          params.Type,
		Detail:        params.Detail,
		Outcome:       outcome,
		OutcomeDetail: params.OutcomeDetail,
	}

	err = premis.AppendEventXMLForEachObject(doc, eventSummary, params.Agent)
	if err != nil {
		return nil, err
	}

	err = doc.WriteToFile(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	return &AddPREMISEventResult{}, nil
}
