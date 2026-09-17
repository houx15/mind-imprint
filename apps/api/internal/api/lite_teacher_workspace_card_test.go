package api_test

// The AI 布置作业 findings of the 2026-09-17 real-user walk: the AI could not
// fill a project's 驱动问题 or a writing's 题目 (so an AI-made project or
// essay could never be published), said it had filled them anyway, put 《》
// inside the title, echoed 2026-09-18T21:00 to the teacher, and a turn after
// she named two students failed with 「没有依据的人数：2」.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

// lastToolResult is the content of the last tool message the model was sent.
func lastToolResult(t *testing.T, prov *gateway.SequenceStubProvider) string {
	t.Helper()
	req := prov.Requests[len(prov.Requests)-1]
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == gateway.RoleTool {
			return req.Messages[i].Content
		}
	}
	t.Fatal("no tool result was sent to the model")
	return ""
}

func workspaceTurnWithCard(classID, text string, card map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": text, "artifact": card,
	})
	return string(b)
}

func TestWorkspaceSetsProjectAndWritingFields(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"kind":"project","title":"《手机学习时间调查》","dueAt":"2026-09-23T18:00",`+
			`"drivingQuestion":"手机是我们学习的工具，还是时间黑洞？","description":"设计一份问卷。"}`),
		wsText("驱动问题已写入，截止 2026-09-23T18:00。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "布置一个手机学习时间的调查项目，下周三交"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	out := decodeWorkspaceTurn(t, rec)
	if out.Patch["drivingQuestion"] != "手机是我们学习的工具，还是时间黑洞？" || out.Patch["description"] != "设计一份问卷。" {
		t.Fatalf("patch = %+v", out.Patch)
	}
	if out.Patch["title"] != "手机学习时间调查" {
		t.Fatalf("title = %v, want it without 《》", out.Patch["title"])
	}
	if out.Reply != "驱动问题已写入，截止 9月23日 18:00。" {
		t.Fatalf("reply = %q, want the date in words", out.Reply)
	}

	// Writing: 题目, 目标字数 (a numeric string is accepted), 语言.
	prov2 := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"kind":"writing","prompt":"学校该不该允许学生用 AI 写作业？","targetWords":"600","lang":"zh"}`),
		wsText("题目和目标字数已经填好。"),
	)
	h2, _, teacher2, classID2, _ := liteTeacherFixtureWithProvider(t, prov2)
	rec = postWorkspaceTurn(t, h2, teacher2, workspaceTurnBody(classID2, "布置一篇 600 字议论文"))
	if rec.Code != http.StatusOK {
		t.Fatalf("writing turn = %d; body=%s", rec.Code, rec.Body)
	}
	out = decodeWorkspaceTurn(t, rec)
	if out.Patch["prompt"] != "学校该不该允许学生用 AI 写作业？" || out.Patch["targetWords"] != "600" || out.Patch["lang"] != "zh" {
		t.Fatalf("writing patch = %+v", out.Patch)
	}
}

// A cell that belongs to another kind is refused, and the model is told why.
func TestWorkspaceRefusesAFieldOfAnotherKind(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"drivingQuestion":"为什么？"}`),
		wsText("这是阅读作业，没有驱动问题这一栏。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnBody(classID, "加一个驱动问题"))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if out := decodeWorkspaceTurn(t, rec); len(out.Patch) != 0 {
		t.Fatalf("patch = %+v, want nothing written", out.Patch)
	}
	if got := lastToolResult(t, prov); !strings.Contains(got, "驱动问题只属于项目作业") {
		t.Fatalf("tool result = %s", got)
	}
}

// Live, 2026-09-17: 「驱动问题已写入」 five turns running, cell empty.
func TestWorkspaceClaimOfAnEmptyCellIsRewritten(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsText("好了，驱动问题已写入。还需要改什么吗？"),
		wsToolCall("set_fields", `{"drivingQuestion":"手机是工具还是黑洞？"}`),
		wsText("驱动问题已写入。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	card := map[string]any{"kind": "project", "title": "手机学习时间调查", "dueInput": "2026-09-23T18:00"}
	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnWithCard(classID, "驱动问题你帮我写一个。", card))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if out := decodeWorkspaceTurn(t, rec); out.Patch["drivingQuestion"] != "手机是工具还是黑洞？" {
		t.Fatalf("patch = %+v, want the rewrite to have written the cell", out.Patch)
	}
	if got := lastUserMessage(t, prov, 1); !strings.Contains(got, "「驱动问题」已经填好，但作业卡上这一栏是空的") {
		t.Fatalf("rewrite request = %q", got)
	}
	// The model is shown which required cell is still empty.
	if sys := prov.Requests[0].Messages[0].Content; !strings.Contains(sys, "驱动问题：（空，发布前必须填写）") {
		t.Fatalf("system prompt does not name the empty cell:\n%s", sys)
	}
}

// Live, 2026-09-17: she named two students, the card showed 已选 2/4, and the
// next turn (a tapped option, no recipient tool) failed on 「2」.
func TestWorkspaceCountOfStudentsOnTheCardIsGrounded(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsText("好的，这 2 名学生用问卷调查。还需要调整什么吗？"),
	)
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	second := createStudent(t, pool, SeedSchoolID, "card-count-2@demo.local")
	enrollStudent(t, pool, second, classID)
	third := createStudent(t, pool, SeedSchoolID, "card-count-3@demo.local")
	enrollStudent(t, pool, third, classID)
	card := map[string]any{"kind": "project", "userIds": []string{second.String(), third.String()}}
	rec := postWorkspaceTurn(t, h, teacher, workspaceTurnWithCard(classID, "问卷调查", card))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
}

// Live, 2026-09-17 (Thursday): 「周五晚上九点」 became 9月20日, a Sunday.
func TestWorkspaceDeadlineOnTheWrongWeekdayIsRewritten(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(
		wsToolCall("set_fields", `{"title":"AI 当数学家教的正确用法","dueAt":"2026-09-20T21:00"}`),
		wsText("截止时间定在周五晚上九点。"),
		wsToolCall("set_fields", `{"dueAt":"2026-09-18T21:00"}`),
		wsText("截止时间定在周五晚上九点。"),
	)
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	b, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "text": "标题、截止时间你帮我定吧。",
		"artifact": map[string]any{"kind": "reading"},
		"turns": []map[string]string{
			{"role": "teacher", "text": "这周请全班读一篇关于用 AI 学数学的文章，周五晚上九点前完成。"},
			{"role": "ai", "text": "用哪篇文章？"},
		},
	})
	rec := postWorkspaceTurn(t, h, teacher, string(b))
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if out := decodeWorkspaceTurn(t, rec); out.Patch["dueInput"] != "2026-09-18T21:00" {
		t.Fatalf("dueInput = %v, want the Friday", out.Patch["dueInput"])
	}
	if got := lastUserMessage(t, prov, 2); !strings.Contains(got, "老师说的是周五，但截止时间 9月20日 是周日") {
		t.Fatalf("rewrite request = %q", got)
	}
	// The prompt carries the calendar.
	if sys := prov.Requests[0].Messages[0].Content; !strings.Contains(sys, "不要自己推算星期几") {
		t.Fatalf("system prompt has no calendar:\n%s", sys)
	}
}

// Live, 2026-09-17: she tapped 「手机学习时长是否影响成绩」 and the model,
// reading only the id, asked what 「usage_compare」 stood for.
func TestWorkspaceTappedOptionReachesTheModelWithItsLabel(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("好的。"))
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	b, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "choiceId": "usage_compare", "choiceLabel": "手机学习时长是否影响成绩",
		"artifact": map[string]any{"kind": "project"},
	})
	if rec := postWorkspaceTurn(t, h, teacher, string(b)); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if got := lastUserMessage(t, prov, 0); !strings.Contains(got, "点选了选项「手机学习时长是否影响成绩」") || !strings.Contains(got, "usage_compare") {
		t.Fatalf("model input = %q", got)
	}
}

// Live, 2026-09-17: six turns in, her message naming the two students had left
// the history window; a reply naming the students on the card failed.
func TestWorkspaceStudentsOnTheCardMayBeNamed(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(wsText("已选定王思远和张雨桐。还需要补充说明吗？"))
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, prov)
	wang := createStudent(t, pool, SeedSchoolID, "card-name-wang@demo.local")
	enrollStudent(t, pool, wang, classID)
	renameLiteStudent(t, pool, wang, "王思远")
	zhang := createStudent(t, pool, SeedSchoolID, "card-name-zhang@demo.local")
	enrollStudent(t, pool, zhang, classID)
	renameLiteStudent(t, pool, zhang, "张雨桐")
	card := map[string]any{"kind": "project", "userIds": []string{wang.String(), zhang.String()}}
	if rec := postWorkspaceTurn(t, h, teacher, workspaceTurnWithCard(classID, "不用了，就这样。", card)); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	// A student who is not on the card still needs evidence.
	prov2 := gateway.NewSequenceStubProvider(wsText("刘子航也可以加上。"))
	h2, pool2, teacher2, classID2, _ := liteTeacherFixtureWithProvider(t, prov2)
	liu := createStudent(t, pool2, SeedSchoolID, "card-name-liu@demo.local")
	enrollStudent(t, pool2, liu, classID2)
	renameLiteStudent(t, pool2, liu, "刘子航")
	rec := postWorkspaceTurn(t, h2, teacher2, workspaceTurnWithCard(classID2, "不用了，就这样。", map[string]any{"kind": "project"}))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("name not on the card = %d, want 502; body=%s", rec.Code, rec.Body)
	}
}
