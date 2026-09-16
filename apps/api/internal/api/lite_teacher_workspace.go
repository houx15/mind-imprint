package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// lite_teacher_workspace.go — 教师工作台的一轮：左边对话，右边的作业卡由工具写。
//
// This is the only teacher route that calls a model synchronously. It runs a
// bounded tool loop on the dialogue tier: the model may reach for the six
// tools in liteworkspace.AssignmentTools, the server executes them and feeds
// the results back inside the same turn, and the loop stops after
// liteworkspace.ToolLoopMax MODEL CALLS — one of which has to be the call that
// answers, so the tool budget is one less than that number.
//
// 🚨 教师可见性边界：没有一个工具读学生与印记的对话记录。The tools here reach
// the roster, the assignment rows behind it and the embedded reading library —
// atom_message is not queried from this file, and a tool added later must keep
// it that way.

// liteTeacherWorkspacePurpose is the llm_call purpose for every model call a
// workspace turn makes, including the ones inside the tool loop.
const liteTeacherWorkspacePurpose = "lite_teacher_workspace"

// liteWorkspaceSearchLimit bounds what search_library hands the model. Eight
// articles is what §5.1 specifies; the whole catalogue would crowd out the
// transcript and buy nothing — she is choosing one.
const liteWorkspaceSearchLimit = 8

// liteWorkspaceMaxInstructionsRunes bounds the instructions a tool writes into
// the card. It is liteassign's own cap (maxInstructionsRunes, unexported, in
// liteassign/payload.go), which ValidateInstructions enforces at publish.
//
// The title has a DIFFERENT and much smaller cap: maxAssignmentTitleRunes
// (200, in lite_teacher_assignments.go), enforced by parseAssignmentTitle.
// One shared cap would let a tool write a 2000-rune title that the card
// displays and the publish endpoint then rejects with 「请填写作业标题，不超过
// 200 字」 — a failure at the last step, over a value we handed her ourselves.
const liteWorkspaceMaxInstructionsRunes = 2000

// liteWorkspaceTurnRequest is §4.4's request. reportId is not read yet: only
// the assignment surface exists, and the other two arrive with D2/D3.
type liteWorkspaceTurnRequest struct {
	Surface  string               `json:"surface"`
	ClassID  string               `json:"classId"`
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
}

// liteWorkspaceCardDTO is one tool result on the canvas. kind is "students"
// today; D2 adds more.
type liteWorkspaceCardDTO struct {
	Kind string `json:"kind"`
	Rows any    `json:"rows"`
}

// liteWorkspaceArtifact is the part of the assignment draft the model is shown.
// The draft carries more (rubric, personalized picks, saved picks), and none of
// it is the model's to read or write — projecting here keeps the prompt bounded
// and keeps a field the model cannot change out of its sight.
type liteWorkspaceArtifact struct {
	Kind          string   `json:"kind"`
	Title         string   `json:"title"`
	Instructions  string   `json:"instructions"`
	DueInput      string   `json:"dueInput"`
	ReadingSource string   `json:"readingSource"`
	Slug          string   `json:"slug"`
	Tier          *int     `json:"tier"`
	UserIDs       []string `json:"userIds"`
}

// authTeacherClassFromBody is authTeacherClass for a route whose class id
// travels in the request body rather than in the path. Same ownership query,
// same not-found answer for a class the caller does not teach.
//
// A sibling rather than a parameter on authTeacherClass: that helper reads
// r.PathValue("id") and four shipped routes depend on it.
func (a *API) authTeacherClassFromBody(w http.ResponseWriter, r *http.Request, raw string) (sqlc.Class, bool) {
	classID, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Class{}, false
	}
	cls, err := a.assertTeacherOwnsClass(r.Context(), classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Class{}, false
	}
	return cls, true
}

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
	// Only the assignment surface exists. home and parentReport (§5.2, §5.3)
	// carry different tools and a different artifact, so accepting their names
	// here would answer with the wrong tool set rather than say no.
	if req.Surface != "assignment" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_surface", "该工作台尚未开放："+req.Surface, nil))
		return
	}
	cls, ok := a.authTeacherClassFromBody(w, r, req.ClassID)
	if !ok {
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
	roster, err := a.liteWorkspaceRoster(ctx, cls.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(mctx, gateway.ClassDialogue)
	if err != nil {
		// The cause goes out, not a bare 500. A routing failure is a
		// misconfigured model binding, and 「服务器内部错误」 would send whoever
		// is on call reading logs to learn what this line already knows. No
		// key reaches this string: Resolved carries the key, the error does not.
		httpx.WriteError(w, r, errLiteWorkspaceTurn(err.Error()))
		return
	}

	// The card's kind decides which cells exist, so the run has to know it
	// before any tool writes one. A card with no kind yet is a reading card:
	// that is what the client's own empty draft is (assignmentLogic's
	// emptySettings), and a first turn must not be told it cannot set a
	// material.
	card := liteWorkspaceParseArtifact(req.Artifact)
	run := &liteWorkspaceRun{
		roster: roster, kind: card.Kind,
		materialSet: card.Slug != "" || card.ReadingSource == "personalized",
	}
	if run.kind == "" {
		run.kind = "reading"
	}
	// An option she tapped that carried an article sets the material before the
	// model runs, through the same setMaterial every other path uses. The model
	// is then told it is done rather than asked to do it: the old route was
	// three model calls — search for its own choice id, fail, search again —
	// and this one is none.
	chosen := liteWorkspaceChosenArticle(run, req)

	turns := liteworkspace.TrimTurns(req.Turns)
	// run.patch is passed so the card the model reads is the card as it stands
	// NOW, including what tapping an option just wrote. Showing it the pre-turn
	// card instead put the note 「材料已经设成这一篇了」 next to a card whose
	// material cell was empty, and it believed the cell: every run re-set the
	// material by hand, which is the round trip the option payload removes.
	msgs := liteWorkspaceMessages(cls.Name, roster, req.Artifact, run.patch, turns, said+chosen)

	reply, choices, aerr := a.runLiteWorkspaceLoop(mctx, u.ID, resolved, msgs, run)
	if aerr != nil {
		httpx.WriteError(w, r, aerr)
		return
	}

	// 🚨 grounded excludes the model's own earlier turns on purpose. A
	// fabricated name's source IS the model's earlier words, so feeding them
	// back as evidence would launder exactly the failure this check exists to
	// catch (§6).
	grounded := append([]string{}, run.namesReturned...)
	grounded = append(grounded, liteWorkspaceNamesTeacherTyped(roster, turns, typed)...)

	choices = liteworkspace.ClampChoices(choices)
	checked := liteWorkspaceCheckedText(reply, choices, run.patch)
	if bad := liteworkspace.UngroundedNames(
		checked, liteWorkspaceRosterNames(roster), grounded,
	); len(bad) > 0 {
		httpx.WriteError(w, r, errLiteWorkspaceTurn("回复里出现了本轮没有依据的学生姓名："+strings.Join(bad, "、")))
		return
	}

	// The same rule for the other half of §6. A name is checked against the
	// roster; a head count had nothing but the prompt behind it, and a live run
	// showed the prompt does not always hold — 「发给全班 3 人」 reached a teacher
	// with no tool having counted anything. A wrong number about her own class
	// is the failure this section exists to prevent, so it fails the turn the
	// way a fabricated name does.
	if bad := liteworkspace.UngroundedCounts(checked, liteWorkspaceGroundedCounts(run, turns, typed)); len(bad) > 0 {
		httpx.WriteError(w, r, errLiteWorkspaceTurn("回复里出现了本轮没有依据的人数："+liteWorkspaceJoinInts(bad)))
		return
	}

	// An empty patch and an empty card list go out as {} and [], not null: the
	// client walks both on every turn, and a null would make "no tool wrote
	// anything" a separate case at every call site.
	patch := run.patch
	if patch == nil {
		patch = map[string]any{}
	}
	cards := run.cards
	if cards == nil {
		cards = []liteWorkspaceCardDTO{}
	}
	httpx.WriteJSON(w, http.StatusOK, liteWorkspaceTurnDTO{
		Reply: reply, Choices: choices, Patch: patch, Cards: cards,
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
			ActiveDaysThisWeek: int(row.ActiveDaysThisWeek),
			OverdueAssignments: int(overdue[row.ID]),
			WritingsDone:       int(row.WritingsDone),
		})
	}
	return out, nil
}

func liteWorkspaceRosterNames(roster []liteworkspace.Student) []string {
	out := make([]string, 0, len(roster))
	for _, s := range roster {
		out = append(out, s.Name)
	}
	return out
}

// liteWorkspaceCheckedText is everything this turn puts in front of the
// teacher, joined for one name check.
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
func liteWorkspaceCheckedText(reply string, choices []liteworkspace.Choice, patch map[string]any) string {
	var b strings.Builder
	b.WriteString(reply)
	for _, c := range choices {
		b.WriteString("\n")
		b.WriteString(c.ID)
		b.WriteString("\n")
		b.WriteString(c.Label)
		// The slug is checked too. It is validated against the catalogue
		// before it gets here, so it cannot carry a fabricated name today —
		// but it travels to the client and back like the id does, and a field
		// that is exempt from the check is a field someone will later widen.
		b.WriteString("\n")
		b.WriteString(c.Slug)
	}
	for _, v := range patch {
		switch value := v.(type) {
		case string:
			b.WriteString("\n")
			b.WriteString(value)
		case []string:
			for _, s := range value {
				b.WriteString("\n")
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

// liteWorkspaceNamesTeacherTyped returns the roster names the TEACHER wrote,
// this turn or earlier in the thread. Her own words are evidence; the model's
// are not, so ai turns are skipped here.
//
// The teacher turns in `turns` are client-supplied and could in principle
// carry a string the model minted (a clicked option). That door is shut on the
// way out instead: liteWorkspaceCheckedText checks option ids and labels
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

// liteWorkspaceParseArtifact projects the draft the client sent onto the part
// of it this endpoint reads. An absent or unparseable artifact is the zero
// value, not an error: a first turn has no card yet.
func liteWorkspaceParseArtifact(raw json.RawMessage) liteWorkspaceArtifact {
	var art liteWorkspaceArtifact
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &art)
	}
	return art
}

// liteWorkspaceChosenArticle applies the article an option carried, when she
// tapped one, and returns the line telling the model it is already done.
//
// It runs through run.setMaterial rather than writing the patch itself: what
// "the material is this article" means to the card must have one implementation,
// or the tool path and the click path drift apart over a field the class reads.
// An unknown slug is ignored in silence — the tool result goes nowhere here,
// and a click is not a turn the teacher can be shown an error for.
func liteWorkspaceChosenArticle(run *liteWorkspaceRun, req liteWorkspaceTurnRequest) string {
	slug := strings.TrimSpace(req.ChoiceSlug)
	if slug == "" || strings.TrimSpace(req.Text) != "" {
		return ""
	}
	art, found := library.BySlug(slug)
	if !found {
		return ""
	}
	// 🚨 Not a back door. The same kind guard as the tool path: a writing card
	// has no material row, so applying it here would put the article exactly
	// where the browser pass found it — nowhere — while the note below told the
	// model it had landed. The note says what really happened either way.
	if run.kind != "reading" {
		return "\n（她点的这个选项是库里的《" + art.Title + "》，但现在这份作业是" + run.kind +
			"，没有阅读材料这一栏，所以材料没有设上。要用这篇就先把类型设成 reading，再设材料。）"
	}
	run.setMaterial(map[string]any{"source": "library", "slug": art.Slug})
	return "\n（她点的这个选项对应库里的《" + art.Title + "》，材料已经设成这一篇了，不用再查一次。）"
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
func liteWorkspaceGroundedCounts(run *liteWorkspaceRun, turns []liteworkspace.Turn, typed string) []int {
	out := append([]int{}, run.countsReturned...)
	out = append(out, len(run.roster))
	out = append(out, liteworkspace.StatedCounts(typed)...)
	for _, t := range turns {
		if t.Role == "teacher" {
			out = append(out, liteworkspace.StatedCounts(t.Text)...)
		}
	}
	return out
}

func liteWorkspaceJoinInts(ns []int) string {
	parts := make([]string, 0, len(ns))
	for _, n := range ns {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, "、")
}

// liteWorkspaceMessages builds the prompt: one system message carrying the
// rules plus the card as it stands, then the trimmed transcript, then what she
// just said.
//
// The card state goes in because she can edit any cell while a turn is in
// flight. A model that cannot see the card describes changes it did not make.
func liteWorkspaceMessages(className string, roster []liteworkspace.Student, artifact json.RawMessage, applied map[string]any, turns []liteworkspace.Turn, said string) []gateway.ChatMessage {
	system := liteworkspace.AssignmentSystem(liteworkspace.SystemContext{
		ClassName:    className,
		TodayBeijing: time.Now().In(liteworkspace.BeijingOffset).Format("2006-01-02"),
		StudentCount: len(roster),
	})
	if card := liteWorkspaceCardState(artifact, applied); card != "" {
		system += "\n\n## 作业卡现在的内容\n\n" + card
	}
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

// liteWorkspaceCardState renders the card's filled cells. An unparseable or
// absent artifact renders nothing: a first turn has no card yet, and that is
// not an error.
//
// applied is what this turn already wrote before the model ran — today, the
// article she tapped. It is overlaid by marshalling it back over the parsed
// artifact rather than by copying field by field: json.Unmarshal only touches
// the keys that are present, and the patch's keys are the artifact's own tags,
// so a field added to either side is covered without anyone editing this.
func liteWorkspaceCardState(raw json.RawMessage, applied map[string]any) string {
	if len(raw) == 0 && len(applied) == 0 {
		return ""
	}
	art := liteWorkspaceParseArtifact(raw)
	if len(applied) > 0 {
		if b, err := json.Marshal(applied); err == nil {
			_ = json.Unmarshal(b, &art)
		}
	}
	var lines []string
	add := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			lines = append(lines, "- "+label+"："+value)
		}
	}
	add("种类", art.Kind)
	add("标题", art.Title)
	add("说明", art.Instructions)
	add("截止时间", art.DueInput)
	add("材料来源", art.ReadingSource)
	add("文章 slug", art.Slug)
	if art.Tier != nil {
		add("难度档", fmt.Sprint(*art.Tier))
	}
	if n := len(art.UserIDs); n > 0 {
		add("已选学生", fmt.Sprintf("%d 名", n))
	}
	if len(lines) == 0 {
		return "（还是空的）"
	}
	return strings.Join(lines, "\n")
}

// liteWorkspaceRun accumulates what one turn's tools produced.
type liteWorkspaceRun struct {
	roster []liteworkspace.Student
	// kind is the card's homework type as it stands: seeded from the artifact,
	// updated when set_fields writes it. It decides which cells the card has,
	// so set_material reads it before writing a material into a card that has
	// nowhere to show one.
	kind string
	// materialSet is whether the card has a reading material right now,
	// seeded from the artifact and kept current by the tools. readingSource
	// alone cannot answer it: the client's empty draft already carries
	// "library" with no article behind it.
	materialSet bool
	// patch holds only the draft fields a tool actually wrote. The client
	// applies it field by field and drops the ones she edited meanwhile
	// (§4.5), which only works if an untouched field is absent, not zero.
	patch map[string]any
	cards []liteWorkspaceCardDTO
	// namesReturned is what list_students handed back this turn — the evidence
	// side of the grounding check.
	namesReturned []string
	// countsReturned is how many students each tool counted this turn. It is
	// the evidence side of the head-count check: a reply may state a number a
	// tool produced, and nothing else.
	countsReturned []int
	// question and choices are set by ask_choice, which ends the turn.
	question string
	choices  []liteworkspace.Choice
	asked    bool
}

func (run *liteWorkspaceRun) write(field string, value any) {
	if run.patch == nil {
		run.patch = map[string]any{}
	}
	run.patch[field] = value
}

// runLiteWorkspaceLoop runs the bounded tool loop and returns the reply the
// teacher sees. Every model call inside it is metered, including a call that
// produced nothing usable — it was paid for either way.
func (a *API) runLiteWorkspaceLoop(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, msgs []gateway.ChatMessage, run *liteWorkspaceRun) (string, []liteworkspace.Choice, *httpx.APIError) {
	tools := liteworkspace.AssignmentTools()
	for i := 0; i < liteworkspace.ToolLoopMax; i++ {
		res, err := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{Messages: msgs, Tools: tools})
		a.recordLiteLLMCall(ctx, userID, uuid.Nil, liteTeacherWorkspacePurpose, resolved, res.Usage)
		if err != nil {
			return "", nil, errLiteWorkspaceTurn(err.Error())
		}
		if len(res.ToolCalls) == 0 {
			// A stop with neither tools nor text is a dead turn. Say so rather
			// than render an empty bubble she would answer into.
			if strings.TrimSpace(res.Text) == "" {
				return "", nil, errLiteWorkspaceTurn("模型没有返回内容")
			}
			return res.Text, nil, nil
		}
		msgs = append(msgs, gateway.ChatMessage{
			Role: gateway.RoleAssistant, Content: res.Text, ToolCalls: res.ToolCalls,
		})
		for _, tc := range res.ToolCalls {
			msgs = append(msgs, gateway.ChatMessage{
				Role: gateway.RoleTool, ToolCallID: tc.ID, Content: run.execute(tc),
			})
		}
		if run.asked {
			// ask_choice ends the turn: the question IS the reply.
			return run.question, run.choices, nil
		}
	}
	return "", nil, errLiteWorkspaceTurn("工具调用次数超出上限")
}

// execute runs one tool call and returns the tool result the model reads next.
//
// A bad argument comes back as a tool result, not as a failed turn: the model
// minted it and can fix it on the next round, and liteworkspace.ToolLoopMax
// bounds how long it may keep trying. A failure the model cannot fix (a model
// or transport failure) ends the turn instead.
func (run *liteWorkspaceRun) execute(tc gateway.ToolCall) string {
	switch tc.Name {
	case "set_fields":
		return run.setFields(tc.Args)
	case "search_library":
		return run.searchLibrary(tc.Args)
	case "set_material":
		return run.setMaterial(tc.Args)
	case "list_students":
		return run.listStudents(tc.Args)
	case "set_recipients":
		return run.setRecipients(tc.Args)
	case "ask_choice":
		return run.askChoice(tc.Args)
	}
	return liteWorkspaceToolError("没有这个工具：" + tc.Name)
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

func (run *liteWorkspaceRun) setFields(args map[string]any) string {
	written := make([]string, 0, 4)
	cleared := false
	if kind, ok := toolString(args, "kind"); ok && kind != "" {
		switch kind {
		case "reading", "writing", "project":
			// 🚨 A card must never be left in a state it cannot render. Only a
			// reading homework has a material row, so turning one into a
			// writing homework has to take the material with it — otherwise
			// the article stays in the draft where nothing shows it, the model
			// goes on describing it, and she publishes a writing task whose
			// article silently is not there. That is the exact state the
			// browser pass photographed.
			//
			// Blocked-and-told is not an option here: there is no tool for
			// clearing a material, so refusing would leave the model with an
			// instruction it cannot carry out. Clearing and SAYING SO leaves
			// her a coherent card and the model something true to tell her.
			if kind != "reading" && run.materialSet {
				run.write("readingSource", "library")
				run.write("slug", "")
				run.write("tier", nil)
				run.materialSet = false
				cleared = true
			}
			run.write("kind", kind)
			run.kind = kind
			written = append(written, "kind")
		default:
			return liteWorkspaceToolError("作业种类只能是 reading、writing 或 project，收到：" + kind)
		}
	}
	if title, ok := toolString(args, "title"); ok && title != "" {
		run.write("title", liteWorkspaceClampRunes(title, maxAssignmentTitleRunes))
		written = append(written, "title")
	}
	if ins, ok := toolString(args, "instructions"); ok && ins != "" {
		run.write("instructions", liteWorkspaceClampRunes(ins, liteWorkspaceMaxInstructionsRunes))
		written = append(written, "instructions")
	}
	if due, ok := toolString(args, "dueAt"); ok && due != "" {
		// Parsed only to reject a relative phrase. What the card holds is the
		// Beijing wall-clock string the datetime input reads back, so the
		// instant is never converted twice.
		if _, err := liteworkspace.BeijingWallToUTC(due); err != nil {
			return liteWorkspaceToolError(err.Error() + "，请按 2006-01-02T15:04 写出北京时间的绝对时刻")
		}
		run.write("dueInput", due)
		written = append(written, "dueAt")
	}
	if len(written) == 0 {
		return liteWorkspaceToolError("没有给出任何字段")
	}
	out := map[string]any{"written": written}
	if cleared {
		out["note"] = "类型不是 reading 了，作业卡上没有阅读材料这一栏，原来选的文章已经清掉。" +
			"跟老师说清楚这件事，不要再提那篇文章；她要保留文章就把类型改回 reading"
	}
	return liteWorkspaceToolOK(out)
}

func (run *liteWorkspaceRun) searchLibrary(args map[string]any) string {
	query, _ := toolString(args, "query")
	tier, _ := toolInt(args, "tier")
	if tier < 0 || tier > 5 {
		return liteWorkspaceToolError("难度档需在 1 到 5 之间，0 表示不限")
	}
	arts := liteworkspace.SearchLibrary(library.All(), query, toolStrings(args, "disciplines"), tier, liteWorkspaceSearchLimit)
	rows := make([]map[string]any, 0, len(arts))
	for _, art := range arts {
		tiers := make([]int, 0, len(art.Levels))
		for _, l := range art.Levels {
			tiers = append(tiers, l.Tier)
		}
		rows = append(rows, map[string]any{
			"slug": art.Slug, "title": art.Title, "zhTitle": art.ZhTitle,
			"disciplines": art.Disciplines, "tiers": tiers,
		})
	}
	return liteWorkspaceToolOK(map[string]any{"articles": rows})
}

// errLiteWorkspaceWrongKind is the tool result for setting a material on a card
// that has no place to put one.
//
// 🚨 This is the defect the browser pass found and the live tests could not:
// the model set kind=writing, searched the library, and told the teacher
// 「材料：已选「美国气候队」这篇报道」 — while the card, which renders the
// material row only for a reading homework, showed nothing. Her students would
// have received a writing task with no article after she was told one was
// chosen. The claim was in prose; the truth was in a cell she could not see.
//
// The remedy is in the message because the model demonstrably acts on a tool
// error that names one: that is how it recovered from every invented slug.
func errLiteWorkspaceWrongKind(kind string) string {
	label := map[string]string{"writing": "写作", "project": "项目"}[kind]
	if label == "" {
		label = kind
	}
	return liteWorkspaceToolError(label + "作业没有阅读材料这一栏，材料设不上去。" +
		"请先用 set_fields 把类型设成 reading，再设材料；如果这次确实是" + label + "作业，就别提材料，也不要跟老师说已经选好了文章")
}

func (run *liteWorkspaceRun) setMaterial(args map[string]any) string {
	if run.kind != "reading" {
		return errLiteWorkspaceWrongKind(run.kind)
	}
	source, _ := toolString(args, "source")
	switch source {
	case "library":
		slug, _ := toolString(args, "slug")
		art, found := library.BySlug(slug)
		if !found {
			// Name the requirement, not just the failure. The slug is nearly
			// always a guess derived from the title; a message that only says
			// 「不在库里」 invites another guess, which is two more model calls.
			return liteWorkspaceToolError("文章不在阅读库里：" + slug +
				"。slug 必须原样复制 search_library 结果里的那一个，不能按标题自己拼。请先用 search_library 查，再把结果里的 slug 填进来")
		}
		run.write("readingSource", "library")
		run.write("slug", art.Slug)
		run.materialSet = true
		if tier, ok := toolInt(args, "tier"); ok {
			if _, has := art.LevelAt(tier); !has {
				return liteWorkspaceToolError("这篇文章没有这一档")
			}
			run.write("tier", tier)
		}
		return liteWorkspaceToolOK(map[string]any{"slug": art.Slug, "title": art.Title})
	case "personalized":
		// 🚨 The patch only. Who reads what is computed by the existing
		// personalized-reading preview endpoint, and a second implementation
		// here would be a second answer to the same question.
		run.write("readingSource", "personalized")
		run.materialSet = true
		return liteWorkspaceToolOK(map[string]any{"source": "personalized"})
	}
	return liteWorkspaceToolError("材料来源只能是 library 或 personalized，收到：" + source)
}

func (run *liteWorkspaceRun) listStudents(args map[string]any) string {
	raw, _ := toolString(args, "filter")
	filter, ok := liteworkspace.ParseStudentFilter(raw)
	if !ok {
		return liteWorkspaceToolError("没有这个条件：" + raw + "，只能是 all、inactive_this_week、has_overdue、no_writing_yet")
	}
	rows := liteworkspace.FilterStudents(run.roster, filter)
	out := make([]map[string]any, 0, len(rows))
	for _, s := range rows {
		out = append(out, map[string]any{"id": s.ID, "name": s.Name})
		run.namesReturned = append(run.namesReturned, s.Name)
	}
	run.cards = append(run.cards, liteWorkspaceCardDTO{Kind: "students", Rows: rows})
	run.countsReturned = append(run.countsReturned, len(out))
	return liteWorkspaceToolOK(map[string]any{"filter": string(filter), "students": out, "count": len(out)})
}

func (run *liteWorkspaceRun) setRecipients(args map[string]any) string {
	rawFilter, hasFilter := toolString(args, "filter")
	explicit := toolStrings(args, "userIds")
	if hasFilter && rawFilter != "" && len(explicit) > 0 {
		return liteWorkspaceToolError("filter 和 userIds 只能给一个：按条件发就只给 filter，指定人就只给 userIds")
	}
	if hasFilter && rawFilter != "" {
		return run.recipientsByFilter(rawFilter)
	}

	known := make(map[string]bool, len(run.roster))
	for _, s := range run.roster {
		known[s.ID] = true
	}
	ids := make([]string, 0, len(run.roster))
	seen := make(map[string]bool, len(run.roster))
	for _, id := range explicit {
		if !known[id] {
			// Name the shape, not just the miss. 「all」 arrived here as a user
			// id once, and a message that only says 「不在班里」 invites another
			// guess — which costs two more model calls in a turn budgeted for
			// six.
			return liteWorkspaceToolError("这个学生不在班里：" + id +
				"。userIds 必须是 list_students 返回的 id，原样复制；要发给一整类学生就改用 filter")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return liteWorkspaceToolError("请至少选择一名学生，或者给一个 filter")
	}
	run.recorded(ids)
	return liteWorkspaceToolOK(map[string]any{"count": len(ids)})
}

// recipientsByFilter resolves a closed-set condition straight to recipients.
// It goes through the same ParseStudentFilter and FilterStudents that
// list_students uses, so 「本周未活跃」 cannot come to mean one thing when she
// looks at the list and another when the assignment goes out.
//
// It also puts the matched students on the canvas, exactly as list_students
// does: she is about to send homework to a group she named by condition, and
// she should see who that turned out to be without asking.
func (run *liteWorkspaceRun) recipientsByFilter(raw string) string {
	filter, ok := liteworkspace.ParseStudentFilter(raw)
	if !ok {
		return liteWorkspaceToolError("没有这个条件：" + raw + "，只能是 all、inactive_this_week、has_overdue、no_writing_yet")
	}
	rows := liteworkspace.FilterStudents(run.roster, filter)
	if len(rows) == 0 {
		return liteWorkspaceToolError("这个条件下现在没有学生：" + raw + "，换一个条件，或者直接给 userIds")
	}
	ids := make([]string, 0, len(rows))
	for _, s := range rows {
		ids = append(ids, s.ID)
		run.namesReturned = append(run.namesReturned, s.Name)
	}
	run.cards = append(run.cards, liteWorkspaceCardDTO{Kind: "students", Rows: rows})
	run.recorded(ids)
	return liteWorkspaceToolOK(map[string]any{"filter": string(filter), "count": len(ids)})
}

// recorded writes the recipients and remembers how many there are. The count
// is evidence: a reply may state a number the tools produced, and only that.
func (run *liteWorkspaceRun) recorded(ids []string) {
	run.write("userIds", ids)
	run.countsReturned = append(run.countsReturned, len(ids))
}

func (run *liteWorkspaceRun) askChoice(args map[string]any) string {
	question, _ := toolString(args, "question")
	if question == "" {
		return liteWorkspaceToolError("没有给出问题")
	}
	raw, _ := args["options"].([]any)
	choices := make([]liteworkspace.Choice, 0, len(raw))
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
		// A slug is checked HERE, while the turn still has budget. The point
		// of the field is that "use this article" survives to the next turn;
		// a slug that is not in the catalogue would not survive anything, and
		// finding that out two turns later is what this field exists to stop.
		if slug, given := toolString(obj, "slug"); given && slug != "" {
			art, found := library.BySlug(slug)
			if !found {
				return liteWorkspaceToolError("选项 " + id + " 的 slug 不在阅读库里：" + slug +
					"。slug 必须原样复制 search_library 结果里的那一个，不能按标题自己拼")
			}
			choice.Slug = art.Slug
		}
		choices = append(choices, choice)
	}
	if len(choices) < 2 {
		return liteWorkspaceToolError("请给出 2 到 4 个选项")
	}
	run.question, run.choices, run.asked = question, choices, true
	return liteWorkspaceToolOK(map[string]any{"options": len(choices)})
}
