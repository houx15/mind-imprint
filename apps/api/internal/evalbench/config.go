// Package evalbench implements the offline evaluation workbench. It deliberately
// has no database dependency: an experiment is fully described by files plus
// server-side model credentials.
package evalbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const SchemaVersion = 3

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type Config struct {
	SchemaVersion  int                     `json:"schemaVersion"`
	Name           string                  `json:"name"`
	SuccessfulRuns int                     `json:"successfulRuns"`
	MaxAttempts    int                     `json:"maxAttempts"`
	Cases          []CaseConfig            `json:"cases"`
	Models         map[string]ModelProfile `json:"models"`
	Variants       []VariantConfig         `json:"variants"`
	Comparator     ModelUse                `json:"comparator"`
	Timeouts       TimeoutConfig           `json:"timeouts,omitempty"`

	baseDir string
}

// TimeoutConfig contains per-provider-call limits. A zero value selects the
// evalbench defaults so existing experiment configs remain valid.
type TimeoutConfig struct {
	CandidateSeconds  int `json:"candidateSeconds,omitempty"`
	ComparatorSeconds int `json:"comparatorSeconds,omitempty"`
}

type CaseConfig struct {
	ID          string `json:"id"`
	ProjectData string `json:"projectData"`
	GoldReport  string `json:"goldReport"`
}

type ModelProfile struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

type VariantConfig struct {
	ID        string          `json:"id"`
	Evaluator string          `json:"evaluator"`
	Model     string          `json:"model"`
	Params    json.RawMessage `json:"params"`
}

type ModelUse struct {
	Model         string `json:"model"`
	PromptVersion string `json:"promptVersion"`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("evalbench: read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("evalbench: parse config: %w", err)
	}
	c.baseDir = filepath.Dir(path)
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.baseDir, path)
}

func (c Config) Case(id string) (CaseConfig, bool) {
	for _, cs := range c.Cases {
		if cs.ID == id {
			return cs, true
		}
	}
	return CaseConfig{}, false
}

func (c Config) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("evalbench: schemaVersion = %d, want %d (v1 used the retired DualAxis report)", c.SchemaVersion, SchemaVersion)
	}
	if c.Name == "" {
		return fmt.Errorf("evalbench: name is required")
	}
	if c.SuccessfulRuns <= 0 || c.MaxAttempts < c.SuccessfulRuns {
		return fmt.Errorf("evalbench: require maxAttempts >= successfulRuns > 0")
	}
	if c.Timeouts.CandidateSeconds < 0 || c.Timeouts.ComparatorSeconds < 0 {
		return fmt.Errorf("evalbench: timeout seconds cannot be negative")
	}
	if len(c.Cases) == 0 || len(c.Variants) == 0 || len(c.Models) == 0 {
		return fmt.Errorf("evalbench: cases, variants, and models must be non-empty")
	}
	caseIDs := map[string]bool{}
	for _, cs := range c.Cases {
		if !safeID.MatchString(cs.ID) || cs.ProjectData == "" || cs.GoldReport == "" || caseIDs[cs.ID] {
			return fmt.Errorf("evalbench: each case needs unique id, projectData, and goldReport")
		}
		if filepath.Ext(cs.GoldReport) != ".json" {
			return fmt.Errorf("evalbench: case %q goldReport must be a JSON file", cs.ID)
		}
		caseIDs[cs.ID] = true
	}
	for id, p := range c.Models {
		if id == "" {
			return fmt.Errorf("evalbench: model profile id is required")
		}
		if err := ValidateModelProfile(p); err != nil {
			return fmt.Errorf("evalbench: model profile %q: %w", id, err)
		}
	}
	variantIDs := map[string]bool{}
	for i, v := range c.Variants {
		if !safeID.MatchString(v.ID) || v.Evaluator == "" || v.Model == "" || variantIDs[v.ID] {
			return fmt.Errorf("evalbench: each variant needs unique id, evaluator, and model")
		}
		if _, ok := c.Models[v.Model]; !ok {
			return fmt.Errorf("evalbench: variant %q references unknown model %q", v.ID, v.Model)
		}
		if len(v.Params) == 0 {
			c.Variants[i].Params = json.RawMessage(`{}`)
			v.Params = c.Variants[i].Params
		}
		var object map[string]any
		if err := json.Unmarshal(v.Params, &object); err != nil {
			return fmt.Errorf("evalbench: variant %q params must be an object", v.ID)
		}
		variantIDs[v.ID] = true
	}
	for label, use := range map[string]ModelUse{"comparator": c.Comparator} {
		if use.Model == "" || use.PromptVersion == "" {
			return fmt.Errorf("evalbench: %s needs model and promptVersion", label)
		}
		if _, ok := c.Models[use.Model]; !ok {
			return fmt.Errorf("evalbench: %s references unknown model %q", label, use.Model)
		}
	}
	if c.Comparator.PromptVersion != ComparatorPromptVersion {
		return fmt.Errorf("evalbench: comparator promptVersion = %q, want %q", c.Comparator.PromptVersion, ComparatorPromptVersion)
	}
	return nil
}

// ValidateModelProfile ensures the experiment only uses providers implemented
// by gateway.
func ValidateModelProfile(p ModelProfile) error {
	if p.Model == "" || (p.Provider != "deepseek" && p.Provider != "anthropic" && p.Provider != "glm") {
		return fmt.Errorf("must use supported provider and model")
	}
	if p.ReasoningEffort != "" {
		if p.Provider != "glm" {
			return fmt.Errorf("reasoningEffort is currently supported only for glm")
		}
		switch p.ReasoningEffort {
		case "low", "high", "max":
		default:
			return fmt.Errorf("glm reasoningEffort must be low, high, or max")
		}
	}
	return nil
}
