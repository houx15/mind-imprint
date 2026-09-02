package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
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
	}
	return fmt.Errorf("pbl: unknown produce kind %q", p.Kind)
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
	_, err = a.d.Queries.CreatePblArtifact(ctx, sqlc.CreatePblArtifactParams{
		AtomID: atomID, SessionID: scope, Kind: kind,
		Title: strings.TrimSpace(in.Title), Payload: payload,
		Guessed: guessed, Admits: admits,
	})
	return err
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
			row, err := a.d.Queries.CreatePblTreeNode(ctx, sqlc.CreatePblTreeNodeParams{
				AtomID: atomID, Tree: "main", ParentID: parent,
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
