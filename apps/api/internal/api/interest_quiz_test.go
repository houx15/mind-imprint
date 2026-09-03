package api_test

// interest_quiz_test.go — 觉醒协议三个端点的库级测试。
//
// 测的是「读代码看不出对错」的那几件事（AGENTS.md 的测试原则）：
//
//   - 中途退出**不算做过**。一个在第三屏关掉页面的学生，那条邀请必须还会再
//     出现。这条只要写反一次就永久地把一部分学生挡在门外，而且没有人会报告它。
//   - 交卷之后，词真的种进了**同一棵树**（GET /interest/tree 能读到），并且
//     evidence 是她自己写的那句话。
//   - **重做是再长几个词**：第二次作答给同一个词再添一条来源，强度上升。这条
//     依赖 ref_id 存的是作答 id 而不是 NULL —— 写成 NULL 的话第二次会撞进
//     UNIQUE 里，静默地一个词都不加。
//   - 写得太短时**不发那次调用**，也绝不编一个词出来。
//   - 一次作答只有她本人能交。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"mindimprint/api/internal/gateway"
)

type quizAttemptJSON struct {
	ID                string `json:"id"`
	Navigator         string `json:"navigator"`
	AnchorWork        string `json:"anchorWork"`
	AnchorReason      string `json:"anchorReason"`
	Hook              string `json:"hook"`
	ChallengeChoice   string `json:"challengeChoice"`
	ChallengeAttempts int    `json:"challengeAttempts"`
	FinishedAt        string `json:"finishedAt"`
}

type quizStatusJSON struct {
	Taken  bool             `json:"taken"`
	Latest *quizAttemptJSON `json:"latest"`
}

type quizResultJSON struct {
	Attempt quizAttemptJSON `json:"attempt"`
	Lenses  []struct {
		ID       string `json:"id"`
		Zh       string `json:"zh"`
		Field    string `json:"field"`
		Asks     string `json:"asks"`
		Syllabus []struct {
			Board string `json:"board"`
			Label string `json:"label"`
		} `json:"syllabus"`
	} `json:"lenses"`
	Keywords []struct {
		TextZh   string `json:"textZh"`
		Field    string `json:"field"`
		Evidence string `json:"evidence"`
	} `json:"keywords"`
	Harvested bool `json:"harvested"`
}

// quizStubProvider 回一份合法的采集应答，好让「交卷 → 种词 → 树上看得到」这条
// 主线能被真的走一遍。evidence 必须是下面 reasonText 的**原样摘录**，否则
// ParseHarvestReply 会（正确地）把它丢掉。
const reasonText = "他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。"

func quizStubProvider() gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"keywords":[{"zh":"选择与代价","en":"Choice and cost",` +
			`"field":"self","note":"你在意一个人在最难的时候还能不能自己做主。",` +
			`"evidence":"他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。"}]}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 60, OutputTokens: 30}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func quizStatus(t *testing.T, h http.Handler, cookie *http.Cookie) quizStatusJSON {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/interest/quiz", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /interest/quiz = %d; body=%s", rec.Code, rec.Body)
	}
	var out quizStatusJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode status: %v; body=%s", err, rec.Body)
	}
	return out
}

func startQuiz(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/interest/quiz", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /interest/quiz = %d; body=%s", rec.Code, rec.Body)
	}
	var out quizAttemptJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode start: %v; body=%s", err, rec.Body)
	}
	if out.ID == "" {
		t.Fatal("开一次作答没有拿到 id")
	}
	return out.ID
}

func finishQuizRaw(h http.Handler, cookie *http.Cookie, id string, body map[string]any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/v1/interest/quiz/"+id, bytes.NewReader(raw))
	h.ServeHTTP(rec, withCookie(req, cookie))
	return rec
}

func finishQuiz(t *testing.T, h http.Handler, cookie *http.Cookie, id string, body map[string]any) quizResultJSON {
	t.Helper()
	rec := finishQuizRaw(h, cookie, id, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /interest/quiz/%s = %d; body=%s", id, rec.Code, rec.Body)
	}
	var out quizResultJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode result: %v; body=%s", err, rec.Body)
	}
	return out
}

func fullAttempt() map[string]any {
	return map[string]any{
		"navigator":         "资深向导",
		"anchorWork":        "《进击的巨人》里的利威尔",
		"anchorReason":      reasonText,
		"hook":              "character",
		"challengeChoice":   "evidence",
		"challengeAttempts": 2,
	}
}

/* ── 邀请要不要出现 ─────────────────────────────────────────────────────── */

func TestInterestQuiz_NotTakenOnAFreshAccount(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	got := quizStatus(t, h, cookie)
	if got.Taken {
		t.Error("新账号不该是「做过了」")
	}
	if got.Latest != nil {
		t.Errorf("新账号不该有最近一次作答：%+v", got.Latest)
	}
}

// 🚨 这个测试守的是整个设计里最容易写反的一条：**开了但没交，不算做过。**
// 写反了的后果是一个在第三屏关掉页面的学生从此再也见不到那条邀请，而她既不会
// 报告这件事，我们也不会从任何日志里看出来。
func TestInterestQuiz_AbandonedAttemptDoesNotCountAsTaken(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	_ = startQuiz(t, h, cookie) // 开了，然后走开

	if got := quizStatus(t, h, cookie); got.Taken {
		t.Error("中途退出的作答被算成了「做过了」——那条邀请从此再也不会出现")
	}
}

func TestInterestQuiz_FinishedAttemptCountsAndCarriesTheFriction(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := startQuiz(t, h, cookie)
	finishQuiz(t, h, cookie, id, fullAttempt())

	got := quizStatus(t, h, cookie)
	if !got.Taken {
		t.Fatal("交过卷之后应该是「做过了」")
	}
	if got.Latest == nil {
		t.Fatal("没有回最近一次作答")
	}
	// 摩擦是信号，不是噪音（铁律④）：她试了几次才选中证据那一条，必须留下来。
	if got.Latest.ChallengeAttempts != 2 {
		t.Errorf("错误次数没有记下来：得到 %d，want 2", got.Latest.ChallengeAttempts)
	}
	if got.Latest.AnchorReason != reasonText {
		t.Errorf("她自己写的那段没有原样存下来：%q", got.Latest.AnchorReason)
	}
	if got.Latest.FinishedAt == "" {
		t.Error("交卷时间是空的")
	}
}

/* ── 学科透镜 ───────────────────────────────────────────────────────────── */

func TestInterestQuiz_LensesAreRealDisciplinesWithSyllabus(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := startQuiz(t, h, cookie)
	got := finishQuiz(t, h, cookie, id, fullAttempt())

	if len(got.Lenses) < 2 {
		t.Fatalf("只回了 %d 片透镜", len(got.Lenses))
	}
	// 这一屏的全部价值在于它说得出「这在 IB 里叫什么」。一片没有考纲投影的
	// 透镜，和原型里那个手写的学科名字没有区别。
	withSyllabus := 0
	for _, l := range got.Lenses {
		if l.Zh == "" || l.Asks == "" || l.Field == "" {
			t.Errorf("透镜 %s 的内容没有从学科表补齐", l.ID)
		}
		if len(l.Syllabus) > 0 {
			withSyllabus++
		}
	}
	if withSyllabus == 0 {
		t.Error("没有一片透镜带考纲投影 —— 结果页就说不出「这在 IB 里叫什么」")
	}
}

func TestInterestQuiz_NoHookGivesNoLenses(t *testing.T) {
	// 没选钩子就配上三片学科，是替她做了她没做的选择。
	h, cookie, _, _ := liteHandler(t)
	id := startQuiz(t, h, cookie)
	body := fullAttempt()
	body["hook"] = ""
	if got := finishQuiz(t, h, cookie, id, body); len(got.Lenses) != 0 {
		t.Errorf("没选钩子却回了 %d 片透镜", len(got.Lenses))
	}
}

func TestInterestQuiz_UnknownHookIsDroppedNotStored(t *testing.T) {
	// hook 列上有 CHECK 约束；一个没被清洗掉的野值会让整次交卷 500，
	// 而她刚花了五分钟做完。
	h, cookie, _, _ := liteHandler(t)
	id := startQuiz(t, h, cookie)
	body := fullAttempt()
	body["hook"] = "magic"
	got := finishQuiz(t, h, cookie, id, body)
	if got.Attempt.Hook != "" {
		t.Errorf("野钩子被存进去了：%q", got.Attempt.Hook)
	}
}

/* ── 采集 ───────────────────────────────────────────────────────────────── */

// 她只写了「很帅」时：不发那次调用，也不编一个词。harvested=false 让界面能
// 说出正确的那句话（请她多写一句），而不是「这次没长出词」。
func TestInterestQuiz_ThinReasonNeverCallsTheModel(t *testing.T) {
	prov := &countingProvider{inner: quizStubProvider()}
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := startQuiz(t, h, cookie)

	body := fullAttempt()
	body["anchorReason"] = "很帅"
	got := finishQuiz(t, h, cookie, id, body)

	if got.Harvested {
		t.Error("太短的理由不该报告成「采集跑过了」")
	}
	if len(got.Keywords) != 0 {
		t.Errorf("从「很帅」里编出了 %d 个词：%+v", len(got.Keywords), got.Keywords)
	}
	if n := prov.count(); n != 0 {
		t.Errorf("为一句「很帅」发了 %d 次模型调用", n)
	}
}

func TestInterestQuiz_PlantsIntoTheSameTree(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, quizStubProvider())
	id := startQuiz(t, h, cookie)
	got := finishQuiz(t, h, cookie, id, fullAttempt())

	if !got.Harvested {
		t.Fatal("采集没有跑")
	}
	if len(got.Keywords) != 1 {
		t.Fatalf("种下了 %d 个词，want 1：%+v", len(got.Keywords), got.Keywords)
	}

	// 关键的一半：它必须出现在**同一棵树**上，而不是测试自己的一份数据。
	tree := getTree(t, h, cookie)
	var found bool
	for _, k := range tree.Keywords {
		if k.TextZh != "选择与代价" {
			continue
		}
		found = true
		if len(k.Sources) != 1 || k.Sources[0].Kind != "quiz" {
			t.Errorf("来源不对：%+v", k.Sources)
			continue
		}
		// 树的全部说服力都建立在这一条上：这句话是她自己写的。
		if k.Sources[0].Evidence != reasonText {
			t.Errorf("evidence 不是她的原话：%q", k.Sources[0].Evidence)
		}
	}
	if !found {
		t.Fatalf("测试种下的词没有出现在树上：%+v", tree.Keywords)
	}
}

// 🚨 重做是「再长几个词」，不是清空重来。这条依赖 ref_id 存的是作答 id：
// 存 NULL 的话第二次作答会撞进 keyword_source 的 UNIQUE，静默地什么都不加，
// 而界面还是会显示「已完成」—— 一个完全看不出来的哑火。
func TestInterestQuiz_RetakeAddsASecondSourceAndRaisesStrength(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, quizStubProvider())

	first := startQuiz(t, h, cookie)
	finishQuiz(t, h, cookie, first, fullAttempt())
	strBefore, _ := findKeyword(t, getTree(t, h, cookie), "选择与代价")

	second := startQuiz(t, h, cookie)
	finishQuiz(t, h, cookie, second, fullAttempt())
	strAfter, srcAfter := findKeyword(t, getTree(t, h, cookie), "选择与代价")

	if srcAfter != 2 {
		t.Fatalf("重做之后来源是 %d 条，want 2 —— 重做没有加上去", srcAfter)
	}
	if strAfter <= strBefore {
		t.Errorf("强度没有随第二条来源上升：%d → %d", strBefore, strAfter)
	}
}

/* ── 归属 ───────────────────────────────────────────────────────────────── */

func TestInterestQuiz_CannotFinishAnAttemptThatIsNotHers(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := startQuiz(t, h, cookie)

	// 把这一行改挂到别人名下，再用她的 cookie 去交卷。UPDATE 的 WHERE 里带
	// user_id，所以这次交卷必须什么都改不到。
	if _, err := pool.Exec(t.Context(), `
		UPDATE interest_quiz SET user_id = (
			SELECT id FROM users WHERE id <> interest_quiz.user_id LIMIT 1
		) WHERE id = $1`, mustUUID(id)); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if rec := finishQuizRaw(h, cookie, id, fullAttempt()); rec.Code == http.StatusOK {
		t.Fatalf("交掉了别人的作答：%d %s", rec.Code, rec.Body)
	}
}

func TestInterestQuiz_RejectsAMalformedID(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	if rec := finishQuizRaw(h, cookie, "not-a-uuid", fullAttempt()); rec.Code != http.StatusBadRequest {
		t.Errorf("坏 id 得到 %d，want 400", rec.Code)
	}
}

/* ── helpers ────────────────────────────────────────────────────────────── */

// findKeyword 返回 (强度, 来源条数)。
func findKeyword(t *testing.T, tree treeResp, zh string) (int, int) {
	t.Helper()
	for _, k := range tree.Keywords {
		if k.TextZh == zh {
			return k.Strength, len(k.Sources)
		}
	}
	t.Fatalf("树上找不到 %q：%s", zh, fmt.Sprint(tree.Keywords))
	return 0, 0
}
