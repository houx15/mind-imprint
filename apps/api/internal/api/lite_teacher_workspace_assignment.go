package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// lite_teacher_workspace_assignment.go — the assignment surface of the
// teacher workspace: its tools, its card, and the checks those tools make
// before they write a cell. The loop, metering and §6 checks that every
// surface shares live in lite_teacher_workspace.go.

// The assignment surface is the first liteWorkspaceSurface.
var _ liteWorkspaceSurface = (*liteWorkspaceRun)(nil)

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

// liteWorkspaceMaxTextRunes is set_material's "text" source cap. It is
// liteassign's own cap (maxTextRunes, unexported, in liteassign/payload.go),
// which the traditional form's validateSettings enforces client-side and
// ValidatePayload enforces again at publish — the same number, so a tool
// writing a longer text would pass the card and fail at the last step.
const liteWorkspaceMaxTextRunes = 50000

// liteWorkspaceMinTextRunes is set_material's "text" source floor, applied to
// the passage between the two anchors. Nothing this short is a reading
// material — it is the model pointing at a fragment (a headline, or anchors
// taken from one sentence) rather than at the article she pasted. There is no matching floor on the
// traditional form: she pastes an article by hand and would notice an empty
// box, but a tool result silently writing three words onto the card would
// not be noticed the same way.
const liteWorkspaceMinTextRunes = 20

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
	// Writing: the client draft keeps targetWords as the raw input string.
	Prompt      string `json:"prompt"`
	TargetWords string `json:"targetWords"`
	Lang        string `json:"lang"`
	// Project.
	DrivingQuestion string `json:"drivingQuestion"`
	Description     string `json:"description"`
}

// liteWorkspaceCard is the card as it stands after this turn's writes.
func liteWorkspaceCard(raw json.RawMessage, applied map[string]any) liteWorkspaceArtifact {
	art := liteWorkspaceParseArtifact(raw)
	if len(applied) > 0 {
		if b, err := json.Marshal(applied); err == nil {
			_ = json.Unmarshal(b, &art)
		}
	}
	return art
}

// liteWorkspaceRequiredFields are the cells a homework of this kind cannot
// be published without, with the value each has now. The labels are the
// card's own and the words the model uses about them.
func liteWorkspaceRequiredFields(art liteWorkspaceArtifact) []liteworkspace.CardField {
	fields := []liteworkspace.CardField{
		{Label: "标题", Value: art.Title},
		{Label: "截止时间", Value: art.DueInput},
	}
	switch art.Kind {
	case "writing":
		fields = append(fields,
			liteworkspace.CardField{Label: "题目", Value: art.Prompt},
			liteworkspace.CardField{Label: "目标字数", Value: art.TargetWords})
	case "project":
		fields = append(fields, liteworkspace.CardField{Label: "驱动问题", Value: art.DrivingQuestion})
	}
	return fields
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
	//
	// Chinese words only (§12.1): this note is appended to her turn, and the
	// model repeats what it reads, so a raw kind or an English title here
	// ends up in the reply.
	if run.kind != "reading" {
		return "\n（她点的这个选项是库里的《" + art.ZhTitle + "》，但现在这份作业是" + liteworkspace.KindLabel(run.kind) +
			"作业，没有阅读材料这一栏，所以材料没有设上。要用这篇就先把类型设成阅读，再设材料。）"
	}
	run.setMaterial(map[string]any{"source": "library", "slug": art.Slug})
	return "\n（她点的这个选项对应库里的《" + art.ZhTitle + "》，材料已经设成这一篇了，不用再查一次。）"
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
	art := liteWorkspaceCard(raw, applied)
	var lines []string
	add := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			lines = append(lines, "- "+label+"："+value)
		}
	}
	// Chinese words only, never a wire value: this text goes straight into
	// the model's system prompt, and the model repeats what it reads. A raw
	// "reading" or a raw slug here is how a teacher used to see
	// 「材料来源：library」 in the AI's own reply (spec §12.1).
	add("类型", liteworkspace.KindLabel(art.Kind))
	add("标题", art.Title)
	add("说明", art.Instructions)
	if art.DueInput != "" {
		add("截止时间", liteworkspace.DueLabel(art.DueInput))
	}
	switch art.Kind {
	case "writing":
		add("题目", art.Prompt)
		add("目标字数", art.TargetWords)
		add("语言", map[string]string{"zh": "中文", "en": "英文"}[art.Lang])
	case "project":
		add("驱动问题", art.DrivingQuestion)
		add("补充说明", art.Description)
	}
	if art.Kind == "" || art.Kind == "reading" {
		add("材料来源", liteworkspace.SourceLabel(art.ReadingSource))
	}
	if art.Slug != "" {
		if found, ok := library.BySlug(art.Slug); ok {
			add("文章", "《"+found.ZhTitle+"》")
		} else {
			add("文章", "未找到")
		}
	}
	if art.Tier != nil {
		add("难度", liteworkspace.TierLabel(art.Tier))
	}
	if n := len(art.UserIDs); n > 0 {
		add("已选学生", fmt.Sprintf("%d 名", n))
	}
	// An empty required cell is named, so the model knows what is still
	// missing instead of guessing (it told her the 驱动问题 was written while
	// the cell was empty, five turns running).
	if art.Kind != "" {
		complete := true
		for _, f := range liteWorkspaceRequiredFields(art) {
			if strings.TrimSpace(f.Value) == "" {
				lines = append(lines, "- "+f.Label+"：（空，发布前必须填写）")
				complete = false
			}
		}
		// Measured: with every cell filled the model still told her 题目 and
		// 目标字数 were missing. Saying so is cheaper than letting it guess.
		if complete {
			lines = append(lines, "- 必填栏：都已填好，老师可以发布")
		}
	}
	if len(lines) == 0 {
		return "（还是空的）"
	}
	return strings.Join(lines, "\n")
}

// newLiteWorkspaceAssignment builds the assignment surface for one turn.
//
// The card's kind decides which cells exist, so the run has to know it
// before any tool writes one. A card with no kind yet is a reading card:
// that is what the client's own empty draft is (assignmentLogic's
// emptySettings), and a first turn must not be told it cannot set a
// material.
func (a *API) newLiteWorkspaceAssignment(mctx context.Context, cls sqlc.Class, roster []liteworkspace.Student, artifact json.RawMessage, typed string) *liteWorkspaceRun {
	card := liteWorkspaceParseArtifact(artifact)
	run := &liteWorkspaceRun{
		roster: roster, kind: card.Kind, typed: typed,
		className: cls.Name, artifact: artifact,
		materialSet:    card.Slug != "" || card.ReadingSource == "personalized" || card.ReadingSource == "text",
		materialOnCard: card.Slug != "" || card.ReadingSource == "personalized" || card.ReadingSource == "text",
		// Lazy: most turns never call recommend_articles, and loading every
		// enrolled student's profile eagerly would pay a class-of-30's worth
		// of DB round trips (see loadGroupProfiles) on every turn regardless.
		// mctx (not the request ctx) so this load survives the same way the
		// model call does if she navigates away mid-turn.
		groupProfilesLoad: func() ([]library.Profile, error) {
			return classLibraryProfiles(mctx, a.d.Queries, cls.ID)
		},
	}
	if run.kind == "" {
		run.kind = "reading"
	}
	return run
}

// begin applies the article on an option she tapped before the model runs,
// through the same setMaterial every other path uses. The model is then told
// it is done rather than asked to do it: the old route was three model calls
// — search for its own choice id, fail, search again — and this one is none.
func (run *liteWorkspaceRun) begin(req liteWorkspaceTurnRequest) string {
	for _, t := range req.Turns {
		if t.Role == "teacher" {
			run.teacherTexts = append(run.teacherTexts, t.Text)
		}
	}
	run.teacherTexts = append(run.teacherTexts, req.Text)
	return liteWorkspaceChosenArticle(run, req)
}

// system renders the assignment prompt plus the card as it stands.
//
// The card state goes in because she can edit any cell while a turn is in
// flight. A model that cannot see the card describes changes it did not make.
//
// run.patch is overlaid so the card the model reads is the card as it stands
// NOW, including what tapping an option just wrote. Showing it the pre-turn
// card instead put the note 「材料已经设成这一篇了」 next to a card whose
// material cell was empty, and it believed the cell: every run re-set the
// material by hand, which is the round trip the option payload removes.
func (run *liteWorkspaceRun) system() string {
	system := liteworkspace.AssignmentSystem(liteworkspace.SystemContext{
		ClassName:    run.className,
		TodayBeijing: time.Now().In(liteworkspace.BeijingOffset).Format("2006-01-02"),
		StudentCount: len(run.roster),
	})
	if card := liteWorkspaceCardState(run.artifact, run.patch); card != "" {
		system += "\n\n## 作业卡现在的内容\n\n" + card
	}
	return system
}

func (run *liteWorkspaceRun) tools() []gateway.ChatTool {
	return liteworkspace.AssignmentTools()
}

// ended is ask_choice's: it ends the turn, and the question is the reply.
func (run *liteWorkspaceRun) ended() (string, []liteworkspace.Choice, bool) {
	return run.question, run.choices, run.asked
}

func (run *liteWorkspaceRun) clearEnded() {
	run.question, run.choices, run.asked = "", nil, false
}

// falseClaim: the assignment surface has no page tool at all, and its
// pronouns follow the students list_students named.
func (run *liteWorkspaceRun) falseClaim(text string) string {
	if reason := liteWorkspaceOpenedPageClaim(text); reason != "" {
		return reason
	}
	if liteworkspace.ClaimsPublished(text) {
		return "回复说作业已经布置或发布，但作业还没有发布：你只填作业卡，老师检查后点「发布作业」才会发给学生。" +
			"请说「作业卡已填好，请检查后点「发布作业」」"
	}
	card := liteWorkspaceCard(run.artifact, run.patch)
	if label := liteworkspace.UnfilledClaim(text, liteWorkspaceRequiredFields(card)); label != "" {
		return "回复说「" + label + "」已经填好，但作业卡上这一栏是空的。先调用 set_fields 把它写进去，再告诉老师"
	}
	if _, wrote := run.patch["dueInput"]; wrote {
		if reason := liteworkspace.DueWeekdayMismatch(run.teacherTexts, card.DueInput); reason != "" {
			return reason
		}
	}
	return liteWorkspaceRosterPronounProblem(text, run.roster, run.namesReturned, run.typed)
}

// extraParts is nil: everything the assignment surface shows the teacher is
// the reply, an option or a patch value, which the handler checks itself.
func (run *liteWorkspaceRun) extraParts() []string { return nil }

// groundedNames is what the tools returned this turn plus the students on the
// card now. Measured 2026-09-17: six turns in, her first message (which named
// the two students) had left the history window, and a reply naming the two
// students the card showed failed with 「没有依据的学生姓名」.
func (run *liteWorkspaceRun) groundedNames() []string {
	names := append([]string(nil), run.namesReturned...)
	onCard := map[string]bool{}
	for _, id := range liteWorkspaceCard(run.artifact, run.patch).UserIDs {
		onCard[id] = true
	}
	for _, s := range run.roster {
		if onCard[s.ID] {
			names = append(names, s.Name)
		}
	}
	return names
}

// groundedCounts is what the tools counted this turn plus the students on the
// card now. Measured 2026-09-17: she named two students, the card showed
// 已选 2/4, and the next turn — a tapped option, no recipient tool — failed
// with 「没有依据的人数：2」.
func (run *liteWorkspaceRun) groundedCounts() []int {
	counts := append([]int(nil), run.countsReturned...)
	if n := len(liteWorkspaceCard(run.artifact, run.patch).UserIDs); n > 0 {
		counts = append(counts, n)
	}
	return counts
}

func (run *liteWorkspaceRun) result() (map[string]any, []liteWorkspaceCardDTO) {
	return run.patch, run.cards
}

// navigate is nil: the assignment surface has no open_page tool.
func (run *liteWorkspaceRun) navigate() *liteWorkspaceNavigateDTO { return nil }

// verbatimQuotedSpans is every article Chinese title a tool handed back this
// turn (search_library, recommend_articles, set_material) — the only titles
// the model was ever told to name in its reply (系统 prompt: 「介绍文章时用书
// 名号里的中文标题」), and now handed back already wrapped in 《》 (see the
// tool result comments).
func (run *liteWorkspaceRun) verbatimQuotedSpans() []string {
	return run.titlesReturned
}

// verbatimClassName is the class name (blanked unquoted — see the interface
// method's comment).
func (run *liteWorkspaceRun) verbatimClassName() string {
	return run.className
}

// verbatimSubjectNames is nil: an assignment card is about a class, not one
// student.
func (run *liteWorkspaceRun) verbatimSubjectNames() []string { return nil }

// groundsRosterSize is true: AssignmentSystem states the class size.
func (run *liteWorkspaceRun) groundsRosterSize() bool { return true }

// blanksQuotedSpansForNames is false: a roster name inside a quoted article
// title still needs evidence here.
func (run *liteWorkspaceRun) blanksQuotedSpansForNames() bool { return false }

// liteWorkspaceRun is the assignment surface: it accumulates what one turn's
// tools produced.
type liteWorkspaceRun struct {
	// className and artifact are what system() renders: the class the prompt
	// names and the draft the client sent.
	className string
	artifact  json.RawMessage
	roster    []liteworkspace.Student
	// groupProfilesLoad fetches every enrolled student's library.Profile
	// (classLibraryProfiles, bound to this turn's ctx/queries/class id at
	// construction). It is called at most once per turn, lazily, from
	// loadGroupProfiles — never eagerly here, because most turns never call
	// recommend_articles, and a class of 30 costs roughly ninety DB round
	// trips to build (ListLiteWeekClassStudents + three queries per student
	// inside libraryProfileIn) for a tool most turns do not reach for. It is
	// also swapped out in tests to count and to fail on demand.
	groupProfilesLoad func() ([]library.Profile, error)
	// groupProfiles/groupProfilesErr/groupProfilesLoaded memoise
	// groupProfilesLoad's one call: a turn may call recommend_articles more
	// than once (the model retrying a bad discipline filter, say), and the
	// second call must not pay for the fetch — or repeat a transient
	// failure — again.
	groupProfiles       []library.Profile
	groupProfilesErr    error
	groupProfilesLoaded bool
	// kind is the card's homework type as it stands: seeded from the artifact,
	// updated when set_fields writes it. It decides which cells the card has,
	// so set_material reads it before writing a material into a card that has
	// nowhere to show one.
	kind string
	// teacherTexts is every message she sent in this conversation, oldest
	// first — what she said about the deadline may be several turns back.
	teacherTexts []string
	// typed is what the TEACHER typed this turn (trimmed), before a click's
	// choiceId is folded in — the only text set_material's "text" source cuts
	// its passage from. Never her earlier turns, never the model's words:
	// anchoring against anything wider would let the model point at a
	// sentence in its own prior reply and pass it off as her paste.
	typed string
	// materialSet is whether the card has a reading material right now,
	// seeded from the artifact and kept current by the tools. readingSource
	// alone cannot answer it: the client's empty draft already carries
	// "library" with no article behind it.
	materialSet bool
	// materialOnCard is whether the card had a reading material when the
	// turn began, i.e. one she can see and may have chosen. A material the
	// model set and then cleared within the same turn is not news to her.
	materialOnCard bool
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
	// titlesReturned is every article's Chinese title search_library,
	// recommend_articles or set_material handed back this turn —
	// verbatimQuotedSpans' half of the head-count check (see that method's
	// comment).
	titlesReturned []string
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

// loadGroupProfiles runs groupProfilesLoad the first time recommend_articles
// is called this turn — success or failure alike is cached, so a second call
// in the same turn reads it back instead of hitting the database again.
func (run *liteWorkspaceRun) loadGroupProfiles() ([]library.Profile, error) {
	if !run.groupProfilesLoaded {
		run.groupProfiles, run.groupProfilesErr = run.groupProfilesLoad()
		run.groupProfilesLoaded = true
	}
	return run.groupProfiles, run.groupProfilesErr
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
	case "recommend_articles":
		return run.recommendArticles(tc.Args)
	case "list_students":
		return run.listStudents(tc.Args)
	case "set_recipients":
		return run.setRecipients(tc.Args)
	case "ask_choice":
		return run.askChoice(tc.Args)
	}
	return liteWorkspaceToolError("没有这个工具：" + tc.Name)
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
				run.write("text", "")
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
	if title, ok := toolString(args, "title"); ok && liteworkspace.CleanTitle(title) != "" {
		run.write("title", liteWorkspaceClampRunes(liteworkspace.CleanTitle(title), maxAssignmentTitleRunes))
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
	if msg := run.setKindFields(args, &written); msg != "" {
		return liteWorkspaceToolError(msg)
	}
	if len(written) == 0 {
		return liteWorkspaceToolError("没有给出任何字段")
	}
	out := map[string]any{"written": written}
	// Only a material she could see is worth a sentence. On production
	// (2026-09-17) a fresh card turned into a writing homework got
	// 「之前阅读库里的材料已清掉」 about an article she never saw.
	if cleared && run.materialOnCard {
		out["note"] = "类型不是 reading 了，作业卡上没有阅读材料这一栏，原来选的文章已经清掉。" +
			"跟老师说清楚这件事，不要再提那篇文章；她要保留文章就把类型改回 reading"
	} else if cleared {
		out["note"] = "这一轮选的阅读材料已随类型一起清掉，老师没有看到过它，回复里不要提"
	}
	return liteWorkspaceToolOK(out)
}

// setKindFields writes the cells only one kind of homework has. A cell for
// another kind is refused: the card has nowhere to show it, and the teacher
// would publish without seeing it.
func (run *liteWorkspaceRun) setKindFields(args map[string]any, written *[]string) string {
	current := run.kind
	if current == "" {
		current = "reading" // an empty card is a reading card (see newLiteWorkspaceAssignment)
	}
	kindName := liteworkspace.KindLabel(current)
	only := func(label, kind string) string {
		return label + "只属于" + liteworkspace.KindLabel(kind) + "作业，现在作业卡是" + kindName + "作业；要写这一栏先把 kind 改成 " + kind
	}
	if v, ok := toolString(args, "prompt"); ok && strings.TrimSpace(v) != "" {
		if run.kind != "writing" {
			return only("题目", "writing")
		}
		run.write("prompt", liteWorkspaceClampRunes(strings.TrimSpace(v), liteWorkspaceMaxInstructionsRunes))
		*written = append(*written, "prompt")
	}
	if n, ok := liteWorkspaceTargetWordsArg(args); ok {
		if run.kind != "writing" {
			return only("目标字数", "writing")
		}
		if n < 1 || n > 100000 {
			return "目标字数需在 1 到 100000 之间"
		}
		run.write("targetWords", strconv.Itoa(n))
		*written = append(*written, "targetWords")
	}
	if v, ok := toolString(args, "lang"); ok && v != "" {
		if run.kind != "writing" {
			return only("语言", "writing")
		}
		if v != "zh" && v != "en" {
			return "语言只能是 zh 或 en，收到：" + v
		}
		run.write("lang", v)
		*written = append(*written, "lang")
	}
	if v, ok := toolString(args, "drivingQuestion"); ok && strings.TrimSpace(v) != "" {
		if run.kind != "project" {
			return only("驱动问题", "project")
		}
		run.write("drivingQuestion", liteWorkspaceClampRunes(strings.TrimSpace(v), liteWorkspaceMaxInstructionsRunes))
		*written = append(*written, "drivingQuestion")
	}
	if v, ok := toolString(args, "description"); ok && strings.TrimSpace(v) != "" {
		if run.kind != "project" {
			return only("补充说明", "project")
		}
		run.write("description", liteWorkspaceClampRunes(strings.TrimSpace(v), liteWorkspaceMaxInstructionsRunes))
		*written = append(*written, "description")
	}
	return ""
}

// liteWorkspaceTargetWordsArg reads targetWords as a number or a numeric
// string ("600"): the schema asks for an integer and models send both.
func liteWorkspaceTargetWordsArg(args map[string]any) (int, bool) {
	if n, ok := toolInt(args, "targetWords"); ok {
		return n, n != 0
	}
	if v, ok := toolString(args, "targetWords"); ok && strings.TrimSpace(v) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return -1, true
		}
		return n, true
	}
	return 0, false
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
			// zhTitle is handed back already wrapped in 《》:
			// this is the output-format contract the count check's blanking
			// depends on (verbatimQuotedSpans), not a separate instruction the
			// model has to remember to apply itself.
			"slug": art.Slug, "title": art.Title, "zhTitle": "《" + art.ZhTitle + "》",
			"disciplines": art.Disciplines, "tiers": tiers,
		})
		run.titlesReturned = append(run.titlesReturned, art.ZhTitle)
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
	label := liteworkspace.KindLabel(kind)
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
		// A prior turn may have set a "text" material; switching to a library
		// article must take it with it, or the draft still carries a 50000-rune
		// paste under a field nothing shows anymore (see the kind-change
		// clearing block in setFields for the same rule).
		run.write("text", "")
		run.materialSet = true
		run.titlesReturned = append(run.titlesReturned, art.ZhTitle)
		if tier, ok := toolInt(args, "tier"); ok {
			if _, has := art.LevelAt(tier); !has {
				return liteWorkspaceToolError("这篇文章没有这一档")
			}
			run.write("tier", tier)
		}
		return liteWorkspaceToolOK(map[string]any{"slug": art.Slug, "zhTitle": "《" + art.ZhTitle + "》"})
	case "personalized":
		// 🚨 The patch only. Who reads what is computed by the existing
		// personalized-reading preview endpoint, and a second implementation
		// here would be a second answer to the same question.
		run.write("readingSource", "personalized")
		// Same reason as the text case below: a prior library or text pick
		// must not survive alongside "个性化" in the card state, or it reads
		// 「材料来源：个性化」 next to 「文章：《X》」 for an article nobody chose
		// for personalized reading.
		run.write("slug", "")
		run.write("tier", nil)
		run.write("text", "")
		run.materialSet = true
		return liteWorkspaceToolOK(map[string]any{"source": "personalized"})
	case "text":
		// 铁律①: the model never supplies the material's words. It names the
		// passage by two anchors copied from the teacher's message, and
		// liteworkspace.AnchoredSpan cuts the passage out of run.typed, her
		// own text THIS turn (never an earlier turn, never the model's words;
		// see the field's comment). What is stored is her runes, so a model
		// that invented or summarised a passage has nothing to point at, and
		// a model that turned “” into " still lands on her original text.
		startAnchor, _ := toolString(args, "startAnchor")
		endAnchor, _ := toolString(args, "endAnchor")
		text, err := liteworkspace.AnchoredSpan(run.typed, startAnchor, endAnchor)
		if err != nil {
			return liteWorkspaceToolError(err.Error())
		}
		runes := len([]rune(text))
		if runes < liteWorkspaceMinTextRunes {
			return liteWorkspaceToolError(fmt.Sprintf("两个锚点之间的正文只有 %d 字，至少需要 %d 字，这不像一篇完整的材料。"+
				"请确认 startAnchor 取的是正文开头、endAnchor 取的是正文结尾", runes, liteWorkspaceMinTextRunes))
		}
		if runes > liteWorkspaceMaxTextRunes {
			return liteWorkspaceToolError(fmt.Sprintf("两个锚点之间的正文有 %d 字，文章正文不能超过 50000 字", runes))
		}
		run.write("readingSource", "text")
		run.write("text", text)
		// A prior library or personalized pick must not linger next to it —
		// same reason the personalized case above clears slug/tier.
		run.write("slug", "")
		run.write("tier", nil)
		run.materialSet = true
		return liteWorkspaceToolOK(map[string]any{"source": "text", "runes": runes})
	}
	return liteWorkspaceToolError("材料来源只能是 library、personalized 或 text，收到：" + source)
}

// liteWorkspaceRecommendLimit is fixed, not model-controlled — §12.2 (F.3)
// bounds a class-wide recommendation to as many articles as fit a card
// without her having to scroll a list to compare them.
const liteWorkspaceRecommendLimit = 6

// recommendArticles loads the class's profiles on first use (see
// loadGroupProfiles), then is pure computation: the optional disciplines
// filter narrows the candidate articles the same way search_library's does,
// before library.RecommendForGroup scores what is left.
//
// A load failure comes back as a tool result, not a failed turn — the same
// posture every other tool error here takes: the model can tell her the
// recommendation is not available and carry on with the rest of the card.
func (run *liteWorkspaceRun) recommendArticles(args map[string]any) string {
	profiles, err := run.loadGroupProfiles()
	if err != nil {
		return liteWorkspaceToolError("班级的阅读画像加载失败：" + err.Error())
	}
	arts := library.All()
	if want := toolStrings(args, "disciplines"); len(want) > 0 {
		wantSet := make(map[string]bool, len(want))
		for _, id := range want {
			wantSet[id] = true
		}
		filtered := make([]library.Article, 0, len(arts))
		for _, a := range arts {
			for _, id := range a.Disciplines {
				if wantSet[id] {
					filtered = append(filtered, a)
					break
				}
			}
		}
		arts = filtered
	}
	recs := library.RecommendForGroup(arts, profiles, liteWorkspaceRecommendLimit)
	// rows is the CARD's data (unwrapped — the panel renders zhTitle itself,
	// same shape as every other article card). modelRows is the separate
	// projection the MODEL reads, with zhTitle already wrapped in 《》
	// (see verbatimQuotedSpans): the two must differ here, unlike search_library (which
	// has no card), because this tool's rows feed both.
	rows := make([]map[string]any, 0, len(recs))
	modelRows := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		why := libraryWhyZh(rec.Why)
		rows = append(rows, map[string]any{
			"slug": rec.Article.Slug, "zhTitle": rec.Article.ZhTitle,
			"why": why, "readCount": rec.ReadCount,
		})
		modelRows = append(modelRows, map[string]any{
			"slug": rec.Article.Slug, "zhTitle": "《" + rec.Article.ZhTitle + "》",
			"why": why, "readCount": rec.ReadCount,
		})
		run.titlesReturned = append(run.titlesReturned, rec.Article.ZhTitle)
		// A read count is a head count the model may repeat
		// (「已有 2 人读过」), so it grounds the count check like any other
		// tool-counted number. Only counts above zero are added: an article
		// nobody has read gives the reply no head count to state.
		if rec.ReadCount > 0 {
			run.countsReturned = append(run.countsReturned, rec.ReadCount)
		}
	}
	run.cards = append(run.cards, liteWorkspaceCardDTO{Kind: "articles", Rows: rows})
	return liteWorkspaceToolOK(map[string]any{"articles": modelRows, "tier": library.GroupTier(profiles)})
}

func (run *liteWorkspaceRun) listStudents(args map[string]any) string {
	result, names, count, card, ok := liteWorkspaceListStudentsTool(run.roster, args)
	if !ok {
		return result
	}
	run.namesReturned = append(run.namesReturned, names...)
	run.countsReturned = append(run.countsReturned, count)
	run.cards = append(run.cards, card)
	return result
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
	question, choices, errMsg := liteWorkspaceAskChoiceArgs(args, true)
	if errMsg != "" {
		return liteWorkspaceToolError(errMsg)
	}
	run.question, run.choices, run.asked = question, choices, true
	return liteWorkspaceToolOK(map[string]any{"options": len(choices)})
}
