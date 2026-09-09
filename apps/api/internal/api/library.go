package api

// library.go — 分级阅读（书架）的两个端点。
//
//	GET  /api/v1/library                      书架：全部文章 + 给她的推荐 + 她读过的
//	POST /api/v1/library/{slug}/levels/{tier} 从库里开一篇，返回阅读 id
//
// 库的内容在 internal/library（go:embed 的 articles.json，二十篇 × 五档）。
// 这里只做三件它做不了的事：把她的兴趣树折成一个 Profile、给私有桶里的图签
// 一个能读的链接、把选中的那一档写成一篇真正的阅读。
//
// # 为什么目录一次全发
//
// 二十篇文章的元数据（不含正文）大约十几 KB。一次发完，筛选和搜索就都在前端
// 完成，敲一个字不用打一次请求；「查看全部」那一页也就没有加载态。正文按档次
// 单独取，只在她真的开始读那一篇的时候。
//
// # 图片链接为什么不是常量
//
// 学生桶是私有的（见 internal/oss）。每张图带的是对象键，链接在这里现签，
// 有效期由 OSS_CDN_AUTH_WINDOW 决定（默认两小时）。签名不进 CDN 的缓存键，
// 所以同一张图对所有人只回源一次。OSS 没配时签名会失败 —— 那时候图的 url 是
// 空串，前端不渲染这张图，其余部分照常。一篇读不了的文章比一个白屏好。

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

// 书架上推荐几篇。四是原来那个写死的书架的长度，也是一屏放得下、不用滚的数量。
const libraryRecommendCount = 4

type libraryLevelDTO struct {
	Tier    int    `json:"tier"`
	Name    string `json:"name"`
	Lexile  int    `json:"lexile"` // 0 = 原文，分级不适用
	Words   int    `json:"words"`
	Minutes int    `json:"minutes"`
}

type libraryTagDTO struct {
	ID    string `json:"id"`
	Zh    string `json:"zh"`
	Field string `json:"field"`
}

type libraryArticleDTO struct {
	Slug     string            `json:"slug"`
	Title    string            `json:"title"`
	ZhTitle  string            `json:"zhTitle"`
	Reason   string            `json:"reason"`
	Field    string            `json:"field"`
	Tags     []libraryTagDTO   `json:"tags"`
	CoverURL string            `json:"coverUrl"`
	Levels   []libraryLevelDTO `json:"levels"`
	// ReadingID 非空表示她已经从库里开过这一篇，卡片给「继续读」而不是「开始」。
	ReadingID string `json:"readingId,omitempty"`
	ReadTier  int    `json:"readTier,omitempty"`
	Finished  bool   `json:"finished"`
}

type libraryRecommendationDTO struct {
	Slug string `json:"slug"`
	Tier int    `json:"tier"`
	// Why 是命中的学科（中文名）。空表示这条不是按她的兴趣挑的，界面据此
	// 换一种说法 —— 把猜的说成「按你的兴趣」是在骗人。
	Why []string `json:"why"`
}

type libraryShelfDTO struct {
	Articles    []libraryArticleDTO        `json:"articles"`
	Recommended []libraryRecommendationDTO `json:"recommended"`
	// Fields 是筛选栏：库里真的有文章的那几根主枝。
	Fields []libraryTagDTO `json:"fields"`
	// Tier 是默认难度，见 library.SuggestTier。
	Tier int `json:"tier"`
}

// getLibraryShelf 组书架。
func (a *API) getLibraryShelf(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())

	rows, err := a.d.Queries.ListLibraryReadingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 同一篇可能开过多次（换了一档重开）。ORDER BY created_at DESC 让最近的
	// 那次先到，所以第一次见到的就是要显示在卡片上的那次。
	type readRow struct {
		id       uuid.UUID
		tier     int
		finished bool
	}
	read := make(map[string]readRow, len(rows))
	finishedTop, abandonedTop := 0, 0
	for _, row := range rows {
		if _, seen := read[row.LibrarySlug]; !seen {
			read[row.LibrarySlug] = readRow{row.AtomID, int(row.LibraryTier), row.Status == "finished"}
		}
		tier := int(row.LibraryTier)
		if row.Status == "finished" {
			if tier > finishedTop {
				finishedTop = tier
			}
		} else if tier > abandonedTop {
			abandonedTop = tier
		}
	}

	profile := library.Profile{
		Disciplines: a.interestDisciplines(r),
		ReadSlugs:   make(map[string]bool, len(read)),
		Tier:        library.SuggestTier(finishedTop, abandonedTop),
	}
	for slug := range read {
		profile.ReadSlugs[slug] = true
	}

	articles := library.All()
	out := libraryShelfDTO{
		Articles:    make([]libraryArticleDTO, 0, len(articles)),
		Recommended: make([]libraryRecommendationDTO, 0, libraryRecommendCount),
		Fields:      make([]libraryTagDTO, 0, 7),
		Tier:        profile.Tier,
	}
	for _, f := range library.Fields() {
		out.Fields = append(out.Fields, libraryTagDTO{ID: f, Zh: disciplines.FieldLabels[f], Field: f})
	}
	for _, art := range articles {
		dto := libraryArticleDTO{
			Slug: art.Slug, Title: art.Title, ZhTitle: art.ZhTitle,
			Reason: art.Reason, Field: art.Field,
			Tags:   a.libraryTags(art.Disciplines),
			Levels: make([]libraryLevelDTO, 0, len(art.Levels)),
		}
		if art.Cover != nil {
			dto.CoverURL = a.signObject(art.Cover.Key)
		}
		for _, l := range art.Levels {
			dto.Levels = append(dto.Levels, libraryLevelDTO{
				Tier: l.Tier, Name: l.Name, Lexile: l.Lexile, Words: l.Words, Minutes: l.Minutes,
			})
		}
		if row, ok := read[art.Slug]; ok {
			dto.ReadingID, dto.ReadTier, dto.Finished = row.id.String(), row.tier, row.finished
		}
		out.Articles = append(out.Articles, dto)
	}
	for _, rec := range library.Recommend(articles, profile, libraryRecommendCount) {
		why := make([]string, 0, len(rec.Why))
		for _, id := range rec.Why {
			if d, ok := disciplines.ByID(id); ok {
				why = append(why, d.Zh)
			}
		}
		out.Recommended = append(out.Recommended, libraryRecommendationDTO{
			Slug: rec.Article.Slug, Tier: rec.Tier, Why: why,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// libraryTags 把学科 id 翻成界面上的标签。表外的 id 在 library 包启动校验时
// 就被挡掉了，这里再查一次只是不想让一个漏网的 id 变成一个空标签。
func (a *API) libraryTags(ids []string) []libraryTagDTO {
	out := make([]libraryTagDTO, 0, len(ids))
	for _, id := range ids {
		if d, ok := disciplines.ByID(id); ok {
			out = append(out, libraryTagDTO{ID: d.ID, Zh: d.Zh, Field: d.Field})
		}
	}
	return out
}

// interestDisciplines 把她的兴趣树折成「学科 → 强度」。
//
// 强度 = 词的 strength（1..5，按证据条数算出来的）× 这条词→学科连线的置信度。
// 一个词连到两门学科时两门都加，因为那条连线本来就说「这个词同时属于这两门」。
//
// 读不出来时返回空表而不是报错：树还没长出来的学生（刚注册、还没读完过任何
// 东西）本来就是空表，而那时候书架依然要能打开。
func (a *API) interestDisciplines(r *http.Request) map[string]float64 {
	keywords, err := a.d.Queries.ListInterestKeywords(r.Context(), mustUser(r).ID)
	if err != nil {
		return nil
	}
	strength := make(map[uuid.UUID]float64, len(keywords))
	for _, k := range keywords {
		strength[k.ID] = float64(k.Strength)
	}
	edges, err := a.d.Queries.ListKeywordDisciplinesForUser(r.Context(), mustUser(r).ID)
	if err != nil {
		return nil
	}
	out := make(map[string]float64, len(edges))
	for _, e := range edges {
		out[e.DisciplineID] += strength[e.KeywordID] * float64(e.Confidence)
	}
	return out
}

func mustUser(r *http.Request) User {
	u, _ := UserFromContext(r.Context())
	return u
}

// signObject 给一个私有桶里的对象签一条能读的链接。签不出来（OSS 没配、
// 签名出错）时返回空串，让调用方少渲染一张图而不是整页失败。
func (a *API) signObject(key string) string {
	if a.d.OSS == nil || key == "" {
		return ""
	}
	url, err := a.d.OSS.SignDownload(key)
	if err != nil {
		return ""
	}
	return url
}

// startLibraryReading 从库里开一篇：建 atom + reading + reading_source，一个事务。
//
// 已经开着同一篇的同一档时，返回那一篇，不再建一个新的。她换一档是另一回事
// —— 那是一次真的选择（同一件事换一种写法），值得一条自己的记录。
func (a *API) startLibraryReading(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	slug := r.PathValue("slug")
	art, ok := library.BySlug(slug)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrNotFound("这篇文章不在阅读库里"))
		return
	}
	tier, err := strconv.Atoi(r.PathValue("tier"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_tier", "难度档位要是一个数字", nil))
		return
	}
	lvl, ok := art.LevelAt(tier)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_tier", "这篇文章没有这一档", nil))
		return
	}

	existing, err := a.d.Queries.ListLibraryReadingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, row := range existing {
		if row.LibrarySlug == slug && int(row.LibraryTier) == tier && row.Status != "finished" {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": row.AtomID.String(), "resumed": true})
			return
		}
	}

	figures, err := json.Marshal(lvl.Figures)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	headings, err := json.Marshal(lvl.Headings)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "reading", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 阅读的名字用中文标题：我的阅读那一列里，二十条英文长标题分不出彼此。
	if _, err := qtx.CreateLibraryReading(r.Context(), sqlc.CreateLibraryReadingParams{
		AtomID: at.ID, Title: art.ZhTitle, Lang: art.Lang,
		LibrarySlug: slug, LibraryTier: int16(tier),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.UpsertLibraryReadingSource(r.Context(), sqlc.UpsertLibraryReadingSourceParams{
		AtomID: at.ID, Title: lvl.Title, Body: lvl.Body,
		// 库里的文章没有可以打开的原文链接 —— 它们是我们自己排好的版本。
		// 与其给一条打不开的链接，不如不给。
		SourceUrl: nil, Figures: figures, Headings: headings,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": at.ID.String()})
}

// figureDTO 是正文里的一张图，链接已经签好。
type figureDTO struct {
	After   string `json:"after"`
	URL     string `json:"url"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Caption string `json:"caption"`
	Credit  string `json:"credit"`
}

// readingLayout 读出一篇阅读的版式（图 + 小标题），并把图的链接签好。
//
// 粘贴进来的阅读这两列是空的 —— 那是绝大多数，所以这里对空值不当回事，
// 返回两个 nil，前端照原样渲染纯文字。
func (a *API) readingLayout(figuresJSON, headingsJSON []byte) ([]figureDTO, []string) {
	var figures []library.Figure
	if len(figuresJSON) > 0 {
		if err := json.Unmarshal(figuresJSON, &figures); err != nil {
			return nil, nil
		}
	}
	var headings []string
	if len(headingsJSON) > 0 {
		if err := json.Unmarshal(headingsJSON, &headings); err != nil {
			headings = nil
		}
	}
	if len(figures) == 0 {
		return nil, headings
	}
	out := make([]figureDTO, 0, len(figures))
	for _, f := range figures {
		url := a.signObject(f.Key)
		if url == "" {
			continue // 签不出来就不渲染这一张，不给一个 404 的 <img>
		}
		out = append(out, figureDTO{
			After: f.After, URL: url, Width: f.Width, Height: f.Height,
			Caption: f.Caption, Credit: f.Credit,
		})
	}
	return out, headings
}
