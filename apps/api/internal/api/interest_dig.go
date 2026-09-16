package api

// interest_dig.go —— 「继续深挖」：一个关键词后面的四颗种子。
//
//	GET /api/v1/interest/keywords/{id}/dig
//
// # 它补的是哪个洞
//
// 抽屉能证明一个词是从哪来的，但到此为止 —— 模型对她有观察，却对「那接下来
// 干嘛」一无所知。原型在这里摆过四个通用动词（再读一篇 / 写一篇 / 做个项目 /
// 问印记）；四个空动词摆在一个真的观察后面，教的是这个模型没有真的在看她。
//
// # 三颗种子能直接变成一件真东西
//
// `readings` / `writings` / `pbl_project` 的创建接口都只要一个字段（标题、
// 立意、立意），所以 去读 / 去写 / 去做 三颗种子可以**一键变成 lite 里一个真的
// 房间**。前端负责那一步。想一想那颗不导航到任何地方 —— 它是一个拿着走的问题，
// 假装它是个任务，就又变回四个空动词了。
//
// # 生成一次就存着
//
// 每次打开抽屉换一批建议的教练，说明它对你没有看法。所以生成是惰性 + 缓存的，
// 拿 advisory lock，章**盖在生成之前**（第四次了：0104 / 0116 / 0120 / 0121）。

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

type digSeedDTO struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Why  string `json:"why"`
	// Slug / Tier 只在「去读」那一颗上非空：分级阅读库里真的有的那一篇。
	// 前端据此走 POST /library/{slug}/levels/{tier} —— 和书架同一条路。
	// 空 slug = 这一颗没有落到库上，那颗种子不该显示「在阅读室打开」。
	Slug string `json:"slug,omitempty"`
	Tier int    `json:"tier,omitempty"`
}

type digDTO struct {
	KeywordID string       `json:"keywordId"`
	Seeds     []digSeedDTO `json:"seeds"`
	/**
	 * 生成失败时的后台原话。非空表示这次没有种子，而且我们知道为什么。
	 * **绝不摆四个通用动词顶上** —— 那正是这套东西要取代的东西。
	 */
	Note string `json:"note"`
}

// getKeywordDig —— GET /api/v1/interest/keywords/{id}/dig
func (a *API) getKeywordDig(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_keyword_id", "关键词 id 无效", nil))
		return
	}

	// 归属校验：这个词必须是她的。用 (id, user_id) 一起查，所以别人的词直接
	// 404，而不是「存在但看不了」。
	kw, err := a.d.Queries.GetInterestKeywordForUser(r.Context(), sqlc.GetInterestKeywordForUserParams{
		ID: id, UserID: u.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("这个关键词不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}

	// 生成用脱离请求生命周期的 context：她关掉抽屉时，已经花掉的那次调用不该
	// 被取消 —— 种子照样存下来，下次打开就有了。
	gCtx, cancel := detachedModelCtx(r)
	defer cancel()
	note := a.ensureKeywordDig(gCtx, kw)

	rows, err := a.d.Queries.ListKeywordDig(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := digDTO{KeywordID: id.String(), Seeds: []digSeedDTO{}, Note: note}
	for _, s := range rows {
		seed := digSeedDTO{Kind: s.Kind, Text: s.Text, Why: s.Why}
		// 🚨 存下来的 slug 要**再查一次目录**才发出去。种子生成一次就存着，
		// 而目录是内容：一篇文章可能在她这颗种子生成之后从库里下架。那时候
		// 这一颗仍然显示、仍然可点，点下去是一个 404。查不到就当它没落到库上
		// （libraryByline 出于同一个理由也是这么做的）。
		if s.LibrarySlug != "" && s.LibraryTier > 0 {
			if _, ok := library.BySlug(s.LibrarySlug); ok {
				seed.Slug, seed.Tier = s.LibrarySlug, int(s.LibraryTier)
			}
		}
		out.Seeds = append(out.Seeds, seed)
	}
	if len(out.Seeds) > 0 {
		out.Note = "" // 有种子就不必解释了
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ensureKeywordDig 保证这个词已经试过生成一次种子。返回失败时的原话。
func (a *API) ensureKeywordDig(ctx context.Context, kw sqlc.InterestKeyword) string {
	if at, err := a.d.Queries.GetKeywordDigAt(ctx, kw.ID); err == nil && at.Valid {
		return ""
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		slog.Warn("dig: begin failed", "err", err, "keyword_id", kw.ID)
		return "生成失败：数据库连接异常。"
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`,
		"keyword-dig:"+kw.ID.String()); err != nil {
		slog.Warn("dig: advisory lock failed", "err", err, "keyword_id", kw.ID)
		return "生成失败：数据库锁异常。"
	}
	qtx := a.d.Queries.WithTx(tx)

	// 拿到锁之后再看一次：等锁的那个人可能刚生成完。
	if at, err := qtx.GetKeywordDigAt(ctx, kw.ID); err == nil && at.Valid {
		return ""
	}

	evidences, err := qtx.ListKeywordEvidence(ctx, kw.ID)
	if err != nil {
		slog.Warn("dig: list evidence failed", "err", err, "keyword_id", kw.ID)
		return "生成失败：读取来源失败。"
	}

	// 🚨 盖章在生成之前。见迁移 0121。
	if err := qtx.MarkKeywordDigged(ctx, kw.ID); err != nil {
		slog.Warn("dig: stamp failed", "err", err, "keyword_id", kw.ID)
		return "生成失败：写入失败。"
	}

	// 「去读」那一颗只能落在阅读库上，所以候选和难度档都要先算出来：候选送进
	// prompt 供模型挑，档位跟着她读过的历史走（和书架同一个 SuggestTier）。
	// 读不出她的档案不算失败 —— 候选为空时模型就不该给「去读」那一颗，而解析
	// 那一侧不认任何 slug，所以它编一个也进不来。
	profile, perr := libraryProfileIn(ctx, qtx, kw.UserID)
	if perr != nil {
		slog.Warn("dig: library profile failed", "err", perr, "keyword_id", kw.ID)
	}
	candidates := digLibraryCandidates(profile)

	seeds, note := a.digSeeds(ctx, kw, evidences, candidates)
	for _, s := range seeds {
		if !interest.IsDigKind(string(s.Kind)) || s.Text == "" {
			continue
		}
		tier := 0
		if s.LibrarySlug != "" {
			tier = profile.Tier
			if tier < 1 || tier > 5 {
				tier = 2
			}
		}
		if err := qtx.UpsertKeywordDig(ctx, sqlc.UpsertKeywordDigParams{
			KeywordID: kw.ID, Kind: string(s.Kind), Text: s.Text, Why: s.Why,
			LibrarySlug: s.LibrarySlug, LibraryTier: int32(tier),
		}); err != nil {
			slog.Warn("dig: upsert failed", "err", err, "keyword_id", kw.ID)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Warn("dig: commit failed", "err", err, "keyword_id", kw.ID)
		return "生成失败：提交失败。"
	}
	return note
}

// digLibraryCandidateCount 是送进 prompt 的候选文章篇数。
//
// 十二篇：够让模型在几个学科之间真的挑一下，又不至于把这段名单撑成 prompt 里
// 最长的一块。Recommend 已经按她的兴趣排过序，所以前十二篇就是最相关的十二篇。
const digLibraryCandidateCount = 12

// digLibraryCandidates 把她的阅读档案折成「去读」那一颗能挑的名单。
//
// 用的是书架同一个 Recommend，所以这里挑出来的和她在书架上看到的推荐是同一套
// 排序（她读过的那些已经被 Recommend 剔掉了）。
func digLibraryCandidates(p library.Profile) []interest.LibraryCandidate {
	recs := library.Recommend(library.All(), p, digLibraryCandidateCount)
	out := make([]interest.LibraryCandidate, 0, len(recs))
	for _, rec := range recs {
		title := strings.TrimSpace(rec.Article.ZhTitle)
		if title == "" {
			title = strings.TrimSpace(rec.Article.Title)
		}
		if title == "" {
			continue
		}
		out = append(out, interest.LibraryCandidate{
			Slug: rec.Article.Slug, Title: title, Reason: rec.Article.Reason,
		})
	}
	return out
}

// digSeeds 发那一次调用。返回 (种子, 失败原话)。
//
// 失败时**返回零颗种子加一句原话** —— 绝不用四个通用动词顶上。
func (a *API) digSeeds(
	ctx context.Context, kw sqlc.InterestKeyword, evidences []string,
	candidates []interest.LibraryCandidate,
) ([]interest.Seed, string) {
	if a.d.Provider == nil {
		return nil, "生成失败：模型通道未配置。"
	}
	// ClassCompose：从已陈述的输入派生一个 schema 产物 —— 正是这件事。
	resolved, ok := a.route(ctx, gateway.ClassCompose)
	if !ok {
		return nil, "生成失败：没有可用的模型通道。"
	}
	system, user := interest.BuildDigPrompt(kw.TextZh, kw.Note, evidences, candidates)
	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	a.recordLiteLLMCall(ctx, kw.UserID, uuid.Nil, "interest_dig", resolved, res.Usage)
	if cerr != nil {
		return nil, "生成失败：" + cerr.Error()
	}
	seeds, perr := interest.ParseDigReply(res.Text, candidates)
	if perr != nil {
		return nil, "生成失败：" + perr.Error()
	}
	return seeds, ""
}
