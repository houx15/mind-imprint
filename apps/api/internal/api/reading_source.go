package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/materialize"
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
	// Editable —— 这份正文现在还能不能整份换掉（PUT /source）。
	//
	// 🚨 2026-09-23 产品负责人第 3 条：「自己粘贴文本后，系统会自动分段，
	// 如果学生发现分段分错了，无法重新编辑，只能再开一个新的。」
	//
	// 服务端**一直是允许的** —— refuseIfAnchored 只在已经有东西锚在正文上
	// 之后才拦（那时候换掉正文会把每一张卡、每一条批注悄悄重新指到别的句子
	// 上，见那个函数的注释）。拦住她的是界面：阅读室只在 excerptOnly 的时候
	// 才挂那个粘贴框，于是她粘完一次就再也回不去了。
	//
	// 这一位就是给界面用的：正文还没被锚定时摆一个「重新编辑原文」。
	// 判据由服务端给，客户端不自己猜 —— 猜错的那一侧是她按下去之后拿到 409。
	Editable bool `json:"editable"`
}

// outlineDTO 是导读发给前端的形状。
//
// 🚨 `core` 是**段 id 的列表**，不是 load 那张全表。前端要的就是「哪几段是
// 核心」——把三种标签的全表发过去，客户端还得自己再筛一遍，而筛的规则就会
// 变成第二份真相。支撑/过渡这两类在界面上不显示任何东西，发过去也没人用。
type outlineDTO struct {
	OneLine string `json:"oneLine"`
	// Gist 是这篇的中心思想（2026-09-16）。老数据没有，前端据此整行不显示。
	Gist  string   `json:"gist,omitempty"`
	Shape string   `json:"shape"`
	Core  []string `json:"core"`
	// Parts 是这篇分成的几个部分，带上每一部分的段号范围 —— 前端要显示
	// 「第 1–3 段」，而段号是服务端数的（她看到的段号来自服务端）。
	Parts []outlinePartDTO `json:"parts,omitempty"`
	// Blocks 是这篇一共几段。导读卡上要说「共 12 段，核心 3 段」，而
	// 前端手里的 Blocks 长度就是它 —— 但那份是切出来的，这一份是服务端
	// 数的，两边对不上的时候以服务端为准（她看到的段号来自服务端）。
	Blocks int `json:"blocks"`
}

// outlinePartDTO 是一个部分发给前端的形状。
//
// 🚨 `FromOrd` / `ToOrd` 是**服务端数出来的段号**，不是前端自己数的。她屏幕上
// 每一段左边那个号码由服务端给（见 readingBlockTag 的注释：模型嘴上说第三段、
// 字段里给 b4 那次事故就是两边各数一遍数出来的），这里跟着同一份。
type outlinePartDTO struct {
	Title   string `json:"title"`
	Does    string `json:"does,omitempty"`
	From    string `json:"from"`
	To      string `json:"to"`
	FromOrd int    `json:"fromOrd"`
	ToOrd   int    `json:"toOrd"`
}

// outlineDTOFrom 把存下来的那份导读变成发出去的那份。空的返回 nil，
// 于是 JSON 里整个字段消失，前端因此不必区分「没有导读」和「有一份空导读」。
func outlineDTOFrom(o readingOutline, blocks []Block) *outlineDTO {
	if o.blank() {
		return nil
	}
	ord := make(map[string]int, len(blocks))
	for i, b := range blocks {
		ord[b.ID] = i + 1
	}
	parts := make([]outlinePartDTO, 0, len(o.Parts))
	for _, p := range o.Parts {
		from, okFrom := ord[p.From]
		to, okTo := ord[p.To]
		if !okFrom || !okTo {
			// 这一篇的正文换过（她重新粘了一份），段 id 对不上了。整份切法
			// 丢掉 —— 指着不存在的段落的台阶比没有台阶更糟。
			parts = nil
			break
		}
		parts = append(parts, outlinePartDTO{
			Title: p.Title, Does: p.Does, From: p.From, To: p.To, FromOrd: from, ToOrd: to,
		})
	}
	return &outlineDTO{
		OneLine: o.OneLine,
		Gist:    o.Gist,
		Shape:   o.Shape,
		Core:    o.coreBlockIDs(blocks),
		Parts:   parts,
		Blocks:  len(blocks),
	}
}

// minFetchedArticleRunes —— 抓回来的正文短到这个数以下，就不算一篇文章。
//
// 🚨 2026-09-22 实测的三种「抓成功了但没用」：页面靠脚本渲染（抽出来是空
// 字符串）、只抽到导航条和 cookie 提示、只抽到一句导语。对她都是同一件事：
// 这篇读不了，得自己粘。
//
// 一百二十个字符大约是三四句话。真文章远在这之上（实测 BBC 首页 3553、
// Guardian 7060、Scientific American 7535），所以这条线不会误伤。
// 判在这里而不是抓取器里：这是产品线，抓取器的单测用的是三行的样例页。
const minFetchedArticleRunes = 120

// readableEnoughToRead —— 抓回来的这一份够不够开一间阅读室。
func readableEnoughToRead(body string) bool {
	return len([]rune(strings.TrimSpace(body))) >= minFetchedArticleRunes
}

// fetchFailureDetail 把抓取器那个机器可读的原因原样交出去。
//
// 原因本身就是她能用上的信息：bad_status 是对方挡住了我们，too_large 是页面
// 太大，no_text 是那一页的字得等脚本跑完才有。不带这一格的话，三件事在屏幕上
// 长得一模一样。
func fetchFailureDetail(err error) string {
	if err == nil {
		return ""
	}
	var fe *materialize.FetchError
	if errors.As(err, &fe) {
		if fe.Err != nil {
			return fe.Reason + ": " + fe.Err.Error()
		}
		return fe.Reason
	}
	return err.Error()
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
			// 🚨 带上后台真正说了什么。产品负责人 2026-09-02：「尽可能给出详细
			// 报错信息，方便 debug」—— 而这一条尤其需要：403（对方挡机器人）、
			// too_large、no_text（页面靠脚本渲染）是三件完全不同的事，她能做的
			// 应对也不一样，而原来这三种都只说「抓不到正文」。
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed",
				"链接抓取失败，请把正文直接粘贴进来。", fetchFailureDetail(err)))
			return
		}
		body = strings.TrimSpace(text)
		if title == "" {
			title = fetchedTitle
		}
		// 抓成功但抓回来的不像一篇文章（导航条、cookie 提示、一句导语）。
		// 让它走到下面那条「先把文章正文放进来」是在说她没粘东西 —— 而她粘的
		// 是一个链接，这句话对不上她刚做的那件事。见 readableEnoughToRead。
		if !readableEnoughToRead(body) {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_thin",
				"链接抓取失败：这个页面取回的正文太短，读不成一篇文章。请把正文直接粘贴进来。",
				fmt.Sprintf("fetched %d characters", len([]rune(body)))))
			return
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
	row, err := a.replaceReadingBody(r.Context(), at.ID, title, body, srcURL)
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
		Editable:    a.sourceStillEditable(r, at.ID),
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

// replaceReadingBody 把一篇阅读的正文换成新的一份（粘贴和上传都走这里）。
//
// 她自己放进来的都是正文 —— excerpt_only 归 false。这也是从摘要升级成正文的那
// 条路（产品负责人 2026-09-17：只有摘要时请她下载原文上传、或者粘贴全文）。
//
// 🚨 正文换了，按旧正文排的东西都要清掉：导读和版式由 UpsertReadingSource 自己
// 清，读法清单在这里清。只有摘要的那一篇，她已经在两段话上排好了一份读法，
// 粘成全文之后那份清单指的段落已经不是原来那两段了。清掉之后前端马上请一次
// 重排（POST /plan）；那一次没成，下一次开口时教练那一轮也会按需重排。
// 同一份正文再存一次不动清单。
//
// srcURL 为空而原来那一行有原网址时，原网址留着 —— 上传的 PDF 没有链接，但
// 这篇文章仍然是那一个网页上的那一篇，「打开原文」不该因为她换了一种方式把
// 全文放进来就消失。
func (a *API) replaceReadingBody(ctx context.Context, atomID uuid.UUID, title, body, srcURL string) (sqlc.ReadingSource, error) {
	previous, prevErr := a.d.Queries.GetReadingSource(ctx, atomID)
	if srcURL == "" && prevErr == nil {
		srcURL = derefOr(previous.SourceUrl, "")
	}
	row, err := a.d.Queries.UpsertReadingSource(ctx, sqlc.UpsertReadingSourceParams{
		AtomID: atomID, Title: title, Body: body, SourceUrl: nullableText(srcURL),
		ExcerptOnly: false,
	})
	if err != nil {
		return sqlc.ReadingSource{}, err
	}
	if prevErr == nil && strings.TrimSpace(previous.Body) != strings.TrimSpace(body) {
		if _, err := a.d.Queries.ReplaceReadingTasks(ctx, sqlc.ReplaceReadingTasksParams{
			AtomID: atomID, Positions: []int32{}, Kinds: []string{}, Labels: []string{},
			Details: []string{}, BlockIds: []string{},
		}); err != nil {
			slog.Warn("reading source: clearing the old plan failed", "err", err, "atom_id", atomID)
		}
	}
	return row, nil
}

// sourceStillEditable —— 这份正文现在还能不能整份换掉。
//
// 和 refuseIfAnchored 同一个判据（锚在正文上的东西一条都还没有），只是不写
// 响应、不报错。查不出来时返回 false：少一个按钮，比给一个按下去就 409 的
// 按钮好（AI errors must surface, never fake 的同一条方向 —— 不装作能做）。
func (a *API) sourceStillEditable(r *http.Request, atomID uuid.UUID) bool {
	n, err := a.d.Queries.CountAtomEvidence(r.Context(), atomID)
	return err == nil && n == 0
}
