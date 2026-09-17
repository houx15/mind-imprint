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

	genderCol, err := q.GetUserGender(in.mctx, rep.UserID)
	if err != nil {
		return nil, err
	}

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
		gender:    liteworkspace.GenderOf(genderCol),
		typed:     in.typed,
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

// liteWorkspaceReportNames is her name as frozen on the report and as the
// roster shows it now (when she is on it and it differs). It is both the name
// evidence and the subject name the shared check blanks. No classmate is
// grounded here: a classmate's name inside one of her titles or quotes is
// handled by blanking that span where it is quoted (verbatimQuotedSpans), so
// an unquoted mention of the classmate still fails.
func liteWorkspaceReportNames(f liteparent.Facts, roster []liteworkspace.Student, userID uuid.UUID) []string {
	var out []string
	if f.StudentName != "" {
		out = append(out, f.StudentName)
	}
	for _, s := range roster {
		if s.ID == userID.String() && s.Name != "" && s.Name != f.StudentName {
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
	// gender is her users.gender as it is now (not frozen on the report):
	// the teacher may set it after the report was drafted.
	gender string
	// typed is what the teacher typed this turn.
	typed string

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
		Pronoun:      liteworkspace.Pronoun(run.gender),
		Sections:     run.sectionList(),
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

func (run *liteWorkspaceReport) clearEnded() {
	run.question, run.choices, run.asked = "", nil, false
}

// falseClaim: revise_section rewrites a section this report already has. It
// cannot add, remove or reorder sections, and nothing here opens a page.
func (run *liteWorkspaceReport) falseClaim(text string) string {
	if reason := liteWorkspaceOpenedPageClaim(text); reason != "" {
		return reason
	}
	if liteworkspace.OffersSectionChange(text) {
		return "报告的段落是固定的，不能新增或删除段落，只能改写这几段：" + run.sectionList()
	}
	if label := liteworkspace.NamesMissingSection(text, run.missingSectionLabels()); label != "" {
		return "这份报告没有「" + label + "」段落，只能改写这几段：" + run.sectionList()
	}
	if run.asked && len(run.revised) == 0 && liteworkspace.NamesSectionAndChange(run.typed, run.sectionLabels()) {
		return "老师已经说了改哪一段、怎么改，不要反问。请直接调用 revise_section 改写那一段；" +
			"事实里没有的内容不要写，在回复里说明哪一部分事实里没有"
	}
	return liteworkspace.PronounProblem(text, run.pronouns())
}

// pronouns is what the report may call her: the gender the teacher set, and
// any pronoun the teacher used this turn.
func (run *liteWorkspaceReport) pronouns() liteworkspace.PronounsAllowed {
	var p liteworkspace.PronounsAllowed
	p.Allow(run.gender)
	p.AllowTyped(run.typed)
	return p
}

// sectionLabels is the heading of every section this report shows.
func (run *liteWorkspaceReport) sectionLabels() []string {
	out := make([]string, 0, len(run.sections))
	for _, k := range run.sections {
		out = append(out, liteparent.SectionLabels[k])
	}
	return out
}

// missingSectionLabels is the heading of every section this report does not
// show.
func (run *liteWorkspaceReport) missingSectionLabels() []string {
	var out []string
	for _, k := range liteparent.SectionKeys {
		if !slices.Contains(run.sections, k) {
			out = append(out, liteparent.SectionLabels[k])
		}
	}
	return out
}

// extraParts is nil: a revised section is a patch value, which the handler
// checks itself.
func (run *liteWorkspaceReport) extraParts() []string { return nil }

func (run *liteWorkspaceReport) groundedNames() []string { return run.names }

// groundedCounts is 1 and nothing else. The facts carry no count of people,
// and the report is about one student, so 「一名学生」 or 「作为一位读者」 about
// her must not fail the turn.
//
// This is a trade-off, and it leaves a gap: 「只有一人交了作业」 also passes,
// although nothing counted anyone. The draft path has the same gap
// (checkChineseCounts skips 一 on its own), so the revision is no weaker than
// the draft. An Arabic 1 in a section still has to be a fact, because
// CheckLiteParentSections checks every digit.
func (run *liteWorkspaceReport) groundedCounts() []int { return []int{1} }

// groundsRosterSize is false: ReportSystem never states the class size, so a
// class size in the reply has nothing behind it.
func (run *liteWorkspaceReport) groundsRosterSize() bool { return false }

// blanksQuotedSpansForNames is true: a section may quote her title that
// contains a classmate's name, and CheckLiteParentSections accepts that. A
// span that is exactly a roster name is still checked (the handler drops it
// from the blanking).
func (run *liteWorkspaceReport) blanksQuotedSpansForNames() bool { return true }

// verbatimSubjectNames is her name (see liteWorkspaceReportNames).
func (run *liteWorkspaceReport) verbatimSubjectNames() []string { return run.names }

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
// not a head count, and a classmate's name inside 《李明推荐的雨水花园》 is not
// the reply naming 李明. The shared name and count checks blank these spans
// only where they are written quoted. The accepted spans cover a quoted
// fragment of a longer quote, which the quote check accepts.
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
	if !known {
		return liteWorkspaceToolError("没有这个段落：" + raw + "。只能是：" + run.sectionList())
	}
	label := liteparent.SectionLabels[key]
	if !slices.Contains(run.sections, key) {
		// A real key this report does not show is named by its heading: a
		// tool result is model input, and a key there reaches her reply.
		return liteWorkspaceToolError("这份报告没有「" + label + "」段落。只能是：" + run.sectionList())
	}
	text, _ := toolString(args, "text")
	if text == "" {
		return liteWorkspaceToolError("没有给出「" + label + "」的正文")
	}
	if n := utf8.RuneCountInString(text); n > liteworkspace.ReviseSectionMaxRunes {
		return liteWorkspaceToolError(fmt.Sprintf("「%s」有 %d 字，超过 %d 字的上限，请缩短后重写",
			label, n, liteworkspace.ReviseSectionMaxRunes))
	}
	if reason := liteworkspace.PronounProblem(text, run.pronouns()); reason != "" {
		return liteWorkspaceToolError("「" + label + "」没有通过检查：" + reason + "。请改好后重新调用 revise_section。")
	}
	if err := agent.CheckLiteParentSections(map[string]string{key: text}, []string{key}, run.facts, run.others); err != nil {
		return liteWorkspaceToolError("「" + label + "」没有通过检查：" + liteWorkspaceReportCheckError(key, err) + "。" +
			"引文和作品标题必须逐字出自事实，数字只用事实里的阿拉伯数字，不写其他学生的名字。请改好后重新调用 revise_section。")
	}
	if run.revised == nil {
		run.revised = map[string]string{}
	}
	run.revised[key] = text
	run.quoted = append(run.quoted, liteWorkspaceReportSpans(text)...)
	return liteWorkspaceToolOK(map[string]any{"section": label})
}

// sectionList is this report's section headings for a tool error. Headings
// only: revise_section accepts them, and a key here is a key in her reply.
func (run *liteWorkspaceReport) sectionList() string {
	parts := make([]string, 0, len(run.sections))
	for _, k := range run.sections {
		parts = append(parts, "「"+liteparent.SectionLabels[k]+"」")
	}
	return strings.Join(parts, "、")
}

// liteWorkspaceReportCheckCauses maps each cause CheckLiteParentSections can
// return to Chinese. The list is taken from the code:
// compose_lite_parent.go's checkChineseCounts, and liteweekly.CheckProse
// (extractSpans, the quote/title/digit/name checks). CheckProse's "unknown
// evidence code" cannot occur: the parent report passes no codes. The agent's
// strings stay English because the draft path logs and returns them as they
// are; the rewrite happens here, before a tool result reaches the model.
var liteWorkspaceReportCheckCauses = []struct{ english, chinese string }{
	{"chinese numeral count: ", "人数或数量用了中文数字，请改成事实里的阿拉伯数字："},
	{"unmatched closing mark: ", "有一个右引号或右书名号没有对应的左边："},
	{"unclosed quote: ", "有一个引号或书名号没有闭合："},
	{"quote not in corpus: ", "引文不是事实里学生的原话："},
	{"title not in titles: ", "作品标题不在事实里："},
	{"digit not in facts: ", "数字不在事实里："},
	{"mentions other student: ", "写了其他学生的名字："},
}

// liteWorkspaceReportCheckError rewrites one CheckLiteParentSections error
// for the model: the "<key>: " prefix is dropped (the caller names the
// section by its heading) and the English cause becomes Chinese, keeping the
// part after it (the quote, the digit, the name). A cause not in the table is
// reported without its English text.
func liteWorkspaceReportCheckError(key string, err error) string {
	msg := strings.TrimPrefix(err.Error(), key+": ")
	for _, c := range liteWorkspaceReportCheckCauses {
		if rest, ok := strings.CutPrefix(msg, c.english); ok {
			return c.chinese + rest
		}
	}
	return "文字不符合生成报告时的规则"
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
