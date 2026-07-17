package agent

// course_step.go — the Course runtime (agent-spec §5.3): the coach executes an
// authored script, planner OFF, `advance` held by the coach behind a
// structural floor. Mirrors chat_step.go's shape; shares nothing with
// RunAgentStep, which is project-graph-coupled.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// CourseSession is the runtime's view of one student's run through one course.
type CourseSession struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	SkillID  string
	Phase    string
}

// FloorState is everything the phase floor is computed from. Every field comes
// from the STORE — never from the client (DEC-12.2): a floor the client can
// assert is not a floor.
type FloorState struct {
	ViewedSteps        []int
	DispositionedCards []string // session cards in status completed OR skipped
	StudentTurns       int
}

// NextPhase returns the current phase's single successor in the skill's binding
// order. Course advance is forward-only, one phase at a time.
func NextPhase(sk skills.Skill, cur string) (string, bool) {
	order, err := sk.LinearOrder()
	if err != nil {
		return "", false
	}
	for i, id := range order {
		if id == cur && i+1 < len(order) {
			return order[i+1], true
		}
	}
	return "", false
}

// CheckFloor evaluates the closed set of course floor kinds and returns the
// items that are NOT met. The floor is the structural half of phase advance:
// it can only refuse. An unknown kind counts as unmet — Load already rejects
// it, so this is defense in depth, and failing closed is the safe direction.
func CheckFloor(items []skills.FloorItem, st FloorState) []skills.FloorItem {
	var unmet []skills.FloorItem
	for _, f := range items {
		ok := false
		switch f.Kind {
		case "steps_viewed":
			ok = true
			for _, want := range f.Steps {
				if !containsInt(st.ViewedSteps, want) {
					ok = false
					break
				}
			}
		case "card_dispositioned":
			ok = containsStr(st.DispositionedCards, f.CardID)
		case "student_turns_at_least":
			ok = st.StudentTurns >= f.N
		}
		if !ok {
			unmet = append(unmet, f)
		}
	}
	return unmet
}

// CourseCardCandidate returns the phase's declared card, unless a session card
// instance for it already exists in ANY status — a card is surfaced once per
// session, the same offer discipline Chat uses.
func CourseCardCandidate(sk skills.Skill, phase string, cards []ScopedCard) (string, bool) {
	c, ok := sk.Contracts[phase]
	if !ok || len(c.Cards) == 0 {
		return "", false
	}
	want := c.Cards[0]
	for _, existing := range cards {
		if existing.CardID == want {
			return "", false
		}
	}
	return want, true
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsStr(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// CourseStore is RunCourseStep's isolated persistence seam (project-free). The
// sqlc adapter is coursestore.go; course_step_test.go uses an in-memory fake.
type CourseStore interface {
	GetSession(ctx context.Context, sessionID uuid.UUID) (CourseSession, error)
	SetSessionPhase(ctx context.Context, sessionID uuid.UUID, phase string) error
	LoadPhaseHistory(ctx context.Context, sessionID uuid.UUID, phase string, limit int) ([]ChatTurn, error)
	CountStudentTurns(ctx context.Context, sessionID uuid.UUID, phase string) (int, error)
	CreateSessionMessage(ctx context.Context, sessionID uuid.UUID, phase, role, content string) (uuid.UUID, error)
	ListSessionCards(ctx context.Context, sessionID uuid.UUID) ([]ScopedCard, error)
	CreateSessionCardInstance(ctx context.Context, sessionID uuid.UUID, cardID string) (uuid.UUID, error)
	CreateSessionMaterial(ctx context.Context, sessionID uuid.UUID, title, text string) (uuid.UUID, error)
	ListSessionMaterials(ctx context.Context, sessionID uuid.UUID) ([]ScopedMaterial, error)
	ViewedSteps(ctx context.Context, userID, courseID uuid.UUID) ([]int32, error) // course_progress.completed_ordinals
	InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error
	RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, prompt, completion int32) error
}

// CourseDeps carries everything one course turn needs. CourseTitle is the
// script's course name for the context recipe — the API handler already has
// the course row, so this seam needs no query for it.
type CourseDeps struct {
	Store       CourseStore
	Provider    gateway.Provider
	Resolved    gateway.Resolved
	Skill       skills.Skill
	CourseTitle string
	UserID      uuid.UUID
	SessionID   uuid.UUID
}

// CourseIntent is the student action driving one course turn.
type CourseIntent string // "ask" | "request_advance"

// CourseStepResult is what one course turn produced.
type CourseStepResult struct {
	Reply    string     // "" = silence (enforcement reject)
	Advanced string     // "" = stayed; else the new phase id
	Offer    *CardOffer // session card surfaced at I4
}

// RunCourseStep drives one course turn. Two intents:
//
//	ask            — the student asked something; the coach replies (and may
//	                 surface this phase's declared card as an offer).
//	request_advance — the student wants the next phase. The STRUCTURAL FLOOR is
//	                 checked first, in Go, with store-side inputs: unmet means
//	                 no model call at all. Only with the floor met does the
//	                 coach judge the soft condition (DEC-12.2).
func RunCourseStep(ctx context.Context, deps CourseDeps, intent CourseIntent, studentMessage string) (CourseStepResult, error) {
	sess, err := deps.Store.GetSession(ctx, deps.SessionID)
	if err != nil {
		return CourseStepResult{}, err
	}
	phase, ok := deps.Skill.Contracts[sess.Phase]
	if !ok {
		return CourseStepResult{}, fmt.Errorf("course step: session phase %q is not in skill %s", sess.Phase, deps.Skill.ID)
	}

	if intent == "request_advance" {
		return runCourseAdvance(ctx, deps, sess, phase)
	}
	return runCourseAsk(ctx, deps, sess, phase, studentMessage)
}

// buildScript renders the session script the coach executes.
func buildScript(sk skills.Skill, courseTitle, cur string) CourseScript {
	order, err := sk.LinearOrder()
	if err != nil {
		return CourseScript{CourseTitle: courseTitle, CurrentIdx: -1}
	}
	titles := make([]string, 0, len(order))
	idx := -1
	for i, id := range order {
		titles = append(titles, sk.Contracts[id].Title)
		if id == cur {
			idx = i
		}
	}
	return CourseScript{CourseTitle: courseTitle, PhaseTitles: titles, CurrentIdx: idx}
}

// cardSummary describes the phase's card instance for the context recipe.
func cardSummary(sk skills.Skill, phase skills.Contract, cards []ScopedCard) string {
	if len(phase.Cards) == 0 {
		return ""
	}
	for _, c := range cards {
		if c.CardID == phase.Cards[0] {
			return phase.Cards[0] + "：" + c.Status
		}
	}
	return ""
}

// floorOwedText says what an unmet floor is still waiting for — one plain
// sentence, authored, no model call. This is what a refused advance looks
// like: a sentence in the ask panel, never a lock (铁律 2).
func floorOwedText(unmet []skills.FloorItem, phase skills.Contract) string {
	if len(unmet) == 0 {
		return ""
	}
	switch unmet[0].Kind {
	case "steps_viewed":
		return "这一节还有没看完的部分——先看完，我们再往下走。"
	case "card_dispositioned":
		return "先把这张卡过一遍吧——做完，或者你觉得不需要就跳过，都行。"
	case "student_turns_at_least":
		return "先说说你的想法，哪怕只有一句。说完我们就继续。"
	default:
		return "这一阶段还差一点：" + phase.Goal
	}
}

// courseMeter records the llm_call row whenever tokens were spent — including
// when enforcement rejected the output. The call happened; the cost is real.
// Named courseMeter (not meter) to avoid a collision-inviting bare name in the
// shared agent package.
func courseMeter(ctx context.Context, deps CourseDeps, usage gateway.ChatUsage) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	if err := deps.Store.RecordCourseLLMCall(ctx, deps.UserID, deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); err != nil {
		slog.Warn("course step: record llm_call failed", "err", err)
	}
}

// mintPhaseCard mints the phase's declared card offer — the anchor material,
// the session card_instance, and the card_surfaced event — if the phase
// declares a card not yet surfaced (CourseCardCandidate's any-status check)
// and has an anchor material to hang it on. Store-only: it never calls the
// model, so calling it from the unmet-floor refusal path in runCourseAdvance
// does not break that path's zero-LLM-call guarantee.
//
// Called from BOTH runCourseAsk and runCourseAdvance's card_dispositioned-
// unmet refusal, so a refusal always arrives WITH the card it is asking for —
// DEC-12.2's no-dead-end guarantee. Without this, a student entering `guided`
// and pressing next-arrow before ever chatting would be told to "go do the
// card" about a card that does not exist yet.
func mintPhaseCard(ctx context.Context, deps CourseDeps, sess CourseSession, phase skills.Contract, cards []ScopedCard) (*CardOffer, error) {
	cardID, ok := CourseCardCandidate(deps.Skill, sess.Phase, cards)
	if !ok {
		return nil, nil
	}
	if phase.AnchorMaterial == nil {
		// A misauthored phase: it declares a card but no anchor material to
		// hang it on. Distinct from "no card declared" — this is a config
		// bug, not a normal no-op, so it gets a warning naming the phase.
		slog.Warn("course: phase declares a card but has no anchor material — cannot mint offer",
			"phase", sess.Phase, "card_id", cardID)
		return nil, nil
	}
	matID, err := deps.Store.CreateSessionMaterial(ctx, sess.ID, phase.AnchorMaterial.Title, phase.AnchorMaterial.Text)
	if err != nil {
		return nil, err
	}
	ciID, err := deps.Store.CreateSessionCardInstance(ctx, sess.ID, cardID)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{"card_id": cardID})
	if err := deps.Store.InsertUserEvent(ctx, deps.UserID, "course", "card_surfaced", payload); err != nil {
		slog.Warn("course: append card_surfaced event failed", "err", err)
	}
	return &CardOffer{CardInstanceID: ciID, MaterialID: matID, CardID: cardID}, nil
}

func runCourseAsk(ctx context.Context, deps CourseDeps, sess CourseSession, phase skills.Contract, studentMessage string) (CourseStepResult, error) {
	if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "student", studentMessage); err != nil {
		return CourseStepResult{}, err
	}
	// Spec §7: course events are full-weight evidence. `unprompted` is derived
	// from intent — an `ask` is student-initiated. Only STUDENT messages get
	// this event; an assistant reply is not student evidence.
	askPayload, _ := json.Marshal(map[string]bool{"unprompted": true})
	if err := deps.Store.InsertUserEvent(ctx, deps.UserID, "course", "course_message", askPayload); err != nil {
		slog.Warn("course ask: append course_message event failed", "err", err)
	}
	history, err := deps.Store.LoadPhaseHistory(ctx, sess.ID, sess.Phase, 12)
	if err != nil {
		return CourseStepResult{}, err
	}
	cards, err := deps.Store.ListSessionCards(ctx, sess.ID)
	if err != nil {
		return CourseStepResult{}, err
	}

	script := buildScript(deps.Skill, deps.CourseTitle, sess.Phase)
	out, usage, err := ProposeCourseReply(ctx, deps.Provider, deps.Resolved,
		BuildCourseContext(script, phase, history, cardSummary(deps.Skill, phase, cards), "ask"))
	courseMeter(ctx, deps, usage)
	if err != nil {
		// Silence, not an error: a rejected output is never shown, never
		// persisted, and never surfaced to the student as a failure.
		slog.Warn("course ask: output rejected — staying silent", "err", err, "phase", sess.Phase)
		return CourseStepResult{}, nil
	}
	if out.Type != "reply" {
		// The coach does not advance on a question. An advance here is a
		// script error, not a phase transition.
		slog.Warn("course ask: non-reply output ignored", "type", out.Type, "phase", sess.Phase)
		return CourseStepResult{}, nil
	}
	if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", out.Body); err != nil {
		return CourseStepResult{}, err
	}

	res := CourseStepResult{Reply: out.Body}
	offer, err := mintPhaseCard(ctx, deps, sess, phase, cards)
	if err != nil {
		return CourseStepResult{}, err
	}
	res.Offer = offer
	return res, nil
}

func runCourseAdvance(ctx context.Context, deps CourseDeps, sess CourseSession, phase skills.Contract) (CourseStepResult, error) {
	// The floor first, from store-side inputs only. An unmet floor short-
	// circuits BEFORE the model: no call, no tokens, no advance (DEC-12.2).
	viewed, err := deps.Store.ViewedSteps(ctx, deps.UserID, sess.CourseID)
	if err != nil {
		return CourseStepResult{}, err
	}
	cards, err := deps.Store.ListSessionCards(ctx, sess.ID)
	if err != nil {
		return CourseStepResult{}, err
	}
	turns, err := deps.Store.CountStudentTurns(ctx, sess.ID, sess.Phase)
	if err != nil {
		return CourseStepResult{}, err
	}
	var dispositioned []string
	for _, c := range cards {
		if c.Status == "completed" || c.Status == "skipped" {
			dispositioned = append(dispositioned, c.CardID)
		}
	}
	st := FloorState{ViewedSteps: intsOf(viewed), DispositionedCards: dispositioned, StudentTurns: turns}

	if unmet := CheckFloor(phase.Floor, st); len(unmet) > 0 {
		owed := floorOwedText(unmet, phase)
		if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", owed); err != nil {
			return CourseStepResult{}, err
		}
		// A refusal must arrive WITH the card it is asking for (DEC-12.2's
		// no-dead-end guarantee) — mint it here too, not only on `ask`.
		// Store-only: no model call, so llmCalls stays 0 on this path.
		offer, err := mintPhaseCard(ctx, deps, sess, phase, cards)
		if err != nil {
			return CourseStepResult{}, err
		}
		return CourseStepResult{Reply: owed, Offer: offer}, nil
	}

	next, ok := NextPhase(deps.Skill, sess.Phase)
	if !ok {
		done := "这节课到这里就走完了。"
		if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", done); err != nil {
			return CourseStepResult{}, err
		}
		return CourseStepResult{Reply: done}, nil
	}

	history, err := deps.Store.LoadPhaseHistory(ctx, sess.ID, sess.Phase, 12)
	if err != nil {
		return CourseStepResult{}, err
	}
	script := buildScript(deps.Skill, deps.CourseTitle, sess.Phase)
	out, usage, err := ProposeCourseReply(ctx, deps.Provider, deps.Resolved,
		BuildCourseContext(script, phase, history, cardSummary(deps.Skill, phase, cards), "advance"))
	courseMeter(ctx, deps, usage)
	if err != nil {
		slog.Warn("course advance: output rejected — staying silent", "err", err, "phase", sess.Phase)
		return CourseStepResult{}, nil
	}

	// Forward-only, one phase at a time — enforced here, not requested in the
	// prompt. An advance naming anything but the single successor is refused.
	if out.Type == "advance" && out.To == next {
		if err := deps.Store.SetSessionPhase(ctx, sess.ID, next); err != nil {
			return CourseStepResult{}, err
		}
		payload, _ := json.Marshal(map[string]string{"to": next})
		if err := deps.Store.InsertUserEvent(ctx, deps.UserID, "course", "phase_advanced", payload); err != nil {
			slog.Warn("course advance: append phase_advanced event failed", "err", err)
		}
		return CourseStepResult{Advanced: next}, nil
	}
	if out.Type == "advance" {
		slog.Warn("course advance: refused a non-successor target", "to", out.To, "want", next)
		return CourseStepResult{}, nil
	}
	if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", out.Body); err != nil {
		return CourseStepResult{}, err
	}
	return CourseStepResult{Reply: out.Body}, nil
}

func intsOf(xs []int32) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		out = append(out, int(x))
	}
	return out
}
