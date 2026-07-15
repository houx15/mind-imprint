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
}

type planBody struct {
	Route  []string `json:"route"`
	Reason string   `json:"reason"`
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
	head := "" // the plan's current contract
	if len(route) > 0 {
		head = route[0]
	} else {
		for _, id := range order { // no plan: first non-solid in topo order
			if !reports[id].Solid {
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
	if current == "" && len(stations) > 0 { // fully done: last station is current
		current = stations[len(stations)-1].Code
	}
	return stations, current, nil
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

// projectCoach builds the coach rail: anchor (latest intervention's, else the
// current station title) + the time-ordered thread merging the intervention
// thread (flag/ai) with the student's own chat_messages (5c: "both sides").
func projectCoach(d ProjectData, currentTitle string) CoachDTO {
	type stamped struct {
		at  time.Time
		msg CoachMessageDTO
	}
	all := make([]stamped, 0, len(d.Interventions)+len(d.ChatMessages))
	for _, iv := range d.Interventions {
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
	if n := len(d.Interventions); n > 0 {
		c.Anchor = anchorLabel(d.Interventions[n-1].Anchor)
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
func projectEquipment(d ProjectData, specByID func(string) (cards.Spec, bool)) []EquipCardDTO {
	nudged := map[string]bool{}
	for _, iv := range d.Interventions {
		if iv.CardInstanceID.Valid {
			nudged[uuidFromPg(iv.CardInstanceID)] = true
		}
	}
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

// projectOnboarding reads the decode_task graph nodes for the S0 view.
func projectOnboarding(d ProjectData) OnboardingDTO {
	ob := OnboardingDTO{RubricRows: []RubricRowDTO{}, PlanSteps: []string{}}
	for _, n := range d.Nodes {
		switch n.Type {
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
		}
	}
	return ob
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
	return StudioProjection{
		Project:       ProjectHeader{Title: d.Project.Title, QualLabel: d.Project.Qualification},
		Stations:      stations,
		ActiveStation: current,
		Coach:         coach,
		Onboarding:    projectOnboarding(d),
		Materials:     projectMaterials(d),
		ActiveCard:    projectActiveCard(d, materialByCardInstance(d)),
		Structure:     projectStructure(specByID, d),
		Writing:       projectWriting(sk, d),
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
