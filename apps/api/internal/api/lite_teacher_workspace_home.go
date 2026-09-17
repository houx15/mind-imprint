package api

import (
	"context"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteweekly"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// lite_teacher_workspace_home.go — the home surface of the teacher workspace
// (§12.5, D2): the class chat a teacher opens from her class summary, to ask
// about her class and get sent to the right page. The loop, metering and §6
// checks that every surface shares live in lite_teacher_workspace.go.

// The home surface is the second liteWorkspaceSurface.
var _ liteWorkspaceSurface = (*liteWorkspaceHome)(nil)

// newLiteWorkspaceHome builds the home surface for one turn. Both loaders are
// lazy and memoised: most turns call at most one of class_snapshot and
// list_assignments, so eagerly running both would pay for a query the turn
// never needed. mctx (not the request ctx) so a deferred load survives the
// same way the model call does if she navigates away mid-turn.
func (a *API) newLiteWorkspaceHome(mctx context.Context, cls sqlc.Class, roster []liteworkspace.Student, typed string) *liteWorkspaceHome {
	return &liteWorkspaceHome{
		className: cls.Name, classID: cls.ID.String(), roster: roster, typed: typed,
		snapshotLoad: func() (liteweekly.ClassWeekStats, []liteClassWeekCardDTO, []liteClassWeekCardDTO, error) {
			// Same week the class card above this chat already shows (§12.5
			// item 1): the CURRENT, in-progress Beijing week, not
			// the last completed one.
			students, err := a.loadLiteClassWeek(mctx, cls.ID, liteweek.WeekStart(time.Now()))
			if err != nil {
				return liteweekly.ClassWeekStats{}, nil, nil, err
			}
			stats := liteClassWeekStats(students)
			_, praise, watch := liteClassWeekCards(students)
			return stats, praise, watch, nil
		},
		assignmentsLoad: func() ([]AssignmentSummaryDTO, error) {
			return a.classAssignmentSummaries(mctx, cls.ID)
		},
	}
}

// begin: the home surface has nothing in the request to apply before the
// model runs — no artifact, no option-carried article. Unlike the assignment
// surface's chosen-article path, a click on a home option is just her turn's
// choiceId, read from the roster/tool state the same way typed text is.
func (run *liteWorkspaceHome) begin(liteWorkspaceTurnRequest) string { return "" }

func (run *liteWorkspaceHome) system() string {
	return liteworkspace.HomeSystem(liteworkspace.SystemContext{
		ClassName:    run.className,
		TodayBeijing: time.Now().In(liteworkspace.BeijingOffset).Format("2006-01-02"),
		StudentCount: len(run.roster),
	})
}

func (run *liteWorkspaceHome) tools() []gateway.ChatTool { return liteworkspace.HomeTools() }

// ended is ask_choice's: it ends the turn, and the question is the reply.
func (run *liteWorkspaceHome) ended() (string, []liteworkspace.Choice, bool) {
	return run.question, run.choices, run.asked
}

func (run *liteWorkspaceHome) clearEnded() {
	run.question, run.choices, run.asked = "", nil, false
}

// falseClaim: the home surface offers pages but never opens one, and a reply
// that points at a button must have one under it. Measured 2026-09-17: the
// model wrote 「请点击下方按钮前往周子涵的学习页」 without calling open_page.
func (run *liteWorkspaceHome) falseClaim(text string) string {
	if reason := liteWorkspaceOpenedPageClaim(text); reason != "" {
		return reason
	}
	if run.nav == nil && !run.asked && liteworkspace.PointsAtButton(text) {
		return "回复让老师点击下方按钮，但本轮没有调用 open_page，下方没有按钮。需要给入口时先调用 open_page"
	}
	if liteworkspace.OffersMessage(text) {
		return "回复或选项提出提醒、通知或联系学生，但没有工具能给学生发消息。" +
			"可以建议老师布置一份作业（学生会在收件箱看到），或前往学生的学习页"
	}
	return liteWorkspaceRosterPronounProblem(text, run.roster, run.namesReturned, run.typed)
}

// extraParts carries the navigate label, when open_page set one: it names a
// real page, and on target=student or target=assignment a real student's
// name or a real assignment's title, so it goes through the same §6 checks
// as the reply and the option labels.
func (run *liteWorkspaceHome) extraParts() []string {
	if run.nav == nil {
		return nil
	}
	return []string{run.nav.Label}
}

func (run *liteWorkspaceHome) groundedNames() []string { return run.namesReturned }

func (run *liteWorkspaceHome) groundedCounts() []int { return run.countsReturned }

// result: the home surface writes no draft field — there is no artifact to
// patch — only the cards its tools produced.
func (run *liteWorkspaceHome) result() (map[string]any, []liteWorkspaceCardDTO) {
	return nil, run.cards
}

func (run *liteWorkspaceHome) navigate() *liteWorkspaceNavigateDTO { return run.nav }

// liteWorkspaceHome is the home surface: it accumulates what one turn's tools
// produced, the same shape liteWorkspaceRun holds for the assignment surface.
type liteWorkspaceHome struct {
	className string
	classID   string
	roster    []liteworkspace.Student
	// typed is what the TEACHER typed this turn (trimmed). falseClaim reads
	// it: a pronoun she used herself is allowed in the reply.
	typed string

	// snapshotLoad fetches this week's class stats and cards
	// (loadLiteClassWeek + liteClassWeekStats + liteClassWeekCards, bound to
	// this turn's mctx/queries/class id at construction). Called at most once
	// per turn, lazily — most turns never call class_snapshot.
	snapshotLoad   func() (liteweekly.ClassWeekStats, []liteClassWeekCardDTO, []liteClassWeekCardDTO, error)
	snapshotLoaded bool
	snapshotStats  liteweekly.ClassWeekStats
	snapshotPraise []liteClassWeekCardDTO
	snapshotWatch  []liteClassWeekCardDTO
	snapshotErr    error
	// assignmentsLoad fetches this class's non-archived assignments with
	// their per-status counts (classAssignmentSummaries — the same function
	// listLiteAssignments' HTTP route uses). Called at most once per turn,
	// lazily: list_assignments calls it directly, and open_page{target:
	// assignment} calls it too (to find the assignment's title), so a turn
	// that does both must not pay for the query twice.
	assignmentsLoad   func() ([]AssignmentSummaryDTO, error)
	assignmentsLoaded bool
	assignments       []AssignmentSummaryDTO
	assignmentsErr    error

	cards []liteWorkspaceCardDTO
	// namesReturned is what a tool handed back this turn — the evidence side
	// of the name check: class_snapshot's praise/watch names, list_students'
	// rows, and a student's name once open_page has confirmed her userId is
	// on the roster.
	namesReturned []string
	// countsReturned is the evidence side of the head-count check: every
	// number a tool actually counted this turn (a status count, a roster
	// filter's size).
	countsReturned []int
	// titlesReturned is every assignment title a tool handed back this turn
	// (list_assignments, open_page{target:assignment}) —
	// verbatimQuotedSpans' half of the head-count check. A title such as
	// 「3 人小组汇报」 is catalogue/teacher data, not a model claim, but its
	// digits still sit inside text §6 checks (the reply, a choice label, the
	// navigate label); see verbatimQuotedSpans' comment on
	// lite_teacher_workspace.go for why its digits are blanked — ONLY where
	// quoted — from checked text rather than grounded as counts.
	titlesReturned []string
	// question and choices are set by ask_choice, which ends the turn.
	question string
	choices  []liteworkspace.Choice
	asked    bool
	// nav is the page open_page offered this turn, or nil.
	nav *liteWorkspaceNavigateDTO
}

// verbatimQuotedSpans is every assignment title a tool handed back this turn.
func (run *liteWorkspaceHome) verbatimQuotedSpans() []string {
	return run.titlesReturned
}

// verbatimClassName is the class name (blanked unquoted — see the interface
// method's comment).
func (run *liteWorkspaceHome) verbatimClassName() string {
	return run.className
}

// verbatimSubjectNames is nil: the home surface is about a class, not one
// student.
func (run *liteWorkspaceHome) verbatimSubjectNames() []string { return nil }

// groundsRosterSize is true: HomeSystem states the class size.
func (run *liteWorkspaceHome) groundsRosterSize() bool { return true }

// blanksQuotedSpansForNames is false: a roster name inside a quoted
// assignment title still needs evidence here.
func (run *liteWorkspaceHome) blanksQuotedSpansForNames() bool { return false }

// snapshot runs snapshotLoad the first time class_snapshot is called this
// turn; a second call in the same turn reads the memoised result instead of
// hitting the database again.
func (run *liteWorkspaceHome) snapshot() (liteweekly.ClassWeekStats, []liteClassWeekCardDTO, []liteClassWeekCardDTO, error) {
	if !run.snapshotLoaded {
		run.snapshotStats, run.snapshotPraise, run.snapshotWatch, run.snapshotErr = run.snapshotLoad()
		run.snapshotLoaded = true
	}
	return run.snapshotStats, run.snapshotPraise, run.snapshotWatch, run.snapshotErr
}

// assignmentRows runs assignmentsLoad the first time list_assignments or
// open_page{target:assignment} is called this turn.
func (run *liteWorkspaceHome) assignmentRows() ([]AssignmentSummaryDTO, error) {
	if !run.assignmentsLoaded {
		run.assignments, run.assignmentsErr = run.assignmentsLoad()
		run.assignmentsLoaded = true
	}
	return run.assignments, run.assignmentsErr
}

// execute runs one tool call and returns the tool result the model reads
// next. A bad argument comes back as a tool result, not a failed turn — the
// model minted it and can fix it on the next round, within
// liteworkspace.ToolLoopMax.
func (run *liteWorkspaceHome) execute(tc gateway.ToolCall) string {
	switch tc.Name {
	case "class_snapshot":
		return run.classSnapshot()
	case "list_students":
		return run.listStudents(tc.Args)
	case "list_assignments":
		return run.listAssignments()
	case "open_page":
		return run.openPage(tc.Args)
	case "ask_choice":
		return run.askChoice(tc.Args)
	}
	return liteWorkspaceToolError("没有这个工具：" + tc.Name)
}

// classSnapshot answers class_snapshot: this week's participation and the
// students who earned a praise or a watch card, the same cards the class
// weekly report itself would render for them.
//
// AssignmentRate is NOT added to countsReturned — same reason
// liteClassSummaryFacts excludes it (lite_class_summary.go): it is a
// percentage, not a head count, and a bare number sitting in the grounded
// set would let a reply that happened to restate it, followed by 人/位/名,
// pass as if it had counted something.
func (run *liteWorkspaceHome) classSnapshot() string {
	stats, praise, watch, err := run.snapshot()
	if err != nil {
		return liteWorkspaceToolError("本周统计加载失败：" + err.Error())
	}
	praiseRows := make([]map[string]any, 0, len(praise))
	for _, c := range praise {
		praiseRows = append(praiseRows, map[string]any{"name": c.Name, "称谓": run.pronounOf(c.UserID), "evidence": c.Evidence})
		run.namesReturned = append(run.namesReturned, c.Name)
	}
	watchRows := make([]map[string]any, 0, len(watch))
	for _, c := range watch {
		watchRows = append(watchRows, map[string]any{"name": c.Name, "称谓": run.pronounOf(c.UserID), "evidence": c.Evidence})
		run.namesReturned = append(run.namesReturned, c.Name)
	}
	run.countsReturned = append(run.countsReturned, stats.ClassSize, stats.ActiveStudents, stats.Finished)
	run.cards = append(run.cards, liteWorkspaceCardDTO{Kind: "classSnapshot", Rows: map[string]any{
		"classSize": stats.ClassSize, "activeStudents": stats.ActiveStudents,
		"finished": stats.Finished, "assignmentRate": stats.AssignmentRate,
		"praise": praise, "watch": watch,
	}})
	return liteWorkspaceToolOK(map[string]any{
		"classSize": stats.ClassSize, "activeStudents": stats.ActiveStudents,
		"finished": stats.Finished, "assignmentRate": stats.AssignmentRate,
		"praise": praiseRows, "watch": watchRows,
	})
}

func (run *liteWorkspaceHome) listStudents(args map[string]any) string {
	result, names, count, card, ok := liteWorkspaceListStudentsTool(run.roster, args)
	if !ok {
		return result
	}
	run.namesReturned = append(run.namesReturned, names...)
	run.countsReturned = append(run.countsReturned, count)
	run.cards = append(run.cards, card)
	return result
}

// listAssignments answers list_assignments: this class's homework, with the
// same per-status counts listLiteAssignments' HTTP route computes
// (classAssignmentSummaries — no second query). The model reads Chinese
// words only (kind label, status label): a raw "reading" or "not_started"
// in a tool result is exactly the kind of value the model has repeated
// verbatim before (spec §12.1's rule 3).
func (run *liteWorkspaceHome) listAssignments() string {
	rows, err := run.assignmentRows()
	if err != nil {
		return liteWorkspaceToolError("作业列表加载失败：" + err.Error())
	}
	modelRows := make([]map[string]any, 0, len(rows))
	for _, as := range rows {
		counts := make(map[string]int, len(as.Counts))
		for status, n := range as.Counts {
			counts[liteassign.StatusLabel(status)] = n
			run.countsReturned = append(run.countsReturned, n)
		}
		run.titlesReturned = append(run.titlesReturned, as.Title)
		modelRows = append(modelRows, map[string]any{
			// title is handed back already wrapped in 《》:
			// the output-format contract the count check's blanking depends
			// on (verbatimQuotedSpans), not a separate instruction the model
			// has to remember to apply itself. The card's own rows (below)
			// stay unwrapped — the panel renders the title itself.
			"id": as.ID, "kind": liteworkspace.KindLabel(as.Kind), "title": "《" + as.Title + "》",
			"dueAt": as.DueAt, "counts": counts,
		})
	}
	run.cards = append(run.cards, liteWorkspaceCardDTO{Kind: "assignments", Rows: rows})
	return liteWorkspaceToolOK(map[string]any{"assignments": modelRows})
}

// openPage answers open_page: it validates target against the closed set
// (§12.5 item 5) and, for student/assignment, that the id belongs to this
// class, then records the navigate offer. It never navigates anything
// itself — the panel renders run.nav as a button, and only her click moves
// her there.
func (run *liteWorkspaceHome) openPage(args map[string]any) string {
	target, _ := toolString(args, "target")
	switch target {
	case "classWeekly":
		run.nav = &liteWorkspaceNavigateDTO{View: "classWeekly", ClassID: run.classID, Label: "本周报告"}
	case "student":
		userID, _ := toolString(args, "userId")
		name, ok := run.rosterName(userID)
		if !ok {
			return liteWorkspaceToolError("这个学生不在班里：" + userID +
				"。userId 必须是 list_students 返回的 id，原样复制")
		}
		run.namesReturned = append(run.namesReturned, name)
		id := userID
		run.nav = &liteWorkspaceNavigateDTO{View: "student", ClassID: run.classID, UserID: &id, Label: name + "的学习页"}
	case "assignmentNew":
		run.nav = &liteWorkspaceNavigateDTO{View: "assignmentNew", ClassID: run.classID, Label: "布置作业"}
	case "assignment":
		assignmentID, _ := toolString(args, "assignmentId")
		rows, err := run.assignmentRows()
		if err != nil {
			return liteWorkspaceToolError("作业列表加载失败：" + err.Error())
		}
		title, found := "", false
		for _, as := range rows {
			if as.ID == assignmentID {
				title, found = as.Title, true
				break
			}
		}
		if !found {
			return liteWorkspaceToolError("这份作业不属于这个班：" + assignmentID +
				"。assignmentId 必须是 list_assignments 返回的 id，原样复制")
		}
		run.titlesReturned = append(run.titlesReturned, title)
		id := assignmentID
		// Label is 《标题》, not the bare title: the label
		// travels through extraParts() into the count check exactly like the
		// reply does, and only a QUOTED occurrence of a title is blanked —
		// so an unquoted label would defeat the very check it has to pass.
		run.nav = &liteWorkspaceNavigateDTO{View: "assignment", ClassID: run.classID, AssignmentID: &id, Label: "《" + title + "》"}
	case "parentReports":
		run.nav = &liteWorkspaceNavigateDTO{View: "parentReports", ClassID: run.classID, Label: "家长报告"}
	default:
		return liteWorkspaceToolError("没有这个页面：" + target +
			"，只能是 classWeekly、student、assignmentNew、assignment 或 parentReports")
	}
	return liteWorkspaceToolOK(map[string]any{"navigate": true, "label": run.nav.Label})
}

// pronounOf is the pronoun the model may use for userID: the gender the
// teacher set on her roster row, or liteworkspace.PronounUnset.
func (run *liteWorkspaceHome) pronounOf(userID string) string {
	return liteWorkspacePronounOf(run.roster, userID)
}

// rosterName reports the display name of userID on run.roster, when she is a
// current member of the class.
func (run *liteWorkspaceHome) rosterName(userID string) (string, bool) {
	for _, s := range run.roster {
		if s.ID == userID {
			return s.Name, true
		}
	}
	return "", false
}

func (run *liteWorkspaceHome) askChoice(args map[string]any) string {
	question, choices, errMsg := liteWorkspaceAskChoiceArgs(args, false)
	if errMsg != "" {
		return liteWorkspaceToolError(errMsg)
	}
	run.question, run.choices, run.asked = question, choices, true
	return liteWorkspaceToolOK(map[string]any{"options": len(choices)})
}
