package api

import (
	"context"
	"encoding/json"
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

// liteWorkspaceTurnRequest is §4.4's request. reportId is not read yet: it
// belongs to the parentReport surface, which arrives with D3.
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

// liteWorkspaceCardDTO is one tool result on the canvas. kind is "students" or
// "articles" today; D2 adds more.
type liteWorkspaceCardDTO struct {
	Kind string `json:"kind"`
	Rows any    `json:"rows"`
}

// liteWorkspaceSurface is what one page of the workspace contributes to a
// turn. Everything else — metering, the loop bound, the §6 checks, slug
// replacement, option clamping — belongs to the shared handler, so a new
// surface cannot skip any of it.
//
// A surface value lives for one turn. The handler builds it after the
// ownership check, calls begin once, then system and tools, then execute for
// every tool call the model makes, and reads the rest after the loop.
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
	// checkedParts is every string this turn puts in front of the teacher,
	// one per field, for the §6 checks. reply and choices are the loop's
	// output; the surface adds what its tools wrote.
	checkedParts(reply string, choices []liteworkspace.Choice) []string
	// groundedNames and groundedCounts are what this turn's tools returned:
	// the tool half of the §6 evidence. The handler adds her own words and
	// the roster size.
	groundedNames() []string
	groundedCounts() []int
	// result is what goes back to the canvas: the fields a tool wrote and the
	// tool results the panel renders. Either may be nil.
	result() (patch map[string]any, cards []liteWorkspaceCardDTO)
}

// liteWorkspaceSurfaceOpen reports whether a surface is served yet. home and
// parentReport (§5.2, §5.3) carry different tools and a different canvas, so
// accepting their names before their surfaces exist would answer with the
// wrong tool set rather than say no.
func liteWorkspaceSurfaceOpen(surface string) bool {
	switch surface {
	case "assignment":
		return true
	}
	return false
}

// newLiteWorkspaceSurface builds the surface req.Surface names, for one turn.
// The handler has already checked liteWorkspaceSurfaceOpen, so every name that
// reaches here has a case; home and parentReport add theirs with D2/D3.
func (a *API) newLiteWorkspaceSurface(mctx context.Context, cls sqlc.Class, roster []liteworkspace.Student, req liteWorkspaceTurnRequest, typed string) liteWorkspaceSurface {
	// "assignment" is the only open surface, so this is not a switch yet.
	return a.newLiteWorkspaceAssignment(mctx, cls, roster, req.Artifact, typed)
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
	if !liteWorkspaceSurfaceOpen(req.Surface) {
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

	surface := a.newLiteWorkspaceSurface(mctx, cls, roster, req, typed)
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

	reply, choices, aerr := a.runLiteWorkspaceLoop(mctx, u.ID, resolved, msgs, surface)
	if aerr != nil {
		httpx.WriteError(w, r, aerr)
		return
	}

	// 🚨 grounded excludes the model's own earlier turns on purpose. A
	// fabricated name's source IS the model's earlier words, so feeding them
	// back as evidence would launder exactly the failure this check exists to
	// catch (§6).
	grounded := append([]string{}, surface.groundedNames()...)
	grounded = append(grounded, liteWorkspaceNamesTeacherTyped(roster, turns, typed)...)

	choices = liteworkspace.ClampChoices(choices)
	// The name check reads every field as one blob. It compares whole names, so
	// a name cannot be assembled out of the end of one field and the start of
	// the next, and joining costs nothing.
	parts := surface.checkedParts(reply, choices)
	if bad := liteworkspace.UngroundedNames(
		strings.Join(parts, "\n"), liteWorkspaceRosterNames(roster), grounded,
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
	//
	// 🚨 Field by field, never joined. UngroundedCounts strips whitespace before
	// matching, so any separator a join could use dissolves, and a 说明 ending
	// 「难度 3」 beside a label starting 「人工智能方向」 became a claim about
	// 3 people that neither field made.
	countGrounds := liteWorkspaceGroundedCounts(surface.groundedCounts(), len(roster), turns, typed)
	for _, part := range parts {
		if bad := liteworkspace.UngroundedCounts(part, countGrounds); len(bad) > 0 {
			httpx.WriteError(w, r, errLiteWorkspaceTurn("回复里出现了本轮没有依据的人数："+liteWorkspaceJoinInts(bad)))
			return
		}
	}

	// Slug replacement runs AFTER both §6 checks, never before: those checks
	// have to see the model's own words, and a slug the model invented (one
	// that library.BySlug cannot find) is left exactly as it is written, which
	// is what makes it visible to a human reading the failure later.
	reply = liteWorkspaceDeslugged(reply)
	for i := range choices {
		choices[i].Label = liteWorkspaceDeslugged(choices[i].Label)
		// Article is enriched from the catalogue, not from the model's words,
		// so it runs on the untouched Slug rather than on liteWorkspaceDeslugged's
		// output.
		choices[i].Article = a.liteWorkspaceChoiceArticle(choices[i].Slug)
	}

	// An empty patch and an empty card list go out as {} and [], not null: the
	// client walks both on every turn, and a null would make "no tool wrote
	// anything" a separate case at every call site.
	patch, cards := surface.result()
	if patch == nil {
		patch = map[string]any{}
	}
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
// already proved "text" is a literal substring of it. The real reason is
// narrower: the count check does not see the SUBSTRING RELATIONSHIP, only
// the digits in front of it. A cut can start mid-number — typed says
// 「1200人」, the model's substring starts at 「200人」, still a real
// substring — and StatedCounts(text) then reads a head count (200) that was
// never stated by anyone and is not in typed's own count list (1200). That
// is not a fabrication; it is where the paste happened to be cut. Checking
// "text" against §6 would fail a turn over content already proven honest,
// so it is not checked at all.

func liteWorkspaceCheckedParts(reply string, choices []liteworkspace.Choice, patch map[string]any) []string {
	out := []string{reply}
	for _, c := range choices {
		// The slug is checked too. It is validated against the catalogue
		// before it gets here, so it cannot carry a fabricated name today —
		// but it travels to the client and back like the id does, and a field
		// that is exempt from the check is a field someone will later widen.
		out = append(out, c.ID, c.Label, c.Slug)
	}
	for k, v := range patch {
		if k == "text" {
			continue
		}
		switch value := v.(type) {
		case string:
			out = append(out, value)
		case []string:
			out = append(out, value...)
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
func liteWorkspaceGroundedCounts(fromTools []int, rosterSize int, turns []liteworkspace.Turn, typed string) []int {
	out := append([]int{}, fromTools...)
	out = append(out, rosterSize)
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
func (a *API) runLiteWorkspaceLoop(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, msgs []gateway.ChatMessage, surface liteWorkspaceSurface) (string, []liteworkspace.Choice, *httpx.APIError) {
	tools := surface.tools()
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
				Role: gateway.RoleTool, ToolCallID: tc.ID, Content: surface.execute(tc),
			})
		}
		if reply, choices, done := surface.ended(); done {
			// A tool that ends the turn (ask_choice) supplies the reply: the
			// question IS the reply.
			return reply, choices, nil
		}
	}
	return "", nil, errLiteWorkspaceTurn("工具调用次数超出上限")
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
