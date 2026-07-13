package studio

import (
	"encoding/json"
	"sort"
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
	Project       sqlc.Project
	Nodes         []sqlc.GraphNode
	Edges         []sqlc.GraphEdge
	GateStates    []sqlc.GraphNode // gate_state nodes (recorded)
	Plan          *sqlc.GraphNode  // plan node, nil if none
	Interventions []sqlc.Intervention
	Cards         []sqlc.CardInstance
	ChatMessages  []sqlc.ChatMessage
	Materials     []sqlc.Material
	SourceLog     []sqlc.SourceLogEntry
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

// projectEquipment maps card_instances to the 装备栏 chips. spont = 提示后 when an
// intervention references the instance (agent-surfaced), else 自发.
func projectEquipment(d ProjectData, specByID func(string) (cards.Spec, bool)) []EquipCardDTO {
	nudged := map[string]bool{}
	for _, iv := range d.Interventions {
		if iv.CardInstanceID.Valid {
			nudged[uuidFromPg(iv.CardInstanceID)] = true
		}
	}
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
		out = append(out, EquipCardDTO{ID: ci.ID.String(), Name: name, Spont: spont, Meth: cardMeth(ci.CardID)})
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
	}, nil
}

// projectMaterials derives each source's dossier state. Nothing here invents a
// judgment: locked/role exist only because the student completed a CRAAP card
// and the mint wrote them (agent.GraphEffects). tier/takeaway exist only
// because the student wrote a source-log entry. anchors are exactly the
// persisted card_instances.anchors whose own material_id targets this source.
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
	// Anchors carry their own material_id — card_instances has no such column.
	anchorsByMaterial := map[string][]json.RawMessage{}
	for _, c := range d.Cards {
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
			if s.Tier != nil {
				dto.Tier = *s.Tier
			}
		}
		if as, ok := anchorsByMaterial[id]; ok {
			dto.Anchors = as
		}
		out = append(out, dto)
	}
	return out
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
