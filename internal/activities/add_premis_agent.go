package activities

import (
	"context"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/premis"
)

const AddPREMISAgentName = "add-premis-agent"

type AddPREMISAgentParams struct {
	PREMISFilePath string
	Agent          premis.Agent
}

type AddPREMISAgentResult struct{}

type AddPREMISAgentActivity struct{}

func NewAddPREMISAgent() *AddPREMISAgentActivity {
	return &AddPREMISAgentActivity{}
}

func (md *AddPREMISAgentActivity) Execute(
	ctx context.Context,
	params *AddPREMISAgentParams,
) (*AddPREMISAgentResult, error) {
	doc, err := premis.ParseOrInitialize(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	err = premis.AppendAgentXML(doc, params.Agent)
	if err != nil {
		return nil, err
	}

	err = doc.WriteToFile(params.PREMISFilePath)
	if err != nil {
		return nil, err
	}

	return &AddPREMISAgentResult{}, nil
}
