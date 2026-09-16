package pbl

// A composition retains the student's chosen revision pair and original words.
// Later draft edits must not change the process shown by this immutable version.
type ProcessComparison struct {
	BeforeVersionID string `json:"beforeVersionId"`
	AfterVersionID  string `json:"afterVersionId"`
	Feedback        string `json:"feedback"`
	Observation     string `json:"observation"`
}
