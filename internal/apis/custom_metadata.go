package apis

import (
	"encoding/json"
	"fmt"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/apis/gen"
)

const (
	CustomMetadataKey               = "apis"
	DecisionOptionCancelIngest      = "Cancel ingest"
	DecisionOptionContinueOverwrite = "Continue and overwrite"
	DecisionOptionContinueAppend    = "Continue and append"
)

type CustomMetadata struct {
	ImportTaskID string `json:"importTaskId"`
	Decision     string `json:"decision"`
}

func (m CustomMetadata) Marshal() ([]byte, error) {
	if m.ImportTaskID == "" {
		return nil, fmt.Errorf("APIS custom metadata requires import task ID")
	}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal APIS custom metadata: %w", err)
	}

	return data, nil
}

func (m *CustomMetadata) Unmarshal(data []byte) error {
	if m == nil {
		return fmt.Errorf("APIS custom metadata destination is nil")
	}
	if err := json.Unmarshal(data, m); err != nil {
		return fmt.Errorf("unmarshal APIS custom metadata: %w", err)
	}
	if m.ImportTaskID == "" {
		return fmt.Errorf("APIS custom metadata requires import task ID")
	}

	return nil
}

func (m CustomMetadata) ImportBehaviour() (gen.ImportBehaviourType, error) {
	switch m.Decision {
	case "", DecisionOptionContinueAppend:
		return gen.ImportBehaviourTypeAppendOnly, nil
	case DecisionOptionContinueOverwrite:
		return gen.ImportBehaviourTypeOverwriteAndAppend, nil
	case DecisionOptionCancelIngest:
		return "", fmt.Errorf("APIS import task %q was canceled during preprocessing", m.ImportTaskID)
	default:
		return "", fmt.Errorf("unsupported APIS decision %q for import task %q", m.Decision, m.ImportTaskID)
	}
}
