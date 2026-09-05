package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// pbl_course_test.go —— 项目 → 课程 → 项目 这条环。
//
// 产品负责人 2026-09-04：「if trigger course inside pbl parts, it should also be
// end-looped, namely the course interaction and finished results go back to the
// running project and AI makes responses.」
//
// 每条断言都落在**印记收到的上文**上，而不是某个内部函数的返回值：这条环断的
// 一直是从库到 prompt 那一段路，只测中间那个函数照样看不出来（同
// pbl_closed_loop_test.go 的做法）。

type pblCourseRow struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Why        string `json:"why"`
	Takeaway   string `json:"takeaway"`
	FinishedAt string `json:"finishedAt"`
	Title      string `json:"title"`
	Available  bool   `json:"available"`
}

func listProjectCourses(t *testing.T, h http.Handler, c *http.Cookie, pid string) []pblCourseRow {
	t.Helper()
	rec := pblReq(t, h, c, "GET", "/api/v1/pbl/projects/"+pid+"/courses", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET courses = %d; body=%s", rec.Code, rec.Body)
	}
	var rows []pblCourseRow
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode courses: %v — body=%s", err, rec.Body)
	}
	return rows
}

// 印记看不见课程库就挑不出课，只会编一个 slug。这一条守的是那份目录真的进了
// prompt，而且带着 slug（不带 slug 它没法原样抄）。
func TestPblCourse_CatalogueReachesTheCoach(t *testing.T) {
	prov := pblStub(`{"reply":"先说说你手上这件事。","hook":"","hook_kind":"","produce":null}`)
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	seedCourseWithDefinition(t, pool, "vibe-coding", testCourseDefinitionJSON)
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"我想做个网站"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	ctx := lastUserText(prov)
	if !strings.Contains(ctx, "【课程库】") || !strings.Contains(ctx, "vibe-coding") {
		t.Fatalf("课程库没进上文，印记挑不出课：\n%s", ctx)
	}
}

// 编出来的 slug 不落库。她点开一门不存在的课看到的是 404，那比印记这一轮什么都
// 没做糟得多。
func TestPblCourse_InventedSlugIsRejected(t *testing.T) {
	prov := pblStub(`{"reply":"给你一课。","hook":"","hook_kind":"",
		  "tool":"course","tool_reason":"你要写代码了",
		  "produce":{"kind":"course","payload":{"slug":"no-such-course","why":"学写代码"}}}`)
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	seedCourseWithDefinition(t, pool, "vibe-coding", testCourseDefinitionJSON)
	pid := newProjectViaAPI(t, h, cookie)

	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我要开始写了"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if rows := listProjectCourses(t, h, cookie, pid); len(rows) != 0 {
		t.Fatalf("一个编出来的 slug 落库了：%+v", rows)
	}
	// 那件工具也不该递出去：它的界面读的就是这条指派，没有指派就是一块白板。
	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	if strings.Contains(rec.Body.String(), `"tool":"course"`) {
		t.Fatalf("界面会是空的，这件工具不该递出去：%s", rec.Body)
	}
}

// 完整的一环：印记挑课 → 指派落库 → 她写下收获 → 下一轮印记的上文里有那句话。
func TestPblCourse_ClosesTheLoop(t *testing.T) {
	prov := pblStub(`{"reply":"这一课讲的正是你缺的那一块。","hook":"","hook_kind":"",
		  "tool":"course","tool_reason":"你说要自己写页面，先补这一课",
		  "produce":{"kind":"course","payload":{"slug":"vibe-coding","why":"你要自己写页面了"}}}`)
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	seedCourseWithDefinition(t, pool, "vibe-coding", testCourseDefinitionJSON)
	pid := newProjectViaAPI(t, h, cookie)

	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我要自己把页面写出来"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn 1 = %d; body=%s", rec.Code, rec.Body)
	}

	rows := listProjectCourses(t, h, cookie, pid)
	if len(rows) != 1 || rows[0].Slug != "vibe-coding" {
		t.Fatalf("指派没落库：%+v", rows)
	}
	if rows[0].Why == "" {
		t.Fatalf("没有理由的课是伏击，这一格不该是空的：%+v", rows[0])
	}
	if !rows[0].Available || rows[0].Title == "" {
		t.Fatalf("课程本身没补上，界面只会显示一个 slug：%+v", rows[0])
	}

	// 她上完回来，写下这一课对项目有什么用。
	const takeaway = "我知道怎么把一个页面拆成组件了，下一步先把首页搭出来"
	rec := pblPost(t, h, cookie,
		"/api/v1/pbl/projects/"+pid+"/courses/"+rows[0].ID+"/finish",
		`{"takeaway":"`+takeaway+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("finish = %d; body=%s", rec.Code, rec.Body)
	}

	// 🚨 环闭上的证据：下一轮印记的上文里有她写的那句话。
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"接下来呢"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}
	ctx := lastUserText(prov)
	if !strings.Contains(ctx, takeaway) {
		t.Fatalf("她上完课写的那句话没回到印记那儿，环没闭上：\n%s", ctx)
	}
	// 上完的课不该再被递一次（目录本身不带状态）。
	if !strings.Contains(ctx, "已经上过的课") {
		t.Fatalf("上文没说哪几门已经上过，印记会重复递：\n%s", ctx)
	}
}

// 空的 takeaway 拒绝：回灌读的就是这一句，没有它这次上课在对话里等于没发生。
func TestPblCourse_FinishNeedsATakeaway(t *testing.T) {
	prov := pblStub(`{"reply":"给你一课。","hook":"","hook_kind":"",
		  "produce":{"kind":"course","payload":{"slug":"vibe-coding","why":"你要自己写页面了"}}}`)
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	seedCourseWithDefinition(t, pool, "vibe-coding", testCourseDefinitionJSON)
	pid := newProjectViaAPI(t, h, cookie)
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我要自己把页面写出来"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	rows := listProjectCourses(t, h, cookie, pid)
	if len(rows) != 1 {
		t.Fatalf("指派没落库：%+v", rows)
	}

	rec := pblPost(t, h, cookie,
		"/api/v1/pbl/projects/"+pid+"/courses/"+rows[0].ID+"/finish", `{"takeaway":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空的 takeaway = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// 一门只给 pro 的课，印记在 lite 项目里挑不到 —— 目录里没有，硬塞也落不了库。
func TestPblCourse_ProOnlyCourseIsNotOfferedInLite(t *testing.T) {
	prov := pblStub(`{"reply":"给你一课。","hook":"","hook_kind":"",
		  "produce":{"kind":"course","payload":{"slug":"pro-course","why":"补这一块"}}}`)
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	seedCourseWithDefinition(t, pool, "pro-course", testCourseDefinitionJSON)
	setCourseAudience(t, pool, "pro-course", []string{"pro"})
	pid := newProjectViaAPI(t, h, cookie)

	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我要开始写了"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	if ctx := lastUserText(prov); strings.Contains(ctx, "pro-course") {
		t.Fatalf("一门只给 pro 的课进了 lite 项目的挑课目录：\n%s", ctx)
	}
	if rows := listProjectCourses(t, h, cookie, pid); len(rows) != 0 {
		t.Fatalf("一门 lite 学生看不见的课被指派了：%+v", rows)
	}
}
