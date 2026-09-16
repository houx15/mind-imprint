package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

// reading_source.go — the article the student is reading. One reading, one
// article: PUT replaces it wholesale (a student who pastes twice meant the
// second one) — but ONLY while nothing is anchored into it yet. See
// refuseIfAnchored.

// refuseIfAnchored blocks a source replacement once ANY process evidence hangs
// off this reading: a card, a margin note, or a single line of transcript.
//
// Block ids are POSITIONAL ("b1" is simply the first paragraph) and anchors
// carry rune offsets into the body they were made against. Swapping the
// article underneath therefore does not orphan those rows — which would at
// least be visible — it silently RE-POINTS every one of them at whatever
// prose now happens to occupy those coordinates. A CRAAP card would come back
// hanging off a sentence the student never read, and 铁律④ says that row is
// evidence a report gets generated from.
//
// Not reachable from today's UI (the room offers the paste box only when the
// reading has no article), so this costs a student nothing; it exists because
// the endpoint is reachable without the UI, and because "not reachable today"
// is not a property that survives a redesign.
//
// The empty case is deliberately permissive: pasting the wrong thing and
// immediately re-pasting is a normal correction, and nothing points at the old
// text yet.
func (a *API) refuseIfAnchored(w http.ResponseWriter, r *http.Request, atomID uuid.UUID) bool {
	n, err := a.d.Queries.CountAtomEvidence(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return true
	}
	if n > 0 {
		httpx.WriteError(w, r, httpx.ErrSourceLocked())
		return true
	}
	return false
}

type sourceDTO struct {
	Title     string `json:"title"`
	SourceURL string `json:"sourceUrl"`
	// Byline 是「来源 · …」那一行，只有从分级阅读库开来的那些才有。
	//
	// 🚨 它**不存在 reading_source 上**，是每次现查目录的。署名是内容，跟着
	// articles.json 走；而 reading 上已经记着 library_slug + library_tier
	// （迁移 0142），凭这两个就能查到，不必为一列文本再动一次 reading_source ——
	// 那张表 pro 也在用。顺带的好处是改一次署名（比如把两种写法统一）只要重
	// 生成 articles.json，不必回头去改已经开出去的每一条阅读记录。
	Byline string  `json:"byline,omitempty"`
	Blocks []Block `json:"blocks"`
	// ExcerptOnly —— Blocks 里放的只是这篇的摘要/导语，不是正文（迁移 0156）。
	//
	// 从探索地图点开一条新闻时，三条取正文的路都没走通就会是这样。阅读室据此
	// 在正文下面摆一条「我们无法直接获取正文，如果想要阅读全文，请跳转原网站」
	// 加一颗跳转按钮 —— 这是 2026-09-16 那条裁定里唯一需要数据支持的一半：
	// 不说出来的话，两句话的摘要长得和一篇很短的文章一模一样。
	ExcerptOnly bool `json:"excerptOnly,omitempty"`
	// 版式，只有从分级阅读库开来的那些才有（迁移 0142）。图不在 Blocks 里：
	// 工具卡挂在段 id 上，一张占了段 id 的图会被当成一段课文引回给学生。每张
	// 图记着自己跟在哪一段之后（after，空串 = 题图），渲染时插在段与段之间。
	Figures []figureDTO `json:"figures,omitempty"`
	// Headings 是要渲染成小标题的段 id。它们仍然是段（SplitBlocks 不认识
	// Markdown），只是长得不一样。
	Headings []string `json:"headings,omitempty"`
	// Outline 是导读：这篇在问什么、它怎么组织、哪几段承重。排读法那一次
	// 算出来的（reading_outline.go），阅读室把它摆在正文顶上。
	// 排读法之前它是空的，那时候整个字段省略。
	Outline *outlineDTO `json:"outline,omitempty"`
}

// outlineDTO 是导读发给前端的形状。
//
// 🚨 `core` 是**段 id 的列表**，不是 load 那张全表。前端要的就是「哪几段是
// 核心」——把三种标签的全表发过去，客户端还得自己再筛一遍，而筛的规则就会
// 变成第二份真相。支撑/过渡这两类在界面上不显示任何东西，发过去也没人用。
type outlineDTO struct {
	OneLine string   `json:"oneLine"`
	Shape   string   `json:"shape"`
	Core    []string `json:"core"`
	// Blocks 是这篇一共几段。导读卡上要说「共 12 段，核心 3 段」，而
	// 前端手里的 Blocks 长度就是它 —— 但那份是切出来的，这一份是服务端
	// 数的，两边对不上的时候以服务端为准（她看到的段号来自服务端）。
	Blocks int `json:"blocks"`
}

// outlineDTOFrom 把存下来的那份导读变成发出去的那份。空的返回 nil，
// 于是 JSON 里整个字段消失，前端因此不必区分「没有导读」和「有一份空导读」。
func outlineDTOFrom(o readingOutline, blocks []Block) *outlineDTO {
	if o.blank() {
		return nil
	}
	return &outlineDTO{
		OneLine: o.OneLine,
		Shape:   o.Shape,
		Core:    o.coreBlockIDs(blocks),
		Blocks:  len(blocks),
	}
}

// isHTTPURL reports whether s parses as a URL with an http or https scheme
// and a host. Anything else is refused by putReadingSourceLite.
func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "http" || scheme == "https") && u.Host != ""
}

func (a *API) putReadingSourceLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	// Before anything else — refusing costs nothing, and the URL branch below
	// would otherwise burn a server-side fetch on a replacement we will reject.
	if a.refuseIfAnchored(w, r, at.ID) {
		return
	}
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Text)
	srcURL := strings.TrimSpace(req.URL)

	// The URL is stored verbatim and later rendered as a link on the teacher's
	// item page. Only http/https may be stored: a `javascript:` or `data:`
	// URL there would run in the teacher's session. Checked before the
	// early-return and the fetch below, so a rejected URL is never fetched
	// or written.
	if srcURL != "" && !isHTTPURL(srcURL) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_url", "请输入以 http 或 https 开头的链接", nil))
		return
	}

	// 🚨 已经有正文了就别再抓一次。
	//
	// 从地图进来的那一篇，正文在建的时候就已经放好了（feed 自带，见
	// mintReadingForPlanet）。而前端在「现在读」之后照样会带着 url 调一次这里 ——
	// 那一次会去抓原页面，抓到就把好好的正文覆盖成另一份，抓不到（走查里就是
	// 403/400）就直接报错，把一篇本来能读的文章变成一条红字。
	//
	// 只挡「带 url、不带正文」这一种。她**粘**一份新的进来（body 非空）照旧
	// 覆盖 —— 那是她明确要换掉这一篇，和这条无关。
	if body == "" && srcURL != "" {
		if existing, err := a.d.Queries.GetReadingSource(r.Context(), at.ID); err == nil &&
			len(SplitBlocks(existing.Body)) > 0 {
			httpx.WriteJSON(w, http.StatusOK, sourceDTO{
				Title: existing.Title, SourceURL: derefOr(existing.SourceUrl, ""),
				Blocks: SplitBlocks(existing.Body), ExcerptOnly: existing.ExcerptOnly,
			})
			return
		}
	}

	// A URL is fetched server-side through the same guarded fetcher the pro
	// side uses; a pasted body is taken as-is.
	if body == "" && srcURL != "" {
		if a.d.Fetcher == nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_unavailable", "暂时无法抓取链接，请直接粘贴正文。", nil))
			return
		}
		fetchedTitle, text, _, err := a.d.Fetcher.FetchReadable(r.Context(), srcURL)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed", "这个链接抓不到正文，请直接粘贴。", nil))
			return
		}
		body = strings.TrimSpace(text)
		if title == "" {
			title = fetchedTitle
		}
	}

	if len(SplitBlocks(body)) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先把文章正文放进来。", nil))
		return
	}
	if title == "" {
		title = "未命名文章"
	}

	// 她自己粘进来的、或者抓成功的那一份，都是正文 —— excerpt_only 归 false。
	// 这也是从摘要升级成正文的那条路：她在摘要那一屏粘了全文，这一行就把
	// 「跳转原网站」那一条收掉了。
	row, err := a.d.Queries.UpsertReadingSource(r.Context(), sqlc.UpsertReadingSourceParams{
		AtomID: at.ID, Title: title, Body: body, SourceUrl: nullableText(srcURL),
		ExcerptOnly: false,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: srcURL, Blocks: SplitBlocks(row.Body),
	})
}

func (a *API) getReadingSourceLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404: nothing pasted yet
		return
	}
	figures, headings := a.readingLayout(row.Figures, row.Headings)
	blocks := SplitBlocks(row.Body)
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: derefOr(row.SourceUrl, ""), Blocks: blocks,
		Byline:  a.libraryByline(r, at.ID),
		Figures: figures, Headings: headings,
		Outline:     outlineDTOFrom(decodeOutline(row.Outline), blocks),
		ExcerptOnly: row.ExcerptOnly,
	})
}

// libraryByline 查这一篇阅读的署名，查不到就返回空串。
//
// 只有从分级阅读库开出来的阅读有署名：她自己粘进来的那些，我们不知道是谁写的，
// 编一个出来比不写更糟。三种「没有」——不是库里来的、目录里查不到这个 slug、
// 这一档没有署名（第一批语料全是这样）——都归到空串，界面据此整行不显示。
//
// 查不到目录不是错误：一篇文章可能在她开了之后从库里下架，而那条阅读记录仍然
// 要能读。所以这里从不返回 error，最坏情况是少一行字。
func (a *API) libraryByline(r *http.Request, atomID uuid.UUID) string {
	rd, err := a.d.Queries.GetReading(r.Context(), atomID)
	if err != nil || rd.LibrarySlug == "" {
		return ""
	}
	art, ok := library.BySlug(rd.LibrarySlug)
	if !ok {
		return ""
	}
	lvl, ok := art.LevelAt(int(rd.LibraryTier))
	if !ok {
		return ""
	}
	return lvl.Byline
}

// nullableText returns a *string for s: nil for an empty/blank string (so the
// store column persists NULL), the trimmed value's address otherwise.
func nullableText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
