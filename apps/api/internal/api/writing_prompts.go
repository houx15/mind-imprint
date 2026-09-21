package api

// writing_prompts.go —— 写作题库那一屏的接口。
//
// 三条路，形状和分级阅读库那一族一致：
//
//	GET  /api/v1/writing-prompts            翻库（筛 + 搜 + 翻页 + 推荐）
//	GET  /api/v1/writing-prompts/{id}       看一道题
//	POST /api/v1/writing-prompts/{id}/start 从这道题开一篇写作
//
// # 🚨 筛和搜都在服务端做
//
// 705 道题，一次全发给前端是 900KB。而且「筛完之后每一维还剩哪些值」这件事
// 只有看得见全库的人算得出来 —— 放到前端去算，等于前端也得有全库。
// 翻页同理：她要的是第 7 页那 24 张卡，不是让浏览器先拿到 705 张再扔掉 681 张。
//
// # 老师端和学生端走同一条路
//
// 题库是内容，两边看到的是同一份。老师端不需要另一条接口，只是它那边
// 不需要「她开过哪几道」这一层（`mine`）。

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/promptlib"
	"mindimprint/api/internal/store/sqlc"
)

// writingPromptDTO 是一张卡。
//
// 比 promptlib.Prompt 多一样：难度的中文名 —— 界面不该自己再写一份对照表。
//
// 🚨 这里**没有**「她已经从这道题开过写作」那一层。分级阅读库的卡片有
// （reading.library_slug，迁移 0142），作文题这边要一个同样的新列，
// 而这台机器上 sqlc 跑不起来（Rosetta 没了之后那个 x86 二进制废了，
// 见 [[macos27-killed-x86-dev-binaries-2026-09-19]]）。手写一份「生成的」
// 代码比缺这块牌子更糟，所以先不做：卡片一律是「开始写作」。
type writingPromptDTO struct {
	ID         string   `json:"id"`
	Category   string   `json:"category"`
	Lang       string   `json:"lang"`
	Year       int      `json:"year"`
	Type       string   `json:"type"`
	Source     string   `json:"source"`
	Region     string   `json:"region,omitempty"`
	TaskType   string   `json:"taskType"`
	Text       string   `json:"text"`
	WordLimit  string   `json:"wordLimit,omitempty"`
	Minutes    int      `json:"minutes,omitempty"`
	FullScore  int      `json:"fullScore,omitempty"`
	SourceURL  string   `json:"sourceUrl,omitempty"`
	Topics     []string `json:"topics"`
	Difficulty int      `json:"difficulty"`
	DiffLabel  string   `json:"diffLabel"`
}

// 详情那一条多给两样：要求原文和编者注。列表里不给 —— 705 道题的
// requirements 加起来又是一大块，而卡片上根本不显示它。
type writingPromptDetailDTO struct {
	writingPromptDTO
	Requirements string `json:"requirements,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type writingPromptFacetDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type writingPromptRecDTO struct {
	Prompt writingPromptDTO `json:"prompt"`
	Why    []string         `json:"why"`
}

type writingPromptListDTO struct {
	Items    []writingPromptDTO `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Pages    int                `json:"pages"`
	Facets   struct {
		Langs        []writingPromptFacetDTO `json:"langs"`
		Categories   []writingPromptFacetDTO `json:"categories"`
		Difficulties []writingPromptFacetDTO `json:"difficulties"`
		Topics       []writingPromptFacetDTO `json:"topics"`
		Years        []writingPromptFacetDTO `json:"years"`
	} `json:"facets"`
	Recommended []writingPromptRecDTO `json:"recommended"`
}

const writingPromptRecommendCount = 4

func writingPromptDTOOf(p promptlib.Prompt) writingPromptDTO {
	// 🚨 nil 切片 marshal 成 null，前端 `.map()` 当场崩
	// （[[go-nil-slice-becomes-null-2026-09-19]]）。话题是会为空的那一维。
	topics := p.Topics
	if topics == nil {
		topics = []string{}
	}
	return writingPromptDTO{
		ID: p.ID, Category: p.Category, Lang: p.Lang, Year: p.Year,
		Type: p.Type, Source: p.Source, Region: p.Region, TaskType: p.TaskType,
		Text: p.Text, WordLimit: p.WordLimit, Minutes: p.Minutes,
		FullScore: p.FullScore, SourceURL: p.SourceURL,
		Topics: topics, Difficulty: p.Difficulty, DiffLabel: promptlib.DiffLabel(p.Difficulty),
	}
}

func langLabelZh(l string) string {
	if l == promptlib.LangZH {
		return "中文"
	}
	return "英文"
}

func facetsOf(in []promptlib.Facet, label func(string) string) []writingPromptFacetDTO {
	out := make([]writingPromptFacetDTO, 0, len(in))
	for _, f := range in {
		l := f.Value
		if label != nil {
			l = label(f.Value)
		}
		out = append(out, writingPromptFacetDTO{Value: f.Value, Label: l, Count: f.Count})
	}
	return out
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// listWritingPrompts 翻库。
func (a *API) listWritingPrompts(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	q := r.URL.Query()

	query := promptlib.Query{
		Lang:       strings.TrimSpace(q.Get("lang")),
		Category:   strings.TrimSpace(q.Get("category")),
		Difficulty: atoiOr(q.Get("difficulty"), 0),
		Topic:      strings.TrimSpace(q.Get("topic")),
		Year:       atoiOr(q.Get("year"), 0),
		Q:          strings.TrimSpace(q.Get("q")),
		Page:       atoiOr(q.Get("page"), 1),
		PageSize:   atoiOr(q.Get("pageSize"), promptlib.DefaultPageSize),
	}
	res, err := promptlib.Search(query)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	out := writingPromptListDTO{
		Items: make([]writingPromptDTO, 0, len(res.Items)),
		Total: res.Total, Page: res.Page, PageSize: res.PageSize, Pages: res.Pages,
		Recommended: make([]writingPromptRecDTO, 0, writingPromptRecommendCount),
	}
	for _, p := range res.Items {
		out.Items = append(out.Items, writingPromptDTOOf(p))
	}
	out.Facets.Langs = facetsOf(res.Facets.Langs, langLabelZh)
	out.Facets.Categories = facetsOf(res.Facets.Categories, nil)
	out.Facets.Difficulties = facetsOf(res.Facets.Difficulties, func(v string) string {
		return promptlib.DiffLabel(atoiOr(v, 0))
	})
	out.Facets.Topics = facetsOf(res.Facets.Topics, nil)
	out.Facets.Years = facetsOf(res.Facets.Years, nil)

	// 推荐只在**第一页且没筛没搜**的时候给：她已经在翻结果了，
	// 上面再压一排「猜你想写」是在抢她自己那一列的位置。
	if query.Page <= 1 && query.Q == "" && query.Category == "" && query.Topic == "" && query.Year == 0 {
		for _, rec := range promptlib.Recommend(promptlib.Profile{
			Lang: query.Lang, Difficulty: query.Difficulty, Seed: u.ID.String(),
		}, writingPromptRecommendCount) {
			dto := writingPromptDTOOf(rec.Prompt)
			why := rec.Why
			if why == nil {
				why = []string{}
			}
			out.Recommended = append(out.Recommended, writingPromptRecDTO{Prompt: dto, Why: why})
		}
	}

	httpx.WriteJSON(w, http.StatusOK, out)
}

// getWritingPrompt 看一道题。
func (a *API) getWritingPrompt(w http.ResponseWriter, r *http.Request) {
	p, ok := promptlib.ByID(r.PathValue("id"))
	if !ok {
		httpx.WriteError(w, r, httpx.ErrNotFound("没有这道题。"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingPromptDetailDTO{
		writingPromptDTO: writingPromptDTOOf(p),
		Requirements:     p.Requirements,
		Notes:            p.Notes,
	})
}

// startWritingFromPrompt 从一道题开一篇写作。
//
// 🚨 **题面进 assigned_prompt，不进她的第一句话。** 她没说过这段话，
// 把它当成 atom_message 存进去，整条链路下游都会把它当作「她说的」
// （createWritingInTx 的 sayIdea=false 正是为这件事留的）。
// 标题用一个短的题目名，正文那段完整题面由 SetWritingAssignedPrompt 存。
func (a *API) startWritingFromPrompt(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	p, ok := promptlib.ByID(r.PathValue("id"))
	if !ok {
		httpx.WriteError(w, r, httpx.ErrNotFound("没有这道题。"))
		return
	}
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	id, err := a.createWritingFromPrompt(r.Context(), u.ID, p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id.String()})
}

// writingPromptTitle 卡片和房间标题上那一行。
//
// 题面动辄几百字，直接当标题会把房间顶栏撑爆。取出处加题型 ——
// 「2026年北京中考 · 命题作文」——她一眼知道这是哪张卷子上的哪种题。
func writingPromptTitle(p promptlib.Prompt) string {
	src := strings.TrimSpace(p.Source)
	if len([]rune(src)) > 40 {
		src = string([]rune(src)[:40]) + "…"
	}
	if p.TaskType == "" {
		return src
	}
	return src + " · " + p.TaskType
}

func (a *API) createWritingFromPrompt(ctx context.Context, userID uuid.UUID, p promptlib.Prompt) (uuid.UUID, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	// sayIdea=false：这段字不是她说的，不进 atom_message。
	id, err := createWritingInTx(ctx, qtx, userID, writingPromptTitle(p), p.Lang, false)
	if err != nil {
		return uuid.Nil, err
	}
	full := strings.TrimSpace(p.Text)
	if req := strings.TrimSpace(p.Requirements); req != "" {
		full += "\n\n" + req
	}
	if err := qtx.SetWritingAssignedPrompt(ctx, sqlc.SetWritingAssignedPromptParams{
		AtomID: id, AssignedPrompt: &full,
	}); err != nil {
		return uuid.Nil, err
	}
	// 🚨 题目自己写着「不少于800字」，就不要再在「开始之前」里问她一遍
	// （产品负责人 2026-09-21：这几样应该是定好的）。语言跟着题走，
	// 字数从 word_limit 里解出来；解不出来就空着 —— 宁可空着也不猜一个，
	// 篇幅从来不是一道门槛（铁律②）。
	if n := promptlib.TargetWords(p.WordLimit); n > 0 {
		words := int32(n)
		if err := qtx.SetWritingTargetWords(ctx, sqlc.SetWritingTargetWordsParams{
			AtomID: id, TargetWords: &words,
		}); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
