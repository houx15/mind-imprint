package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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

type showcaseConfig struct {
	Name            string   `json:"name"`
	Bio             string   `json:"bio"`
	Tagline         string   `json:"tagline"`
	Interests       []string `json:"interests"`
	Layout          string   `json:"layout"`
	Palette         string   `json:"palette"`
	Font            string   `json:"font"`
	WritingStyle    string   `json:"writingStyle"`
	ReadingStyle    string   `json:"readingStyle"`
	SectionOrder    []string `json:"sectionOrder"`
	SelectedWorkIDs []string `json:"selectedWorkIds"`
}

type showcaseWork struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	PublicPath string `json:"publicPath,omitempty"`
}

type showcaseState struct {
	Draft                 showcaseConfig `json:"draft"`
	Revision              int32          `json:"revision"`
	Published             bool           `json:"published"`
	HasUnpublishedChanges bool           `json:"hasUnpublishedChanges"`
	URL                   string         `json:"url"`
	AvailableWorks        []showcaseWork `json:"availableWorks"`
	HasLegacySite         bool           `json:"hasLegacySite"`
}

func defaultShowcase(name string) showcaseConfig {
	return showcaseConfig{Name: strings.TrimSpace(name), Interests: []string{}, Layout: "folio", Palette: "paper", Font: "sans", WritingStyle: "cards", ReadingStyle: "shelf", SectionOrder: []string{"writing", "reading", "project"}, SelectedWorkIDs: []string{}}
}

func oneOf(v string, allowed ...string) bool {
	for _, x := range allowed {
		if v == x {
			return true
		}
	}
	return false
}
func trimShowcaseRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func normalizeShowcase(c showcaseConfig) (showcaseConfig, error) {
	c.Name, c.Bio, c.Tagline = strings.TrimSpace(c.Name), strings.TrimSpace(c.Bio), strings.TrimSpace(c.Tagline)
	if len([]rune(c.Name)) > 80 || len([]rune(c.Tagline)) > 200 || len([]rune(c.Bio)) > 2000 {
		return c, errors.New("主页文字超过长度限制")
	}
	if !oneOf(c.Layout, "folio", "journal", "studio") || !oneOf(c.Palette, "paper", "forest", "ocean", "rose", "night", "sunshine") || !oneOf(c.Font, "sans", "serif", "mono") || !oneOf(c.WritingStyle, "cards", "list") || !oneOf(c.ReadingStyle, "shelf", "list") {
		return c, errors.New("展示样式无效")
	}
	clean := func(in []string, max, width int) ([]string, bool) {
		if len(in) > max {
			return nil, false
		}
		out := make([]string, 0, len(in))
		seen := map[string]bool{}
		for _, v := range in {
			v = trimShowcaseRunes(v, width)
			if v == "" || seen[v] {
				return nil, false
			}
			seen[v] = true
			out = append(out, v)
		}
		return out, true
	}
	var ok bool
	if c.Interests, ok = clean(c.Interests, 12, 60); !ok {
		return c, errors.New("兴趣内容无效")
	}
	if c.SelectedWorkIDs, ok = clean(c.SelectedWorkIDs, 36, 32); !ok {
		return c, errors.New("作品选择无效")
	}
	if len(c.SectionOrder) != 3 {
		return c, errors.New("板块顺序无效")
	}
	seen := map[string]bool{}
	for _, v := range c.SectionOrder {
		if !oneOf(v, "writing", "reading", "project") || seen[v] {
			return c, errors.New("板块顺序无效")
		}
		seen[v] = true
	}
	return c, nil
}

func decodeShowcase(raw []byte, fallback string) showcaseConfig {
	c := defaultShowcase(fallback)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}
	c.Interests = showcaseNonNil(c.Interests)
	c.SectionOrder = showcaseNonNil(c.SectionOrder)
	c.SelectedWorkIDs = showcaseNonNil(c.SelectedWorkIDs)
	return c
}
func showcaseNonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func (a *API) showcaseWorks(r *http.Request, userID uuid.UUID) ([]showcaseWork, error) {
	rows, err := a.d.Queries.ListShowcaseWorks(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]showcaseWork, 0, len(rows))
	uid := userID.String()
	for _, w := range rows {
		x := showcaseWork{ID: pbl.SiteItemID(uid, w.AtomID.String()), Kind: w.Kind, Title: w.Title, Summary: trimShowcaseRunes(w.Summary, 240)}
		if w.Kind != "project" {
			x.PublicPath = publicWorkPath(w.ShareToken)
		}
		out = append(out, x)
	}
	return out, nil
}

func (a *API) showcaseState(r *http.Request, u User, row sqlc.PblShowcase) (showcaseState, error) {
	works, err := a.showcaseWorks(r, u.ID)
	if err != nil {
		return showcaseState{}, err
	}
	draft := decodeShowcase(row.Draft, u.DisplayName)
	active := row.PublishedAt.Valid
	state := showcaseState{Draft: draft, Revision: row.Revision, Published: active, AvailableWorks: works}
	if active {
		var site sqlc.PblSite
		site, err = a.d.Queries.GetPblSite(r.Context(), u.ID)
		if err == nil && site.ShareToken != nil && *site.ShareToken != "" {
			state.URL = publicSiteURL(r, a.d.CORSOrigins, *site.ShareToken)
		} else {
			state.Published = false
		}
	}
	state.HasUnpublishedChanges = !bytes.Equal(compactJSON(row.Draft), compactJSON(row.PublishedConfig))
	if site, siteErr := a.d.Queries.GetPblSite(r.Context(), u.ID); siteErr == nil {
		state.HasLegacySite = !state.Published && site.ShareToken != nil && *site.ShareToken != ""
		if state.HasLegacySite {
			state.URL = publicSiteURL(r, a.d.CORSOrigins, *site.ShareToken)
		}
	} else if !errors.Is(siteErr, pgx.ErrNoRows) {
		return showcaseState{}, siteErr
	}
	return state, nil
}

func compactJSON(v []byte) []byte {
	var b bytes.Buffer
	if json.Compact(&b, v) != nil {
		return v
	}
	return b.Bytes()
}

func (a *API) getPblShowcase(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	row, err := a.d.Queries.EnsurePblShowcase(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state, err := a.showcaseState(r, u, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, state)
}

func (a *API) putPblShowcase(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var in struct {
		Draft            showcaseConfig `json:"draft"`
		ExpectedRevision int32          `json:"expectedRevision"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&in); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_json", "请求内容无效", nil))
		return
	}
	c, err := normalizeShowcase(in.Draft)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_showcase", err.Error(), nil))
		return
	}
	works, err := a.showcaseWorks(r, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	allowed := map[string]bool{}
	for _, x := range works {
		allowed[x.ID] = true
	}
	for _, id := range c.SelectedWorkIDs {
		if !allowed[id] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_work", "选择的作品不存在", nil))
			return
		}
	}
	blob, _ := json.Marshal(c)
	_, _ = a.d.Queries.EnsurePblShowcase(r.Context(), u.ID)
	row, err := a.d.Queries.SavePblShowcase(r.Context(), sqlc.SavePblShowcaseParams{UserID: u.ID, Revision: in.ExpectedRevision, Draft: blob})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("主页已在其他位置修改，请刷新后重试"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state, err := a.showcaseState(r, u, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, state)
}

func (a *API) publishPblShowcase(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var in struct {
		ExpectedRevision int32 `json:"expectedRevision"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&in); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_json", "请求内容无效", nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.d.Queries.WithTx(tx)
	row, err := q.GetPblShowcase(r.Context(), u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存主页"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.Revision != in.ExpectedRevision {
		httpx.WriteError(w, r, httpx.ErrConflict("主页已在其他位置修改，请刷新后重试"))
		return
	}
	c, err := normalizeShowcase(decodeShowcase(row.Draft, u.DisplayName))
	if err != nil || c.Name == "" || (c.Bio == "" && c.Tagline == "") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("showcase_incomplete", "请填写姓名，并填写简介或标语", nil))
		return
	}
	works, err := q.ListShowcaseWorks(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	allowed := map[string]bool{}
	for _, x := range works {
		allowed[pbl.SiteItemID(u.ID.String(), x.AtomID.String())] = true
	}
	for _, id := range c.SelectedWorkIDs {
		if !allowed[id] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_work", "选择的作品不存在", nil))
			return
		}
	}
	if _, err = q.PublishPblShowcase(r.Context(), sqlc.PublishPblShowcaseParams{UserID: u.ID, Revision: in.ExpectedRevision}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	site, err := q.GetPblSite(r.Context(), u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		site, err = q.EnsurePblSite(r.Context(), sqlc.EnsurePblSiteParams{UserID: u.ID, AtomID: pgtype.UUID{}})
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	token := site.ShareToken
	if token == nil || *token == "" {
		fresh, e := newShareToken()
		if e != nil {
			httpx.WriteError(w, r, e)
			return
		}
		token = &fresh
	}
	if _, err = q.SetPblSiteShare(r.Context(), sqlc.SetPblSiteShareParams{UserID: u.ID, ShareToken: token, PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.getPblShowcase(w, r)
}

func (a *API) unpublishPblShowcase(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := a.d.Queries.WithTx(tx)
	if _, err = q.UnpublishPblShowcase(r.Context(), u.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err = q.SetPblSiteShare(r.Context(), sqlc.SetPblSiteShareParams{UserID: u.ID, ShareToken: nil, PublishedAt: pgtype.Timestamptz{}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.getPblShowcase(w, r)
}

func (a *API) publicShowcase(r *http.Request, userID uuid.UUID) (*showcaseConfig, []showcaseWork, error) {
	row, err := a.d.Queries.GetPblShowcase(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) || !row.PublishedAt.Valid {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	c := decodeShowcase(row.PublishedConfig, "")
	all, err := a.showcaseWorks(r, userID)
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]showcaseWork{}
	for _, x := range all {
		if x.Kind == "project" || x.PublicPath != "" {
			byID[x.ID] = x
		}
	}
	selected := make([]showcaseWork, 0, len(c.SelectedWorkIDs))
	for _, id := range c.SelectedWorkIDs {
		if x, ok := byID[id]; ok {
			selected = append(selected, x)
		}
	}
	return &c, selected, nil
}
