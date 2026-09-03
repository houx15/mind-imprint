package api

// interest_harvest.go —— 关键词从她真做过的事情里长出来的那一步。
//
// # 为什么采集不发生在「完成」那一刻
//
// 最直觉的接法是把采集挂在 finishReading / finishWritingAtom 上。没有这么做，
// 有两个理由：
//
//  1. finishReading 现在是一次**瞬时翻转**（它自己的注释就这么写的：a plain
//     synchronous flip）。在上面挂一次三秒的模型调用，会把一个「点一下就好」
//     的动作变成一次等待，而她点完之后要看的报告本身还要再等一次生成。
//  2. 这个代码库里**没有** fire-and-forget 的先例。所有 `go func()` 不是 SSE
//     心跳就是同一个请求里 WaitGroup 汇合的并行取数；没有一处是「handler 返回
//     之后还在后台跑」。为一个新功能开这个先例，代价是无人观测的 goroutine 和
//     一类只在生产上出现的 bug。
//
// 所以采集是**惰性的**，跟 atom_report 一样：在她打开自己的树时才跑，用 advisory
// lock 防并发，跑完盖章。代价付在她正盯着那棵树等它长的时候 —— 那是这次等待唯一
// 说得通的地方。
//
// river 的表已经迁移过了，但客户端和 worker 都还没有（P4 才建）。等 river 真的
// 起来，这一整块应该变成一个入队的任务，采集回到「完成」那一刻，惰性这层就可以
// 删掉。在那之前，这是不引入新架构的最诚实的做法。
//
// # 「尝试过一次」，不是「长出过词」
//
// atom.interest_harvested_at 记的是尝试，不是产出。一篇很薄的阅读完全可能一个
// 词都采不出来，而那个结果和「从没采过」在 interest_keyword 里长得一模一样 ——
// 按「有没有长出词」判断，就会对同一篇薄阅读每次打开树都重发一次旗舰调用，
// 永远采不到，永远重来。这条教训直接来自 reading.questions_at（迁移 0104）。

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/store/sqlc"
)

// harvestBatch 是一次打开树最多补采几个 atom。
//
// 三个，并行跑，所以墙钟时间约等于一次调用。她一口气读完五篇再打开树，会补上
// 最近的三篇，剩下两篇下次进来时补 —— 用一个上限换「树不会因为一次积压而转半
// 分钟」，比让她盯着转圈强。
const harvestBatch = 3

// harvestBodyRuneBudget 限制喂给采集器的文本量。
//
// lite 没有压缩层，一篇长文加上她全部批注可以轻松过万字。采集要的是「她关心
// 什么」，不是通读全文，所以给一个宽裕但有界的预算。
const harvestBodyRuneBudget = 6000

// harvestPending 把她已完成、但还没采过的 atom 补上关键词。
//
// 整个过程是**尽力而为**：任何一步失败都只是少长几个词，绝不冒泡成 HTTP 错误
// ——她请求的是「看我的树」，不是「跑一次采集」。
func (a *API) harvestPending(ctx context.Context, userID uuid.UUID) {
	rows, err := a.d.Queries.ListUnharvestedFinishedAtoms(ctx,
		sqlc.ListUnharvestedFinishedAtomsParams{UserID: userID, Limit: harvestBatch})
	if err != nil {
		slog.Warn("interest harvest: list pending failed", "err", err, "user_id", userID)
		return
	}
	if len(rows) == 0 {
		return
	}

	// 并行。同一个请求里 WaitGroup 汇合的并行取数在这个包里是有先例的
	// （evaluation_generate.go），而三次串行的旗舰调用会让这棵树转十秒。
	var wg sync.WaitGroup
	for _, row := range rows {
		wg.Add(1)
		go func(atomID uuid.UUID, kind string) {
			defer wg.Done()
			a.harvestOneAtom(ctx, userID, atomID, kind)
		}(row.ID, row.Kind)
	}
	wg.Wait()
}

// harvestOneAtom 采集一个 atom。
//
// 顺序是：取锁 → 在锁里复查有没有人已经采过 → 取文本 → 一次模型调用 → 解析 →
// 种词 → 盖章。锁在调用之前，永远不在之后，所以两个并发的第一次打开最多只花
// 一次钱：输的那个在锁上等着，醒来时读到赢家盖的章，直接返回。
func (a *API) harvestOneAtom(ctx context.Context, userID, atomID uuid.UUID, kind string) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		slog.Warn("interest harvest: begin failed", "err", err, "atom_id", atomID)
		return
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// 命名空间后缀，避免和别的端点在同一个 atom 上取的 advisory lock 撞车
	// （见 reading_questions.go 的 ":questions"）。
	if _, err := tx.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtext($1))", atomID.String()+":interest"); err != nil {
		slog.Warn("interest harvest: lock failed", "err", err, "atom_id", atomID)
		return
	}
	qtx := a.d.Queries.WithTx(tx)
	stamped, err := qtx.GetAtomInterestHarvestedAt(ctx, atomID)
	if err != nil {
		slog.Warn("interest harvest: recheck failed", "err", err, "atom_id", atomID)
		return
	}
	if stamped.Valid {
		return // 赢家已经采过了，一分钱都不用再花。
	}

	title, body := a.gatherHarvestText(ctx, atomID, kind)
	// 盖章在**前**：即使下面模型调用失败，这个 atom 也算尝试过了。让它不停重试
	// 的代价（每次打开树都重发一次旗舰调用）远大于漏掉几个词。
	if err := qtx.MarkAtomInterestHarvested(ctx, atomID); err != nil {
		slog.Warn("interest harvest: stamp failed", "err", err, "atom_id", atomID)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Warn("interest harvest: commit failed", "err", err, "atom_id", atomID)
		return
	}

	if strings.TrimSpace(body) == "" {
		return // 她什么也没留下，没有可采的
	}

	// gateway.Collect 对 nil provider 会 panic。这一路今天走不到（跑到这里时
	// provider 总是装好的），但它和 harvestQuiz 是同一个形状，而那一路在测试里
	// 真的会拿到 nil —— 两处用同一个守卫，省得下一个人踩。
	if a.d.Provider == nil {
		return
	}
	resolved, ok := a.route(ctx, gateway.ClassCompose)
	if !ok {
		slog.Warn("interest harvest: no provider resolved", "atom_id", atomID)
		return
	}
	system, user := interest.BuildHarvestPrompt(kind, title, truncateRunes(body, harvestBodyRuneBudget))
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	a.recordLiteLLMCall(ctx, userID, atomID, "interest_harvest", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("interest harvest: provider call failed", "err", cerr, "atom_id", atomID)
		return
	}
	hs, perr := interest.ParseHarvestReply(res.Text)
	if perr != nil {
		// 不长词，也不编词。见 memory: ai-errors-must-surface-never-fake。
		slog.Warn("interest harvest: unparseable reply", "err", perr, "atom_id", atomID)
		return
	}
	// 🚨 evidence 必须真的出自她写下的字，而不是出自 prompt 的脚手架。
	// 见 interest.KeepGrounded —— 这条是真模型实测抓出来的。
	before := len(hs)
	hs = interest.KeepGrounded(hs, body)
	if n := before - len(hs); n > 0 {
		slog.Warn("interest harvest: dropped ungrounded keywords", "dropped", n, "atom_id", atomID)
	}
	a.plantKeywords(ctx, userID, kind, atomID, title, hs)
}

// gatherHarvestText 取出这个 atom 里**她自己写的**东西。
//
// 每一种 atom 的「她的原话」在不同的地方，这是这个函数存在的全部理由：
//
//	reading  她的收获 + 她划的句子和批注（正文是别人的，不进采集）
//	writing  她的正文
//	project  她的立题，以及她做过的判断
//
// 🚨 阅读里**不喂文章正文**。喂了，采集器会去总结那篇文章的话题（「珊瑚」
// 「气候」），而不是找她关心的东西 —— 而话题标签正是设计里明确说了不要的那种
// 关键词。她划出来的句子是她的选择，所以它算她的。
func (a *API) gatherHarvestText(ctx context.Context, atomID uuid.UUID, kind string) (title, body string) {
	var b strings.Builder
	switch kind {
	case "reading":
		if rd, err := a.d.Queries.GetReading(ctx, atomID); err == nil {
			title = rd.Title
		}
		if tk, err := a.d.Queries.GetReadingTakeaway(ctx, atomID); err == nil {
			if s := strings.TrimSpace(tk.Text); s != "" {
				fmt.Fprintf(&b, "她写下的收获：\n%s\n\n", s)
			}
		}
		if anns, err := a.d.Queries.ListAtomAnnotations(ctx, atomID); err == nil && len(anns) > 0 {
			b.WriteString("她划出来的句子，以及她在旁边写的话：\n")
			for _, an := range anns {
				if q := strings.TrimSpace(an.Quote); q != "" {
					fmt.Fprintf(&b, "- 「%s」", q)
					if n := strings.TrimSpace(an.Note); n != "" {
						fmt.Fprintf(&b, " —— 她写：%s", n)
					}
					b.WriteString("\n")
				}
			}
		}
	case "writing":
		if wr, err := a.d.Queries.GetWriting(ctx, atomID); err == nil {
			title = wr.Title
		}
		if dr, err := a.d.Queries.GetWritingDraft(ctx, atomID); err == nil {
			if s := strings.TrimSpace(dr.Body); s != "" {
				fmt.Fprintf(&b, "她写的正文：\n%s\n", s)
			}
		}
	case "project":
		if p, err := a.d.Queries.GetPblProject(ctx, atomID); err == nil {
			title = p.Name
			if s := strings.TrimSpace(p.Idea); s != "" {
				fmt.Fprintf(&b, "她的立题：\n%s\n\n", s)
			}
		}
		if ds, err := a.d.Queries.ListPblDecisions(ctx, atomID); err == nil && len(ds) > 0 {
			b.WriteString("她做过的判断，以及理由：\n")
			for _, d := range ds {
				w := strings.TrimSpace(d.Why)
				if w == "" {
					continue
				}
				fmt.Fprintf(&b, "- 关于「%s」，她选了「%s」，因为：%s",
					strings.TrimSpace(d.Subject), strings.TrimSpace(d.Choice), w)
				// 她明知道放弃了什么 —— 那句话往往比选择本身更能说明她在意什么。
				if g := strings.TrimSpace(d.GaveUp); g != "" {
					fmt.Fprintf(&b, "（她知道这样会失去：%s）", g)
				}
				b.WriteString("\n")
			}
		}
	}
	return title, b.String()
}
