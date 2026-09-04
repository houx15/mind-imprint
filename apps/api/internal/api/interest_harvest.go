package api

// interest_harvest.go —— 关键词从她真做过的事情里长出来的那一步。
//
// # 采集在后台跑，不在她的请求里
//
// 调度住在 interest_jobs.go：完成时入队 + 每两分钟一次扫尾。这里只负责「采一个
// atom」这件事本身。
//
// 2026-09-04 之前这一段是**惰性**的：她打开树的时候，顺手补采最近三个还没采过
// 的 atom。当时那么写有两条理由 —— finishReading 是一次瞬时翻转不能挂三秒调用，
// 而且这个代码库里没有 fire-and-forget 的先例。**队列把两条都解决了**，所以那
// 层惰性删掉了，GET /interest/tree 现在是一次纯读。
//
// # 「尝试过一次」，不是「长出过词」
//
// atom.interest_harvested_at 记的是尝试，不是产出。一篇很薄的阅读完全可能一个
// 词都采不出来，而那个结果和「从没采过」在 interest_keyword 里长得一模一样 ——
// 按「有没有长出词」判断，就会对同一篇薄阅读反复重发旗舰调用，永远采不到，
// 永远重来。这条教训直接来自 reading.questions_at（迁移 0104）。

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interest"
)

// harvestBodyRuneBudget 限制喂给采集器的文本量。
//
// lite 没有压缩层，一篇长文加上她全部批注可以轻松过万字。采集要的是「她关心
// 什么」，不是通读全文，所以给一个宽裕但有界的预算。
const harvestBodyRuneBudget = 6000

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
	resolved, ok := a.route(ctx, gateway.ClassDigest)
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
