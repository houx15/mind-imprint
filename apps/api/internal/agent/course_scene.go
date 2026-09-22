package agent

// course_scene.go — runtime Opening/Closing narration generation for the
// student course player (Course Runtime Slice 7). GenerateCourseScene turns
// teacher-approved facts + permitted student-history signals into a short,
// restrained Chinese narration, modelled on reportgen.go's (ctx, prov,
// resolved, in) -> (result, usage, error) shape. It produces ONLY the
// narration text; TTS synthesis, OSS storage, metering and the static
// fallback all live in the api layer (internal/api/course_scene.go). The
// generator receives only the signals the caller was permitted to pass
// (§20 signal minimization) and reports which of them it actually used.
//
// §6.1 (opening): greeting + what the student will do + estimated time +
// (optional, evidence-supported) connection to prior learning + an invitation
// to begin. §6.2 (closing): consistent with the prepared summary, mentions
// only evidence that actually exists, distinguishes completion from mastery,
// makes no unsupported improvement claims, and names the takeaways + transfer
// applications.

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// sceneGenMaxTokens keeps the completion cap generous even though the
// narration is short: a reasoning tier can spend hidden thinking tokens and a
// small cap would starve the visible reply to empty (memory
// llm-reasoning-model-budgets). The chaperone tier disables thinking, but the
// cap stays safe regardless of tier.
const sceneGenMaxTokens = 4000

// SceneGenInput is the flattened, permitted fact + signal set for one runtime
// scene. Which is "opening" or "closing"; only the fields relevant to that
// slot are populated by the caller.
type SceneGenInput struct {
	Which                string
	CourseTitle          string
	EstimatedMinutes     int
	Objectives           []string
	LearningPreview      []string
	PreparedSummary      string
	Takeaways            []string
	TransferApplications []string
	// AllowedSignals are the permitted student-history signal types (§20). Only
	// these may influence the narration.
	AllowedSignals []string
	// SignalEvidence maps an allowed signal type to its resolved value. A signal
	// with no (or empty) evidence is not mentioned and is not reported as used.
	SignalEvidence map[string]string
}

// SceneGenResult is the generated narration plus the signal types that
// actually informed it (feeds RuntimeSceneResult.usedSignalTypes).
type SceneGenResult struct {
	Text            string
	UsedSignalTypes []string
}

// GenerateCourseScene runs one non-streaming completion to produce the scene
// narration. usedSignalTypes is derived deterministically: an allowed signal
// counts as used exactly when it carries non-empty evidence (and is therefore
// the only material placed in the prompt), so the report is honest regardless
// of the model's phrasing. Usage is returned even on error so the caller can
// still meter a call whose tokens were already spent.
func GenerateCourseScene(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in SceneGenInput) (SceneGenResult, gateway.ChatUsage, error) {
	used := usedSignals(in.AllowedSignals, in.SignalEvidence)

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: sceneSystemPrompt(in.Which)},
			{Role: gateway.RoleUser, Content: sceneUserPrompt(in, used)},
		},
		MaxTokens: sceneGenMaxTokens,
	}
	res, err := gateway.Collect(ctx, prov, resolved, req)
	if err != nil {
		return SceneGenResult{}, res.Usage, err
	}
	text := strings.TrimSpace(res.Text)
	if text == "" {
		return SceneGenResult{}, res.Usage, fmt.Errorf("agent: course scene generation returned empty text")
	}
	return SceneGenResult{Text: text, UsedSignalTypes: used}, res.Usage, nil
}

// usedSignals returns, in AllowedSignals order, the signal types that carry
// non-empty evidence — the only ones that reach the prompt or the result.
func usedSignals(allowed []string, evidence map[string]string) []string {
	var used []string
	for _, s := range allowed {
		if strings.TrimSpace(evidence[s]) != "" {
			used = append(used, s)
		}
	}
	return used
}
