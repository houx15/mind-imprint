package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_produce.go —— 印记这一轮做出来的东西，落成真的行。
//
// 🚨 2026-09-02 之前，印记**没有任何办法**做出计划、决定、成果、分工、结构。
// CoachOutput 只有 reply / hook / tool 三样。于是这五块界面永远是空的、按钮
// 永远是灰的，而其中三块还写着「到对话里请印记给一个」——让学生去求一件印记
// 结构上做不到的事。表、端点、前端客户端函数全都写好了，链子就断在模型这一头。
//
// 这个文件把断口接上：把 produce 分派给各自的创建逻辑。
//
// 铁律边界（AGENTS.md）：这里做的都是**确定性的系统步骤**——生成计划、按她已
// 经说清楚的事派生大纲、把一个选择摊开给她判断。她要交的正文永远是她自己写。
// 印记交出来的成果是拿给她**审**的，settle 那一刀在她手里。
//
// 失败不影响这一轮对话：她该看见的回话已经写进去了，产出没落上是我们的问题，
// 不该把她那一轮也拖没。记进日志，下一轮印记还可以再做一次。

var errNoStepForSubsteps = errors.New("pbl: no step matches stepTitle")

// 挑课那三种拒绝。都不是"服务端坏了"，是"印记这一轮没挑成"——所以它们和别的
// produce 失败一样只进日志，不影响她看见的那句回话。
var (
	errNoCourseSlug   = errors.New("pbl: course produce names no slug")
	errNoCourseReason = errors.New("pbl: course produce gives no reason")
	errUnknownCourse  = errors.New("pbl: course slug is not in this student's catalogue")
)

// applyPblProduce 把印记做出来的东西落库。调用点在这一轮提交之后。
func (a *API) applyPblProduce(
	ctx context.Context, atomID uuid.UUID, scope pgtype.UUID, p *pbl.Produced,
) error {
	switch p.Kind {
	case "plan":
		return a.producePlan(ctx, atomID, p.Payload)
	case "decision":
		return a.produceDecision(ctx, atomID, scope, p.Payload)
	case "artifact":
		return a.produceArtifact(ctx, atomID, scope, p.Payload)
	case "substeps":
		return a.produceSubsteps(ctx, atomID, p.Payload)
	case "structure":
		return a.produceStructure(ctx, atomID, p.Payload)
	case "site_content":
		return a.produceSiteContent(ctx, atomID, p.Payload)
	case "course":
		// 从课程库里挑一门给她上（见 pbl_course.go）。
		return a.produceCourse(ctx, atomID, scope, p.Payload)
	}
	return fmt.Errorf("pbl: unknown produce kind %q", p.Kind)
}

/* ── 主页内容 ─────────────────────────────────────────────────────────── */

// produceSiteContent —— 第四关「我来生成」：印记把她在这个项目里说过的话摆到
// 她自己的主页上。
//
// ## 这里替掉的是什么
//
// 2026-09-03 之前，这些字是她在 `SiteStudio` 的九个带 label 的输入框里敲进去的。
// 那正是产品负责人否掉的那件事（"don't let students enter forms"）。现在她只是
// 在对话里回答印记的问题，摆放由印记做——**摆放**，不是撰写。
//
// ## 三道闸，一道都不能少
//
//  1. 只在主页项目里。别的项目里这个 kind 不该出现，出现了是错，不是特性。
//  2. `pbl.GroundSiteDraft`：一句话在她自己说过的文字里逐字找不到，就不上页面。
//     这是铁律①在代码里的那道闸，而不是 prompt 里的一句嘱咐。
//  3. 合并而不是覆盖：印记这一轮只摆得出她刚说清楚的那部分，覆盖等于把上一轮
//     已经摆好的东西擦掉。
//
// 全被闸掉（她一句话都没说过，而模型编了一整页）时返回错误：这一轮的对话照旧
// 落库，产出没落上会记进日志，下一轮印记还能再来一次。悄悄写进一页她没说过的
// 话，比什么都不写糟得多。
func (a *API) produceSiteContent(ctx context.Context, atomID uuid.UUID, raw json.RawMessage) error {
	u, ok := UserFromContext(ctx)
	if !ok {
		return errors.New("pbl: site_content without a user in context")
	}
	proj, err := a.d.Queries.GetPblProject(ctx, atomID)
	if err != nil {
		return err
	}
	if proj.Kind != "website" {
		return fmt.Errorf("pbl: site_content in a %q project", proj.Kind)
	}

	var in pbl.SiteDraft
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	own, err := a.studentOwnWords(ctx, atomID)
	if err != nil {
		return err
	}
	grounded, dropped := pbl.GroundSiteDraft(clampDraft(in), own)
	if len(dropped) > 0 {
		slog.Warn("pbl: site_content dropped lines she never said",
			"atom", atomID, "dropped", len(dropped), "first", dropped[0])
	}

	row, err := a.d.Queries.GetPblSite(ctx, u.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var current pbl.SiteDraft
	if len(row.Content) > 0 {
		_ = json.Unmarshal(row.Content, &current)
	}
	merged := mergeSiteDraft(current, grounded)
	if siteDraftEmpty(merged) {
		return fmt.Errorf("pbl: site_content grounded to nothing (%d lines dropped)", len(dropped))
	}

	if _, err := a.d.Queries.EnsurePblSite(ctx, sqlc.EnsurePblSiteParams{
		UserID: u.ID, AtomID: pgtype.UUID{Bytes: atomID, Valid: true},
	}); err != nil {
		return err
	}
	blob, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = a.d.Queries.SetPblSiteContent(ctx,
		sqlc.SetPblSiteContentParams{UserID: u.ID, Content: blob})
	return err
}

// studentOwnWords 是她在这个项目里**自己敲进去的全部文字**，`GroundSiteDraft`
// 拿它当语料。
//
// 主线加每一条支线：她想清楚一件事往往正是在支线里，把支线漏掉，等于告诉她
// 「你在那儿说的话不算」。工具里的产出（便签、结构、审核意见）也算她的话——
// 那些同样是她敲的，而且第二、三关的东西主要落在那里。
func (a *API) studentOwnWords(ctx context.Context, atomID uuid.UUID) (string, error) {
	var b strings.Builder
	add := func(role, content string) {
		if role == "student" && strings.TrimSpace(content) != "" {
			b.WriteString(content)
			b.WriteString("\n")
		}
	}

	main, err := a.d.Queries.ListPblMainThread(ctx, atomID)
	if err != nil {
		return "", err
	}
	for _, m := range main {
		add(m.Role, m.Content)
	}
	sessions, err := a.d.Queries.ListPblSessions(ctx, atomID)
	if err != nil {
		return "", err
	}
	for _, s := range sessions {
		rows, err := a.d.Queries.ListPblSessionMessages(ctx,
			sqlc.ListPblSessionMessagesParams{
				AtomID: atomID, SessionID: pgtype.UUID{Bytes: s.ID, Valid: true},
			})
		if err != nil {
			return "", err
		}
		for _, m := range rows {
			add(m.Role, m.Content)
		}
	}
	tools, err := a.d.Queries.ListPblTools(ctx, atomID)
	if err != nil {
		return "", err
	}
	for _, t := range tools {
		if len(t.Result) > 0 {
			// 工具结果的形状每件不同，所以这里不解析，整块 JSON 当语料——
			// 逐字包含只需要她那些字出现过，键名多出来不影响判断。
			b.Write(t.Result)
			b.WriteString("\n")
		}
		if strings.TrimSpace(t.StudentNote) != "" {
			b.WriteString(t.StudentNote)
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// mergeSiteDraft 用 next 里非空的部分盖住 current，空的部分保留 current。
func mergeSiteDraft(current, next pbl.SiteDraft) pbl.SiteDraft {
	str := func(a, b string) string {
		if strings.TrimSpace(b) != "" {
			return b
		}
		return a
	}
	list := func(a, b []string) []string {
		if len(b) > 0 {
			return b
		}
		return a
	}
	out := pbl.SiteDraft{
		Role:     str(current.Role, next.Role),
		Headline: str(current.Headline, next.Headline),
		Lead:     str(current.Lead, next.Lead),
		Now:      str(current.Now, next.Now),
		Motto:    list(current.Motto, next.Motto),
		Tags:     list(current.Tags, next.Tags),
		About:    list(current.About, next.About),
		NowList:  list(current.NowList, next.NowList),
		// 联系方式只由她自己在界面上填，产出永远不碰它。
		Contact: current.Contact,
		Blurbs:  map[string]string{},
	}
	for k, v := range current.Blurbs {
		out.Blurbs[k] = v
	}
	for k, v := range next.Blurbs {
		if strings.TrimSpace(v) != "" {
			out.Blurbs[k] = v
		}
	}
	return out
}

// siteDraftEmpty 报告这一份草稿里她一个字都没有。
func siteDraftEmpty(d pbl.SiteDraft) bool {
	if strings.TrimSpace(d.Role+d.Headline+d.Lead+d.Now+d.Contact) != "" {
		return false
	}
	if len(d.Motto)+len(d.Tags)+len(d.About)+len(d.NowList)+len(d.Blurbs) > 0 {
		return false
	}
	return true
}

/* ── 计划 ──────────────────────────────────────────────────────────────── */

func (a *API) producePlan(ctx context.Context, atomID uuid.UUID, raw json.RawMessage) error {
	var in struct {
		Summary string `json:"summary"`
		Reason  string `json:"reason"`
		Steps   []struct {
			Title     string `json:"title"`
			Blurb     string `json:"blurb"`
			Goal      string `json:"goal"`
			YouBring  string `json:"youBring"`
			IBring    string `json:"iBring"`
			Decide    string `json:"decide"`
			ThenBring string `json:"thenBring"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	if len(in.Steps) == 0 {
		return errors.New("pbl: plan has no steps")
	}
	// 🚨 每一步都要说清楚她判断什么。一步她什么都不用判断，就是一步不该占她
	// 时间的步骤——和 proposePblPlan 里那道关同一条理由，所以同样在这儿拦住，
	// 而不是把一份空心的计划悄悄放进去。
	for i, s := range in.Steps {
		if strings.TrimSpace(s.Decide) == "" {
			return fmt.Errorf("pbl: step %d says nothing she decides", i+1)
		}
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := a.d.Queries.WithTx(tx)
	if _, err := q.LockAtom(ctx, atomID); err != nil {
		return err
	}
	next, err := q.NextPblPlanVersion(ctx, atomID)
	if err != nil {
		return err
	}
	v, err := q.CreatePblPlanVersion(ctx, sqlc.CreatePblPlanVersionParams{
		AtomID: atomID, Version: next,
		Summary: strings.TrimSpace(in.Summary), Reason: strings.TrimSpace(in.Reason),
		DecidedBy: "ai",
	})
	if err != nil {
		return err
	}
	for i, s := range in.Steps {
		if _, err := q.CreatePblPlanStep(ctx, sqlc.CreatePblPlanStepParams{
			VersionID: v.ID, Ordinal: int32(i + 1),
			Title: strings.TrimSpace(s.Title), Blurb: s.Blurb, Goal: s.Goal,
			YouBring: s.YouBring, IBring: s.IBring,
			Decide: strings.TrimSpace(s.Decide), ThenBring: s.ThenBring,
			Status: "tentative",
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

/* ── 要她拿主意的选择 ──────────────────────────────────────────────────── */

func (a *API) produceDecision(
	ctx context.Context, atomID uuid.UUID, scope pgtype.UUID, raw json.RawMessage,
) error {
	var in struct {
		Subject string `json:"subject"`
		Options []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"options"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		return errors.New("pbl: decision has no subject")
	}
	// 一个选项的"选择"不是选择。
	var good int
	for _, o := range in.Options {
		if strings.TrimSpace(o.Label) != "" {
			good++
		}
	}
	if good < 2 {
		return errors.New("pbl: a decision needs at least two options")
	}

	d, err := a.d.Queries.CreatePblDecision(ctx, sqlc.CreatePblDecisionParams{
		AtomID: atomID, SessionID: scope, Subject: subject,
	})
	if err != nil {
		return err
	}
	for i, o := range in.Options {
		label := strings.TrimSpace(o.Label)
		if label == "" {
			continue
		}
		if _, err := a.d.Queries.CreatePblDecisionOption(ctx, sqlc.CreatePblDecisionOptionParams{
			DecisionID: d.ID, Label: label,
			Description: strings.TrimSpace(o.Description),
			// 选项是印记提的，作者恒为 yinji；选哪个由她定。
			Author: "yinji", Ordinal: int32(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

/* ── 交给她审的东西 ────────────────────────────────────────────────────── */

func (a *API) produceArtifact(
	ctx context.Context, atomID uuid.UUID, scope pgtype.UUID, raw json.RawMessage,
) error {
	var in struct {
		Kind    string   `json:"kind"`
		Title   string   `json:"title"`
		Body    string   `json:"body"`
		URL     string   `json:"url"`
		Guessed []string `json:"guessed"`
		Admits  []string `json:"admits"`
		// 🚨 交东西的同时说清楚每一部分该看什么。
		//
		// createPblReviewPlan 那个端点的注释写的就是「印记交东西时，连着说清楚
		// 每一部分该看什么」——可 produce 的 payload 里一直没有这两格，于是模型
		// 无从填，审核那一屏永远 marks: 0 / dimensions: 0。表、端点、前端的
		// splitByMarks/MarkRow/AnswerBox 全都写好了，链子又断在模型这一头。
		//
		// 少了它们，「审核助手」就只剩"读一段文字，然后点通过"——而
		// docs/2026-09-01-pbl-detail.md 要的是划出来的句子带着问题、几个必须
		// 留意的方面。没有这些，她不知道该看什么，通过就成了走过场。
		Marks []struct {
			Part     string `json:"part"`
			PartNote string `json:"partNote"`
			Quote    string `json:"quote"`
			Question string `json:"question"`
		} `json:"marks"`
		Dimensions []struct {
			Prompt string `json:"prompt"`
			Why    string `json:"why"`
		} `json:"dimensions"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	kind := strings.TrimSpace(in.Kind)
	if !pblArtifactKinds[kind] {
		return fmt.Errorf("pbl: unknown artifact kind %q", kind)
	}
	body, url := strings.TrimSpace(in.Body), strings.TrimSpace(in.URL)
	if body == "" && url == "" {
		// 既没有正文也没有链接的成果，到审核那一屏就是一块空白，而她还被要求
		// 对它下判断。
		return errors.New("pbl: artifact has neither body nor url")
	}
	payload, err := json.Marshal(map[string]string{"body": body, "url": url})
	if err != nil {
		return err
	}
	// guessed / admits 在库里是 jsonb。
	guessed, err := json.Marshal(nonNil(in.Guessed))
	if err != nil {
		return err
	}
	admits, err := json.Marshal(nonNil(in.Admits))
	if err != nil {
		return err
	}
	art, err := a.d.Queries.CreatePblArtifact(ctx, sqlc.CreatePblArtifactParams{
		AtomID: atomID, SessionID: scope, Kind: kind,
		Title: strings.TrimSpace(in.Title), Payload: payload,
		Guessed: guessed, Admits: admits,
	})
	if err != nil {
		return err
	}

	// 划出来的句子。没带问题的一条只是在把字标黄，跳过它——和
	// createPblReviewPlan 那道校验同一条理由。
	//
	// 这几条落不上不该让整份成果作废：她手里有东西可审，比"审得很讲究"要紧。
	var ord int32
	for _, m := range in.Marks {
		q := strings.TrimSpace(m.Question)
		if q == "" {
			continue
		}
		if _, merr := a.d.Queries.CreatePblReviewMark(ctx, sqlc.CreatePblReviewMarkParams{
			ArtifactID: art.ID, Part: strings.TrimSpace(m.Part),
			PartNote: strings.TrimSpace(m.PartNote), Quote: strings.TrimSpace(m.Quote),
			Question: q, Ordinal: ord,
		}); merr != nil {
			return merr
		}
		ord++
	}
	ord = 0
	for _, d := range in.Dimensions {
		prompt := strings.TrimSpace(d.Prompt)
		if prompt == "" {
			continue
		}
		if _, derr := a.d.Queries.CreatePblReviewDimension(ctx, sqlc.CreatePblReviewDimensionParams{
			ArtifactID: art.ID, Prompt: prompt, Why: strings.TrimSpace(d.Why), Ordinal: ord,
		}); derr != nil {
			return derr
		}
		ord++
	}
	return nil
}

// nonNil 让空切片编码成 []，不是 null——前端读的是数组。
func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

/* ── 某一步的分工 ──────────────────────────────────────────────────────── */

func (a *API) produceSubsteps(ctx context.Context, atomID uuid.UUID, raw json.RawMessage) error {
	var in struct {
		StepTitle string `json:"stepTitle"`
		Items     []struct {
			Title string `json:"title"`
			Owner string `json:"owner"`
			Why   string `json:"why"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	if len(in.Items) == 0 {
		return errors.New("pbl: substeps are empty")
	}
	// 分工挂在计划的某一步上。找不到那一步就不落——挂错地方比没挂更难查。
	v, err := a.d.Queries.GetPblLivePlan(ctx, atomID)
	if err != nil {
		return err
	}
	steps, err := a.d.Queries.ListPblPlanSteps(ctx, v.ID)
	if err != nil {
		return err
	}
	want := strings.TrimSpace(in.StepTitle)
	var stepID uuid.UUID
	var found bool
	for _, s := range steps {
		if s.Title == want {
			stepID, found = s.ID, true
			break
		}
	}
	if !found {
		return errNoStepForSubsteps
	}
	for i, it := range in.Items {
		title := strings.TrimSpace(it.Title)
		if title == "" {
			continue
		}
		if _, err := a.d.Queries.CreatePblSubstep(ctx, sqlc.CreatePblSubstepParams{
			StepID: stepID, Title: title, Owner: substepOwner(it.Owner),
			Reason: strings.TrimSpace(it.Why), Ordinal: int32(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

// substepOwner 认不出的归属退回「一起做」，不报错。
//
// 归属是一份**建议**，她本来就可以改；为了一个拼错的词把整份分工丢掉，代价
// 和收益完全不成比例。
func substepOwner(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "yinji", "ai":
		return "yinji"
	case "student", "me":
		return "student"
	}
	return "both"
}

/* ── 结构 ──────────────────────────────────────────────────────────────── */

type produceNode struct {
	Title    string        `json:"title"`
	Body     string        `json:"body"`
	Children []produceNode `json:"children"`
}

func (a *API) produceStructure(ctx context.Context, atomID uuid.UUID, raw json.RawMessage) error {
	var in struct {
		Nodes []produceNode `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	if len(in.Nodes) == 0 {
		return errors.New("pbl: structure has no nodes")
	}
	var write func(ns []produceNode, parent pgtype.UUID, depth int) error
	write = func(ns []produceNode, parent pgtype.UUID, depth int) error {
		// 三层封顶，和界面上的 depth < 3 一致。再深就不是"看得见形状"了。
		if depth >= 3 {
			return nil
		}
		for i, n := range ns {
			title := strings.TrimSpace(n.Title)
			if title == "" {
				continue
			}
			// 🚨 depth 一定要写进去。这个函数一直**算**着 depth（用来在第三层
			// 打住），却从来没把它存下来——印记建的每一个节点 Depth 都是 0。
			// 界面拿 depth 决定列（x = 16 + depth*190）和配色，于是那棵树十五个
			// 节点全挤在同一列、全是同一个颜色，连线缩成一截短斜杠。
			//
			// 「分层配色」这件事从来没在生产上生效过。又一次「算了但没存」。
			row, err := a.d.Queries.CreatePblTreeNode(ctx, sqlc.CreatePblTreeNodeParams{
				AtomID: atomID, Tree: "main", ParentID: parent, Depth: int16(depth),
				Title: title, Body: strings.TrimSpace(n.Body),
				Author: "yinji", Ordinal: int32(i),
			})
			if err != nil {
				return err
			}
			if err := write(n.Children, pgtype.UUID{Bytes: row.ID, Valid: true}, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return write(in.Nodes, pgtype.UUID{}, 0)
}
