package studio

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
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

// cardMeth is the equipment-bar (装备栏) 「工具说明书」 lookup — EquipCardDTO.Meth
// feeds EquipmentBar.tsx's onOpen(card.meth), which the web MethodologyModal
// keys its copy bank by. craap/sift both map to "sift_craap" DELIBERATELY,
// not as a stopgap: the front-end METHODOLOGY bank
// (apps/web/src/studio/MethodologyModal.tsx) has exactly ONE combined entry
// covering both — "SIFT × CRAAP 信息核查", explicitly framed as "两套互补的核查
// 方法" — there is no narrower craap-only or sift-only copy to repoint this at,
// and MethodologyModal's own unknown-id fallback (DEFAULT_METHODOLOGY) IS that
// same sift_craap entry, so returning "" here would render byte-identical
// copy anyway.
//
// Verified reachable (Task 11, spec-read-together-redesign): craap/sift no
// longer surface as LIVE studio rail cards (the studio turn/submit paths stop
// proposing them — studioturn.go/projectcards.go's SuppressSurfaceCardIDs),
// and the reading room's own HangingCard never opens MethodologyModal. But
// projectEquipment (below) lists EVERY card_instance for the project
// regardless of which surface summoned it or its current status — so a
// craap/sift card completed in the reading room still appears as an
// equipment-bar chip in the STUDIO, and clicking it DOES reach this mapping.
// TestProjectEquipment_CraapSiftMeth pins the current, intentional value so a
// future change here is a deliberate decision, not a silent drift.
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

// selfScoreBands are the 3 universal self-assessment levels (band index 0..2).
var selfScoreBands = []string{"还需努力", "基本达到", "稳了"}

// retroPrompts nudge causal reflection (RL-4: prompts only, never prose).
var retroPrompts = []string{
	"哪一步真正改变了你的判断？为什么？",
	"如果重来一次，你会在哪一步做得不同？",
	"有没有一个证据或反例，让你不得不修改原来的想法？",
}

// ProjectMaterials exposes the material-dossier projection (projectMaterials)
// to callers outside the full Project() build. The Read-library's enter-reading
// endpoint reuses it to return one material's MaterialSource DTO: real
// blocks/anchors/timeSpentS/etc. when the material already has them, and the
// zero/false derive-never-decorate defaults (locked=false, role/tier/takeaway
// "", anchors [], timeSpentS 0, lateralRead/isLateralInstrument/siftSkipped
// false, lateralRelation/lateralJudgment "") for a freshly fetched one.
func ProjectMaterials(d ProjectData) []MaterialDTO { return projectMaterials(d) }

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
