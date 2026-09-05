package api

// pbl_course.go —— 项目里的「去上一课」。
//
// 产品负责人 2026-09-04：「make the pbl part possible to direct students to
// join a course when need. like we teach them to code something, we can host
// some course teaching vibecoding, teaching product thinking, etc.」
//
// 四段都在这个文件里：
//
//	印记挑课（produce "course"）
//	  → 指派落库（pbl_course_assignment）
//	  → 她在项目里打开课程、上完、写下这一课对这个项目有什么用
//	  → 回灌（gatherPblCourseWork）→ 印记接着往下说
//
// 🚨 第四段是产品负责人点名要的：「if trigger course inside pbl parts,
// it should also be end-looped, namely the course interaction and finished
// results go back to the running project and AI makes responses.」
// 少了它，上课就变成了项目外面的一件事——她花四十分钟学完回来，印记还在问
// 上一轮那个问题。

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// maxCourseCatalogue 是喂给印记的课程条数上限。
//
// 目录进的是每一轮对话的 prompt，所以它是**按轮计费**的：课程库涨到两百门，
// 每一轮都要多付两百行。三十门已经够它挑（这个产品的课不是搜索引擎的语料），
// 再多就该做一层检索而不是硬塞。
const maxCourseCatalogue = 30

/* ── 目录：印记挑课时看得见什么 ─────────────────────────────────────────── */

// pblCourseOptions 返回这个学生能上的课，给 prompt 用。
//
// 🚨 用的是和 /courses 目录同一条读法（ListCourses + 受众过滤），所以印记看见
// 的库存和她在课程页看见的**是同一份**。分成两条读法的那一天，印记会递一门她
// 点进去是 404 的课。
//
// 出错返回空：一次课程库读失败不该让整轮对话失败。印记这一轮就是不递课，
// 下一轮再说。
func pblCourseOptions(rows []agent.CourseSummaryRow) []pbl.CourseOption {
	out := make([]pbl.CourseOption, 0, len(rows))
	for _, c := range rows {
		out = append(out, pbl.CourseOption{
			Slug: c.Slug, Title: c.Title, Blurb: c.Blurb, TimeLabel: c.TimeLabel,
		})
		if len(out) >= maxCourseCatalogue {
			break
		}
	}
	return out
}

// courseTitlesBySlug 是 slug → 标题的小表，回灌与「已上过」两处共用。
func courseTitlesBySlug(rows []agent.CourseSummaryRow) map[string]string {
	out := make(map[string]string, len(rows))
	for _, c := range rows {
		out[c.Slug] = c.Title
	}
	return out
}

// listCoursesForCaller 是"这个请求者能看见哪些课"的**唯一**一份实现：已发布 +
// 受众对得上。课程目录端点和印记的挑课目录都从这里读。
//
// 🚨 一轮对话里有三处要它（挑课目录、已上过的课名、回灌里那几句），所以调用点
// 取一次、往下传，而不是各查各的：那是同一份数据在同一个请求里读三遍，还可能
// 三遍读出不一样的东西（一门课正好在这中间被下线）。
func (a *API) listCoursesForCaller(ctx context.Context) ([]agent.CourseSummaryRow, error) {
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rows, err := store.ListCourses(ctx, false) // 草稿不进项目：印记递不出一门还没上线的课
	if err != nil {
		return nil, err
	}
	edition := a.callerEdition(ctx)
	out := make([]agent.CourseSummaryRow, 0, len(rows))
	for _, c := range rows {
		if courseVisibleTo(c.Audience, edition) {
			out = append(out, c)
		}
	}
	return out, nil
}

/* ── 印记挑课 ───────────────────────────────────────────────────────────── */

// produceCourse 把印记这一轮挑的那门课落成一条指派。
//
// 🚨 slug 必须是库里真有、她这个版本看得见的一门。编出来的、下线了的、只给另
// 一个版本的，一律拒绝——落进去她点开是 404，那比印记这一轮什么都没做糟得多。
func (a *API) produceCourse(ctx context.Context, atomID uuid.UUID, scope pgtype.UUID, raw json.RawMessage) error {
	var in struct {
		Slug string `json:"slug"`
		Why  string `json:"why"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	slug := strings.TrimSpace(in.Slug)
	why := strings.TrimSpace(in.Why)
	if slug == "" {
		return errNoCourseSlug
	}
	// 没有理由的课和没有理由的工具一样是伏击（同 summonPblTool 的那道关）。
	if why == "" {
		return errNoCourseReason
	}
	rows, err := a.listCoursesForCaller(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, c := range rows {
		if c.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		return errUnknownCourse
	}
	_, err = a.d.Queries.AssignPblCourse(ctx, sqlc.AssignPblCourseParams{
		AtomID: atomID, SessionID: scope, CourseSlug: slug, Why: why,
	})
	return err
}

/* ── 她这边：看见指派、上完、写下这一课有什么用 ──────────────────────── */

// pblCourseDTO 是「去上一课」这件工具的界面读的一行。
//
// 课程的标题/简介/时长/步数是**服务端补上的**，不是让前端拿到 slug 再去
// /courses 拉一遍：那是一次 N+1，而且中间那一段（拉到一半、课下线了）会让这张
// 卡片显示一个光秃秃的 slug。
type pblCourseDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	// 印记为什么现在递这一课，用「你」跟她说的那句话。
	Why string `json:"why"`
	// 她上完回来写下的那句：这一课对我这个项目有什么用。
	Takeaway   string  `json:"takeaway"`
	FinishedAt *string `json:"finishedAt"`
	CreatedAt  string  `json:"createdAt"`

	// 课程本身。课下线了或者她的版本看不见了，这几样是空的，界面照实说
	// 「这门课已经不在了」，而不是画一张点不开的卡。
	Title     string `json:"title"`
	Blurb     string `json:"blurb"`
	TimeLabel string `json:"timeLabel"`
	StepCount int    `json:"stepCount"`
	CoverURL  string `json:"coverUrl"`
	// 这门课现在还看得见吗。false = 下线了 / 不给这个版本了。
	Available bool `json:"available"`
	// 她在课程里的进度："" = 没开始，"in-progress"，"completed"。
	// 🚨 这一格是「上完了」这件事的真相源——不是 takeaway 有没有写。她可以写完
	// 感想却没上完课，也可以上完了还没写；界面要分得清这两件事。
	CourseStatus   string `json:"courseStatus"`
	CompletedSteps int    `json:"completedSteps"`
}

func (a *API) listPblCourses(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	out, err := a.pblCourseDTOs(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// pblCourseDTOs 把这个项目的课程指派，连同课程本身和她的进度，拼成界面读的行。
func (a *API) pblCourseDTOs(ctx context.Context, atomID uuid.UUID) ([]pblCourseDTO, error) {
	rows, err := a.d.Queries.ListPblCourseAssignments(ctx, atomID)
	if err != nil {
		return nil, err
	}
	out := make([]pblCourseDTO, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}

	// 课程本身。一次拉全目录再按 slug 查表，而不是一门一查——指派最多几门，
	// 目录也就几十行，多一次往返不如少几次。
	byslug := map[string]agent.CourseSummaryRow{}
	if courses, cerr := a.listCoursesForCaller(ctx); cerr == nil {
		for _, c := range courses {
			byslug[c.Slug] = c
		}
	}
	// 她的进度。同样一次拉全（catalog 那条批量查询），不按课 N+1。
	var progress map[string]agent.CourseCatalogProgress
	if u, ok := UserFromContext(ctx); ok {
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		progress, _ = store.ListProgressForUser(ctx, u.ID)
	}

	for _, x := range rows {
		dto := pblCourseDTO{
			ID: x.ID.String(), Slug: x.CourseSlug, Why: x.Why, Takeaway: x.Takeaway,
			CreatedAt: x.CreatedAt.Format(time.RFC3339),
		}
		if x.FinishedAt.Valid {
			s := x.FinishedAt.Time.Format(time.RFC3339)
			dto.FinishedAt = &s
		}
		if c, ok := byslug[x.CourseSlug]; ok {
			dto.Available = true
			dto.Title, dto.Blurb, dto.TimeLabel = c.Title, c.Blurb, c.TimeLabel
			dto.StepCount = c.StepCount
			dto.CoverURL = a.resolveCourseCoverURL(c.Slug, c.Cover)
		}
		if p, ok := progress[x.CourseSlug]; ok {
			dto.CourseStatus, dto.CompletedSteps = p.Status, p.CompletedSteps
		}
		out = append(out, dto)
	}
	return out, nil
}

// finishPblCourse —— 她上完回来了，写下这一课对这个项目有什么用。
//
// 🚨 这是闭环的那一刀。takeaway 必填不是礼貌：回灌读的就是它，一次没留下任何
// 一句话的上课，对印记来说和没发生过一样。
//
// 这一刀本身**不叫模型**——它只落库。接着说话的那一轮由前端紧跟着发一次空文本
// 的 /turn（和做完任何一件工具之后一样，见 ProjectRoom.finishToolInstance），
// 那时候 gatherPblToolWork 已经能读到这条 takeaway 了。两件事分开，是因为落库
// 失败和模型失败是两种错，她该分别看见。
func (a *API) finishPblCourse(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一课不存在"))
		return
	}
	row, err := a.d.Queries.GetPblCourseAssignment(r.Context(), id)
	if err != nil || row.AtomID != atomID || row.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一课不存在"))
		return
	}
	var req struct {
		Takeaway string `json:"takeaway"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	takeaway := strings.TrimSpace(req.Takeaway)
	if takeaway == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_takeaway",
			"请写下这一课对你的项目有什么用", nil))
		return
	}
	if _, err := a.d.Queries.FinishPblCourseAssignment(r.Context(), sqlc.FinishPblCourseAssignmentParams{
		ID: id, Takeaway: takeaway,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := a.pblCourseDTOs(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

/* ── 回灌 ───────────────────────────────────────────────────────────────── */

// gatherPblCourseWork 是这条链子的最后一段：她上完的课，回到印记的 prompt 里。
//
// 只收上完的（有 takeaway 的）。挂在那儿还没上的不进——那是一件待办，不是一份
// 产出，喂进去只会让印记催她。
func (a *API) gatherPblCourseWork(ctx context.Context, atomID uuid.UUID, title map[string]string) []string {
	rows, err := a.d.Queries.ListPblCourseAssignments(ctx, atomID)
	if err != nil || len(rows) == 0 {
		return nil
	}
	var out []string
	for _, x := range rows {
		take := strings.TrimSpace(x.Takeaway)
		if take == "" {
			continue
		}
		name := title[x.CourseSlug]
		if name == "" {
			name = x.CourseSlug
		}
		out = append(out, "他上完了《"+name+"》这一课，回来写下："+take)
	}
	return out
}

// pblCoursesTaken 列出她在这个项目里已经上完的课名——目录不带状态，不说它就会
// 被重复递（和 pblToolState 同一个道理）。
func (a *API) pblCoursesTaken(ctx context.Context, atomID uuid.UUID, title map[string]string) []string {
	rows, err := a.d.Queries.ListPblCourseAssignments(ctx, atomID)
	if err != nil {
		return nil
	}
	var out []string
	for _, x := range rows {
		if !x.FinishedAt.Valid {
			continue
		}
		if name := title[x.CourseSlug]; name != "" {
			out = append(out, "《"+name+"》")
		} else {
			out = append(out, x.CourseSlug)
		}
	}
	return out
}

// pblHasOpenCourse 说的是：这件工具现在打开，里面有课可上吗。
//
// 有一门**还没上完**的指派才算有东西可做。全上完了就没有——否则印记会对着一屋
// 子上完的课再递一次「去上一课」（和 pblToolHasContent 里 decision/artifact
// 那两条同一条理由）。
func (a *API) pblHasOpenCourse(ctx context.Context, atomID uuid.UUID) bool {
	rows, err := a.d.Queries.ListPblCourseAssignments(ctx, atomID)
	if err != nil {
		return false
	}
	for _, x := range rows {
		if !x.FinishedAt.Valid {
			return true
		}
	}
	return false
}
