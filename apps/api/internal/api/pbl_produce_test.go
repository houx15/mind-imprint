package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// pbl_produce_test.go —— 印记做出来的东西，要真的出现在她的界面上。
//
// 🚨 这一串是整条链子最容易断、而且断了完全看不出来的一环。
//
// 2026-09-02 查出来的：CoachOutput 只有 reply / hook / tool 三样，印记**没有
// 任何办法**做出计划、决定、成果、分工、结构。审核助手 / 理性决策 / 分工建议 /
// 结构审查 / 计划这五块界面因此永远是空的、按钮永远是灰的，而其中三块还写着
// 「到对话里请印记给一个」——让学生去求一件印记结构上做不到的事。
//
// 表、端点、前端客户端函数全都写好了，只有模型那一头不知道自己能做这些。前端
// 测试全绿，Go 测试全绿，编译通过，界面"渲染正确"——它只是永远没有东西可渲染。
// 所以这里每一条都从**一轮真的对话**出发，走到**她那一屏读到的数据**为止。

// coachProducing 让假模型在一轮里做出一件东西。
func coachProducing(kind, payload string) string {
	return `{"reply":"我先给你出一版，你看看哪里不对。","produce":{"kind":"` +
		kind + `","payload":` + payload + `}}`
}

func turnProducing(t *testing.T, h http.Handler, c *http.Cookie, pid string) {
	t.Helper()
	rec := pblPost(t, h, c, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"那就开始吧"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
}

// 计划：每个学生一进项目看到的就是这一块。没有生产者时它永远写着「计划待生成」。
func TestPblProduce_PlanReachesThePanel(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("plan", `{
	  "summary":"先弄清楚剩饭到底有多少，再想怎么少",
	  "reason":"你现在说的都是猜的",
	  "steps":[
	    {"title":"去食堂看三天","decide":"你判断哪一顿剩得最多"},
	    {"title":"问三个同学","decide":"你判断他们说的和你看到的是不是一回事"}]}`)))
	pid := newProjectViaAPI(t, h, cookie)
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/plan", "")
	var env struct {
		Plan *struct {
			Summary    string  `json:"summary"`
			ApprovedAt *string `json:"approvedAt"`
			Steps      []struct {
				Title  string `json:"title"`
				Decide string `json:"decide"`
			} `json:"steps"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode plan: %v — %s", err, rec.Body)
	}
	if env.Plan == nil || env.Plan.Summary == "" {
		t.Fatalf("印记出的计划没到面板上：%s", rec.Body)
	}
	if len(env.Plan.Steps) != 2 || env.Plan.Steps[0].Title != "去食堂看三天" {
		t.Fatalf("步骤不对：%+v", env.Plan.Steps)
	}
	// 🚨 看得见，但还没生效——她得先审、先按确认。这一版就该是未批准状态。
	if env.Plan.ApprovedAt != nil {
		t.Fatalf("印记提的计划自己就生效了，没经过她：%+v", env.Plan)
	}
}

// 🚨 每一步都要说清楚她判断什么。一步她什么都不用判断，就是一步不该占她时间
// 的步骤——一份空心的计划宁可不落，也不能悄悄放进去。
func TestPblProduce_PlanWithoutADecisionIsRefused(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("plan", `{
	  "summary":"随便走走","steps":[{"title":"先做点什么","decide":""}]}`)))
	pid := newProjectViaAPI(t, h, cookie)
	// 这一轮本身要照常成功：她该看见印记说的话，产出落不下不是她的事。
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/plan", "")
	var env struct {
		Plan *json.RawMessage `json:"plan"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Plan != nil && string(*env.Plan) != "null" {
		t.Fatalf("一份没说清楚她判断什么的计划被放进去了：%s", rec.Body)
	}
}

// 决定：理性决策那一屏在这之前永远是「暂时没有需要决策的内容」。
func TestPblProduce_DecisionReachesHer(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("decision", `{
	  "subject":"先跟谁说这件事",
	  "options":[
	    {"label":"先给食堂","description":"只有他们能真的把菜量改了"},
	    {"label":"先发班群","description":"反馈快，但同学说了也改不了食堂的量"}]}`)))
	pid := newProjectViaAPI(t, h, cookie)
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/decisions", "")
	var ds []struct {
		Subject string `json:"subject"`
		Options []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ds); err != nil {
		t.Fatalf("decode decisions: %v — %s", err, rec.Body)
	}
	if len(ds) != 1 || ds[0].Subject != "先跟谁说这件事" {
		t.Fatalf("决定没到她手边：%s", rec.Body)
	}
	if len(ds[0].Options) != 2 {
		t.Fatalf("选项数不对：%+v", ds[0].Options)
	}
	// 每个选项都要说清楚它意味着什么，否则她只是在两个标签之间猜。
	if ds[0].Options[0].Description == "" {
		t.Fatalf("选项没说明它意味着什么：%+v", ds[0].Options[0])
	}
}

// 🚨 一个选项的"选择"不是选择。
func TestPblProduce_DecisionNeedsTwoOptions(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("decision", `{
	  "subject":"要不要做","options":[{"label":"做"}]}`)))
	pid := newProjectViaAPI(t, h, cookie)
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/decisions", "")
	var ds []json.RawMessage
	_ = json.Unmarshal(rec.Body.Bytes(), &ds)
	if len(ds) != 0 {
		t.Fatalf("只有一个选项的「选择」被放进去了：%s", rec.Body)
	}
}

// 成果：审核助手那一屏在这之前永远是「暂时没有需要审核的内容」。
func TestPblProduce_ArtifactReachesReview(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("artifact", `{
	  "kind":"draft","title":"给食堂的一封信",
	  "body":"第一段。\n\n第二段。",
	  "guessed":["我猜你想先说数据"],
	  "admits":["第二段我写得太长了"]}`)))
	pid := newProjectViaAPI(t, h, cookie)
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var as []struct {
		Kind    string   `json:"kind"`
		Title   string   `json:"title"`
		Guessed []string `json:"guessed"`
		Admits  []string `json:"admits"`
		Payload struct {
			Body string `json:"body"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &as); err != nil {
		t.Fatalf("decode artifacts: %v — %s", err, rec.Body)
	}
	if len(as) != 1 || as[0].Title != "给食堂的一封信" {
		t.Fatalf("成果没到审核那一屏：%s", rec.Body)
	}
	if as[0].Payload.Body == "" {
		t.Fatal("成果没有正文——她要审的就是那段字")
	}
	// 印记自己说清楚猜了什么、哪里还不对，她才有得可判。
	if len(as[0].Guessed) != 1 || len(as[0].Admits) != 1 {
		t.Fatalf("印记没交代猜了什么、哪里不对：%+v", as[0])
	}
}

// 既没正文也没链接的成果，到审核那一屏就是一块空白，而她还被要求对它下判断。
func TestPblProduce_EmptyArtifactIsRefused(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("artifact", `{
	  "kind":"draft","title":"空的"}`)))
	pid := newProjectViaAPI(t, h, cookie)
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var as []json.RawMessage
	_ = json.Unmarshal(rec.Body.Bytes(), &as)
	if len(as) != 0 {
		t.Fatalf("一份没有内容的成果被送去让她审了：%s", rec.Body)
	}
}

// 结构：结构审查那一屏在这之前永远写着「到对话里请印记先给一个」，
// 而印记做不到。
func TestPblProduce_StructureReachesTheMindMap(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(coachProducing("structure", `{
	  "nodes":[
	    {"title":"现状","children":[{"title":"每天剩多少"},{"title":"剩的都是什么"}]},
	    {"title":"办法"}]}`)))
	pid := newProjectViaAPI(t, h, cookie)
	turnProducing(t, h, cookie, pid)

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tree", "")
	var tree struct {
		Nodes []struct {
			Title    string  `json:"title"`
			ParentID *string `json:"parentId"`
			Author   string  `json:"author"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode tree: %v — %s", err, rec.Body)
	}
	if len(tree.Nodes) != 4 {
		t.Fatalf("结构没长出来，共 %d 个节点：%s", len(tree.Nodes), rec.Body)
	}
	// 层级要真的建起来，不能摊平成一列——摊平了就不是"看得见形状"。
	var withParent int
	for _, n := range tree.Nodes {
		if n.ParentID != nil && *n.ParentID != "" {
			withParent++
		}
		if n.Author != "yinji" {
			t.Fatalf("这棵结构是印记给的，作者不对：%+v", n)
		}
	}
	if withParent != 2 {
		t.Fatalf("子节点没挂上父节点，挂上的有 %d 个：%+v", withParent, tree.Nodes)
	}
}

// 认不出的 kind 当作没做。半个产出比没有产出更糟：界面会为它腾出位置，
// 然后摆一块空白。
func TestPblProduce_UnknownKindIsIgnoredAndTheTurnStillLands(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我们接着说。","produce":{"kind":"随便","payload":{"x":1}}}`))
	pid := newProjectViaAPI(t, h, cookie)
	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"好"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("一个认不出的产出把她那一轮拖没了：%d %s", rec.Code, rec.Body)
	}
	var dto struct {
		Reply string `json:"reply"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &dto)
	if dto.Reply == "" {
		t.Fatalf("她没收到印记的回话：%s", rec.Body)
	}
}
