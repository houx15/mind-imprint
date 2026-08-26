package evalbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"mindimprint/api/internal/evalreport"
)

// GoldReport is the JSON-only human annotation contract used by Evalbench.
// The wrapper keeps gold artifacts distinct from candidate EvaluationReports.
type GoldReport struct {
	Report evalreport.Report `json:"report"`
}

// LoadGoldReport reads a JSON Gold artifact and returns both its validated
// report value and exact source bytes for reproducibility snapshots and hashes.
func LoadGoldReport(path string) (GoldReport, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return GoldReport{}, nil, fmt.Errorf("evalbench: read JSON gold: %w", err)
	}
	gold, err := ParseGoldReport(raw)
	if err != nil {
		return GoldReport{}, nil, err
	}
	return gold, raw, nil
}

// ParseGoldReport strictly validates the report plus optional exporter metadata.
// status and _meta are provenance only and never reach the comparator. Gold is
// user-provided input: Evalbench does not generate, transform, or enrich it.
func ParseGoldReport(raw []byte) (GoldReport, error) {
	var envelope struct {
		Report json.RawMessage `json:"report"`
		Status string          `json:"status,omitempty"`
		Meta   json.RawMessage `json:"_meta,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil {
		return GoldReport{}, fmt.Errorf("evalbench: parse JSON gold envelope: %w", err)
	}
	if err := requireEOF(dec); err != nil {
		return GoldReport{}, err
	}
	if len(envelope.Report) == 0 {
		return GoldReport{}, fmt.Errorf("evalbench: JSON gold is missing report")
	}
	if len(envelope.Meta) > 0 {
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(envelope.Meta, &meta); err != nil || meta == nil {
			return GoldReport{}, fmt.Errorf("evalbench: JSON gold _meta must be an object")
		}
	}
	if err := validateSchemaJSON(evaluationReportSchema, envelope.Report); err != nil {
		return GoldReport{}, fmt.Errorf("evalbench: invalid JSON gold report: %w", err)
	}
	report, err := evalreport.Validate(envelope.Report)
	if err != nil {
		return GoldReport{}, fmt.Errorf("evalbench: parse JSON gold report: %w", err)
	}
	normalizeReportArrays(&report)
	if err := validateCompleteReport(report); err != nil {
		return GoldReport{}, fmt.Errorf("evalbench: invalid JSON gold report: %w", err)
	}
	return GoldReport{Report: report}, nil
}

func requireEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("evalbench: JSON gold contains multiple values")
		}
		return fmt.Errorf("evalbench: parse JSON gold: %w", err)
	}
	return nil
}
