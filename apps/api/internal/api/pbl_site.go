package api

// pbl_site.go — S5 · 她的主页。
//
// spec §4：她还没有主页的时候，第一个项目就是做一个。**不是推荐，是第一个项目
// 就是它。** 这条规则以前只以两种形式存在过：0108 迁移注释里的一句话，和
// ProjectsLanding.tsx 上一句会自己消失的灰字提示。真正拦住路的东西一样都没有，
// 所以产品负责人从来没触发过它。spec §12 说得很清楚：门槛写在 handler 里，
// 「置灰的按钮是装饰」。这个文件是那道门。
//
// 🚨 这里也是这个产品唯一一处把未成年人的作品发到公网上的地方（另一处是
// atom_report_share.go）。那份文件头列的三条补偿措施在这里逐条照搬，因为它们
// 是仅有的保护：
//
//  1. token 不可猜：crypto/rand 16 字节，绝不用 user id 或 atom id；
//  2. 撤销即时且彻底：token 置回 NULL，下一个请求就查不到，没有缓存、没有宽限；
//  3. 公开载荷只有这一页，别的什么都没有：没有账号、没有邮箱、没有 atom id、
//     没有任何能寻址到她其他东西的 id（见 pbl.SiteItemID）。
//
// 还多一条这里独有的：**页面上没有她自己的字就发不出去**（pbl.SiteMissing）。
// 原型能发布一个一个字都不属于她的页面，因为空的地方全被示例内容填满了。

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pblSiteDTO 是她自己那一侧看到的东西：页面本身、她的草稿、还缺什么、发布状态。
type pblSiteDTO struct {
	Layout string `json:"layout"`
	// Palette 是第三关她定下的配色。零值（三个颜色都空）= 还没定，渲染端用
	// 版式自带的那一套。
	Palette pbl.Palette `json:"palette"`
	// HeroURL 是头图。空 = 她没要头图，那是一个合法的选择。
	HeroURL   string          `json:"heroUrl"`
	Draft     pbl.SiteDraft   `json:"draft"`
	Content   pbl.SiteContent `json:"content"`
	Missing   []string        `json:"missing"`
	Published bool            `json:"published"`
	URL       string          `json:"url"`
	ProjectID string          `json:"projectId"`
}

const (
	maxSiteTextRunes = 2000
	maxSiteListItems = 12
)

/* ── 组装 ─────────────────────────────────────────────────────────────── */

// loadSiteContent 把她真实的行读出来，和她写的字合成一页。
//
// 每次都重新读，不存快照：她昨天写完一篇文章，今天主页上就该有；主页存的是她
// 写的那些字，不是她做过的事的一张旧照片。
func (a *API) loadSiteContent(r *http.Request, userID uuid.UUID, name string, row sqlc.PblSite) (pbl.SiteContent, error) {
	ctx := r.Context()

	var draft pbl.SiteDraft
	if len(row.Content) > 0 {
		// 存坏的 JSON 不该让她的主页变成 500——但也绝不静默换成一份示例内容。
		// 解不开就是一份空草稿，于是页面是空的，发布门槛会拦住它并说缺什么。
		_ = json.Unmarshal(row.Content, &draft)
	}

	writings, err := a.d.Queries.ListSiteWritingsByUser(ctx, userID)
	if err != nil {
		return pbl.SiteContent{}, err
	}
	readings, err := a.d.Queries.ListSiteReadingsByUser(ctx, userID)
	if err != nil {
		return pbl.SiteContent{}, err
	}
	projects, err := a.d.Queries.ListSiteProjectsByUser(ctx, userID)
	if err != nil {
		return pbl.SiteContent{}, err
	}

	uid := userID.String()
	in := pbl.SiteInput{
		DisplayName: name,
		Draft:       draft,
		Since:       row.CreatedAt,
		Updated:     row.UpdatedAt,
	}
	for _, w := range writings {
		it := pbl.SiteItem{
			AtomID: pbl.SiteItemID(uid, w.AtomID.String()),
			Title:  w.Title,
			Words:  int(w.Words),
			Kind:   "文章",
		}
		if w.FinishedAt.Valid {
			it.When, it.HasWhen = w.FinishedAt.Time, true
		}
		in.Posts = append(in.Posts, it)
	}
	for _, rd := range readings {
		it := pbl.SiteItem{
			AtomID: pbl.SiteItemID(uid, rd.AtomID.String()),
			Title:  rd.Title,
			Source: hostOf(rd.SourceUrl),
			Kind:   "在读",
		}
		if rd.FinishedAt.Valid {
			it.When, it.HasWhen = rd.FinishedAt.Time, true
		}
		in.Reads = append(in.Reads, it)
	}
	for _, p := range projects {
		// 名字空着就用她当初写下的那句话——那仍然是她自己的说法。
		title := strings.TrimSpace(p.Name)
		if title == "" {
			title = firstLine(p.Idea)
		}
		kind := "做过的"
		if p.Status == "review" {
			kind = "在做"
		}
		in.Projects = append(in.Projects, pbl.SiteItem{
			AtomID:  pbl.SiteItemID(uid, p.AtomID.String()),
			Title:   title,
			When:    p.AtomCreatedAt,
			HasWhen: true,
			Kind:    kind,
		})
	}
	return pbl.BuildSite(in), nil
}

// hostOf 把来源网址收成一个域名。整条 URL 印在她的主页上是一行噪音，而域名是
// 真实个人站上「来源」那一栏实际会写的东西。解不开就原样留着，不猜。
func hostOf(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n。！？"); i > 0 {
		s = s[:i]
	}
	if r := []rune(s); len(r) > 40 {
		s = string(r[:40])
	}
	return s
}

// ensureSite 读她的主页行，没有就建一个。
func (a *API) ensureSite(r *http.Request, userID uuid.UUID) (sqlc.PblSite, error) {
	row, err := a.d.Queries.GetPblSite(r.Context(), userID)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.PblSite{}, err
	}
	// atom_id 留空：她可能还没开主页项目就先来看了一眼。
	return a.d.Queries.EnsurePblSite(r.Context(),
		sqlc.EnsurePblSiteParams{UserID: userID, AtomID: pgtype.UUID{}})
}

// sitePalette 读这一行里存着的配色。解不开就是零值，而零值不合法
// （ValidPalette 为假），于是渲染端退回版式自带的那一套、发布闸也拦得住——
// 一份坏配色不会变成一页半新半旧的东西。
func sitePalette(row sqlc.PblSite) pbl.Palette {
	var p pbl.Palette
	if len(row.Palette) > 0 {
		_ = json.Unmarshal(row.Palette, &p)
	}
	return p
}

// siteDTO 把一行装成她那一侧要的全部。
func (a *API) siteDTO(r *http.Request, u User, row sqlc.PblSite) (pblSiteDTO, error) {
	content, err := a.loadSiteContent(r, u.ID, u.DisplayName, row)
	if err != nil {
		return pblSiteDTO{}, err
	}
	var draft pbl.SiteDraft
	if len(row.Content) > 0 {
		_ = json.Unmarshal(row.Content, &draft)
	}
	dto := pblSiteDTO{
		Layout:    row.Layout,
		Palette:   sitePalette(row),
		HeroURL:   a.signedOrEmpty(row.HeroKey),
		Draft:     normalizeDraft(draft),
		Content:   content,
		Missing:   pbl.SiteMissing(content),
		Published: row.ShareToken != nil && *row.ShareToken != "",
	}
	if dto.Missing == nil {
		dto.Missing = []string{}
	}
	if dto.Published {
		dto.URL = publicSiteURL(r, a.d.CORSOrigins, *row.ShareToken)
	}
	if row.AtomID.Valid {
		dto.ProjectID = uuid.UUID(row.AtomID.Bytes).String()
	}
	return dto, nil
}

// normalizeDraft 把草稿里每一个 nil 切片换成空切片。
//
// 🚨 Go 的 nil 切片 marshal 出来是 `null`，不是 `[]`。前端拿到 `null` 之后
// `draft.motto.filter(...)` 直接抛 TypeError，整个工作面白屏——2026-09-03 的浏览
// 器 walk 抓到的就是这个：`Cannot read properties of null (reading 'filter')`。
//
// 修在这一侧而不是只在前端兜：这是**接口的形状**问题。`motto: []string` 承诺的
// 是一个列表，`null` 不是列表。listPblProjects 里那句「`[]` 是空看板，`null` 是
// 前端崩溃」讲的是同一件事，只是那次先想到了。
func normalizeDraft(d pbl.SiteDraft) pbl.SiteDraft {
	if d.Motto == nil {
		d.Motto = []string{}
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.About == nil {
		d.About = []string{}
	}
	if d.NowList == nil {
		d.NowList = []string{}
	}
	if d.Blurbs == nil {
		d.Blurbs = map[string]string{}
	}
	return d
}

// publicSiteURL — 她的主页在前端的真实地址，`/p/:token`。
//
// 与 publicShareURL 同一套推导（Origin → 配置的 CORS 源 → 请求自身），因为
// 生产环境里 API 和前端是两个域名，用 API 的域名拼出来的链接点不开。
func publicSiteURL(r *http.Request, corsOrigins []string, token string) string {
	origin := r.Header.Get("Origin")
	if origin == "" && len(corsOrigins) > 0 {
		origin = corsOrigins[0]
	}
	if origin == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		origin = scheme + "://" + r.Host
	}
	return strings.TrimRight(origin, "/") + "/p/" + token
}

/* ── 她自己那一侧 ─────────────────────────────────────────────────────── */

// GET /api/v1/pbl/site
func (a *API) getPblSite(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	row, err := a.ensureSite(r, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.siteDTO(r, u, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// PUT /api/v1/pbl/site/content — 她写的字。
//
// 整份草稿覆盖写。逐字段 PATCH 在这里没有价值：她是在一个编辑面里改自己的页面，
// 一次提交就是一版。
func (a *API) putPblSiteContent(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var draft pbl.SiteDraft
	if err := json.NewDecoder(r.Body).Decode(&draft); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	draft = clampDraft(draft)

	if _, err := a.ensureSite(r, u.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blob, err := json.Marshal(draft)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.SetPblSiteContent(r.Context(),
		sqlc.SetPblSiteContentParams{UserID: u.ID, Content: blob})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.siteDTO(r, u, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// clampDraft 截断过长的输入，并丢掉空条目。不改写、不补写。
func clampDraft(d pbl.SiteDraft) pbl.SiteDraft {
	cut := func(s string) string {
		s = strings.TrimSpace(s)
		if r := []rune(s); len(r) > maxSiteTextRunes {
			return string(r[:maxSiteTextRunes])
		}
		return s
	}
	list := func(xs []string) []string {
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			if s := cut(x); s != "" {
				out = append(out, s)
			}
			if len(out) >= maxSiteListItems {
				break
			}
		}
		return out
	}
	d.Role, d.Headline, d.Lead = cut(d.Role), cut(d.Headline), cut(d.Lead)
	d.Now, d.Contact = cut(d.Now), cut(d.Contact)
	d.Motto, d.Tags, d.About, d.NowList = list(d.Motto), list(d.Tags), list(d.About), list(d.NowList)
	blurbs := map[string]string{}
	for k, v := range d.Blurbs {
		if s := cut(v); s != "" {
			blurbs[k] = s
		}
	}
	d.Blurbs = blurbs
	return d
}

// putPblSiteLook —— 第三关：版式（她管它叫「风格」）和配色。
//
// 🚨 取代了 PUT /pbl/site/layout。那一条要她**写一句理由**才落定——那是
// SiteStudio 那个表单里的一格，而产品负责人 2026-09-03 把整个表单否掉了。
//
// 理由并没有消失，它换了个地方：这一关的配色是从她第一关留下的关键词派生出来
// 的，每一组都写着「它为什么配那几个词」。她挑的时候理由已经在屏幕上了，而且
// 那是一条能被第五关拿去检查的理由——比一段临时写来解锁按钮的话结实得多。
func (a *API) putPblSiteLook(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var req struct {
		Layout  string      `json:"layout"`
		Palette pbl.Palette `json:"palette"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	layout := strings.TrimSpace(req.Layout)
	if !pbl.IsSiteLayout(layout) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_layout", "这个版式不存在", nil))
		return
	}
	// 🚨 她提交的颜色同样要验。一个 "warm beige" 存进去之后，浏览器会把整条 CSS
	// 声明丢掉，而坏掉的是她已经发布出去的那一页。
	if !pbl.ValidPalette(req.Palette) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_palette",
			"配色不完整：三个颜色都要是 #RRGGBB", nil))
		return
	}

	if _, err := a.ensureSite(r, u.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blob, err := json.Marshal(req.Palette)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.SetPblSiteLook(r.Context(),
		sqlc.SetPblSiteLookParams{UserID: u.ID, Layout: layout, Palette: blob})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.siteDTO(r, u, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

/* ── 发布与撤销 ───────────────────────────────────────────────────────── */

// POST /api/v1/pbl/site/publish
func (a *API) publishPblSite(w http.ResponseWriter, r *http.Request) {
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

	row, err := a.ensureSite(r, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	content, err := a.loadSiteContent(r, u.ID, u.DisplayName, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 🚨 门槛一：页面上得有她自己的字。
	if missing := pbl.SiteMissing(content); len(missing) > 0 {
		httpx.WriteError(w, r, httpx.ErrConflict(
			"这一页还缺你自己写的：" + strings.Join(missing, "、") + "。发出去的是你的主页，得先有你说的话。"))
		return
	}
	// 门槛二：调子得是她定的。
	//
	// 🚨 这里原来查的是 `layout_why`——她为版式写下的那一句理由。那一格随
	// SiteStudio 一起退役了（2026-09-04），而这道闸留着，于是页面上每一个字都齐了
	// 也发不出去，报错还指着一个界面上已经不存在的输入框。浏览器 walk 抓到的。
	//
	// 换成查配色：第三关她挑的那一组，是从她第一关的关键词派生出来的，理由印在
	// 每一组底下。这仍然是「没有判断就不落定」，只是那个判断换成了一次她看得见
	// 效果的选择。
	if !pbl.ValidPalette(sitePalette(row)) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先在「视觉基调」里定下配色和风格。"))
		return
	}

	// 已经发过就把**原来那条**链接还回去。重新铸一个会让她已经发出去的链接
	// 悄悄失效——那是她自己都不知道自己弄坏了一件东西的那种坏。
	if row.ShareToken != nil && *row.ShareToken != "" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"url": publicSiteURL(r, a.d.CORSOrigins, *row.ShareToken), "published": true,
		})
		return
	}

	token, err := newShareToken()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if _, err := a.d.Queries.SetPblSiteShare(r.Context(),
		sqlc.SetPblSiteShareParams{UserID: u.ID, ShareToken: &token, PublishedAt: now}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"url": publicSiteURL(r, a.d.CORSOrigins, token), "published": true,
	})
}

// DELETE /api/v1/pbl/site/publish — 撤销。幂等。
func (a *API) revokePblSite(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	if _, err := a.d.Queries.SetPblSiteShare(r.Context(), sqlc.SetPblSiteShareParams{
		UserID: u.ID, ShareToken: nil, PublishedAt: pgtype.Timestamptz{},
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/* ── 公开的那一面 ─────────────────────────────────────────────────────── */

// GET /api/v1/public/sites/{token} — 注册时**不**套 protected/liteOnly。
//
// 未知 token 和已撤销的 token 必须无法区分：两者都落到 pgx.ErrNoRows → 一个
// 干巴巴的 404，不透露这个地址上曾经有过东西。
//
// 载荷只有 content 和 layout。没有账号、没有邮箱、没有学校、没有 atom id、没有
// 她其他任何东西的句柄。不要往这里加字段——公开端点多漏一个字段，就是对所有人
// 永远地漏。
func (a *API) getPublicSite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	row, err := a.d.Queries.GetPblSiteByShareToken(r.Context(), &token)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404，不带细节
		return
	}

	// 🚨 手抄字段：每加一列都要记得在这里也抄一遍，而忘记**不会报错**。
	//
	// 2026-09-04 的浏览器 walk 抓到的就是这个：第三关加了 palette 和 hero_key，
	// 这里没抄，于是访客打开她的主页看到的是版式自带的锈红，而不是她挑的靛蓝。
	// 她那一侧的预览是对的（走的是另一条组装路径），所以这个 bug 只有真的用一个
	// 无 session 的浏览器打开公开链接才看得见。
	//
	// 现在 walk 里量了计算出来的 --st-accent：肉眼在缩略图上分不清 #9C3B26 和
	// #2F5D8A，而这两者的差别正是「她挑了配色」这件事是真是假。
	site := sqlc.PblSite{
		UserID: row.UserID, Layout: row.Layout, LayoutWhy: row.LayoutWhy,
		Content: row.Content, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Palette: row.Palette, HeroKey: row.HeroKey,
	}
	content, err := a.loadSiteContent(r, row.UserID, row.DisplayName, site)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// spec §15：不可索引。她是未成年人，链接是给她发给家人和朋友的，不是给
	// 搜索引擎的。响应头和前端页面里的 meta 各一道。
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	// 配色和头图跟着一起给：访客看到的必须是她定下的那一页，不是版式的默认样子。
	var palette pbl.Palette
	if len(site.Palette) > 0 {
		_ = json.Unmarshal(site.Palette, &palette)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"layout":  row.Layout,
		"palette": palette,
		"heroUrl": a.signedOrEmpty(site.HeroKey),
		"content": content,
	})
}

/* ── spec §4 的那道门 ─────────────────────────────────────────────────── */

// siteGateOpen 报告她能不能开一个自由项目。
//
// 门开着的条件只有一个：**她的主页已经发布了**。spec §4 说第一个项目就是做主页，
// 产品负责人 2026-09-03 把它定成一道完整的门（full gate）而不是一句建议。
//
// 为什么用「已发布」而不是「主页项目存在」：一个开着没做的主页项目挡不住任何事，
// 她开一个然后转头去做别的，门就等于没有。发布是这件事真正做完的那一刻——页面
// 在线上，有地址，能给别人看。
func (a *API) siteGateOpen(r *http.Request, userID uuid.UUID) (bool, error) {
	row, err := a.d.Queries.GetPblSite(r.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.ShareToken != nil && *row.ShareToken != "", nil
}

// POST /api/v1/pbl/site/project — 开始（或回到）主页项目。
//
// 幂等：她已经有一个主页项目时返回原来那个，不建第二个。页面是单数的，项目也
// 应该是。
// seedWebsiteRoutine 把那份「建议的路线」落成第 1 版计划。
//
// ## 为什么建项目的时候就落，而不是等印记第一轮自己生成
//
// 主页项目的题目和路线都是定好的（产品负责人 2026-09-03："with defined topic
// and suggested routine"）。让模型每次现编一份五步计划，只会得到五份不一样的
// 路线，而这条路线本身是产品的一部分，不是一次模型输出。
//
// 落的是 `decided_by = "ai"`、每步 `tentative` 的**未批准**版本——她进来看见的
// 是一份真的任务清单，并且仍然要自己审一遍才开始（「请审核计划并确认，或提出
// 修改意见」）。approvePblPlan 那一刀仍然在她手里，项目也仍然要她批了才 running。
//
// 幂等由调用点保证：主页项目一个人只有一个，已存在就早早返回，走不到这里。
func seedWebsiteRoutine(ctx context.Context, qtx *sqlc.Queries, atomID uuid.UUID) error {
	v, err := qtx.CreatePblPlanVersion(ctx, sqlc.CreatePblPlanVersionParams{
		AtomID: atomID, Version: 1,
		Summary: pbl.RoutineSummary, Reason: pbl.RoutineReason, DecidedBy: "ai",
	})
	if err != nil {
		return err
	}
	for i, s := range pbl.WebsiteRoutine() {
		if _, err := qtx.CreatePblPlanStep(ctx, sqlc.CreatePblPlanStepParams{
			VersionID: v.ID, Ordinal: int32(i + 1),
			Title: s.Title, Blurb: s.Blurb, Goal: s.Goal,
			YouBring: s.YouBring, IBring: s.IBring,
			Decide: s.Decide, ThenBring: s.ThenBring,
			Status: "tentative",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) startPblSiteProject(w http.ResponseWriter, r *http.Request) {
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

	if existing, err := a.d.Queries.GetPblWebsiteProjectByUser(r.Context(), u.ID); err == nil {
		httpx.WriteJSON(w, http.StatusOK, pblProjectDTO{
			ID: existing.AtomID.String(), Idea: existing.Idea, Kind: existing.Kind,
			Name: existing.Name, CoverGround: existing.CoverGround, CoverGlyph: existing.CoverGlyph,
			Status:         existing.Status,
			CreatedAt:      existing.AtomCreatedAt.Format(time.RFC3339),
			LastActivityAt: existing.LastActivityAt.Format(time.RFC3339),
		})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	// atom + 项目 + 主页行，一个事务。缺任何一半都是一个渲染不出来的状态。
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// idea 是这个项目的起点，也是过程评估唯一读得到的「她当初怎么说的」。主页
	// 项目不是她写下的一句话，所以这里写的是这个项目实际是什么。
	p, err := qtx.CreatePblProject(r.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID,
		// idea 存的是**这个项目要回答的问题**，不是一句施工说明。见
		// internal/pbl/website.go · WebsiteIdea。
		Idea: pbl.WebsiteIdea,
		// 🚨 kind 在这里是强制的，不是判出来的。0112 之后类别归她自己填，
		// 唯独这一个由 spec §4 定死——这是唯一一个「项目是什么」不需要问的项目。
		Kind: "website",
		Name: pbl.WebsiteName,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := seedWebsiteRoutine(r.Context(), qtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.EnsurePblSite(r.Context(), sqlc.EnsurePblSiteParams{
		UserID: u.ID, AtomID: pgtype.UUID{Bytes: at.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, pblProjectDTO{
		ID: p.AtomID.String(), Idea: p.Idea, Kind: p.Kind, Name: p.Name,
		CoverGround: p.CoverGround, CoverGlyph: p.CoverGlyph, Status: p.Status,
		CreatedAt:      at.CreatedAt.Format(time.RFC3339),
		LastActivityAt: at.CreatedAt.Format(time.RFC3339),
	})
}
