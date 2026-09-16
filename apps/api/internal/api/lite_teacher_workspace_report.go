package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteworkspace"
)

// lite_teacher_workspace_report.go — the parent report surface of the teacher
// workspace (§5.3, §12.6, D3): a teacher revises one section of a parent
// report by talking with AI. The loop, metering and §6 checks that every
// surface shares live in lite_teacher_workspace.go.
//
// revise_section never writes the database. It returns the section as a patch
// ({"body": {"<section>": "<text>"}}), and the editor saves it through the
// report's own PATCH, in the same queue as her typing. Before a section goes
// into the patch it passes agent.CheckLiteParentSections, the function the
// draft itself is checked with.
//
// 🚨 Like every other workspace file, nothing here reads atom_message. The
// facts are the ones frozen on the report row, which never carry chat text.

// The parent report surface is the third liteWorkspaceSurface.
var _ liteWorkspaceSurface = (*liteWorkspaceReport)(nil)

// liteWorkspaceSubjectFromReport resolves the parentReport surface. It follows
// loadTeacherParentReport: load the report, then check the caller teaches its
// class. A malformed id, a missing report and a class she does not teach all
// answer 404. req.ClassID is ignored: the class comes from the report row, so
// the roster the §6 checks read is the report's class.
//
// It does not require the student to still be enrolled; build does, so the
// refusal carries the same code as a refused redraft.
func liteWorkspaceSubjectFromReport(a *API, ctx context.Context, req liteWorkspaceTurnRequest) (liteWorkspaceSubject, error) {
	rid, err := uuid.Parse(strings.TrimSpace(req.ReportID))
	if err != nil {
		return liteWorkspaceSubject{}, httpx.ErrNotFound("资源不存在")
	}
	rep, err := a.d.Queries.GetLiteParentReport(ctx, rid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return liteWorkspaceSubject{}, httpx.ErrNotFound("资源不存在")
		}
		return liteWorkspaceSubject{}, err
	}
	cls, err := a.assertTeacherOwnsClass(ctx, rep.ClassID)
	if err != nil {
		return liteWorkspaceSubject{}, err
	}
	return liteWorkspaceSubject{class: cls, report: rep}, nil
}

// newLiteWorkspaceReport builds the parent report surface for one turn. It
// runs before the model is routed, so each refusal here costs no model call:
//
//   - she has left the class: 409 student_left, the answer redraft gives
//     (requireParentStudentEnrolled), for the same reason;
//   - the stored facts, hidden set or body cannot be read: the error as it is.
func (a *API) newLiteWorkspaceReport(in liteWorkspaceSurfaceInput) (liteWorkspaceSurface, error) {
	rep := in.subject.report
	q := a.d.Queries
	if err := requireParentStudentEnrolled(in.mctx, q, rep); err != nil {
		return nil, err
	}
	var frozen liteparent.Facts
	if err := json.Unmarshal(rep.Facts, &frozen); err != nil {
		return nil, err
	}
	hidden, err := liteParentHidden(rep.Hidden)
	if err != nil {
		return nil, err
	}
	facts := liteparent.VisibleFacts(frozen, hidden)
	others, err := a.loadLiteParentOtherNames(in.mctx, rep.ClassID, rep.UserID, facts.StudentName)
	if err != nil {
		return nil, err
	}
	stored, err := liteParentSections(rep.Body)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		// No body yet (the draft was rejected at generation): the draft is
		// what the editor would show, and it is NULL too in that case.
		if stored, err = liteParentSections(rep.Draft); err != nil {
			return nil, err
		}
	}
	sections := liteparent.SectionsWithFacts(facts)
	current := liteWorkspaceReportCurrent(in.req.Artifact, stored, sections)

	className := facts.ClassName
	if className == "" {
		className = in.subject.class.Name
	}
	return &liteWorkspaceReport{
		facts:     facts,
		others:    others,
		sections:  sections,
		current:   current,
		className: className,
		names:     liteWorkspaceReportNames(facts, in.roster, rep.UserID),
	}, nil
}

// liteWorkspaceReportArtifact is the canvas the client sends: the body as the
// editor shows it now, unsaved typing included.
type liteWorkspaceReportArtifact struct {
	Body map[string]string `json:"body"`
}

// liteWorkspaceReportCurrent is the text of each visible section as the
// teacher sees it: the artifact's body where it has the section, the stored
// body otherwise. The artifact is client-supplied and only goes into the
// prompt, so each section is capped at the editor's own limit rather than
// trusted for its size.
func liteWorkspaceReportCurrent(raw json.RawMessage, stored map[string]string, sections []string) map[string]string {
	var art liteWorkspaceReportArtifact
	if len(raw) > 0 {
		// A malformed artifact is read as no artifact: the stored body is a
		// correct, if older, view of the same report.
		_ = json.Unmarshal(raw, &art)
	}
	out := make(map[string]string, len(sections))
	for _, k := range sections {
		text, ok := art.Body[k]
		if !ok {
			text = stored[k]
		}
		out[k] = liteWorkspaceClampRunes(text, liteParentBodySectionMax)
	}
	return out
}

// liteWorkspaceReportNames is the name evidence every turn of this surface
// starts with. The report is about her, so her name (as frozen on the report,
// and as the roster shows it now) is always grounded. A classmate's name is
// grounded only when it sits inside one of her own titles, quotes or keywords:
// CheckLiteParentSections accepts it there, and without this the shared name
// check would fail a section that quotes her verbatim.
func liteWorkspaceReportNames(f liteparent.Facts, roster []liteworkspace.Student, userID uuid.UUID) []string {
	var out []string
	if f.StudentName != "" {
		out = append(out, f.StudentName)
	}
	corpus := liteparent.Corpus(f) + "\n" + liteparent.TitleCorpus(f)
	for _, s := range roster {
		if s.Name == "" {
			continue
		}
		if s.ID == userID.String() || strings.Contains(corpus, s.Name) {
			out = append(out, s.Name)
		}
	}
	return out
}

// liteWorkspaceReport is the parent report surface for one turn.
type liteWorkspaceReport struct {
	// facts are the visible facts: the frozen facts less what she has hidden.
	// A hidden quote is not in the prompt and fails the quote check.
	facts     liteparent.Facts
	others    []string
	sections  []string
	current   map[string]string
	className string
	names     []string

	// revised is what revise_section accepted this turn, by section key.
	revised map[string]string
	// quoted is every 「」/“”/《》 span inside an accepted section. Each one
	// passed the quote or title check, so it is her words or her work's
	// title; see verbatimQuotedSpans.
	quoted []string

	question string
	choices  []liteworkspace.Choice
	asked    bool
}

// begin: nothing in the request is applied before the model runs.
func (run *liteWorkspaceReport) begin(liteWorkspaceTurnRequest) string { return "" }

func (run *liteWorkspaceReport) system() string {
	return liteworkspace.ReportSystem(liteworkspace.ReportSystemContext{
		TodayBeijing: time.Now().In(liteworkspace.BeijingOffset).Format("2006-01-02"),
		ClassName:    run.className,
		StudentName:  run.facts.StudentName,
		Canvas:       run.canvas(),
	})
}

// canvas is the report as the model reads it: each visible section under its
// Chinese heading, then the facts. No section key appears here (§12.1): the
// model is told the keys only in revise_section's schema, and the tool also
// accepts a heading.
func (run *liteWorkspaceReport) canvas() string {
	var b strings.Builder
	b.WriteString("## 报告现在的内容\n")
	for _, k := range run.sections {
		fmt.Fprintf(&b, "\n### %s\n", liteparent.SectionLabels[k])
		if text := strings.TrimSpace(run.current[k]); text != "" {
			b.WriteString(text)
		} else {
			b.WriteString("（空）")
		}
		b.WriteString("\n")
	}
	// FactsText is the text the draft was written from, verbatim: titles
	// already sit in 《》 and her words in 「」.
	b.WriteString("\n## 可用的事实\n\n")
	b.WriteString(liteparent.FactsText(run.facts))
	return b.String()
}

func (run *liteWorkspaceReport) tools() []gateway.ChatTool { return liteworkspace.ReportTools() }

// ended is ask_choice's: it ends the turn, and the question is the reply.
func (run *liteWorkspaceReport) ended() (string, []liteworkspace.Choice, bool) {
	return run.question, run.choices, run.asked
}

// extraParts is nil: a revised section is a patch value, which the handler
// checks itself.
func (run *liteWorkspaceReport) extraParts() []string { return nil }

func (run *liteWorkspaceReport) groundedNames() []string { return run.names }

// groundedCounts is 1 and nothing else. The facts carry no count of people,
// and the report is about one student, so 「一名学生」 or 「作为一位读者」 about
// her is not a claim about the class. Any other head count fails the turn.
// Digits in a section are checked against the facts by
// CheckLiteParentSections, which is stricter than this.
func (run *liteWorkspaceReport) groundedCounts() []int { return []int{1} }

func (run *liteWorkspaceReport) result() (map[string]any, []liteWorkspaceCardDTO) {
	if len(run.revised) == 0 {
		return nil, nil
	}
	return map[string]any{"body": run.revised}, nil
}

// navigate is nil: the parent report surface has no open_page tool.
func (run *liteWorkspaceReport) navigate() *liteWorkspaceNavigateDTO { return nil }

// verbatimQuotedSpans is every title and quote in the visible facts, plus
// every quoted span inside a section this turn accepted. A title such as
// 《3人小组实验》 or a quote such as 「我们3人一组」 is her work or her words,
// not a head count; the shared check blanks it only where it is written
// quoted. The accepted spans cover a quoted fragment of a longer quote, which
// the quote check accepts.
func (run *liteWorkspaceReport) verbatimQuotedSpans() []string {
	out := append([]string{}, run.quoted...)
	for _, items := range [][]liteparent.Item{run.facts.Readings, run.facts.Writings, run.facts.Projects} {
		for _, it := range items {
			out = append(out, it.Title)
		}
	}
	for _, m := range run.facts.Moments {
		out = append(out, m.Quote)
	}
	for _, k := range run.facts.Keywords {
		out = append(out, k.Text)
	}
	return out
}

// verbatimClassName is the class name as frozen on the report, which is the
// one the model reads in the facts.
func (run *liteWorkspaceReport) verbatimClassName() string { return run.className }

// execute runs one tool call and returns the tool result the model reads next.
func (run *liteWorkspaceReport) execute(tc gateway.ToolCall) string {
	switch tc.Name {
	case "revise_section":
		return run.reviseSection(tc.Args)
	case "ask_choice":
		question, choices, errMsg := liteWorkspaceAskChoiceArgs(tc.Args, false)
		if errMsg != "" {
			return liteWorkspaceToolError(errMsg)
		}
		run.question, run.choices, run.asked = question, choices, true
		return liteWorkspaceToolOK(map[string]any{"options": len(choices)})
	}
	return liteWorkspaceToolError("没有这个工具：" + tc.Name)
}

// reportSection reads revise_section's section argument: a key, or the
// Chinese heading of one. ok=false when it names neither.
func reportSection(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := liteparent.SectionLabels[s]; ok {
		return s, true
	}
	for k, label := range liteparent.SectionLabels {
		if strings.TrimSpace(raw) == label {
			return k, true
		}
	}
	return "", false
}

// reviseSection answers revise_section. A section this report does not show,
// an empty or over-long text, and a text that fails the draft's own checks
// are tool errors: the patch is left as it was and the model can retry within
// the loop bound. An accepted text replaces any earlier revision of the same
// section in this turn.
func (run *liteWorkspaceReport) reviseSection(args map[string]any) string {
	raw, _ := toolString(args, "section")
	key, known := reportSection(raw)
	if !known || !slices.Contains(run.sections, key) {
		return liteWorkspaceToolError("这份报告没有这个段落：" + raw + "。只能是：" + run.sectionList())
	}
	label := liteparent.SectionLabels[key]
	text, _ := toolString(args, "text")
	if text == "" {
		return liteWorkspaceToolError("没有给出「" + label + "」的正文")
	}
	if n := utf8.RuneCountInString(text); n > liteworkspace.ReviseSectionMaxRunes {
		return liteWorkspaceToolError(fmt.Sprintf("「%s」有 %d 字，超过 %d 字的上限，请缩短后重写",
			label, n, liteworkspace.ReviseSectionMaxRunes))
	}
	if err := agent.CheckLiteParentSections(map[string]string{key: text}, []string{key}, run.facts, run.others); err != nil {
		return liteWorkspaceToolError("「" + label + "」没有通过检查（" + err.Error() + "）。" +
			"引文和作品标题必须逐字出自事实，数字只用事实里的阿拉伯数字，不写其他学生的名字。请改好后重新调用 revise_section。")
	}
	if run.revised == nil {
		run.revised = map[string]string{}
	}
	run.revised[key] = text
	run.quoted = append(run.quoted, liteWorkspaceReportSpans(text)...)
	return liteWorkspaceToolOK(map[string]any{"section": label})
}

// sectionList is this report's sections for a tool error, key and heading.
func (run *liteWorkspaceReport) sectionList() string {
	parts := make([]string, 0, len(run.sections))
	for _, k := range run.sections {
		parts = append(parts, k+"（"+liteparent.SectionLabels[k]+"）")
	}
	return strings.Join(parts, "、")
}

// liteWorkspaceReportSpans returns the text inside every closed 「」, “” and
// 《》 span of s, the three pairs CheckProse verifies. It is called only on a
// text that passed CheckProse, so every span is closed and verified.
func liteWorkspaceReportSpans(s string) []string {
	closeOf := map[rune]rune{'「': '」', '“': '”', '《': '》'}
	runes := []rune(s)
	var out []string
	for i := 0; i < len(runes); i++ {
		closeCh, ok := closeOf[runes[i]]
		if !ok {
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != closeCh {
			j++
		}
		if j >= len(runes) {
			break
		}
		out = append(out, string(runes[i+1:j]))
		i = j
	}
	return out
}
