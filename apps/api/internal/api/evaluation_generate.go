package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// evaluation_generate.go — the real evaluation-report generator (replaces
// evalreport.Placeholder). Two halves:
//   - assembleFacts: deterministic FACT fields (counters, milestones, timeline,
//     materials, toolUsage) computed straight from the recorded data.
//   - four flagship LLM calls (agent/reportgen.go), partitioned by data-context,
//     for the judgments + prose; grounded in the evidence-candidate index and
//     post-validated by evalreport.ValidateRefs.
//
// Best-effort by construction: a failed/absent LLM call degrades that section
// (empty / conservative floor) rather than failing the whole report, so a
// report always stores. The FACT half never needs the model.

// reportCallStat is one LLM call's benchmark record.
type reportCallStat struct {
	Name         string `json:"name"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	Millis       int64  `json:"millis"`
	OK           bool   `json:"ok"`
	Err          string `json:"err,omitempty"`
}

// reportGenStats is the per-generation benchmark payload (per-call usage +
// timing + the ref-drop rate). The prod path discards it.
type reportGenStats struct {
	Calls       []reportCallStat `json:"calls"`
	RefsTotal   int              `json:"refsTotal"`
	RefsDropped int              `json:"refsDropped"`
	Candidates  int              `json:"candidates"`
	mu          *sync.Mutex      // guards concurrent Calls appends (pointer → safe to copy on return; unexported → not marshaled)
}

const trajectoryRuneBudget = 9000
const draftExcerptRunes = 1800
const maxRenderedCandidates = 140

// runReportGeneration gathers a project's recorded data, computes the FACT half,
// runs the four LLM calls, and assembles + validates the Report. Returns the
// report and per-call stats. Only a data-gather failure returns an error;
// LLM-call failures degrade sections and are recorded in stats.
func (a *API) runReportGeneration(ctx context.Context, projectID uuid.UUID, reportID, studentName string) (evalreport.Report, reportGenStats, error) {
	var stats reportGenStats
	stats.mu = &sync.Mutex{}
	q := a.d.Queries
	pg := pgtype.UUID{Bytes: projectID, Valid: true}

	p, err := q.GetProject(ctx, projectID)
	if err != nil {
		return evalreport.Report{}, stats, err
	}
	proposal, _ := q.GetProjectProposal(ctx, projectID)
	refs, _ := q.ListReferences(ctx, projectID)
	cards, _ := q.ListCardInstancesByProject(ctx, pg)
	msgs, _ := q.ListChatMessagesByProject(ctx, pg)
	events, _ := q.ListEventsByProject(ctx, pg)
	leads, _ := q.ListExplorationLeads(ctx, projectID)
	citations, _ := q.ListCitationsByProject(ctx, projectID)
	checkpoints, _ := q.ListRevisionCheckpoints(ctx, projectID)
	plans, _ := q.ListPlanItems(ctx, projectID)

	// Finished body text (essay, falling back to proposal / edit buffer).
	body := a.finishedBody(ctx, projectID)

	// Title = research question, else project title.
	title := strings.TrimSpace(p.Title)
	if obj := strings.TrimSpace(proposal.Objective); obj != "" {
		title = obj
	}

	// Evidence index (the only citable ids) + rendered candidate list.
	idx, ierr := BuildEvidenceIndex(ctx, q, projectID)
	if ierr != nil {
		idx = evalreport.NewEvidenceIndex(nil)
	}
	stats.Candidates = idx.Len()
	candidates := renderCandidates(idx)

	genCtx := agent.ReportGenContext{
		Title:       title,
		Candidates:  candidates,
		Trajectory:  buildTrajectoryDigest(proposal, plans, refs, cards, events, body),
		Prompts:     buildPromptsDigest(msgs, events),
		RiskSignals: buildRiskSignalsDigest(body, refs, citations, leads),
		Counters:    "",
	}

	// FACT half — deterministic.
	counters := evalreport.Counters{
		AITurns:        countAssistantTurns(msgs),
		MaterialsRead:  len(refs),
		WordsWritten:   evalreport.CountWords(body),
		AICommentCount: intFrom(q.CountAnnotationsByProject(ctx, projectID)),
		EditCount:      len(checkpoints),
	}
	genCtx.Counters = fmt.Sprintf("AI轮次%d · 收集材料%d · 正文%d字 · AI批注%d · 修订%d",
		counters.AITurns, counters.MaterialsRead, counters.WordsWritten, counters.AICommentCount, counters.EditCount)

	milestones := a.assembleMilestones(ctx, projectID, p, events)

	// LLM half — four calls, each metered + timed.
	resolved, ok := a.resolveEval(ctx)
	var rubricRes agent.RubricResult
	var promptLens evalreport.PromptLens
	var risks []evalreport.RiskEntry
	var abstract evalreport.Abstract
	if ok && a.d.Provider != nil {
		// A (promptLens), B (risks), C (rubric) are context-independent — run them
		// concurrently. D (abstract) synthesises C's axis results, so it waits.
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); promptLens = a.timedPromptLens(ctx, projectID, resolved, genCtx, &stats) }()
		go func() { defer wg.Done(); risks = a.timedRisks(ctx, projectID, resolved, genCtx, &stats) }()
		go func() { defer wg.Done(); rubricRes = a.timedRubric(ctx, projectID, resolved, genCtx, &stats) }()
		wg.Wait()
		abstract = a.timedAbstract(ctx, projectID, resolved, genCtx, rubricRes, &stats)
	} else {
		stats.Calls = append(stats.Calls, reportCallStat{Name: "resolver", OK: false, Err: "no eval resolver / provider"})
	}

	rep := evalreport.Report{
		Version:   1,
		ReportID:  reportID,
		ProjectID: projectID.String(),
		Basics: evalreport.Basics{
			// Type (project qualification) is intentionally left empty until
			// multi-type support lands — it is not shown anywhere in the report.
			Title:     title,
			StartDate: p.CreatedAt.UTC().Format(time.RFC3339),
			// EndDate = the project-finished milestone (the report is generated at
			// finish, so this is normally set); nil → header reads "进行中".
			EndDate:    milestones.ProjectFinished,
			Milestones: milestones, Counters: counters,
		},
		Abstract:    abstract,
		Events:      a.assembleTimeline(events),
		Materials:   assembleMaterials(refs, citations, leads),
		Depth:       fillDepth(rubricRes.Depth),
		Autonomy:    fillAutonomy(rubricRes.Autonomy),
		PromptLens:  promptLens,
		ToolUsage:   assembleToolUsage(cards),
		Risks:       risks,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	rep.Student.ID = p.UserID.String()
	rep.Student.Name = studentName

	drop := evalreport.ValidateRefs(&rep, idx)
	stats.RefsTotal, stats.RefsDropped = drop.Total, drop.Dropped
	return rep, stats, nil
}

// --- timed LLM-call wrappers (meter + record stat) ---

func (a *API) timedRubric(ctx context.Context, pid uuid.UUID, r gateway.Resolved, in agent.ReportGenContext, s *reportGenStats) agent.RubricResult {
	t0 := time.Now()
	res, usage, err := agent.GenerateRubric(ctx, a.d.Provider, r, in)
	a.recordReportCall(ctx, pid, r, "eval_report_rubric", usage, t0, err, s)
	return res
}

func (a *API) timedPromptLens(ctx context.Context, pid uuid.UUID, r gateway.Resolved, in agent.ReportGenContext, s *reportGenStats) evalreport.PromptLens {
	t0 := time.Now()
	res, usage, err := agent.GeneratePromptLens(ctx, a.d.Provider, r, in)
	a.recordReportCall(ctx, pid, r, "eval_report_promptlens", usage, t0, err, s)
	return res
}

func (a *API) timedRisks(ctx context.Context, pid uuid.UUID, r gateway.Resolved, in agent.ReportGenContext, s *reportGenStats) []evalreport.RiskEntry {
	t0 := time.Now()
	res, usage, err := agent.GenerateRisks(ctx, a.d.Provider, r, in)
	a.recordReportCall(ctx, pid, r, "eval_report_risks", usage, t0, err, s)
	return res
}

func (a *API) timedAbstract(ctx context.Context, pid uuid.UUID, r gateway.Resolved, in agent.ReportGenContext, axes agent.RubricResult, s *reportGenStats) evalreport.Abstract {
	t0 := time.Now()
	res, usage, err := agent.GenerateAbstract(ctx, a.d.Provider, r, in, axes)
	a.recordReportCall(ctx, pid, r, "eval_report_abstract", usage, t0, err, s)
	return res
}

func (a *API) recordReportCall(ctx context.Context, pid uuid.UUID, r gateway.Resolved, purpose string, usage gateway.ChatUsage, t0 time.Time, err error, s *reportGenStats) {
	a.meterCall(ctx, pid, r, purpose, usage)
	st := reportCallStat{
		Name: purpose, InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		Millis: time.Since(t0).Milliseconds(), OK: err == nil,
	}
	if err != nil {
		st.Err = err.Error()
	}
	s.mu.Lock()
	s.Calls = append(s.Calls, st)
	s.mu.Unlock()
}

// --- deterministic FACT assembly ---

func (a *API) finishedBody(ctx context.Context, projectID uuid.UUID) string {
	if snap, err := a.d.Queries.GetLatestSnapshot(ctx, sqlc.GetLatestSnapshotParams{ProjectID: projectID, DocKind: "essay"}); err == nil && strings.TrimSpace(snap.Content) != "" {
		return snap.Content
	}
	if buf, err := a.d.Queries.GetEditBuffer(ctx, sqlc.GetEditBufferParams{ProjectID: projectID, DocKind: "essay"}); err == nil && strings.TrimSpace(buf) != "" {
		return buf
	}
	if snap, err := a.d.Queries.GetLatestSnapshot(ctx, sqlc.GetLatestSnapshotParams{ProjectID: projectID, DocKind: "proposal"}); err == nil {
		return snap.Content
	}
	return ""
}

func (a *API) assembleMilestones(ctx context.Context, projectID uuid.UUID, p sqlc.Project, events []sqlc.Event) evalreport.Milestones {
	m := evalreport.Milestones{}
	started := p.CreatedAt.UTC().Format(time.RFC3339)
	m.Started = &started
	if ts, err := a.d.Queries.GetFrameworkFinishedAt(ctx, pgtype.UUID{Bytes: projectID, Valid: true}); err == nil && !ts.IsZero() {
		s := ts.UTC().Format(time.RFC3339)
		m.FrameworkFinished = &s
	}
	if ts, err := a.d.Queries.GetWritingFinish(ctx, sqlc.GetWritingFinishParams{ProjectID: projectID, DocKind: "proposal"}); err == nil {
		s := ts.UTC().Format(time.RFC3339)
		m.ProposalFinished = &s
	}
	if ts, err := a.d.Queries.GetWritingFinish(ctx, sqlc.GetWritingFinishParams{ProjectID: projectID, DocKind: "essay"}); err == nil {
		s := ts.UTC().Format(time.RFC3339)
		m.WritingFinished = &s
	}
	// Project finished = the latest project_finished event, if any.
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "project_finished" {
			s := events[i].CreatedAt.UTC().Format(time.RFC3339)
			m.ProjectFinished = &s
			break
		}
	}
	return m
}

// assembleTimeline maps the notable events into a compact reader-facing
// timeline (milestones, source adds, card completions, reviews, finishes).
func (a *API) assembleTimeline(events []sqlc.Event) []evalreport.EventEntry {
	kindOf := map[string]string{
		"milestone:framework_finished": "milestone", "project_finished": "milestone",
		"source_added": "reading", "source_opened": "reading", "reading_focus": "reading",
		"card_completed": "review", "review_ordered": "review", "spot_check_ordered": "review",
		"annotation_opened": "review", "version_saved": "writing", "revision_checkpoint": "writing",
		"reflection_written": "writing", "coach_turn": "chat",
	}
	summaryOf := map[string]string{
		"milestone:framework_finished": "立题完成，生成了项目计划", "project_finished": "完成项目，进入过程评估",
		"source_added": "新增了一份来源", "reading_focus": "在阅读中标注了关注点",
		"card_completed": "完成了一张思维工具卡", "review_ordered": "请印记体检了整稿",
		"annotation_opened": "查看了印记的批注", "reflection_written": "写下了自己的反思",
	}
	out := make([]evalreport.EventEntry, 0)
	for _, e := range events {
		kind, ok := kindOf[e.Type]
		if !ok {
			continue // skip low-signal events from the reader timeline
		}
		summary := summaryOf[e.Type]
		if summary == "" {
			summary = e.Type
		}
		out = append(out, evalreport.EventEntry{
			TS: e.CreatedAt.UTC().Format(time.RFC3339), Kind: kind, Summary: summary,
			Ref: &evalreport.Ref{ID: e.ID.String()},
		})
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func assembleMaterials(refs []sqlc.Reference, citations []sqlc.Citation, leads []sqlc.ExplorationLead) []evalreport.MaterialEntry {
	// reference id -> first citation section (claim-level usedIn), else the
	// exploration lead it hangs under (node-level).
	citeSection := map[string]string{}
	for _, c := range citations {
		if _, seen := citeSection[c.ReferenceID.String()]; !seen {
			citeSection[c.ReferenceID.String()] = c.Section
		}
	}
	leadOf := map[string]string{}
	for _, l := range leads {
		if l.ConnectedReferenceID.Valid {
			leadOf[uuidStr(l.ConnectedReferenceID)] = l.Text
		}
	}
	out := make([]evalreport.MaterialEntry, 0, len(refs))
	for _, r := range refs {
		var usedIn *evalreport.Ref
		if sec, ok := citeSection[r.ID.String()]; ok {
			usedIn = &evalreport.Ref{ID: "citation:" + r.ID.String(), Label: sec}
		} else if lead, ok := leadOf[r.ID.String()]; ok {
			usedIn = &evalreport.Ref{Label: evalreport.Label(lead, 40)}
		}
		var url *string
		if strings.TrimSpace(r.Url) != "" {
			u := r.Url
			url = &u
		}
		status := "收集"
		if r.Decision != nil && *r.Decision != "" {
			status = *r.Decision
		}
		out = append(out, evalreport.MaterialEntry{
			MaterialID: r.ID.String(), AddedAt: r.CreatedAt.UTC().Format(time.RFC3339),
			Source: r.Title, URL: url, UsedIn: usedIn, FinalStatus: status,
			Comment: strings.TrimSpace(r.ReadingNote), CannotSupport: strings.TrimSpace(r.EvidenceFinding),
		})
	}
	return out
}

func assembleToolUsage(cards []sqlc.CardInstance) []evalreport.ToolUsageEntry {
	out := make([]evalreport.ToolUsageEntry, 0, len(cards))
	for _, c := range cards {
		if c.Status == "skipped" {
			continue
		}
		out = append(out, evalreport.ToolUsageEntry{
			ToolID: c.CardID, Name: c.CardID, Stage: "", Purpose: c.Status,
			Summary: fmt.Sprintf("状态：%s", c.Status),
		})
	}
	return out
}

// fillDepth / fillAutonomy backfill any missing dimension to a conservative
// floor so the report always carries the full 6+6 even if the model dropped one.
func fillDepth(got []evalreport.DepthDimResult) []evalreport.DepthDimResult {
	byID := map[string]evalreport.DepthDimResult{}
	for _, d := range got {
		byID[d.ID] = d
	}
	out := make([]evalreport.DepthDimResult, 0, 6)
	for _, id := range []string{"D1", "D2", "D3", "D4", "D5", "D6"} {
		if d, ok := byID[id]; ok {
			out = append(out, d)
		} else {
			out = append(out, evalreport.DepthDimResult{ID: id, Level: 1, Summary: "暂无足够证据判定这一维。", Suggestion: ""})
		}
	}
	return out
}

func fillAutonomy(got []evalreport.AutonomyDimResult) []evalreport.AutonomyDimResult {
	byID := map[string]evalreport.AutonomyDimResult{}
	for _, a := range got {
		byID[a.ID] = a
	}
	out := make([]evalreport.AutonomyDimResult, 0, 6)
	for _, id := range []string{"A1", "A2", "A3", "A4", "A5", "A6"} {
		if a, ok := byID[id]; ok {
			out = append(out, a)
		} else {
			out = append(out, evalreport.AutonomyDimResult{ID: id, Band: 0, Summary: "暂无足够证据判定这一维。", Suggestion: ""})
		}
	}
	return out
}

// --- digests + candidate rendering ---

func renderCandidates(idx evalreport.EvidenceIndex) string {
	var b strings.Builder
	n := 0
	for _, c := range idx.Candidates() {
		fmt.Fprintf(&b, "[%s] %s: %s\n", c.ID, c.Kind, c.Label)
		if n++; n >= maxRenderedCandidates {
			break
		}
	}
	return b.String()
}

func buildTrajectoryDigest(prop sqlc.ProjectProposal, plans []sqlc.PlanItem, refs []sqlc.Reference, cards []sqlc.CardInstance, events []sqlc.Event, body string) string {
	var b strings.Builder
	b.WriteString("〔立题框架〕\n")
	fmt.Fprintf(&b, "目标：%s\n缘由：%s\n活动与时间：%s\n资源：%s\n可能的反例：%s\n\n",
		prop.Objective, prop.Reason, prop.Activities, prop.Resources, prop.Counterpoints)
	// The student's own reflection is D6's only valid evidence and usually sits
	// at the END of the draft — surface it up-front so it survives the digest's
	// tail truncation (the draft excerpt below is capped and could bury it).
	if refl := extractReflection(body); refl != "" {
		fmt.Fprintf(&b, "〔学生自写反思〕\n%s\n\n", evalreport.Label(refl, 900))
	}
	if len(plans) > 0 {
		b.WriteString("〔计划阶段〕")
		for _, p := range plans {
			fmt.Fprintf(&b, "%s(%s) ", p.Title, p.Tag)
		}
		b.WriteString("\n\n")
	}
	if len(refs) > 0 {
		b.WriteString("〔来源与阅读〕\n")
		for _, r := range refs {
			cred := ""
			if r.Credibility != nil {
				cred = *r.Credibility
			}
			fmt.Fprintf(&b, "· %s [%s] 归纳:%s 证据:%s\n", strings.TrimSpace(r.Title), cred,
				evalreport.Label(strings.TrimSpace(string(r.Takeaway)), 80), evalreport.Label(strings.TrimSpace(r.EvidenceFinding), 80))
		}
		b.WriteString("\n")
	}
	if len(cards) > 0 {
		b.WriteString("〔思维工具卡〕")
		for _, c := range cards {
			fmt.Fprintf(&b, "%s(%s) ", c.CardID, c.Status)
		}
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(body) != "" {
		fmt.Fprintf(&b, "〔正文节选〕\n%s\n", evalreport.Label(strings.TrimSpace(body), draftExcerptRunes))
	}
	return evalreport.Label(b.String(), trajectoryRuneBudget)
}

func buildPromptsDigest(msgs []sqlc.ChatMessage, events []sqlc.Event) string {
	var b strings.Builder
	for _, m := range msgs {
		if m.Role != "user" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		stage := ""
		if m.Stage != nil {
			stage = *m.Stage
		}
		fmt.Fprintf(&b, "[%s] (%s) %s\n", m.ID.String(), stage, evalreport.Label(content, 200))
	}
	for _, e := range events {
		if e.Type != "reading_focus" {
			continue
		}
		var payload struct {
			StudentText string `json:"student_text"`
		}
		_ = json.Unmarshal(e.Payload, &payload)
		if strings.TrimSpace(payload.StudentText) != "" {
			fmt.Fprintf(&b, "[%s] (阅读) %s\n", e.ID.String(), evalreport.Label(payload.StudentText, 200))
		}
	}
	return evalreport.Label(b.String(), 6000)
}

func buildRiskSignalsDigest(body string, refs []sqlc.Reference, citations []sqlc.Citation, leads []sqlc.ExplorationLead) string {
	var b strings.Builder
	fmt.Fprintf(&b, "〔正文〕\n%s\n\n", evalreport.Label(strings.TrimSpace(body), draftExcerptRunes))
	b.WriteString("〔来源溯源状态〕\n")
	for _, r := range refs {
		has := "有链接"
		if strings.TrimSpace(r.Url) == "" {
			has = "无链接/未溯源"
		}
		fmt.Fprintf(&b, "· %s [%s]\n", strings.TrimSpace(r.Title), has)
	}
	fmt.Fprintf(&b, "\n〔正文引用了 %d 处来源；探索线索 %d 条〕\n", len(citations), len(leads))
	return evalreport.Label(b.String(), 5000)
}

// extractReflection returns the student's self-written reflection section from
// the body (the D6 signal), found by a 反思 / Reflection heading. Returns "" when
// there is no such section — D6 then stays NA rather than penalised.
func extractReflection(body string) string {
	markers := []string{"## 反思", "# 反思", "反思\n", "## Reflection", "# Reflection", "Reflection\n"}
	lower := body
	for _, m := range markers {
		if i := strings.Index(lower, m); i >= 0 {
			tail := strings.TrimSpace(body[i+len(m):])
			if tail != "" {
				return tail
			}
		}
	}
	return ""
}

func countAssistantTurns(msgs []sqlc.ChatMessage) int {
	n := 0
	for _, m := range msgs {
		if m.Role == "assistant" {
			n++
		}
	}
	return n
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func intFrom(v int64, _ error) int { return int(v) }
