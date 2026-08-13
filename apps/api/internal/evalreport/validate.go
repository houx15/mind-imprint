package evalreport

import (
	"encoding/json"
	"fmt"
)

// Validate unmarshals and boundary-checks the envelope. Deep-shape truth lives
// in packages/contracts (Zod); Go only guards the outer envelope.
func Validate(raw []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return Report{}, fmt.Errorf("evalreport: unmarshal: %w", err)
	}
	if r.Version != 1 {
		return Report{}, fmt.Errorf("evalreport: version must be 1, got %d", r.Version)
	}
	if r.ReportID == "" || r.ProjectID == "" {
		return Report{}, fmt.Errorf("evalreport: missing reportId/projectId")
	}
	if r.Student.ID == "" {
		return Report{}, fmt.Errorf("evalreport: missing student.id")
	}
	return r, nil
}
