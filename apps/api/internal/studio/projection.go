package studio

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// ProjectData is the fully-loaded read set for one project (spec §3).
type ProjectData struct {
	Project        sqlc.Project
	Nodes          []sqlc.GraphNode
	Edges          []sqlc.GraphEdge
	GateStates     []sqlc.GraphNode // gate_state nodes (recorded)
	Plan           *sqlc.GraphNode  // plan node, nil if none
	Interventions  []sqlc.Intervention
	Cards          []sqlc.CardInstance
	ChatMessages   []sqlc.ChatMessage
	Materials      []sqlc.Material
	SourceLog      []sqlc.SourceLogEntry
	EditBuffer     string
	LatestSnapshot *sqlc.DraftSnapshot
	Dispositions   []sqlc.Disposition
	Events         []Event
}

// Event is one row of the append-only event stream, projected for the assessor.
// Payload is opaque; consumers read known keys defensively.
type Event struct {
	Type      string          `json:"type"`
	Surface   string          `json:"surface"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

type planBody struct {
	Route  []string `json:"route"`
	Reason string   `json:"reason"`
	Waived []string `json:"waived"`
}

func planRoute(plan *sqlc.GraphNode) []string {
	if plan == nil {
		return nil
	}
	var b planBody
	if err := json.Unmarshal(plan.Body, &b); err != nil {
		return nil
	}
	return b.Route
}

// planWaived reads the plan node's waived set (Task 1's {route, reason,
// waived} body). Absent plan or unparseable body → empty set (nothing
// waived), never an error — this is a rendering concern, not a hard fail.
func planWaived(plan *sqlc.GraphNode) map[string]bool {
	if plan == nil {
		return map[string]bool{}
	}
	var b planBody
	if err := json.Unmarshal(plan.Body, &b); err != nil {
		return map[string]bool{}
	}
	m := make(map[string]bool, len(b.Waived))
	for _, id := range b.Waived {
		m[id] = true
	}
	return m
}

// projectStations maps the skill's contract DAG onto S0..S6 and returns the
// stations plus the current station's code.
func projectStations(sk skills.Skill, d ProjectData) ([]StationDTO, string, error) {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil, "", err
	}
	g := agent.GraphViewFromRows(d.Nodes, d.Edges, nil, d.Cards)
	recorded := agent.RecordedGatesFromNodes(d.GateStates)
	reports := agent.ReconcileGates(sk, g, recorded)

	route := planRoute(d.Plan)
	waived := planWaived(d.Plan)
	head := "" // the plan's current contract
	if len(route) > 0 {
		head = route[0]
	} else {
		for _, id := range order { // no plan: first non-solid, non-waived in topo order
			if !reports[id].Solid && !waived[id] {
				head = id
				break
			}
		}
	}

	stations := make([]StationDTO, 0, len(order))
	current := ""
	for i, id := range order {
		c := sk.Contracts[id]
		rep := reports[id]
		st := StationDTO{Code: stationCode(i), Name: c.Title, View: c.View}
		switch {
		case waived[id]:
			st.State = "waived" // 已跳过 · 可恢复 — honest (not "done"), re-openable
		case rep.Solid:
			st.State = "done"
			// a done contract still on the route can be revisited
			if inRoute(route, id) {
				st.Backflow = true
			}
		case id == head:
			st.State = "current"
			current = st.Code
		default:
			st.State = "locked"
		}
		if total := gateTotal(c); total > 0 {
			st.Gate = &GateDTO{Total: total, Passed: gatePassed(rep, recorded[id])}
		}
		stations = append(stations, st)
	}
	if current == "" && len(stations) > 0 { // fully done/waived: last non-waived is current
		current = stations[len(stations)-1].Code
		for i := len(stations) - 1; i >= 0; i-- {
			if stations[i].State != "waived" {
				current = stations[i].Code
				break
			}
		}
	}
	return stations, current, nil
}

// allDoneOrWaived reports whether every station is either finished or waived —
// i.e. nothing is still current or locked. Used by canFinish when the writing
// station itself is waived.
func allDoneOrWaived(stations []StationDTO) bool {
	for _, st := range stations {
		if st.State != "done" && st.State != "waived" {
			return false
		}
	}
	return true
}

func stationCode(i int) string { return "S" + string(rune('0'+i)) }

func inRoute(route []string, id string) bool {
	for _, r := range route {
		if r == id {
			return true
		}
	}
	return false
}

func gateTotal(c skills.Contract) int {
	return len(c.Gate.Machine) + len(c.Gate.StudentWritten) + len(c.Gate.Human)
}

// gatePassed counts satisfied gate items: machine items reported Pass AND
// Attempted (agent.ItemResult.Attempted — excludes vacuous passes on
// untouched data, per the gate engine's own predicate knowledge), plus
// student_written/human items recorded "solid".
func gatePassed(rep agent.GateReport, rec agent.RecordedGate) int {
	n := 0
	for _, it := range rep.Items {
		if it.Kind == "machine" {
			if it.Pass && it.Attempted {
				n++
			}
			continue
		}
		if rec.Items[it.Name] == "solid" {
			n++
		}
	}
	return n
}

type anchorBody struct {
	Label string `json:"label"`
}

func anchorLabel(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var b anchorBody
	if err := json.Unmarshal(raw, &b); err != nil {
		return ""
	}
	return b.Label
}

// conversationalInterventionTypes is the ALLOWLIST of intervention.Type
// values that render as prose in the coach rail's conversation thread.
// Deliberately an allowlist, not a denylist: a denylist (the previous shape
// here — "everything except review_item/spot_check_item") lets any FUTURE
// machine-readable intervention type fall straight through and render as a
// raw JSON blob, which is exactly the defect that was live in production
// until this slice caught it for review_item/spot_check_item. A new
// machine-readable type must earn its own DTO and panel (the way
// review_item → 整稿体检's work order and spot_check_item → the station 体检
// panels did) rather than silently qualifying for this set by omission.
//
// "question" (coach.go's live studio turn output) and "diagnostic" +
// "flag" (both minted server-side today — "flag" currently only by the
// seed fixture, "diagnostic" by the live coach loop) are the only types
// whose Body is prose meant for a chat bubble; everything else's Body is a
// marshalled struct.
//
// This predicate is used BOTH by the message-thread loop (so a raw JSON blob
// never renders as a chat bubble) AND by the anchor pick below (so the newest
// non-conversational row can never silently collapse the coach rail's anchor
// line to "") — hoisted into one name so the two can never disagree
// (whole-branch review finding on N3f Task 4/5: the anchor pick used to read
// d.Interventions[n-1] unconditionally).
func isConversationalIntervention(t string) bool {
	switch t {
	case "question", "diagnostic", "flag":
		return true
	default:
		return false
	}
}

// projectCoach builds the coach rail: anchor (the latest NON-work-order
// intervention's, else the current station title) + the time-ordered thread
// merging the intervention thread (flag/ai) with the student's own
// chat_messages (5c: "both sides").
func projectCoach(d ProjectData, currentTitle string) CoachDTO {
	type stamped struct {
		at  time.Time
		msg CoachMessageDTO
	}
	all := make([]stamped, 0, len(d.Interventions)+len(d.ChatMessages))
	for _, iv := range d.Interventions {
		// Non-conversational interventions (review_item / spot_check_item /
		// any future machine-readable type) are work-order rows with their
		// own panels (the 整稿体检 work order / the station 体检 panels) — give
		// them their own DTO/panel instead; do not re-add them here.
		if !isConversationalIntervention(iv.Type) {
			continue
		}
		label := anchorLabel(iv.Anchor)
		if iv.Type == "flag" {
			// A flag's headline is its anchor label; the criterion stays a tag.
			all = append(all, stamped{at: iv.CreatedAt, msg: CoachMessageDTO{Kind: "flag", Label: label, Body: iv.Body}})
			continue
		}
		msg := CoachMessageDTO{Kind: "ai", Body: iv.Body, Anchor: label}
		if iv.Criterion != nil {
			msg.Tag = *iv.Criterion
		}
		all = append(all, stamped{at: iv.CreatedAt, msg: msg})
	}
	for _, cm := range d.ChatMessages {
		if cm.Role != "user" {
			continue
		}
		all = append(all, stamped{at: cm.CreatedAt, msg: CoachMessageDTO{Kind: "student", Body: cm.Content}})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })

	c := CoachDTO{Messages: make([]CoachMessageDTO, len(all))}
	for i, st := range all {
		c.Messages[i] = st.msg
	}
	// Walk back to the last conversational intervention for the anchor pick —
	// the same predicate the message loop above already applies, so the two
	// can never disagree about which row is "the latest real one".
	for i := len(d.Interventions) - 1; i >= 0; i-- {
		if !isConversationalIntervention(d.Interventions[i].Type) {
			continue
		}
		c.Anchor = anchorLabel(d.Interventions[i].Anchor)
		break
	}
	if c.Anchor == "" {
		c.Anchor = currentTitle
	}
	return c
}

func cardMeth(cardID string) string {
	switch cardID {
	case "craap", "sift":
		return "sift_craap"
	case "concession", "steelman":
		return "concession"
	default:
		return ""
	}
}

// materialByCardInstance maps card_instance id -> the material it evaluates,
// read off the card_instance--evaluates-->material graph edge SurfaceCard
// mints (agent/card_lifecycle.go). Shared by projectEquipment and
// projectActiveCard so both derive "which material is this card about" from
// the exact same edge scan — never guessed independently (whole-branch
// review finding [5]).
func materialByCardInstance(d ProjectData) map[string]string {
	materialOf := map[string]string{}
	for _, e := range d.Edges {
		if e.Type == "evaluates" && e.FromKind == "card_instance" && e.ToKind == "material" {
			materialOf[e.FromID.String()] = e.ToID.String()
		}
	}
	return materialOf
}

// projectEquipment maps card_instances to the 装备栏 chips. spont = 提示后 when an
// intervention references the instance (agent-surfaced), else 自发. materialId
// comes from the card_instance--evaluates-->material edge SurfaceCard mints
// (agent/card_lifecycle.go) — the studio projection's own claim to "which
// material is this card about", so the client never has to guess it from
// anchor contents or array position (whole-branch review finding [5]).
//
// Note: the assessor's cardUsesFromProject (internal/api/assessment.go) zips
// d.Cards with this function's output by index, assuming identical 1:1
// iteration order — a future filter/reorder here must not silently misalign
// that digest.
func projectEquipment(d ProjectData, specByID func(string) (cards.Spec, bool)) []EquipCardDTO {
	nudged := NudgedCardInstanceIDs(d)
	materialOf := materialByCardInstance(d)
	out := make([]EquipCardDTO, 0, len(d.Cards))
	for _, ci := range d.Cards {
		name := ci.CardID
		if s, ok := specByID(ci.CardID); ok && s.Name != "" {
			name = s.Name
		}
		spont := "自发"
		if nudged[ci.ID.String()] {
			spont = "提示后"
		}
		out = append(out, EquipCardDTO{
			ID: ci.ID.String(), Name: name, Spont: spont, Meth: cardMeth(ci.CardID),
			MaterialID: materialOf[ci.ID.String()],
		})
	}
	return out
}

func uuidFromPg(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

// NudgedCardInstanceIDs is the single shared spontaneous/prompted rule: a
// card_instance is 提示后 (prompted) when an intervention links it, else 自发
// (spontaneous). projectEquipment and api.countDeclaration both derive their
// split from this exact function — one rule, one definition — so the two can
// never silently disagree about a given card.
func NudgedCardInstanceIDs(d ProjectData) map[string]bool {
	nudged := map[string]bool{}
	for _, iv := range d.Interventions {
		if iv.CardInstanceID.Valid {
			nudged[uuidFromPg(iv.CardInstanceID)] = true
		}
	}
	return nudged
}

// projectOnboarding reads the decode_task graph nodes for the S0 view.
func projectOnboarding(d ProjectData) OnboardingDTO {
	ob := OnboardingDTO{RubricRows: []RubricRowDTO{}, PlanSteps: []string{}, StudentWeakPicks: []int{}}
	for _, n := range d.Nodes {
		switch n.Type {
		case "assignment_brief":
			var body struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(n.Body, &body) == nil && body.Text != "" {
				ob.AssignmentText = body.Text
			}
		case "rubric_translation":
			var body struct {
				RestatePrompt string         `json:"restate_prompt"`
				Rows          []RubricRowDTO `json:"rows"`
			}
			if json.Unmarshal(n.Body, &body) == nil {
				ob.RubricRows = append(ob.RubricRows, body.Rows...)
				if body.RestatePrompt != "" {
					ob.RestatePrompt = body.RestatePrompt
				}
			}
		case "milestone_plan":
			var body struct {
				Steps []string `json:"steps"`
			}
			if json.Unmarshal(n.Body, &body) == nil {
				ob.PlanSteps = append(ob.PlanSteps, body.Steps...)
			}
		case "task_restatement":
			var body struct {
				Restate   string `json:"restate"`
				WeakPicks []int  `json:"weak_picks"`
			}
			// Nodes arrive ordered by created_at (ListGraphNodesByProject),
			// so the last task_restatement seen wins — latest restate.
			if json.Unmarshal(n.Body, &body) == nil {
				ob.StudentRestate = body.Restate
				ob.StudentWeakPicks = body.WeakPicks
				if ob.StudentWeakPicks == nil {
					ob.StudentWeakPicks = []int{}
				}
			}
		}
	}
	return ob
}

// projectFraming reads the frame_question graph nodes for the S1 view.
// research_question is minted once at project creation (project_create.go);
// term_definition/provisional_answer/preregistration are written by the S1
// station view (framing.go). A node whose body fails to unmarshal is
// skipped, never fabricated into a blank row (projectOnboarding's own
// defensive rule).
func projectFraming(d ProjectData) FramingDTO {
	fr := FramingDTO{Terms: []TermDefinitionDTO{}, Answers: []string{}, SearchPlan: []string{}}
	for _, n := range d.Nodes {
		switch n.Type {
		case "research_question":
			var body struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(n.Body, &body) != nil {
				continue
			}
			fr.ResearchQuestion = body.Text
		case "term_definition":
			var body struct {
				Term       string `json:"term"`
				Definition string `json:"definition"`
			}
			if json.Unmarshal(n.Body, &body) != nil {
				continue
			}
			fr.Terms = append(fr.Terms, TermDefinitionDTO{Term: body.Term, Definition: body.Definition})
		case "provisional_answer":
			var body struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(n.Body, &body) != nil {
				continue
			}
			fr.Answers = append(fr.Answers, body.Text)
		case "preregistration":
			var body struct {
				Directions []string `json:"directions"`
			}
			if json.Unmarshal(n.Body, &body) != nil {
				continue
			}
			fr.SearchPlan = append(fr.SearchPlan, body.Directions...)
		}
	}
	return fr
}

// projectPerspectives reads every `perspective` node for the S2 view. Rows the
// station view wrote (origin=="station_view") are editable and carry a level;
// rows minted by perspective-matrix carry neither origin nor level and render
// read-only ("" level, editable=false) — the graph does not care which
// surface asserted them, so a card-minted row still counts toward the list.
// recordedGates is Project()'s own recordedGates variable, reused rather than
// recomputed (mirrors canFinish's own read of the whole_draft_review gate).
func projectPerspectives(d ProjectData, recordedGates map[string]agent.RecordedGate) PerspectivesDTO {
	pv := PerspectivesDTO{Rows: []PerspectiveRowDTO{}}
	for _, n := range d.Nodes {
		if n.Type != "perspective" {
			continue
		}
		var body struct {
			Text   string `json:"text"`
			Level  string `json:"level"`
			Origin string `json:"origin"`
		}
		if json.Unmarshal(n.Body, &body) != nil {
			continue
		}
		pv.Rows = append(pv.Rows, PerspectiveRowDTO{
			Text:     body.Text,
			Level:    body.Level,
			Editable: body.Origin == "station_view",
		})
	}
	pv.SourcesPerPerspective = recordedGates["evaluate_perspectives"].Items["sources_per_perspective"] == "solid"
	return pv
}

// Project builds the full StudioProjection. Pure; no I/O.
func Project(sk skills.Skill, specByID func(string) (cards.Spec, bool), d ProjectData) (StudioProjection, error) {
	stations, current, err := projectStations(sk, d)
	if err != nil {
		return StudioProjection{}, err
	}
	// Look up the current station's title from the stations already built by
	// projectStations, instead of re-running sk.TopoOrder() a second time.
	currentTitle := stationTitle(stations, current)
	coach := projectCoach(d, currentTitle)
	coach.Equipment = projectEquipment(d, specByID)
	finished := d.Project.Status == "finished"
	recordedGates := agent.RecordedGatesFromNodes(d.GateStates)
	// canFinish: default arm keys on the S5 whole-draft review (unchanged
	// behavior — S6 reflection stays optional). If draft_polish itself is
	// waived, whole_draft_review can never be produced, so fall back to the
	// general rule: every non-waived station is done. A waived station never
	// walls finish (铁律 2).
	waived := planWaived(d.Plan)
	canFinish := !finished
	if canFinish {
		if waived["draft_polish"] {
			canFinish = allDoneOrWaived(stations)
		} else {
			// nil-map read is safe; absent gate/item → "" → not solid.
			canFinish = recordedGates["draft_polish"].Items["whole_draft_review"] == "solid"
		}
	}
	return StudioProjection{
		Project:       ProjectHeader{Title: d.Project.Title, QualLabel: d.Project.Qualification},
		Stations:      stations,
		ActiveStation: current,
		Coach:         coach,
		Onboarding:    projectOnboarding(d),
		Framing:       projectFraming(d),
		Perspectives:  projectPerspectives(d, recordedGates),
		Materials:     projectMaterials(d),
		ActiveCard:    projectActiveCard(d, materialByCardInstance(d)),
		Structure:     projectStructure(specByID, d),
		Writing:       projectWriting(sk, d),
		Readiness:     projectReadiness(sk, d),
		SelfScore:     projectSelfScore(sk, d),
		Reflection:    projectReflection(d),
		Prediction:    projectPrediction(sk, d),
		SpotChecks:    projectSpotChecks(d, specByID),
		Finished:      finished,
		CanFinish:     canFinish,
		Declaration:   projectDeclaration(d, coach.Equipment, recordedGates),
	}, nil
}

// projectWriting projects the S5 写作 view: the silent buffer, the latest
// immutable snapshot, the word budget, the citations attestation, and the
// whole-draft review work-order (review_item interventions ⋈ dispositions).
// Derive-never-decorate: every field is read back from persisted rows.
func projectWriting(sk skills.Skill, d ProjectData) WritingDTO {
	out := WritingDTO{Buffer: d.EditBuffer, Review: WritingReviewDTO{Items: []WritingReviewItemDTO{}}}
	if sk.WordBudget != nil {
		out.WordBudget = WordBudgetDTO{Min: sk.WordBudget.Min, Max: sk.WordBudget.Max}
	}
	if d.LatestSnapshot != nil {
		wc := agent.CountWords(d.LatestSnapshot.Content)
		inBand := sk.WordBudget != nil && wc >= sk.WordBudget.Min && wc <= sk.WordBudget.Max
		bState, bDelta := agent.BudgetVerdict(wc, sk.WordBudget)
		out.LatestSnapshot = &WritingSnapshotDTO{
			ID:          d.LatestSnapshot.ID.String(),
			Seq:         int(d.LatestSnapshot.Seq),
			CommittedAt: d.LatestSnapshot.CreatedAt.Format(time.RFC3339),
			WordCount:   wc, InBand: inBand,
			Budget: WritingBudgetDTO{State: bState, Delta: bDelta},
		}
	}
	// citations_matched from the draft_polish gate state.
	recorded := agent.RecordedGatesFromNodes(d.GateStates)
	if rec, ok := recorded["draft_polish"]; ok && rec.Items["citations_matched"] == "solid" {
		out.CitationsMatched = true
	}
	// review_item interventions anchored to the latest snapshot, joined with dispositions.
	dispByIv := map[string]sqlc.Disposition{}
	for _, dp := range d.Dispositions {
		dispByIv[dp.InterventionID.String()] = dp // last write wins (ORDER BY created_at)
	}
	for _, iv := range d.Interventions {
		if iv.Type != "review_item" {
			continue
		}
		if d.LatestSnapshot == nil {
			continue // only the current snapshot's review is shown
		}
		var a struct{ Kind, ID, Voice string }
		_ = json.Unmarshal(iv.Anchor, &a)
		if a.Kind != "draft_snapshot" || a.ID != d.LatestSnapshot.ID.String() {
			continue
		}
		item, ok := reviewItemDTOFromIntervention(iv)
		if !ok {
			continue
		}
		item.Voice = a.Voice
		if item.Voice == "" {
			item.Voice = "board"
		}
		if dp, ok := dispByIv[iv.ID.String()]; ok {
			item.Disposition = &DispositionDTO{Action: dp.Action, Reason: dp.Reason}
		}
		out.Review.Items = append(out.Review.Items, item)
	}
	return out
}

// projectReadiness projects the 评估 view's 就绪度 gauge: one GaugeDTO per skill
// review criterion (0457 表D/E/F/H, config order), lit from the latest snapshot's
// BOARD-voice review (sceptic/layperson/executioner are coaching lenses and never
// light readiness — Slice 9 DEC-9.2/board-only). Always emits the full config set,
// so the display is stable and honest before any review (4 unlit cards). RL-3: lit
// is a descriptor cell, never a grade; the projection is the single clamp site.
func projectReadiness(sk skills.Skill, d ProjectData) []GaugeDTO {
	out := make([]GaugeDTO, 0, len(sk.ReviewCriteria))
	// Board-voice review items for the latest snapshot, keyed by criterion code.
	byCode := map[string]agent.ReviewItem{}
	if d.LatestSnapshot != nil {
		for _, iv := range d.Interventions {
			if iv.Type != "review_item" {
				continue
			}
			var a struct{ Kind, ID, Voice string }
			_ = json.Unmarshal(iv.Anchor, &a)
			if a.Kind != "draft_snapshot" || a.ID != d.LatestSnapshot.ID.String() {
				continue
			}
			if a.Voice != "" && a.Voice != "board" {
				continue // only the board voice is the assessment of record
			}
			var it agent.ReviewItem
			if err := json.Unmarshal([]byte(iv.Body), &it); err != nil {
				continue
			}
			byCode[it.CriterionCode] = it
		}
	}
	for _, c := range sk.ReviewCriteria {
		g := GaugeDTO{Code: c.Code, Name: c.Name, Total: c.Points, Level: "empty"}
		if it, ok := byCode[c.Code]; ok {
			lit := it.Points
			if lit < 0 {
				lit = 0
			}
			if lit > c.Points {
				lit = c.Points
			}
			g.Lit = lit
			g.Note = it.Missing
			switch {
			case g.Total > 0 && g.Lit == g.Total:
				g.Level = "full"
			case g.Lit == 0:
				g.Level = "empty"
			default:
				g.Level = "partial"
			}
			if g.Level == "full" {
				g.Note = ""
			}
		}
		out = append(out, g)
	}
	return out
}

// projectPrediction pairs the student's S0 predicted-weakest criteria
// (task_restatement.weak_picks, indices into review_criteria) with the criteria
// the board review actually found non-full. Read-only; no new capture.
func projectPrediction(sk skills.Skill, d ProjectData) PredictionDTO {
	out := PredictionDTO{Predicted: []PredCritDTO{}, Actual: []PredCritDTO{}}
	// Predicted: weak_picks → review_criteria[i].
	predictedCodes := map[string]bool{}
	for _, i := range projectOnboarding(d).StudentWeakPicks {
		if i >= 0 && i < len(sk.ReviewCriteria) {
			c := sk.ReviewCriteria[i]
			out.Predicted = append(out.Predicted, PredCritDTO{Code: c.Code, Name: c.Name})
			predictedCodes[c.Code] = true
		}
	}
	// Actual: non-full gauges — only meaningful once a review has informed them.
	gauges := projectReadiness(sk, d)
	revealed := false
	for _, g := range gauges {
		if g.Lit > 0 || g.Note != "" {
			revealed = true
			break
		}
	}
	out.Revealed = revealed
	if revealed {
		for _, g := range gauges {
			if g.Level != "full" {
				out.Actual = append(out.Actual, PredCritDTO{Code: g.Code, Name: g.Name})
				if predictedCodes[g.Code] {
					out.Overlap++
				}
			}
		}
	}
	return out
}

// selfScoreBands are the 3 universal self-assessment levels (band index 0..2).
var selfScoreBands = []string{"还需努力", "基本达到", "稳了"}

// retroPrompts nudge causal reflection (RL-4: prompts only, never prose).
var retroPrompts = []string{
	"哪一步真正改变了你的判断？为什么？",
	"如果重来一次，你会在哪一步做得不同？",
	"有没有一个证据或反例，让你不得不修改原来的想法？",
}

// projectSelfScore projects the 先自己评一评 card: one dim per review criterion
// with the student's latest picked band (or -1 unpicked). Bands are universal.
func projectSelfScore(sk skills.Skill, d ProjectData) SelfScoreDTO {
	picked := map[string]int{}
	for _, n := range d.Nodes {
		if n.Type != "self_score" {
			continue
		}
		var body struct {
			Scores []struct {
				Code string `json:"code"`
				Band int    `json:"band"`
			} `json:"scores"`
		}
		if json.Unmarshal(n.Body, &body) != nil {
			continue
		}
		for _, s := range body.Scores { // last self_score node wins
			picked[s.Code] = s.Band
		}
	}
	dims := make([]SelfScoreDimDTO, 0, len(sk.ReviewCriteria))
	for _, c := range sk.ReviewCriteria {
		band := -1
		if b, ok := picked[c.Code]; ok {
			band = b
		}
		dims = append(dims, SelfScoreDimDTO{Code: c.Code, Name: c.Name, Band: band})
	}
	return SelfScoreDTO{Dims: dims, Bands: selfScoreBands}
}

// projectReflection projects the 写一段研究回顾 card: the latest student
// reflection text + the prompt chips.
func projectReflection(d ProjectData) ReflectionDTO {
	text := ""
	for _, n := range d.Nodes {
		if n.Type != "reflection" {
			continue
		}
		var body struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &body) == nil {
			text = body.Text // latest wins (nodes ordered by created_at)
		}
	}
	return ReflectionDTO{Text: text, Prompts: retroPrompts}
}

// projectDeclaration projects the S6 AI 使用申报单. Signed comes from the
// recorded reflect_archive gate_state's declaration_signed item — the exact
// same "solid" check agent.CheckGate applies to every human/student_written
// item, so this can never disagree with the gate the student is actually
// walking through.
//
// Before signing, the counters are computed live from equipment (the same
// projectEquipment output Coach.Equipment already carries — its 自发/提示后
// split is not re-derived a second time here) and from d directly. After
// signing, they are read back from the persisted `declaration` node
// (internal/api/declaration.go's signDeclaration mints it in the same
// transaction that flips the gate item) rather than recomputed — otherwise
// the screen would show numbers drifting away from what she actually signed.
func projectDeclaration(d ProjectData, equipment []EquipCardDTO, recordedGates map[string]agent.RecordedGate) DeclarationDTO {
	signed := recordedGates["reflect_archive"].Items["declaration_signed"] == "solid"
	if signed {
		var dto DeclarationDTO
		found := false
		for _, n := range d.Nodes {
			if n.Type != "declaration" {
				continue
			}
			if json.Unmarshal(n.Body, &dto) == nil {
				found = true // latest wins, though signDeclaration is idempotent and mints at most one
			}
		}
		if found {
			dto.Signed = true
			return dto
		}
		// Recorded solid but no persisted node found — shouldn't happen
		// (signDeclaration writes both in one transaction), but fall through
		// to live counts rather than silently claim zeros.
	}

	asks := 0
	for _, e := range d.Events {
		if e.Type == "prompt_sent" {
			asks++
		}
	}
	spont, prompted := 0, 0
	for _, ec := range equipment {
		if ec.Spont == "提示后" {
			prompted++
		} else {
			spont++
		}
	}
	return DeclarationDTO{
		Asks:             asks,
		Dispositions:     len(d.Dispositions),
		CardsSpontaneous: spont,
		CardsPrompted:    prompted,
		AiWrittenProse:   0,
		Signed:           signed,
	}
}

// reviewItemDTOFromIntervention reconstructs a WritingReviewItemDTO from a
// review_item intervention row. iv.Body is the full agent.ReviewItem JSON
// (Task 6's InsertReviewIntervention) — one json.Unmarshal, never
// string-splitting the flat Criterion/Level columns that stay duplicated
// only for SQL filters.
func reviewItemDTOFromIntervention(iv sqlc.Intervention) (WritingReviewItemDTO, bool) {
	var it agent.ReviewItem
	if err := json.Unmarshal([]byte(iv.Body), &it); err != nil {
		return WritingReviewItemDTO{}, false
	}
	criterion := it.CriterionCode + " " + it.CriterionName
	if iv.Criterion != nil {
		criterion = *iv.Criterion // the exact flat-column value InsertReviewIntervention persisted
	}
	return WritingReviewItemDTO{
		InterventionID: iv.ID.String(),
		Criterion:      criterion,
		Band:           it.Band,
		Evidence:       it.Evidence,
		Missing:        it.Missing,
		Fix:            it.Fix,
	}, true
}

// spotCheckAnchor is the {station,fingerprint} anchor Task 4's orderSpotCheck
// writes (api/spotcheck.go) on every spot_check_item intervention it inserts.
type spotCheckAnchor struct {
	Station     string `json:"station"`
	Fingerprint string `json:"fingerprint"`
}

// spotCheckItemDTOFromIntervention reconstructs a SpotCheckItemDTO from a
// spot_check_item intervention row. iv.Body is the full agent.SpotCheckItem
// JSON (api/spotcheck.go's InsertSpotCheckIntervention) — one json.Unmarshal,
// never string-splitting, exactly reviewItemDTOFromIntervention's pattern.
func spotCheckItemDTOFromIntervention(iv sqlc.Intervention) (SpotCheckItemDTO, bool) {
	var it agent.SpotCheckItem
	if err := json.Unmarshal([]byte(iv.Body), &it); err != nil {
		return SpotCheckItemDTO{}, false
	}
	return SpotCheckItemDTO{
		InterventionID: iv.ID.String(),
		TargetID:       it.TargetID,
		TargetName:     it.TargetName,
		Evidence:       it.Evidence,
		Missing:        it.Missing,
		Fix:            it.Fix,
	}, true
}

// projectSpotCheckFx projects one station's spot-check panel: a chosen
// batch's items (joined with dispositions) plus whether ordering is
// currently possible.
//
// A "batch" = every spot_check_item intervention anchored to this station
// sharing one fingerprint (orderSpotCheck inserts a whole batch atomically
// in one call, so every row in a batch shares one fingerprint).
//
// Which batch to show is NOT simply "the most recent one" — recency alone
// mishandles an edit-then-revert sequence: order at state A (batch A, fp
// fpA) -> edit to state B and order again (batch B, fp fpB, later
// CreatedAt) -> revert the text back to exactly state A. The current
// fingerprint is fpA again, and batch A already exists for it, but "most
// recent" would still pick batch B — showing now-irrelevant items AND
// reporting orderable=true (fpA != fpB) even though pressing the button
// would just replay batch A for free. So: prefer the batch whose
// fingerprint equals the CURRENT fingerprint if one exists (orderable=false
// — a real match, nothing to (re)order); only when no batch matches the
// current fingerprint fall back to the most recent batch by CreatedAt
// (orderable=true — deliberately stale-but-visible, so her work order does
// not vanish the moment she starts typing).
//
// The current fingerprint is computed via
// agent.SpotCheckFingerprint(SpotCheckTargets(...)) — the SAME builder
// api/spotcheck.go's orderSpotCheck itself calls (spec §5.7) — so this can
// never disagree with what pressing the button would actually do. A
// station with NO targets at all is not orderable (there is nothing to
// check), even though the "no targets" fingerprint trivially differs from
// any stored one.
func projectSpotCheckFx(d ProjectData, station string, specByID func(string) (cards.Spec, bool)) SpotCheckFxDTO {
	dispByIv := map[string]sqlc.Disposition{}
	for _, dp := range d.Dispositions {
		dispByIv[dp.InterventionID.String()] = dp // last write wins (ORDER BY created_at)
	}

	type row struct {
		at          time.Time
		fingerprint string
		item        SpotCheckItemDTO
	}
	var rows []row
	var latestFP string
	var latestAt time.Time
	haveLatest := false
	haveFP := map[string]bool{}
	for _, iv := range d.Interventions {
		if iv.Type != "spot_check_item" {
			continue
		}
		var a spotCheckAnchor
		if err := json.Unmarshal(iv.Anchor, &a); err != nil || a.Station != station {
			continue
		}
		item, ok := spotCheckItemDTOFromIntervention(iv)
		if !ok {
			continue
		}
		if dp, ok := dispByIv[iv.ID.String()]; ok {
			item.Disposition = &DispositionDTO{Action: dp.Action, Reason: dp.Reason}
		}
		rows = append(rows, row{at: iv.CreatedAt, fingerprint: a.Fingerprint, item: item})
		haveFP[a.Fingerprint] = true
		if !haveLatest || iv.CreatedAt.After(latestAt) {
			latestAt, latestFP, haveLatest = iv.CreatedAt, a.Fingerprint, true
		}
	}

	targets := SpotCheckTargets(d, station, specByID)
	currentFP := agent.SpotCheckFingerprint(targets)

	selectedFP := latestFP
	orderable := false
	if len(targets) > 0 {
		if haveFP[currentFP] {
			selectedFP = currentFP
		} else {
			orderable = true
		}
	}

	items := make([]SpotCheckItemDTO, 0, len(rows))
	for _, rw := range rows {
		if rw.fingerprint == selectedFP {
			items = append(items, rw.item)
		}
	}

	return SpotCheckFxDTO{Items: items, Orderable: orderable}
}

// projectSpotChecks projects the S3/S4 station spot-check panels (信源体检 /
// 论证体检) for StudioProjection's flat, required spotChecks member.
func projectSpotChecks(d ProjectData, specByID func(string) (cards.Spec, bool)) SpotChecksDTO {
	return SpotChecksDTO{
		EvaluateSources: projectSpotCheckFx(d, agent.SpotCheckSources, specByID),
		BuildArgument:   projectSpotCheckFx(d, agent.SpotCheckArgument, specByID),
	}
}

// projectStructure projects the five Toulmin argument slots into 论证构建 role
// cards. Slot order + role labels come from the toulmin card spec (single
// source of truth); status/preview come from the minted graph nodes. A slot is
// "done" only when a node typed for it carries a body.text the student wrote —
// which excludes CRAAP's source_quality evidence node (no text key), so the
// evidence slot is never falsely done. Returns an empty slice until at least
// one slot node exists, so the 结构 pane keeps its placeholder before the
// argument is built.
func projectStructure(specByID func(string) (cards.Spec, bool), d ProjectData) []StructureCardDTO {
	spec, ok := specByID("toulmin")
	if !ok || len(spec.Params.Slots) == 0 {
		return []StructureCardDTO{}
	}
	text := map[string]string{}
	for _, n := range d.Nodes {
		if _, seen := text[n.Type]; seen {
			continue
		}
		var b struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &b) == nil && strings.TrimSpace(b.Text) != "" {
			text[n.Type] = b.Text
		}
	}
	minted := false
	out := make([]StructureCardDTO, 0, len(spec.Params.Slots))
	for _, slot := range spec.Params.Slots {
		card := StructureCardDTO{ID: slot.ID, Role: slot.Role, Status: "empty"}
		if t, ok := text[slot.ID]; ok {
			card.Status, card.Preview = "done", t
			minted = true
		}
		out = append(out, card)
	}
	if !minted {
		return []StructureCardDTO{}
	}
	return out
}

// projectActiveCard projects the one project-wide card_instance that is
// "proposed" or "active". agent.SurfaceCardCandidates' suppression
// (classifier.go) makes at most one such row the INTENDED steady state, but
// that is not a hard invariant nothing can violate — no unique constraint,
// no lock spanning its perceive-time read and the SurfaceCard insert it
// gates, so two concurrent turns could in principle each perceive "nothing
// in flight" and both mint one (FIX 6, whole-branch review: the comment this
// replaces overclaimed a guarantee nothing in the schema or transaction
// boundary actually provides). This defensively takes the first match rather
// than assuming exactly one. nil when none is open — a page reload must be
// able to rehydrate this, or the client loses its only reference to the open
// card while the row stays open server-side, and FIX-D's suppression then
// blocks every future card from surfacing ever again (whole-branch review
// finding, CRITICAL).
func projectActiveCard(d ProjectData, materialOf map[string]string) *ActiveCardDTO {
	for _, c := range d.Cards {
		if c.Status != "proposed" && c.Status != "active" {
			continue
		}
		anchors := []json.RawMessage{}
		_ = json.Unmarshal(c.Anchors, &anchors)
		return &ActiveCardDTO{
			CardInstanceID: c.ID.String(),
			CardID:         c.CardID,
			Status:         c.Status,
			Anchors:        anchors,
			MaterialID:     materialOf[c.ID.String()],
		}
	}
	return nil
}

// projectMaterials derives each source's dossier state. Nothing here invents a
// judgment: locked/role exist only because the student completed a CRAAP card
// and the mint wrote them (agent.GraphEffects). tier/takeaway exist only
// because the student wrote a source-log entry. lateralRead exists only
// because a cross_check mint (Slice 6c) flipped source_log_entry.lateral_read
// on the checked source. anchors are exactly the persisted
// card_instances.anchors whose own material_id targets this source.
// isLateralInstrument exists only because some OTHER cross_check node
// --cites--> this material — the same graph fact agent.SurfaceCardCandidates
// already reads to keep a lateral instrument from ever getting its own SIFT
// proposal (fix-wave finding [4]). siftSkipped exists only because a SIFT
// card_instance targeting this material was itself skipped (FIX 3).
func projectMaterials(d ProjectData) []MaterialDTO {
	nodesByID := map[string]sqlc.GraphNode{}
	for _, n := range d.Nodes {
		nodesByID[n.ID.String()] = n
	}
	// material id → the evidence node its evaluated-as edge points at.
	evidence := map[string]sqlc.GraphNode{}
	for _, e := range d.Edges {
		if e.Type != "evaluated-as" || e.FromKind != "material" {
			continue
		}
		if n, ok := nodesByID[e.ToID.String()]; ok {
			evidence[e.FromID.String()] = n
		}
	}
	log := map[string]sqlc.SourceLogEntry{}
	for _, s := range d.SourceLog {
		if s.MaterialID.Valid {
			log[uuid.UUID(s.MaterialID.Bytes).String()] = s
		}
	}
	// lateralInstrument mirrors agent.SurfaceCardCandidates' own treadmill-
	// guard derivation (classifier.go): a material some cross_check node
	// --cites--> is that cross_check's lateral (found) source, never the
	// material under review. Same graph fact, computed once here so the
	// dossier's chip and the summon rule can never drift apart (whole-branch
	// review finding [4]).
	crossCheckNode := map[string]bool{}
	for _, n := range d.Nodes {
		if n.Type == "cross_check" {
			crossCheckNode[n.ID.String()] = true
		}
	}
	lateralInstrument := map[string]bool{}
	for _, e := range d.Edges {
		if e.Type == "cites" && e.FromKind == "graph_node" && e.ToKind == "material" && crossCheckNode[e.FromID.String()] {
			lateralInstrument[e.ToID.String()] = true
		}
	}
	// siftSkipped: material id -> a SIFT card_instance targeting it was
	// explicitly skipped (FIX 3, whole-branch review). "sift" is the same
	// literal card id agent/classifier.go's unexported siftCardID constant
	// names — there is no shared cross-package constant for it today, same
	// as isLateralInstrument's derivation above needing no shared constant
	// either. agent.SurfaceCardCandidates' siftSurfaced map treats a skip
	// exactly like any other in-flight/terminal status and never re-proposes
	// SIFT on this material again, so a skip here is permanent — the
	// dossier's 需横向阅读 chip must stop claiming an active workflow the
	// summon rule will in fact never offer again.
	siftSkipped := map[string]bool{}
	materialOfCard := materialByCardInstance(d)
	for _, c := range d.Cards {
		if c.CardID != "sift" || c.Status != "skipped" {
			continue
		}
		if mid, ok := materialOfCard[c.ID.String()]; ok {
			siftSkipped[mid] = true
		}
	}
	// material id → the cross_check node reached by ITS OWN "cross-checked-by"
	// edge (the material she actually checked, never the lateral instrument
	// it cites) — read here so lateralNote() can pull her own relation/
	// revised_judgment words back out (whole-branch review: "written but
	// never read").
	crossCheckOf := map[string]sqlc.GraphNode{}
	for _, e := range d.Edges {
		if e.Type != "cross-checked-by" || e.FromKind != "material" {
			continue
		}
		if n, ok := nodesByID[e.ToID.String()]; ok {
			crossCheckOf[e.FromID.String()] = n
		}
	}
	// Anchors carry their own material_id — card_instances has no such column.
	anchorsByMaterial := map[string][]json.RawMessage{}
	for _, c := range d.Cards {
		// A card the student explicitly skipped must not keep re-asking its
		// question: exclude its anchors from lighting up the article on
		// every reload. "active"/"completed" cards still surface theirs.
		if c.Status == "skipped" {
			continue
		}
		var raw []json.RawMessage
		if err := json.Unmarshal(c.Anchors, &raw); err != nil {
			continue
		}
		for _, a := range raw {
			var probe struct {
				MaterialID string `json:"material_id"`
			}
			if err := json.Unmarshal(a, &probe); err != nil || probe.MaterialID == "" {
				continue
			}
			anchorsByMaterial[probe.MaterialID] = append(anchorsByMaterial[probe.MaterialID], a)
		}
	}

	out := make([]MaterialDTO, 0, len(d.Materials))
	for _, m := range d.Materials {
		id := m.ID.String()
		dto := MaterialDTO{
			ID: id, Title: m.Title, Kind: m.Kind, Origin: m.Source,
			Blocks: []MaterialBlockDTO{}, Anchors: []json.RawMessage{},
		}
		if m.SourceUrl != nil {
			dto.SourceURL = *m.SourceUrl
		}
		_ = json.Unmarshal(m.Blocks, &dto.Blocks)
		if n, ok := evidence[id]; ok {
			dto.Locked = true
			dto.Role = riskNote(n.Body)
		}
		if s, ok := log[id]; ok {
			dto.Takeaway = s.Takeaway
			dto.TimeSpentS = s.TimeSpentS
			dto.LateralRead = s.LateralRead
			if s.Tier != nil {
				dto.Tier = *s.Tier
			}
		}
		if as, ok := anchorsByMaterial[id]; ok {
			dto.Anchors = as
		}
		dto.IsLateralInstrument = lateralInstrument[id]
		dto.SiftSkipped = siftSkipped[id]
		if n, ok := crossCheckOf[id]; ok {
			dto.LateralRelation, dto.LateralJudgment = lateralNote(n.Body)
		}
		out = append(out, dto)
	}
	return out
}

// lateralNote reads body.relation/body.revised_judgment off a minted
// cross_check node (agent/card_effects.go's crossCheckBody) — her own chosen
// relation (印证/反驳/限定) and her own revised-judgment sentence, exactly as
// written. Empty strings when either was never answered (fields on
// crossCheckBody are only set when non-blank) — never fabricated.
func lateralNote(body []byte) (relation, judgment string) {
	var b struct {
		Relation        string `json:"relation"`
		RevisedJudgment string `json:"revised_judgment"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return "", ""
	}
	return b.Relation, b.RevisedJudgment
}

// riskNote reads body.source_quality.risk_note off a minted evidence node.
func riskNote(body []byte) string {
	var b struct {
		SourceQuality map[string]string `json:"source_quality"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return ""
	}
	return b.SourceQuality["risk_note"]
}

func stationTitle(stations []StationDTO, code string) string {
	for _, s := range stations {
		if s.Code == code {
			return s.Name
		}
	}
	return ""
}
