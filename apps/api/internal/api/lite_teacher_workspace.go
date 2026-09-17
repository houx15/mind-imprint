package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// lite_teacher_workspace.go — 教师工作台的一轮：左边对话，右边的画布由工具写。
//
// This is the only teacher route that calls a model synchronously. It runs a
// bounded tool loop on the dialogue tier: the model may reach for the tools
// the page's surface offers (liteWorkspaceSurface), the server executes them
// and feeds the results back inside the same turn, and the loop stops after
// liteworkspace.ToolLoopMax MODEL CALLS — one of which has to be the call that
// answers, so the tool budget is one less than that number.
//
// This file is the part every surface shares: decoding, ownership, history
// bounding, metering, the loop, the §6 name and head-count checks, slug
// replacement and option clamping. A surface supplies only its tools, its
// prompt, its tool execution and its grounding inputs. The assignment surface
// is in lite_teacher_workspace_assignment.go.
//
// 🚨 教师可见性边界：没有一个工具读学生与印记的对话记录。The tools reach the
// roster, the assignment rows behind it and the embedded reading library —
// atom_message is not queried from these files, and a tool or surface added
// later must keep it that way.

// liteTeacherWorkspacePurpose is the llm_call purpose for every model call a
// workspace turn makes, including the ones inside the tool loop.
const liteTeacherWorkspacePurpose = "lite_teacher_workspace"

// liteWorkspaceTurnRequest is §4.4's request. reportId is read only by the
// parentReport surface; classId by the other two.
type liteWorkspaceTurnRequest struct {
	Surface  string               `json:"surface"`
	ClassID  string               `json:"classId"`
	ReportID string               `json:"reportId"`
	Artifact json.RawMessage      `json:"artifact"`
	Turns    []liteworkspace.Turn `json:"turns"`
	Text     string               `json:"text"`
	ChoiceID string               `json:"choiceId"`
	// ChoiceSlug is the article the option she tapped carried, echoed back by
	// the client. It is how "use this article" survives a turn boundary: the
	// server holds nothing between turns, so the payload has to make the round
	// trip. Client-supplied and therefore trusted for nothing — it is looked up
	// in the embedded catalogue before it is used, so the worst a forged value
	// can do is pick a different real article for the teacher who forged it.
	ChoiceSlug string `json:"choiceSlug"`
	// ChoiceLabel is the tapped option's label as she saw it. Read by the
	// model beside the id, never used as evidence (it is client-supplied).
	ChoiceLabel string `json:"choiceLabel"`
}

// liteWorkspaceTurnDTO is §4.4's response. patch carries only the fields a
// tool actually wrote; cards carry tool results the panel renders directly,
// which is how numbers and names reach the teacher without passing through
// the model's prose (§6).
type liteWorkspaceTurnDTO struct {
	Reply   string                 `json:"reply"`
	Choices []liteworkspace.Choice `json:"choices"`
	Patch   map[string]any         `json:"patch"`
	Cards   []liteWorkspaceCardDTO `json:"cards"`
	// Navigate is set only by a surface that offers a page to open (today,
	// only home's open_page). omitempty keeps every other surface's response
	// byte-identical to before this field existed.
	Navigate *liteWorkspaceNavigateDTO `json:"navigate,omitempty"`
}

// liteWorkspaceCardDTO is one tool result on the canvas. kind is "students" or
// "articles" today; D2 adds more.
type liteWorkspaceCardDTO struct {
	Kind string `json:"kind"`
	Rows any    `json:"rows"`
	// Filter is the list_students filter a students card shows, so the card
	// can say which list it is (the reply may not name the students).
	Filter string `json:"filter,omitempty"`
}

// liteWorkspaceSurface is what one page of the workspace contributes to a
// turn. Everything else — metering, the loop bound, the §6 checks, slug
// replacement, option clamping — belongs to the shared handler, so a new
// surface cannot skip any of it.
//
// A surface value lives for one turn. The handler builds it after the
// ownership check and before the model is routed, calls begin once, then
// system and tools, then execute for every tool call the model makes, and
// reads the rest after the loop.
type liteWorkspaceSurface interface {
	// begin applies what the request itself carries before the model runs
	// (the assignment surface: the article on an option she tapped) and
	// returns a note appended to her turn, or "".
	begin(req liteWorkspaceTurnRequest) string
	// system is the whole system message, including the canvas as it stands
	// after begin.
	system() string
	tools() []gateway.ChatTool
	// execute runs one tool call and returns the tool result the model reads.
	// A bad argument is a tool result, never a failed turn.
	execute(tc gateway.ToolCall) string
	// ended reports whether a tool ended the turn during the last round, and
	// with which reply and options. The loop checks it after every round.
	ended() (reply string, choices []liteworkspace.Choice, done bool)
	// clearEnded forgets the ask_choice that ended the last round, so the
	// loop can give the model a rewrite (falseClaim).
	clearEnded()
	// falseClaim reports, in Chinese, something the finished reply or an
	// option label says was done or can be done that no tool of this surface
	// does; "" when there is none. text is the reply and every option label,
	// joined with newlines. See liteworkspace/claims.go.
	falseClaim(text string) string
	// extraParts is anything this turn shows the teacher that is neither the
	// reply, an option nor a patch value — those three the handler checks
	// itself (liteWorkspaceCheckedParts). One string per field. nil when the
	// surface shows nothing else.
	extraParts() []string
	// groundedNames and groundedCounts are what this turn's tools returned:
	// the tool half of the §6 evidence. The handler adds her own words and
	// the roster size.
	//
	// 🚨 A surface whose subject is one student (the parent report) must
	// return that student's name from groundedNames on every turn. The name
	// check treats every roster name as needing evidence, so without it every
	// reply that names her fails.
	groundedNames() []string
	groundedCounts() []int
	// result is what goes back to the canvas: the fields a tool wrote and the
	// tool results the panel renders. Either may be nil.
	result() (patch map[string]any, cards []liteWorkspaceCardDTO)
	// navigate is the page open_page offered this turn, or nil. The assignment
	// surface has no such tool and always returns nil, which is why the field
	// is omitempty on the wire — her assignment turns keep the exact response
	// shape they had before D2.
	navigate() *liteWorkspaceNavigateDTO
	// verbatimQuotedSpans is catalogue text this turn's tools handed back — an
	// article or assignment title — that may legitimately contain a
	// 数字+人/位/名/个 shape with NOTHING to do with a student head count
	// (「3 人小组汇报」), but is only safe to blank where the model actually
	// wrote it QUOTED (「」/《》/“”/"…"): the quote is the model following the
	// system prompt's own instruction to wrap a title that way, and the
	// tools hand titles back already wrapped in 《》 for the same reason,
	// so an occurrence wrapped like that IS the title, not prose that merely
	// shares its characters. See liteWorkspaceBlankQuotedSpans.
	//
	// 🚨 An earlier version blanked ANY occurrence, quoted or not, which
	// reintroduced the bypass this whole mechanism exists to close: an
	// assignment titled exactly 「3人」 made the digit invisible to the count
	// check EVERYWHERE in the turn, so 「3人没有交作业，请督促。」 — a claim the
	// title never made — passed too. Quote-anchoring means an unquoted
	// occurrence is still checked; only 「「3人」还没有人开始」 — where 「3人」
	// visibly names the title — is exempt.
	verbatimQuotedSpans() []string
	// verbatimClassName is the class name, blanked UNQUOTED wherever it
	// occurs — matching compose_lite_parent.go's own rule for the same field:
	// an admin-created class name (「高一（3）班」) is never written in quotes
	// by a teacher or by the model, so requiring quotes here would never
	// blank it. "" when the surface has no class in scope (should not happen
	// today — every surface resolves one).
	//
	// 🚨 Not blanked at all when liteworkspace.IsPureHeadCount(name) — a class
	// name that is NOTHING BUT the shape (no realistic admin would create
	// one, but nothing stops it) would have nothing left to check once
	// blanked, which is the one case where blanking removes real information
	// rather than a false alarm.
	verbatimClassName() string
	// verbatimSubjectNames is the name of the one student the page is about,
	// blanked UNQUOTED from every part before the NAME check only. A classmate
	// whose name is part of hers (her 王丽华, classmate 王丽) would otherwise be
	// found inside every mention of her. A classmate whose name contains hers
	// (her 王丽, classmate 王丽华) is checked before the blanking, the order
	// CheckProse uses. nil for a surface with no single student (assignment,
	// home).
	//
	// The count check does not blank these names: it keeps its own blanking
	// (verbatimQuotedSpans, verbatimClassName), so a name that happened to be
	// count-shaped still reads as a count there.
	verbatimSubjectNames() []string
	// groundsRosterSize reports whether the roster size is evidence for the
	// count check. It is only when the surface's system prompt hands the
	// class size to the model (assignment, home). The parent report prompt
	// does not, so there a stated class size has nothing behind it.
	groundsRosterSize() bool
	// blanksQuotedSpansForNames reports whether verbatimQuotedSpans are also
	// blanked, where quoted, before the NAME check. Only the parent report
	// does: a section quoting her own title that contains a classmate's name
	// passes CheckLiteParentSections and must not then fail here. Assignment
	// and home keep the stricter rule: a roster name inside a quoted title
	// still needs evidence (TestWorkspaceHomeBlankingDoesNotWeakenNameCheck).
	blanksQuotedSpansForNames() bool
}

// liteWorkspaceBlankSeparator replaces a blanked span (a quoted title, or an
// unquoted class name). It is deliberately NOT whitespace: StatedCounts
// strips whitespace before it scans, so a newline does not stop a digit on
// one side of a removed span from reading as adjacent to a counter word on
// the other. Measured: blanking an unquoted 「汇报」 out of 「3汇报人没交。」
// with "\n" left 「3\n人没交」, which StatedCounts strips back down to
// 「3人没交」 — a phantom head count neither side of the sentence stated. The
// separator is also not a digit, a CJK numeral or a counter word itself, so
// it cannot supply either half of a count shape on its own.
const liteWorkspaceBlankSeparator = "／"

// liteWorkspaceTitleQuotes are the marks that make a title-shaped span safe
// to blank: the system prompt tells the model to wrap a title one of these
// ways, so a span found wrapped in one IS the model following that contract,
// not prose that happens to share the title's characters. Each pair's open
// and close may be the same rune (straight ASCII quotes, "…").
var liteWorkspaceTitleQuotes = []struct{ open, close string }{
	{"「", "」"},
	{"《", "》"},
	{"“", "”"},
	{"\"", "\""},
}

// liteWorkspaceBlankQuotedSpans blanks each span in spans, but ONLY where it
// appears wrapped in one of liteWorkspaceTitleQuotes — the quote marks are
// removed together with it, replaced as one unit by
// liteWorkspaceBlankSeparator. An UNQUOTED occurrence of the same text is
// left exactly as it is and still runs through the count check: that is what
// stops a title that happens to spell a head count (「3人」) from being usable
// to launder an unrelated, unquoted claim (「3人没有交作业」).
//
// Longest spans first, so a title that is a literal prefix of another's does
// not leave a stray closing quote once the longer one's own quoted form is
// removed.
func liteWorkspaceBlankQuotedSpans(text string, spans []string) string {
	for _, s := range liteWorkspaceDedupeLongestFirst(spans) {
		for _, q := range liteWorkspaceTitleQuotes {
			text = strings.ReplaceAll(text, q.open+s+q.close, liteWorkspaceBlankSeparator)
		}
	}
	return text
}

// liteWorkspaceBlankUnquoted blanks every occurrence of span in text,
// unquoted. Used only for the class name — see verbatimClassName's comment
// for why it is exempt from the quote requirement every title span has.
func liteWorkspaceBlankUnquoted(text, span string) string {
	span = strings.TrimSpace(span)
	if span == "" || liteworkspace.IsPureHeadCount(span) {
		return text
	}
	return strings.ReplaceAll(text, span, liteWorkspaceBlankSeparator)
}

// liteWorkspaceDedupeLongestFirst trims, drops blanks and duplicates, and
// sorts longest-first: a span that itself contains another span (one title a
// literal prefix of another) is handled wholesale before the shorter one's
// own pass runs, so there is no partial match left over to mis-blank.
func liteWorkspaceDedupeLongestFirst(spans []string) []string {
	seen := make(map[string]bool, len(spans))
	clean := make([]string, 0, len(spans))
	for _, s := range spans {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		clean = append(clean, s)
	}
	slices.SortFunc(clean, func(a, b string) int { return len(b) - len(a) })
	return clean
}

// liteWorkspaceNavigateDTO is the page open_page offered this turn (§12.5
// item 6, D2's home surface). The server performs no navigation: this is an
// offer the panel renders as a button, and only her click moves her there.
type liteWorkspaceNavigateDTO struct {
	View         string  `json:"view"`
	ClassID      string  `json:"classId"`
	UserID       *string `json:"userId,omitempty"`
	AssignmentID *string `json:"assignmentId,omitempty"`
	// UserIDs (assignmentNew only) are the recipients the form preselects;
	// absent means the whole class.
	UserIDs []string `json:"userIds,omitempty"`
	// Label is Chinese and names a real page or a real student/assignment —
	// exactly the kind of text §6 exists to check, so the surface that builds
	// it must also return it from extraParts().
	Label string `json:"label"`
}

// liteWorkspaceSubject is what the ownership check loaded for a turn: the
// class the turn is about, plus the surface's own row where it has one. The
// roster is loaded from class, whichever surface resolved it.
type liteWorkspaceSubject struct {
	class sqlc.Class
	// report is the parent report a parentReport turn revises. Zero for
	// every other surface. liteWorkspaceSubjectFromReport sets it.
	report sqlc.LiteParentReport
}

// liteWorkspaceSurfaceInput is what a surface constructor receives.
type liteWorkspaceSurfaceInput struct {
	// mctx is the detached model context: a load a surface defers (the
	// assignment surface's group profiles) must survive her navigating away,
	// the same way the model call does.
	mctx    context.Context
	subject liteWorkspaceSubject
	roster  []liteworkspace.Student
	req     liteWorkspaceTurnRequest
	// typed is what she typed this turn, trimmed.
	typed string
}

// liteWorkspaceSurfaceSpec is one entry of liteWorkspaceSurfaces.
//
// resolve is the ownership check. It returns an error the handler writes as
// it is, so a not-found answer must be httpx.ErrNotFound or pgx.ErrNoRows.
//
// build may fail. The handler calls it before routing the model, so a failed
// build costs no model call and writes no llm_call row.
type liteWorkspaceSurfaceSpec struct {
	resolve func(a *API, ctx context.Context, req liteWorkspaceTurnRequest) (liteWorkspaceSubject, error)
	build   func(a *API, in liteWorkspaceSurfaceInput) (liteWorkspaceSurface, error)
}

// liteWorkspaceSurfaces is every surface the workspace turn serves. A name
// that is not a key here answers 400 unknown_surface: each surface carries
// its own tools and canvas, so a name without an entry is refused rather than
// answered with another surface's tool set.
//
// Read-only after init. Tests register extra entries only from init (see
// lite_teacher_workspace_export_test.go), never while a request runs.
var liteWorkspaceSurfaces = map[string]liteWorkspaceSurfaceSpec{
	"assignment": {
		resolve: liteWorkspaceSubjectFromClass,
		build: func(a *API, in liteWorkspaceSurfaceInput) (liteWorkspaceSurface, error) {
			return a.newLiteWorkspaceAssignment(in.mctx, in.subject.class, in.roster, in.req.Artifact, in.typed), nil
		},
	},
	"home": {
		resolve: liteWorkspaceSubjectFromClass,
		build: func(a *API, in liteWorkspaceSurfaceInput) (liteWorkspaceSurface, error) {
			return a.newLiteWorkspaceHome(in.mctx, in.subject.class, in.roster, in.typed), nil
		},
	},
	"parentReport": {
		resolve: liteWorkspaceSubjectFromReport,
		build: func(a *API, in liteWorkspaceSurfaceInput) (liteWorkspaceSurface, error) {
			return a.newLiteWorkspaceReport(in)
		},
	},
}

// liteWorkspaceSubjectFromClass resolves a surface whose class id travels in
// the request body. Same ownership query as authTeacherClass, and the same
// not-found answer for a malformed id or a class the caller does not teach.
//
// A sibling rather than a parameter on authTeacherClass: that helper reads
// r.PathValue("id") and four shipped routes depend on it.
func liteWorkspaceSubjectFromClass(a *API, ctx context.Context, req liteWorkspaceTurnRequest) (liteWorkspaceSubject, error) {
	classID, err := uuid.Parse(strings.TrimSpace(req.ClassID))
	if err != nil {
		return liteWorkspaceSubject{}, httpx.ErrNotFound("资源不存在")
	}
	cls, err := a.assertTeacherOwnsClass(ctx, classID)
	if err != nil {
		return liteWorkspaceSubject{}, err
	}
	return liteWorkspaceSubject{class: cls}, nil
}

// The parentReport surface resolves with liteWorkspaceSubjectFromReport
// (lite_teacher_workspace_report.go), which reads the report id from the body
// and ignores req.ClassID: the class comes from the report row, so the roster
// the §6 checks read is the report's class.

// errLiteWorkspaceTurn is the visible failure of one workspace turn. 动词+失败
// plus the real cause — never a plausible sentence standing in for a reply
// that did not happen.
func errLiteWorkspaceTurn(reason string) *httpx.APIError {
	return &httpx.APIError{
		Status: http.StatusBadGateway, Code: "ai_dialogue_failed", Message: "对话失败：" + reason,
	}
}

// postLiteTeacherWorkspaceTurn handles POST /api/v1/lite/teacher/workspace/turn.
func (a *API) postLiteTeacherWorkspaceTurn(w http.ResponseWriter, r *http.Request) {
	var req liteWorkspaceTurnRequest
	// The body is capped like every other model-adjacent route in this package.
	// artifact.instructions and turns[].text are teacher-supplied and go
	// straight into a metered prompt, so an unbounded body is an unbounded
	// bill as well as unbounded memory.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	spec, open := liteWorkspaceSurfaces[req.Surface]
	if !open {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_surface", "该工作台尚未开放："+req.Surface, nil))
		return
	}
	subject, err := spec.resolve(a, r.Context(), req)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// typed is what she wrote herself. It is the only part of this request that
	// counts as evidence for the name check below.
	typed := strings.TrimSpace(req.Text)
	said := typed
	if said == "" {
		// A choice she clicked is her turn too, and the model needs to know
		// which one. 🚨 The id is the MODEL's own string, so it goes to the
		// model but never into the evidence: a model that had minted
		// {id: "林知遥-alone"} would otherwise get that name grounded the moment
		// she clicked the button, which is laundering by another route.
		said = strings.TrimSpace(req.ChoiceID)
		// The label she saw goes beside the id. With the id alone the model
		// read 「usage_compare」 as something she typed and asked her what that
		// English abbreviation meant (real-user walk, 2026-09-17).
		if label := strings.TrimSpace(req.ChoiceLabel); said != "" && label != "" {
			said = "（点选了选项「" + liteWorkspaceClampRunes(label, 200) + "」，选项 id：" + said + "）"
		}
	}
	if said == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_turn", "请输入", nil))
		return
	}
	if a.d.Provider == nil {
		httpx.WriteError(w, r, errLiteWorkspaceTurn("未配置模型通道"))
		return
	}
	u, ok := requireTeacherEntitled(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	roster, err := a.liteWorkspaceRoster(ctx, subject.class.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	// Built before the model is routed: a surface that cannot be built costs
	// no model call and writes no llm_call row.
	surface, err := spec.build(a, liteWorkspaceSurfaceInput{
		mctx: mctx, subject: subject, roster: roster, req: req, typed: typed,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resolved, err := a.routeE(mctx, gateway.ClassDialogue)
	if err != nil {
		// The cause goes out, not a bare 500. A routing failure is a
		// misconfigured model binding, and 「服务器内部错误」 would send whoever
		// is on call reading logs to learn what this line already knows. No
		// key reaches this string: Resolved carries the key, the error does not.
		httpx.WriteError(w, r, errLiteWorkspaceTurn(err.Error()))
		return
	}

	// Whatever the request itself carries is applied before the model runs,
	// so the canvas the model reads below is the canvas as it stands NOW.
	chosen := surface.begin(req)

	// req.Turns is HISTORY ONLY — the client never puts the current turn in
	// it (threadLogic.ts's beginTurn sends it separately as Text/ChoiceID),
	// so every item here is capped without exception. TruncateHistory runs
	// here even though the client already truncates before sending: this is
	// a server, and a server never trusts that a client did its own
	// bounding. The current turn (said, below) reaches the model whole
	// through Text/ChoiceID, not through `turns` — a pasted article's
	// set_material call grounds against it.
	turns := liteworkspace.TruncateHistory(liteworkspace.TrimTurns(req.Turns))
	msgs := liteWorkspaceMessages(surface.system(), turns, said+chosen)

	rosterNames := liteWorkspaceRosterNames(roster)
	// Every name a log line hides: the roster, plus her name as frozen on a
	// report, which can differ from the roster after a rename.
	logNames := append(append([]string{}, rosterNames...), surface.verbatimSubjectNames()...)
	reply, choices, aerr := a.runLiteWorkspaceLoop(mctx, r, req.Surface, u.ID, resolved, msgs, surface, logNames)
	if aerr != nil {
		httpx.WriteError(w, r, aerr)
		return
	}

	// 🚨 grounded excludes the model's own earlier turns on purpose. A
	// fabricated name's source IS the model's earlier words, so feeding them
	// back as evidence would launder exactly the failure this check exists to
	// catch (§6).
	//
	// The tool half comes from the surface. A surface whose subject is one
	// student (the parent report) returns her name from groundedNames, or
	// every reply that names her fails here.
	grounded := append([]string{}, surface.groundedNames()...)
	grounded = append(grounded, liteWorkspaceNamesTeacherTyped(roster, turns, typed)...)

	choices = liteworkspace.ClampChoices(choices)
	// The name check reads every field as one blob. It compares whole names, so
	// a name cannot be assembled out of the end of one field and the start of
	// the next, and joining costs nothing.
	patch, cards := surface.result()
	parts, err := liteWorkspaceCheckedParts(reply, choices, patch)
	if err != nil {
		httpx.WriteError(w, r, errLiteWorkspaceTurn("画布内容无法校验："+err.Error()))
		return
	}
	parts = append(parts, surface.extraParts()...)
	quotedSpans := surface.verbatimQuotedSpans()
	var nameSpans []string
	if surface.blanksQuotedSpansForNames() {
		nameSpans = liteWorkspaceSpansForNameCheck(quotedSpans, rosterNames)
	}
	if bad := liteWorkspaceUngroundedNames(
		parts, nameSpans, surface.verbatimSubjectNames(), rosterNames, grounded,
	); len(bad) > 0 {
		liteWorkspaceLogGroundingFailure(r, req.Surface, "names", len(bad), strconv.Itoa(len(grounded))+" 个", strings.Join(parts, " | "), logNames)
		httpx.WriteError(w, r, errLiteWorkspaceTurn("回复里出现了本轮没有依据的学生姓名："+strings.Join(bad, "、")))
		return
	}

	// The same rule for the other half of §6. A name is checked against the
	// roster; a head count had nothing but the prompt behind it, and a live run
	// showed the prompt does not always hold — 「发给全班 3 人」 reached a teacher
	// with no tool having counted anything. A wrong number about her own class
	// is the failure this section exists to prevent, so it fails the turn the
	// way a fabricated name does.
	//
	// 🚨 Field by field, never joined. UngroundedCounts strips whitespace before
	// matching, so any separator a join could use dissolves, and a 说明 ending
	// 「难度 3」 beside a label starting 「人工智能方向」 became a claim about
	// 3 people that neither field made.
	//
	// Each part is blanked of this turn's verbatim spans BEFORE the count
	// check runs — never grounded as digits, which would free that digit to
	// justify an unrelated claim anywhere else in the same turn. A title is
	// blanked only where the model wrote it QUOTED (verbatimQuotedSpans'
	// comment); the class name is blanked unquoted (verbatimClassName's).
	countGrounds := liteWorkspaceGroundedCounts(surface.groundedCounts(), len(roster), surface.groundsRosterSize(), turns, typed)
	className := surface.verbatimClassName()
	for _, part := range parts {
		checked := liteWorkspaceBlankQuotedSpans(part, quotedSpans)
		checked = liteWorkspaceBlankUnquoted(checked, className)
		if bad := liteworkspace.UngroundedCounts(checked, countGrounds); len(bad) > 0 {
			liteWorkspaceLogGroundingFailure(r, req.Surface, "counts", len(bad), liteWorkspaceJoinInts(countGrounds), part, logNames)
			httpx.WriteError(w, r, errLiteWorkspaceTurn("回复里出现了本轮没有依据的人数："+liteWorkspaceJoinInts(bad)))
			return
		}
	}

	// Slug replacement runs AFTER both §6 checks, never before: those checks
	// have to see the model's own words, and a slug the model invented (one
	// that library.BySlug cannot find) is left exactly as it is written, which
	// is what makes it visible to a human reading the failure later.
	// Dates the model copied from the card's input value (2026-09-18T21:00)
	// are written the way a teacher reads them.
	reply = liteworkspace.HumanizeDatetimes(liteWorkspaceDeslugged(reply))
	for i := range choices {
		choices[i].Label = liteworkspace.HumanizeDatetimes(liteWorkspaceDeslugged(choices[i].Label))
		// Article is enriched from the catalogue, not from the model's words,
		// so it runs on the untouched Slug rather than on liteWorkspaceDeslugged's
		// output.
		choices[i].Article = a.liteWorkspaceChoiceArticle(choices[i].Slug)
	}

	// An empty patch and an empty card list go out as {} and [], not null: the
	// client walks both on every turn, and a null would make "no tool wrote
	// anything" a separate case at every call site.
	if patch == nil {
		patch = map[string]any{}
	}
	if cards == nil {
		cards = []liteWorkspaceCardDTO{}
	}
	httpx.WriteJSON(w, http.StatusOK, liteWorkspaceTurnDTO{
		Reply: reply, Choices: choices, Patch: patch, Cards: cards, Navigate: surface.navigate(),
	})
}

// liteWorkspaceRoster loads the class roster in the shape the closed-set
// filters read. It goes through the same two queries the roster endpoint uses,
// so a filter can never disagree with the list the teacher is looking at.
func (a *API) liteWorkspaceRoster(ctx context.Context, classID uuid.UUID) ([]liteworkspace.Student, error) {
	start, end := currentLiteWeek(time.Now())
	rows, err := a.d.Queries.ListLiteClassRoster(ctx, sqlc.ListLiteClassRosterParams{
		ClassID: classID, WeekStart: start, WeekEnd: end,
		WeekStartDay: pgDate(start), WeekEndDay: pgDate(end),
	})
	if err != nil {
		return nil, err
	}
	overdue, err := a.overdueAssignmentsByUser(ctx, classID)
	if err != nil {
		return nil, err
	}
	out := make([]liteworkspace.Student, 0, len(rows))
	for _, row := range rows {
		out = append(out, liteworkspace.Student{
			ID:                 row.ID.String(),
			Name:               row.DisplayName,
			Gender:             liteworkspace.GenderOf(row.Gender),
			ActiveDaysThisWeek: int(row.ActiveDaysThisWeek),
			OverdueAssignments: int(overdue[row.ID]),
			WritingsDone:       int(row.WritingsDone),
		})
	}
	return out, nil
}

// liteWorkspacePronounOf is the pronoun the model may use for userID on
// roster; liteworkspace.PronounUnset when she is not on it or has no gender
// set.
func liteWorkspacePronounOf(roster []liteworkspace.Student, userID string) string {
	for _, s := range roster {
		if s.ID == userID {
			return liteworkspace.Pronoun(s.Gender)
		}
	}
	return liteworkspace.PronounUnset
}

// liteWorkspaceSpansForNameCheck is spans without any span whose trimmed text
// is exactly a roster name. Blanking such a span would remove the name
// itself: a title 「李明」 would let 「「李明」还没有交作业」 through.
func liteWorkspaceSpansForNameCheck(spans, rosterNames []string) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		if !slices.Contains(rosterNames, strings.TrimSpace(s)) {
			out = append(out, s)
		}
	}
	return out
}

// liteWorkspaceUngroundedNames is the name half of §6 over this turn's parts.
//
// Each part first has quotedSpans blanked where they appear QUOTED. The
// caller passes spans only for a surface whose blanksQuotedSpansForNames is
// true (the parent report): there a quoted title of hers is a reference to
// her work, and a roster name inside it (《李明推荐的雨水花园》) is not the
// reply naming 李明. An unquoted echo of the same text is left as it is and
// still checked.
//
// Then subject (the one student the page is about) is blanked unquoted, so a
// classmate whose name is part of hers is not found inside her name. A
// classmate whose name CONTAINS hers would vanish with that blanking, so
// those names are checked first, on the text before it.
//
// The parts are joined only after blanking: the name check compares whole
// names, so joining cannot assemble one.
func liteWorkspaceUngroundedNames(parts, quotedSpans, subject, roster, grounded []string) []string {
	blanked := make([]string, len(parts))
	for i, p := range parts {
		blanked[i] = liteWorkspaceBlankQuotedSpans(p, quotedSpans)
	}
	text := strings.Join(blanked, "\n")
	subjects := liteWorkspaceDedupeLongestFirst(subject)
	var containing []string
	for _, name := range roster {
		for _, s := range subjects {
			if name != s && strings.Contains(name, s) {
				containing = append(containing, name)
				break
			}
		}
	}
	bad := liteworkspace.UngroundedNames(text, containing, grounded)
	for _, s := range subjects {
		text = strings.ReplaceAll(text, s, liteWorkspaceBlankSeparator)
	}
	for _, name := range liteworkspace.UngroundedNames(text, roster, grounded) {
		if !slices.Contains(bad, name) {
			bad = append(bad, name)
		}
	}
	slices.Sort(bad)
	return bad
}

func liteWorkspaceRosterNames(roster []liteworkspace.Student) []string {
	out := make([]string, 0, len(roster))
	for _, s := range roster {
		out = append(out, s.Name)
	}
	return out
}

// liteWorkspaceCheckedParts is everything this turn puts in front of the
// teacher, ONE STRING PER FIELD.
//
// That is the reply, the button labels AND their ids, and every string the
// patch carries. The patch matters most: a name written into the card's title
// or instructions is read by the teacher and then read by her whole class on
// publish, and it would pass a check that only looked at the reply. The button
// ids matter because the client sends the id back as her next turn.
//
// Every string in the patch is checked, not a named list of prose fields: the
// closed-set fields (kind, readingSource, slug, dueInput, user ids) cannot
// carry a classmate's name anyway, and checking everything means a tool added
// later is covered without anyone having to remember this function.
//
// 🚨 The fields stay separate because the count check strips whitespace before
// it matches, which dissolves any separator a join could put between them. A
// 说明 ending 「难度 3」 next to a label starting 「人工智能方向」 read as a claim
// about 3 people, and nothing on either side said anything of the kind. Two
// fields are two sentences; only the name check, which compares whole names,
// can safely read them as one blob.
//
// The "text" field is the one deliberate exception, and the reason is NOT
// "an article has a lot of names and numbers" — typed (what she pasted this
// turn) already grounds those: liteWorkspaceNamesTeacherTyped and
// liteWorkspaceGroundedCounts both read typed, and every roster name or
// stated count inside "text" is inside typed too, because run.setMaterial
// stores only a span of typed's own runes (liteworkspace.AnchoredSpan cuts
// it out between the model's two anchors; the model never supplies the
// words). So "text" is a literal substring of typed. The real reason is
// narrower: the count check does not see the SUBSTRING RELATIONSHIP, only
// the digits in front of it. A cut can start mid-number — typed says
// 「1200人」, the start anchor begins at 「200人」, and the span is still a
// real substring — and StatedCounts(text) then reads a head count (200)
// that was never stated by anyone and is not in typed's own count list
// (1200). That is not a fabrication; it is where the passage happened to be
// cut. Checking "text" against §6 would fail a turn over content that is
// her own words, so it is not checked at all.

func liteWorkspaceCheckedParts(reply string, choices []liteworkspace.Choice, patch map[string]any) ([]string, error) {
	out := []string{reply}
	for _, c := range choices {
		// The slug is checked too. It is validated against the catalogue
		// before it gets here, so it cannot carry a fabricated name today —
		// but it travels to the client and back like the id does, and a field
		// that is exempt from the check is a field someone will later widen.
		out = append(out, c.ID, c.Label, c.Slug)
	}
	if len(patch) == 0 {
		return out, nil
	}
	// The patch is normalised through JSON before it is walked, so the walk
	// only ever sees string, float64, bool, nil, []any and map[string]any.
	// A surface that writes a typed Go value ([]map[string]any, a struct)
	// would otherwise reach a type the walk does not know, and its text would
	// skip both §6 checks with no error. The client receives this same JSON,
	// so what is checked is what she is sent.
	//
	// A patch that cannot be marshalled fails the turn: skipping it would be
	// the silent miss this normalisation exists to prevent.
	raw, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	var normalised map[string]any
	if err := json.Unmarshal(raw, &normalised); err != nil {
		return nil, err
	}
	for _, k := range slices.Sorted(maps.Keys(normalised)) {
		// Top level only. A nested key named "text" (a parent report's
		// section, say) is not the passage setMaterial cut from her own
		// text, so it is checked like everything else.
		if k == "text" {
			continue
		}
		out = liteWorkspacePatchStrings(out, normalised[k])
	}
	return out, nil
}

// liteWorkspacePatchStrings appends every string leaf under v, one part per
// leaf, walking maps and slices to any depth. v is JSON-decoded (see
// liteWorkspaceCheckedParts), so these are the only container types. A parent
// report's patch is {"body": {"<section>": "<text>"}}; a walk that stopped at
// the top level would let revised report text skip both §6 checks. Each leaf
// stays its own part so the head-count check still reads field by field.
//
// Non-string leaves produce no part: tier 3 is not a sentence about 3 people.
// Map keys are walked in sorted order so the parts, and therefore the first
// failing count in the error message, are the same on every run.
func liteWorkspacePatchStrings(out []string, v any) []string {
	switch value := v.(type) {
	case string:
		out = append(out, value)
	case []any:
		for _, item := range value {
			out = liteWorkspacePatchStrings(out, item)
		}
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(value)) {
			out = liteWorkspacePatchStrings(out, value[k])
		}
	}
	return out
}

// liteWorkspaceNamesTeacherTyped returns the roster names the TEACHER wrote,
// this turn or earlier in the thread. Her own words are evidence; the model's
// are not, so ai turns are skipped here.
//
// The teacher turns in `turns` are client-supplied and could in principle
// carry a string the model minted (a clicked option). That door is shut on the
// way out instead: liteWorkspaceCheckedParts checks option ids and labels
// before they leave the server, so a fabricated name never reaches the client
// to be echoed back.
func liteWorkspaceNamesTeacherTyped(roster []liteworkspace.Student, turns []liteworkspace.Turn, typed string) []string {
	var b strings.Builder
	b.WriteString(typed)
	for _, t := range turns {
		if t.Role == "teacher" {
			b.WriteString("\n")
			b.WriteString(t.Text)
		}
	}
	hers := b.String()
	var out []string
	for _, s := range roster {
		if s.Name != "" && strings.Contains(hers, s.Name) {
			out = append(out, s.Name)
		}
	}
	return out
}

// liteWorkspaceGroundedCounts is every number a reply may state as a count of
// people: what a tool counted this turn, the size of the class, and the head
// counts the teacher stated herself.
//
// The roster size is in here because the system prompt hands it to the model in
// its first line (「共 %d 名学生」). A reply that says 「全班 12 人」 for a class of
// twelve is repeating data we supplied, and failing that turn would punish the
// model for being right.
//
// Her side is read with StatedCounts, the same head-count shape the reply is
// checked with, NOT every integer she typed. Reading every integer measured out
// as far too generous: 「这周读一篇气候变化的报道，周五交」 grounded 1 (一篇) and
// 5 (周五), so a reply inventing 「发给全班 5 人」 walked through the check for a
// class of twelve. The case this side exists for still passes — she types
// 「发给 3 名学生」, the reply says 「三人」 — because that is a head count in both
// shapes.
//
// groundRoster is false on a surface whose prompt does not state the class
// size (groundsRosterSize); there the roster size is left out.
func liteWorkspaceGroundedCounts(fromTools []int, rosterSize int, groundRoster bool, turns []liteworkspace.Turn, typed string) []int {
	out := append([]int{}, fromTools...)
	if groundRoster {
		out = append(out, rosterSize)
	}
	out = append(out, liteworkspace.StatedCounts(typed)...)
	for _, t := range turns {
		if t.Role == "teacher" {
			out = append(out, liteworkspace.StatedCounts(t.Text)...)
		}
	}
	return out
}

// liteWorkspaceLogGroundingFailure records a turn the §6 check rejected: the
// surface, how many names or counts were ungrounded, what the turn had as
// evidence, and the model-written text that carried them.
//
// 🚨 No student name reaches the log. The text is teacher-facing model output
// (reply, option labels, patch fields) and names students; every roster name
// in it is replaced with liteworkspace.RedactedName, and a failed name check
// logs how many names failed, not which. The name check only ever reports
// roster names, so redacting the roster covers everything it can report. A
// failed count check still logs the numbers: they are not personal. The text
// is cut at 2,000 runes first: a patch can carry a whole pasted article.
func liteWorkspaceLogGroundingFailure(r *http.Request, surface, kind string, ungrounded int, grounded, text string, names []string) {
	slog.Warn("lite workspace: reply failed the grounding check",
		"request_id", httpx.RequestIDFromContext(r.Context()),
		"surface", surface, "kind", kind, "ungrounded", ungrounded, "grounded", grounded,
		"text", liteworkspace.RedactNames(liteWorkspaceClampRunes(text, 2000), names))
}

func liteWorkspaceJoinInts(ns []int) string {
	parts := make([]string, 0, len(ns))
	for _, n := range ns {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, "、")
}

// liteWorkspaceMessages builds the prompt: the surface's system message, then
// the trimmed transcript, then what she just said.
func liteWorkspaceMessages(system string, turns []liteworkspace.Turn, said string) []gateway.ChatMessage {
	msgs := make([]gateway.ChatMessage, 0, len(turns)+2)
	msgs = append(msgs, gateway.ChatMessage{Role: gateway.RoleSystem, Content: system})
	for _, t := range turns {
		role := gateway.RoleUser
		if t.Role == "ai" {
			role = gateway.RoleAssistant
		}
		msgs = append(msgs, gateway.ChatMessage{Role: role, Content: t.Text})
	}
	return append(msgs, gateway.ChatMessage{Role: gateway.RoleUser, Content: said})
}

// liteWorkspaceDeslugged is the backstop half of spec §12.1's rule 3.
// liteWorkspaceCardState already keeps slugs out of what the model reads, so
// this only catches a slug a tool result put in the model's hands this turn
// (search_library and set_material's arguments and results are wire values)
// and the model then echoed into its own words. Any word this does not
// recognise as a real slug — including an invented one — is left as it is.
func liteWorkspaceDeslugged(text string) string {
	return liteworkspace.ReplaceSlugs(text, func(slug string) (string, bool) {
		art, ok := library.BySlug(slug)
		if !ok {
			return "", false
		}
		return art.ZhTitle, true
	})
}

// liteWorkspaceChoiceArticle builds the card an option about an article shows
// in the conversation. slug already passed askChoice's catalogue check when
// the tool wrote it — that is the "整组拒绝" rule for an unknown slug — so a
// miss here only means the option was never about an article, and the option
// stays a plain button.
//
// libraryArticleDTOFor (library.go) is the same call getLibraryShelf makes,
// so this card's cover link is signed exactly the way the shelf's is.
func (a *API) liteWorkspaceChoiceArticle(slug string) *liteworkspace.ChoiceArticle {
	if slug == "" {
		return nil
	}
	art, ok := library.BySlug(slug)
	if !ok {
		return nil
	}
	dto := a.libraryArticleDTOFor(art)
	return &liteworkspace.ChoiceArticle{
		Slug: dto.Slug, ZhTitle: dto.ZhTitle, CoverURL: dto.CoverURL, Reason: dto.Reason,
	}
}

// runLiteWorkspaceLoop runs the bounded tool loop and returns the reply the
// teacher sees. Every model call inside it is metered, including a call that
// produced nothing usable — it was paid for either way.
func (a *API) runLiteWorkspaceLoop(ctx context.Context, r *http.Request, surfaceName string, userID uuid.UUID, resolved gateway.Resolved, msgs []gateway.ChatMessage, surface liteWorkspaceSurface, logNames []string) (string, []liteworkspace.Choice, *httpx.APIError) {
	tools := surface.tools()
	rewrites := 0
	for i := 0; i < liteworkspace.ToolLoopMax; i++ {
		res, err := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{Messages: msgs, Tools: tools})
		a.recordLiteLLMCall(ctx, userID, uuid.Nil, liteTeacherWorkspacePurpose, resolved, res.Usage)
		if err != nil {
			return "", nil, errLiteWorkspaceTurn(err.Error())
		}
		var reply string
		var choices []liteworkspace.Choice
		if len(res.ToolCalls) == 0 {
			// A stop with neither tools nor text is a dead turn. Say so rather
			// than render an empty bubble she would answer into.
			if strings.TrimSpace(res.Text) == "" {
				return "", nil, errLiteWorkspaceTurn("模型没有返回内容")
			}
			reply = res.Text
			msgs = append(msgs, gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text})
		} else {
			msgs = append(msgs, gateway.ChatMessage{
				Role: gateway.RoleAssistant, Content: res.Text, ToolCalls: res.ToolCalls,
			})
			for _, tc := range res.ToolCalls {
				msgs = append(msgs, gateway.ChatMessage{
					Role: gateway.RoleTool, ToolCallID: tc.ID, Content: surface.execute(tc),
				})
			}
			var done bool
			// A tool that ends the turn (ask_choice) supplies the reply: the
			// question IS the reply.
			if reply, choices, done = surface.ended(); !done {
				continue
			}
		}

		// The turn is finished. Before it goes out, check it does not say
		// something happened that no tool did (liteworkspace/claims.go). The
		// prompt already forbids these; production showed it is not enough.
		// One rewrite, then the turn fails with the reason: a false claim is
		// never shown to her.
		reason := surface.falseClaim(liteWorkspaceClaimText(reply, choices))
		if reason == "" {
			return reply, choices, nil
		}
		slog.Warn("lite workspace: reply made a false claim",
			"request_id", httpx.RequestIDFromContext(r.Context()), "surface", surfaceName,
			"attempt", rewrites+1, "reason", reason,
			"text", liteworkspace.RedactNames(liteWorkspaceClampRunes(liteWorkspaceClaimText(reply, choices), 2000), logNames))
		if rewrites >= liteWorkspaceClaimRewrites {
			return "", nil, errLiteWorkspaceTurn(reason)
		}
		rewrites++
		surface.clearEnded()
		msgs = append(msgs, gateway.ChatMessage{
			Role: gateway.RoleUser, Content: "上一条回复没有通过检查，老师没有看到它：" + reason + "。" +
				"这条检查不是老师说的话。请重新回复老师，不要提到上一条回复，也不要道歉。",
		})
	}
	return "", nil, errLiteWorkspaceTurn("工具调用次数超出上限")
}

// liteWorkspaceClaimRewrites is how many times a turn may be rewritten after
// falseClaim fires.
const liteWorkspaceClaimRewrites = 1

// liteWorkspaceClaimText is what falseClaim reads: the reply and every option
// label, one per line.
func liteWorkspaceClaimText(reply string, choices []liteworkspace.Choice) string {
	parts := []string{reply}
	for _, c := range choices {
		parts = append(parts, c.Label)
	}
	return strings.Join(parts, "\n")
}

// liteWorkspaceRosterPronounProblem is falseClaim's pronoun check for a
// surface about a class: a gendered pronoun is allowed only for a gender a
// student named this turn has, or one the teacher used herself.
func liteWorkspaceRosterPronounProblem(text string, roster []liteworkspace.Student, named []string, typed string) string {
	var p liteworkspace.PronounsAllowed
	p.AllowPronounsOf(roster, named)
	p.AllowTyped(typed)
	return liteworkspace.PronounProblem(text, p)
}

// liteWorkspaceOpenedPageClaim is falseClaim's answer for a reply that says
// a page was opened. It is the same on every surface: none of them opens a
// page.
func liteWorkspaceOpenedPageClaim(text string) string {
	if liteworkspace.ClaimsOpenedPage(text) {
		return "回复说页面已经打开，但没有任何页面被打开。open_page 只在回复下方放一个按钮，老师点了才会跳转，请说「请点击下方按钮前往」"
	}
	return ""
}

func liteWorkspaceToolError(msg string) string {
	b, _ := json.Marshal(map[string]any{"ok": false, "error": msg})
	return string(b)
}

func liteWorkspaceToolOK(v map[string]any) string {
	if v == nil {
		v = map[string]any{}
	}
	v["ok"] = true
	b, _ := json.Marshal(v)
	return string(b)
}

// toolString reads a string argument, trimmed. ok=false when the key is absent
// or holds another type.
func toolString(args map[string]any, key string) (string, bool) {
	v, present := args[key]
	if !present {
		return "", false
	}
	s, isString := v.(string)
	if !isString {
		return "", false
	}
	return strings.TrimSpace(s), true
}

// toolInt reads a number argument. JSON numbers arrive as float64.
func toolInt(args map[string]any, key string) (int, bool) {
	v, present := args[key]
	if !present {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

func toolStrings(args map[string]any, key string) []string {
	raw, _ := args[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// liteWorkspaceListStudentsTool runs list_students' closed-set filter against
// a roster. Both surfaces (assignment and home) call this — same filter
// vocabulary, same result shape, same evidence — so "本周未活跃" cannot mean
// one list on one page and a different list on the other.
//
// ok=false means args did not name a real filter; result is already the tool
// error to return, and the other return values are zero. ok=true means
// result is the tool's success JSON and names/count/card are the §6 evidence
// and canvas row the caller records.
func liteWorkspaceListStudentsTool(roster []liteworkspace.Student, args map[string]any) (result string, names []string, count int, card liteWorkspaceCardDTO, ok bool) {
	raw, _ := toolString(args, "filter")
	filter, valid := liteworkspace.ParseStudentFilter(raw)
	if !valid {
		return liteWorkspaceToolError("没有这个条件：" + raw + "，只能是 all、inactive_this_week、has_overdue、no_writing_yet"),
			nil, 0, liteWorkspaceCardDTO{}, false
	}
	rows := liteworkspace.FilterStudents(roster, filter)
	out := make([]map[string]any, 0, len(rows))
	names = make([]string, 0, len(rows))
	for _, s := range rows {
		out = append(out, map[string]any{"id": s.ID, "name": s.Name, "称谓": liteworkspace.Pronoun(s.Gender)})
		names = append(names, s.Name)
	}
	card = liteWorkspaceCardDTO{Kind: "students", Rows: rows, Filter: string(filter)}
	result = liteWorkspaceToolOK(map[string]any{"filter": string(filter), "students": out, "count": len(out)})
	return result, names, len(out), card, true
}

// liteWorkspaceAskChoiceArgs parses ask_choice's arguments: the question and
// 2 to 4 options. Both surfaces call this — the id/label/2-to-4 rule is one
// rule, not two.
//
// withSlug controls whether an option's slug is read and checked against the
// article catalogue: only the assignment surface's options can mean "use
// this article". errMsg == "" means ok; otherwise question and choices are
// zero and errMsg is the tool error text to return.
func liteWorkspaceAskChoiceArgs(args map[string]any, withSlug bool) (question string, choices []liteworkspace.Choice, errMsg string) {
	question, _ = toolString(args, "question")
	if question == "" {
		return "", nil, "没有给出问题"
	}
	raw, _ := args["options"].([]any)
	out := make([]liteworkspace.Choice, 0, len(raw))
	for _, item := range raw {
		obj, isObject := item.(map[string]any)
		if !isObject {
			continue
		}
		id, _ := toolString(obj, "id")
		label, _ := toolString(obj, "label")
		if id == "" || label == "" {
			continue
		}
		choice := liteworkspace.Choice{ID: id, Label: label}
		if withSlug {
			// A slug is checked HERE, while the turn still has budget. See
			// liteWorkspaceRun.askChoice's original comment: finding out two
			// turns later that "use this article" cannot survive is what this
			// check exists to stop.
			if slug, given := toolString(obj, "slug"); given && slug != "" {
				art, found := library.BySlug(slug)
				if !found {
					return "", nil, "选项 " + id + " 的 slug 不在阅读库里：" + slug +
						"。slug 必须原样复制 search_library 结果里的那一个，不能按标题自己拼"
				}
				choice.Slug = art.Slug
			}
		}
		out = append(out, choice)
	}
	if len(out) < 2 {
		return "", nil, "请给出 2 到 4 个选项"
	}
	return question, out, ""
}

// liteWorkspaceClampRunes truncates to max runes. Each caller passes the cap
// that the publish endpoint enforces for THAT field — the two differ by an
// order of magnitude.
func liteWorkspaceClampRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
