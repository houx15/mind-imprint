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
	"mindimprint/api/internal/store/sqlc"
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
	a.landHarvest(ctx, userID, kind, atomID, title, hs)
}

// landHarvest 决定采出来的这几个词往哪去：直接种，还是先摆成候选等她认。
//
// 🚨 **阅读和写作只提候选，不直接种**（2026-09-07，迁移 0140）。这棵树的说法是
// 「这就是你的模型」，而一个她没点过头的模型只是我们对她的记录。她读完会看到
// 报告，候选词就摆在报告上，认不认由她。
//
// 两条例外，都不是偷懒：
//
//   - **她树上已经有的那个词直接种。** 那是给一个已经认过的词再添一条来源
//     （强度往上走），不是一个新说法。为一个三个月前就认过的词再问一次同意，
//     是把同意变成打卡。
//   - **项目和兴趣测试照旧直接种。** 兴趣测试本身就是她在挑词 —— 那一步已经
//     是同意了，再问一遍是不认账。
func (a *API) landHarvest(
	ctx context.Context,
	userID uuid.UUID,
	kind string,
	atomID uuid.UUID,
	title string,
	hs []interest.Harvested,
) {
	if kind != "reading" && kind != "writing" {
		a.plantKeywords(ctx, userID, kind, atomID, title, hs)
		return
	}

	have, err := a.d.Queries.ListUserInterestIDs(ctx, userID)
	if err != nil {
		// 读不到她已有的词，就当一个都没有：全部走候选。宁可多问一次，也不要
		// 在她没点头的情况下往树上写。
		slog.Warn("interest harvest: list existing failed", "err", err, "atom_id", atomID)
		have = nil
	}
	known := make(map[string]bool, len(have))
	for _, id := range have {
		if id != nil {
			known[*id] = true
		}
	}

	var grow []interest.Harvested
	for _, h := range hs {
		if known[h.InterestID] {
			grow = append(grow, h)
			continue
		}
		if err := a.d.Queries.ProposeInterest(ctx, sqlc.ProposeInterestParams{
			UserID:     userID,
			AtomID:     atomID,
			InterestID: h.InterestID,
			Note:       h.Note,
			Evidence:   h.Evidence,
		}); err != nil {
			slog.Warn("interest harvest: propose failed", "err", err, "interest_id", h.InterestID)
		}
	}
	if len(grow) > 0 {
		a.plantKeywords(ctx, userID, kind, atomID, title, grow)
	}
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
		// 🚨 她在带读里说的话。上面那两样今天**都可能是空的**：归纳表（我的收获）
		// 已经删掉了，而阅读室至今没有写批注的控件（批注是只读的）。也就是说一次
		// 正常走完的阅读——读、跟印记聊、完成——采到的是一个空字符串，
		// harvestOneAtom 于是在调用模型之前就返回，atom 却已经盖了章。结果是
		// 阅读永远长不出词，而地图上写着「读完之后，报告上会提出可以加进你树里
		// 的词」。2026-09-08 的全链路走查就是卡在这里。
		//
		// 只取 role="student"。印记说的话不是她的话，喂回去采出来的会是印记的
		// 用词——这正是 `interest.KeepGrounded` 在防的那件事。
		if msgs, err := a.d.Queries.ListAtomMessages(ctx, atomID); err == nil {
			var said []string
			for _, m := range msgs {
				if m.Role != "student" {
					continue
				}
				if s := strings.TrimSpace(m.Content); s != "" {
					said = append(said, s)
				}
			}
			if len(said) > 0 {
				b.WriteString("她在带读里说的话：\n")
				for _, s := range said {
					fmt.Fprintf(&b, "- %s\n", s)
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
			// An assigned project's idea is the teacher's driving question: harvesting
			// it would grow her tree from words she never wrote.
			if s := strings.TrimSpace(p.Idea); s != "" && !p.Assigned {
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
