package evalbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/evalreport"
)

const PersonaAdapterVersion = "persona-export-v2"

// PersonaExport is deliberately a whitelist. It does not model historical
// assessments, mirrors, summaries, or evaluation reports.
type PersonaExport struct {
	Project struct {
		ID            string    `json:"id"`
		Title         string    `json:"title"`
		Qualification string    `json:"qualification"`
		Status        string    `json:"status"`
		CreatedAt     time.Time `json:"created_at"`
		LastActiveAt  time.Time `json:"last_active_at"`
	} `json:"project"`
	Chat                []personaChat     `json:"chat"`
	Actions             []personaAction   `json:"actions"`
	Materials           []personaMaterial `json:"materials"`
	RevisionCheckpoints []checkpoint      `json:"revision_checkpoints"`
	ProcessGraphNodes   []graphNode       `json:"process_graph_nodes"`
	Outputs             personaOutputs    `json:"outputs"`
}

type personaChat struct {
	At      time.Time `json:"at"`
	Surface string    `json:"surface"`
	Role    string    `json:"role"`
	Content string    `json:"content"`
}
type personaAction struct {
	At      time.Time      `json:"at"`
	Surface string         `json:"surface"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}
type checkpoint struct {
	At       time.Time `json:"at"`
	Artifact string    `json:"artifact"`
	Trigger  string    `json:"trigger"`
	Hash     string    `json:"hash"`
}
type graphNode struct {
	Type string         `json:"type"`
	Body map[string]any `json:"body"`
	At   time.Time      `json:"at"`
}
type personaMaterial struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Source    string    `json:"source"`
	Title     string    `json:"title"`
	SourceURL *string   `json:"source_url"`
	At        time.Time `json:"at"`
	Blocks    []struct {
		Text string `json:"text"`
	} `json:"blocks"`
}
type personaOutputs struct {
	Proposal struct {
		Objective     string `json:"objective"`
		Reason        string `json:"reason"`
		Activities    string `json:"activities"`
		Resources     string `json:"resources"`
		Counterpoints string `json:"counterpoints"`
	} `json:"proposal"`
	Snapshots []struct {
		Seq     int       `json:"seq"`
		DocKind string    `json:"doc_kind"`
		At      time.Time `json:"at"`
		Content string    `json:"content"`
	} `json:"snapshots"`
	Buffers []struct {
		DocKind string    `json:"doc_kind"`
		Content string    `json:"content"`
		Updated time.Time `json:"updated_at"`
	} `json:"buffers"`
	Cards []struct {
		CardID     string   `json:"card_id"`
		Status     string   `json:"status"`
		RubricTags []string `json:"rubric_tags"`
	} `json:"cards"`
	Library []struct {
		Title           string `json:"title"`
		Classification  string `json:"classification"`
		Decision        string `json:"decision"`
		EvidenceFinding string `json:"evidence_finding"`
	} `json:"library"`
	ExplorationLeads []struct {
		Text   string `json:"text"`
		Status string `json:"status"`
		Origin string `json:"origin"`
	} `json:"exploration_leads"`
	Interventions []struct {
		Type string `json:"type"`
		Body string `json:"body"`
	} `json:"interventions"`
	Reflection struct {
		Answers []string `json:"answers"`
	} `json:"reflection"`
}

type AdaptedInput struct {
	Input    Input
	Hash     string
	Warnings []string
}

func LoadPersona(path string) (PersonaExport, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return PersonaExport{}, fmt.Errorf("evalbench: read persona: %w", err)
	}
	var p PersonaExport
	if err := json.Unmarshal(b, &p); err != nil {
		return PersonaExport{}, fmt.Errorf("evalbench: parse persona: %w", err)
	}
	return p, nil
}

// AdaptPersona maps a frozen export to the current EvaluationReport generator
// input. IDs are stable and anonymous: they support offline evidence links,
// never production navigation.
func AdaptPersona(caseID string, p PersonaExport) (AdaptedInput, error) {
	if strings.TrimSpace(caseID) == "" {
		return AdaptedInput{}, fmt.Errorf("evalbench: case id is required")
	}
	warnings := []string{"export has no llm_call rows; aiTurns counts assistant chat messages"}
	started := p.Project.CreatedAt
	if started.IsZero() {
		started = earliest(p.Chat, p.Actions)
	}
	if started.IsZero() {
		started = time.Unix(0, 0).UTC()
		warnings = append(warnings, "project has no timestamp; used Unix epoch")
	}

	chat := append([]personaChat(nil), p.Chat...)
	sort.SliceStable(chat, func(i, j int) bool { return chat[i].At.Before(chat[j].At) })
	candidates := make([]evalreport.Candidate, 0, len(chat)+len(p.Actions)+len(p.Materials))
	var prompts strings.Builder
	assistantTurns := 0
	for i, m := range chat {
		id := fmt.Sprintf("message:export:%03d", i+1)
		candidates = append(candidates, evalreport.Candidate{ID: id, Kind: evalreport.KindChat, Label: evalreport.Label(m.Role+"："+m.Content, 60)})
		if m.Role == "assistant" {
			assistantTurns++
			continue
		}
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
			fmt.Fprintf(&prompts, "[%s] (%s) %s\n", id, m.Surface, evalreport.Label(strings.TrimSpace(m.Content), 200))
		}
	}

	actions := append([]personaAction(nil), p.Actions...)
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].At.Before(actions[j].At) })
	events := make([]evalreport.EventEntry, 0, len(actions))
	for i, a := range actions {
		id := fmt.Sprintf("event:export:%03d", i+1)
		candidates = append(candidates, evalreport.Candidate{ID: id, Kind: evalreport.KindEvent, Label: evalreport.Label(a.Type, 60)})
		if kind, ok := eventKind(a.Type); ok {
			events = append(events, evalreport.EventEntry{TS: stamp(a.At, started), Kind: kind, Summary: eventSummary(a), Ref: &evalreport.Ref{ID: id, Label: a.Type, TS: stamp(a.At, started)}})
		}
	}
	if len(events) > 40 {
		events = events[:40]
	}

	body := latestEssay(p.Outputs)
	materials := adaptMaterials(p, started, &candidates)
	tools := make([]evalreport.ToolUsageEntry, 0, len(p.Outputs.Cards))
	for i, card := range p.Outputs.Cards {
		if card.Status == "skipped" {
			continue
		}
		id := fmt.Sprintf("card:export:%03d", i+1)
		candidates = append(candidates, evalreport.Candidate{ID: id, Kind: evalreport.KindCard, Label: evalreport.Label(card.CardID, 60)})
		tools = append(tools, evalreport.ToolUsageEntry{ToolID: card.CardID, Name: card.CardID, Purpose: card.Status, Summary: "状态：" + card.Status})
	}
	for i, lead := range p.Outputs.ExplorationLeads {
		candidates = append(candidates, evalreport.Candidate{ID: fmt.Sprintf("lead:export:%03d", i+1), Kind: evalreport.KindLead, Label: evalreport.Label(lead.Text, 60)})
	}

	trajectory := buildTrajectory(p, materials, body)
	risk := fmt.Sprintf("〔正文〕\n%s\n\n〔来源数〕%d；〔探索线索〕%d\n", evalreport.Label(body, 1800), len(materials), len(p.Outputs.ExplorationLeads))
	var end *string
	if p.Project.Status == "finished" && !p.Project.LastActiveAt.IsZero() {
		v := p.Project.LastActiveAt.UTC().Format(time.RFC3339)
		end = &v
	}
	comments := countComments(p)
	input := Input{
		ReportID: "evalbench:report:" + caseID, ProjectID: "evalbench:project:" + caseID, StudentID: "evalbench:student:" + caseID, StudentName: "测试画像 " + caseID,
		Basics: evalreport.Basics{Title: p.Project.Title, Type: p.Project.Qualification, StartDate: started.UTC().Format(time.RFC3339), EndDate: end,
			Milestones: evalreport.Milestones{Started: ptr(stamp(started, started))}, Counters: evalreport.Counters{AITurns: assistantTurns, MaterialsRead: len(materials), WordsWritten: evalreport.CountWords(body), AICommentCount: comments, EditCount: len(p.RevisionCheckpoints)}},
		Events: events, Materials: materials, ToolUsage: tools, Candidates: candidates,
		Context: agent.ReportGenContext{Title: p.Project.Title, Candidates: renderCandidates(candidates), Trajectory: trajectory, Prompts: evalreport.Label(prompts.String(), 6000), RiskSignals: evalreport.Label(risk, 5000), Counters: fmt.Sprintf("AI轮次%d · 收集材料%d · 正文%d字 · AI批注%d · 修订%d", assistantTurns, len(materials), evalreport.CountWords(body), comments, len(p.RevisionCheckpoints))},
	}
	// GeneratedAt changes per attempt; it is excluded from the frozen hash.
	hashInput := input
	hashInput.GeneratedAt = ""
	b, err := json.Marshal(hashInput)
	if err != nil {
		return AdaptedInput{}, err
	}
	h := sha256.Sum256(b)
	return AdaptedInput{Input: input, Hash: hex.EncodeToString(h[:]), Warnings: warnings}, nil
}

func adaptMaterials(p PersonaExport, started time.Time, candidates *[]evalreport.Candidate) []evalreport.MaterialEntry {
	out, seen := make([]evalreport.MaterialEntry, 0, len(p.Materials)), map[string]bool{}
	for i, m := range p.Materials {
		key := strings.TrimSpace(m.Title)
		if key == "" {
			continue
		}
		seen[key] = true
		id := fmt.Sprintf("material:export:%03d", i+1)
		*candidates = append(*candidates, evalreport.Candidate{ID: id, Kind: evalreport.KindReference, Label: evalreport.Label(key, 60)})
		var text strings.Builder
		for _, b := range m.Blocks {
			text.WriteString(b.Text)
			text.WriteByte(' ')
		}
		out = append(out, evalreport.MaterialEntry{MaterialID: id, AddedAt: stamp(m.At, started), Source: key, URL: m.SourceURL, FinalStatus: m.Source, Comment: strings.TrimSpace(text.String())})
	}
	for _, l := range p.Outputs.Library {
		if l.Title == "" || seen[l.Title] {
			continue
		}
		id := fmt.Sprintf("material:export:%03d", len(out)+1)
		seen[l.Title] = true
		*candidates = append(*candidates, evalreport.Candidate{ID: id, Kind: evalreport.KindReference, Label: evalreport.Label(l.Title, 60)})
		out = append(out, evalreport.MaterialEntry{MaterialID: id, AddedAt: stamp(started, started), Source: l.Title, FinalStatus: l.Decision, Comment: l.Classification, CannotSupport: l.EvidenceFinding})
	}
	return out
}

func buildTrajectory(p PersonaExport, materials []evalreport.MaterialEntry, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "〔立题框架〕\n目标：%s\n缘由：%s\n活动与时间：%s\n资源：%s\n可能的反例：%s\n\n", p.Outputs.Proposal.Objective, p.Outputs.Proposal.Reason, p.Outputs.Proposal.Activities, p.Outputs.Proposal.Resources, p.Outputs.Proposal.Counterpoints)
	if refl := reflectionText(p); refl != "" {
		fmt.Fprintf(&b, "〔学生自写反思〕\n%s\n\n", evalreport.Label(refl, 900))
	}
	for _, m := range materials {
		fmt.Fprintf(&b, "〔来源〕%s｜%s｜%s\n", m.Source, m.FinalStatus, evalreport.Label(m.CannotSupport, 100))
	}
	if strings.TrimSpace(body) != "" {
		fmt.Fprintf(&b, "\n〔正文节选〕\n%s\n", evalreport.Label(strings.TrimSpace(body), 1800))
	}
	return evalreport.Label(b.String(), 9000)
}

func latestEssay(o personaOutputs) string {
	seq, body := -1, ""
	for _, s := range o.Snapshots {
		if s.DocKind == "essay" && s.Seq > seq {
			seq, body = s.Seq, s.Content
		}
	}
	for _, b := range o.Buffers {
		if b.DocKind == "essay" && strings.TrimSpace(b.Content) != "" {
			body = b.Content
		}
	}
	return body
}
func reflectionText(p PersonaExport) string {
	if len(p.Outputs.Reflection.Answers) > 0 {
		return strings.Join(p.Outputs.Reflection.Answers, "\n")
	}
	for _, n := range p.ProcessGraphNodes {
		if n.Type == "reflection" {
			if v, _ := n.Body["text"].(string); v != "" {
				return v
			}
		}
	}
	return ""
}
func countComments(p PersonaExport) int {
	n := 0
	for _, i := range p.Outputs.Interventions {
		if i.Type == "proposal_annotation" || i.Type == "essay_annotation" || i.Type == "review_item" {
			n++
		}
	}
	return n
}
func eventKind(t string) (string, bool) {
	switch {
	case strings.Contains(t, "source") || strings.Contains(t, "reading"):
		return "reading", true
	case strings.Contains(t, "review") || strings.Contains(t, "card"):
		return "review", true
	case strings.Contains(t, "write") || strings.Contains(t, "draft") || strings.Contains(t, "revision"):
		return "writing", true
	case strings.Contains(t, "plan") || strings.Contains(t, "finish") || strings.Contains(t, "milestone"):
		return "milestone", true
	case strings.Contains(t, "graph") || strings.Contains(t, "explor"):
		return "graph", true
	case strings.Contains(t, "coach") || strings.Contains(t, "chat"):
		return "chat", true
	default:
		return "", false
	}
}
func eventSummary(a personaAction) string {
	if len(a.Payload) == 0 {
		return a.Type
	}
	b, err := json.Marshal(a.Payload)
	if err != nil {
		return a.Type
	}
	return evalreport.Label(a.Type+"："+string(b), 120)
}
func stamp(v, fallback time.Time) string {
	if v.IsZero() {
		v = fallback
	}
	return v.UTC().Format(time.RFC3339)
}
func ptr(s string) *string { return &s }
func earliest(ch []personaChat, ac []personaAction) time.Time {
	var out time.Time
	for _, m := range ch {
		if !m.At.IsZero() && (out.IsZero() || m.At.Before(out)) {
			out = m.At
		}
	}
	for _, a := range ac {
		if !a.At.IsZero() && (out.IsZero() || a.At.Before(out)) {
			out = a.At
		}
	}
	return out
}
func renderCandidates(c []evalreport.Candidate) string {
	cp := append([]evalreport.Candidate(nil), c...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].ID < cp[j].ID })
	var b strings.Builder
	for i, v := range cp {
		if i >= 140 {
			break
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", v.ID, v.Kind, v.Label)
	}
	return b.String()
}
