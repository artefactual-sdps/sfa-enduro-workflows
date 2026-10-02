package datatypes

import "github.com/google/uuid"

// File describes a DIP file and its source in an AIP.
type File struct {
	// The datei ID of the file from the ACTApro document.
	DateiID string
	// The path for the file in the DIP, relative to the DIP root.
	DIPPath string
	// The checksum of the file from the ACTApro document.
	Checksum string
	// The checksum algorithm.
	ChecksumAlgorithm string
	// The UUID of the AIP that contains the file.
	AIPUUID uuid.UUID
	// The AMSS extraction path, including the AIP directory name.
	AIPPath string
}
