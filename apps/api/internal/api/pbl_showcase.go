package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

type showcaseConfig struct {
	Name              string                `json:"name"`
	Bio               string                `json:"bio"`
	Tagline           string                `json:"tagline"`
	Interests         []string              `json:"interests"`
	Layout            string                `json:"layout"`
	Palette           string                `json:"palette"`
	Font              string                `json:"font"`
	Style             string                `json:"style,omitempty"`
	Illustration      string                `json:"illustration,omitempty"`
	HeroTitle         string                `json:"heroTitle,omitempty"`
	AboutLayout       string                `json:"aboutLayout,omitempty"`
	PortfolioLayout   string                `json:"portfolioLayout,omitempty"`
	AvatarKey         string                `json:"avatarKey,omitempty"`
	HeroImageKey      string                `json:"heroImageKey,omitempty"`
	HeroImagePrompt   string                `json:"heroImagePrompt,omitempty"`
	AvatarImagePrompt string                `json:"avatarImagePrompt,omitempty"`
	WritingStyle      string                `json:"writingStyle"`
	ReadingStyle      string                `json:"readingStyle"`
	SectionOrder      []string              `json:"sectionOrder"`
	SelectedWorkIDs   []string              `json:"selectedWorkIds"`
	InterestTreeMode  string                `json:"interestTreeMode,omitempty"`
	AboutConversation []showcaseChatMessage `json:"aboutConversation,omitempty"`
}

type showcaseChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type showcaseInterestBranch struct {
	Label    string   `json:"label"`
	Keywords []string `json:"keywords"`
}

type showcaseInterestTree struct {
	Mode     string                   `json:"mode,omitempty"`
	Title    string                   `json:"title,omitempty"`
	Keywords []string                 `json:"keywords"`
	Branches []showcaseInterestBranch `json:"branches,omitempty"`
}

type showcaseWork struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	PublicPath string `json:"publicPath,omitempty"`
	Date       string `json:"date,omitempty"`
}

type showcasePublication struct {
	Config          showcaseConfig        `json:"config"`
	Works           []showcaseWork        `json:"works"`
	InterestTree    *showcaseInterestTree `json:"interestTree,omitempty"`
	HeroSourceKey   string                `json:"heroSourceKey,omitempty"`
	AvatarSourceKey string                `json:"avatarSourceKey,omitempty"`
}

type showcaseState struct {
	Draft                 showcaseConfig       `json:"draft"`
	Revision              int32                `json:"revision"`
	Published             bool                 `json:"published"`
	HasUnpublishedChanges bool                 `json:"hasUnpublishedChanges"`
	URL                   string               `json:"url"`
	AvailableWorks        []showcaseWork       `json:"availableWorks"`
	HasLegacySite         bool                 `json:"hasLegacySite"`
	HeroImageURL          string               `json:"heroImageUrl"`
	AvatarURL             string               `json:"avatarUrl"`
	AvailableInterestTree showcaseInterestTree `json:"availableInterestTree"`
}

func defaultShowcase(name string) showcaseConfig {
	return showcaseConfig{Name: strings.TrimSpace(name), Interests: []string{}, Layout: "folio", Palette: "paper", Font: "sans", Style: "classic", Illustration: "none", AboutLayout: "classic", PortfolioLayout: "sections", WritingStyle: "cards", ReadingStyle: "shelf", SectionOrder: []string{"writing", "reading", "project"}, SelectedWorkIDs: []string{}, InterestTreeMode: "none", AboutConversation: []showcaseChatMessage{}}
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
	if c.Style == "" {
		c.Style = "classic"
	}
	if c.Illustration == "" {
		c.Illustration = "none"
	}
	if c.AboutLayout == "" {
		c.AboutLayout = "classic"
	}
	if c.PortfolioLayout == "" {
		c.PortfolioLayout = "sections"
	}
	if c.InterestTreeMode == "" {
		c.InterestTreeMode = "none"
	}
	c.Name, c.Bio, c.Tagline = strings.TrimSpace(c.Name), strings.TrimSpace(c.Bio), strings.TrimSpace(c.Tagline)
	c.HeroTitle = strings.TrimSpace(c.HeroTitle)
	if len([]rune(c.Name)) > 80 || len([]rune(c.Tagline)) > 200 || len([]rune(c.Bio)) > 2000 || len([]rune(c.HeroTitle)) > 200 || len([]rune(c.HeroImagePrompt)) > 2000 || len([]rune(c.AvatarImagePrompt)) > 2000 {
		return c, errors.New("主页文字超过长度限制")
	}
	if !oneOf(c.Layout, "folio", "journal", "studio") || !oneOf(c.Palette, "paper", "forest", "ocean", "rose", "night", "sunshine") || !oneOf(c.Font, "sans", "serif", "mono", "rounded", "handwritten", "display") || !oneOf(c.Style, "classic", "cute", "dark", "anime", "mecha", "minimal") || !oneOf(c.Illustration, "none", "clouds", "moon", "sky", "robot") || !oneOf(c.AboutLayout, "classic", "orbit") || !oneOf(c.PortfolioLayout, "sections", "timeline", "planets", "cloud", "calendar", "list") || !oneOf(c.WritingStyle, "cards", "list") || !oneOf(c.ReadingStyle, "shelf", "list") {
		return c, errors.New("展示样式无效")
	}
	if !oneOf(c.InterestTreeMode, "none", "tree", "keywords") {
		return c, errors.New("兴趣树展示方式无效")
	}
	if len(c.AboutConversation) > 20 {
		return c, errors.New("个人介绍对话超过长度限制")
	}
	totalConversationRunes := 0
	for i := range c.AboutConversation {
		m := &c.AboutConversation[i]
		m.Content = strings.TrimSpace(m.Content)
		totalConversationRunes += len([]rune(m.Content))
		if !oneOf(m.Role, "user", "assistant") || m.Content == "" || len([]rune(m.Content)) > 2000 || totalConversationRunes > 20000 {
			return c, errors.New("个人介绍对话无效")
		}
	}
	clean := func(in []string, max, width int) ([]string, bool) {
		if len(in) > max {
			return nil, false
		}
		out := make([]string, 0, len(in))
		seen := map[string]bool{}
		for _, v := range in {
			v = strings.TrimSpace(v)
			if v == "" || len([]rune(v)) > width || seen[v] {
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
	if c.Style == "" {
		c.Style = "classic"
	}
	if c.Illustration == "" {
		c.Illustration = "none"
	}
	if c.AboutLayout == "" {
		c.AboutLayout = "classic"
	}
	if c.PortfolioLayout == "" {
		c.PortfolioLayout = "sections"
	}
	if c.InterestTreeMode == "" {
		c.InterestTreeMode = "none"
	}
	c.Interests = showcaseNonNil(c.Interests)
	c.SectionOrder = showcaseNonNil(c.SectionOrder)
	c.SelectedWorkIDs = showcaseNonNil(c.SelectedWorkIDs)
	if c.AboutConversation == nil {
		c.AboutConversation = []showcaseChatMessage{}
	}
	return c
}
func showcaseNonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func redactShowcaseForPublic(c showcaseConfig) showcaseConfig {
	c.HeroImagePrompt = ""
	c.AvatarImagePrompt = ""
	c.AboutConversation = nil
	return c
}

func ownShowcaseImage(owner uuid.UUID, key string) bool {
	if key == "" {
		return true
	}
	prefix := "users/" + owner.String() + "/"
	return path.Clean(key) == key && strings.HasPrefix(key, prefix) &&
		(strings.HasPrefix(key, prefix+"images/") || strings.HasPrefix(key, prefix+"generated/showcase-")) &&
		!strings.ContainsAny(key, "?#\\")
}

func (a *API) showcaseWorks(r *http.Request, userID uuid.UUID) ([]showcaseWork, error) {
	rows, err := a.d.Queries.ListShowcaseWorks(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]showcaseWork, 0, len(rows))
	uid := userID.String()
	for _, w := range rows {
		x := showcaseWork{ID: pbl.SiteItemID(uid, w.AtomID.String()), Kind: w.Kind, Title: w.Title, Summary: trimShowcaseRunes(w.Summary, 240), Date: w.Date}
		if w.Kind != "project" {
			x.PublicPath = publicWorkPath(w.ShareToken)
		}
		out = append(out, x)
	}
	return out, nil
}

func (a *API) showcaseInterestTree(ctx context.Context, userID uuid.UUID) (showcaseInterestTree, error) {
	rows, err := a.d.Queries.ListInterestKeywords(ctx, userID)
	if err != nil {
		return showcaseInterestTree{}, err
	}
	byField := make(map[string][]string)
	all := make([]string, 0, min(len(rows), 36))
	seen := map[string]bool{}
	for _, row := range rows {
		if len(all) >= 36 {
			break
		}
		word := strings.TrimSpace(row.TextZh)
		if word == "" {
			word = strings.TrimSpace(row.TextEn)
		}
		if word == "" || seen[word] {
			continue
		}
		seen[word] = true
		all = append(all, word)
		byField[row.Field] = append(byField[row.Field], word)
	}
	branches := make([]showcaseInterestBranch, 0, len(disciplines.Fields))
	for _, field := range disciplines.Fields {
		words := byField[field]
		if len(words) == 0 {
			continue
		}
		branches = append(branches, showcaseInterestBranch{Label: disciplines.FieldLabels[field], Keywords: words})
	}
	return showcaseInterestTree{Mode: "tree", Title: "兴趣", Keywords: all, Branches: branches}, nil
}

func (a *API) showcaseState(r *http.Request, u User, row sqlc.PblShowcase) (showcaseState, error) {
	works, err := a.showcaseWorks(r, u.ID)
	if err != nil {
		return showcaseState{}, err
	}
	draft := decodeShowcase(row.Draft, u.DisplayName)
	active := row.PublishedAt.Valid
	state := showcaseState{Draft: draft, Revision: row.Revision, Published: active, AvailableWorks: works}
	state.AvailableInterestTree, err = a.showcaseInterestTree(r.Context(), u.ID)
	if err != nil {
		return showcaseState{}, err
	}
	if ownShowcaseImage(u.ID, draft.HeroImageKey) {
		state.HeroImageURL = a.signedShowcaseImageOrEmpty(draft.HeroImageKey)
	}
	if ownShowcaseImage(u.ID, draft.AvatarKey) {
		state.AvatarURL = a.signedShowcaseImageOrEmpty(draft.AvatarKey)
	}
	if active {
		var site sqlc.PblSite
		site, err = a.d.Queries.GetPblSite(r.Context(), u.ID)
		if err == nil && site.ShareToken != nil && *site.ShareToken != "" {
			state.URL = publicSiteURL(r, a.d.CORSOrigins, *site.ShareToken)
		} else {
			state.Published = false
		}
	}
	var publication showcasePublication
	publishedConfig := decodeShowcase(row.PublishedConfig, "")
	if json.Unmarshal(row.PublishedConfig, &publication) == nil && publication.Config.Layout != "" {
		publishedConfig = publication.Config
		if publication.HeroSourceKey != "" {
			publishedConfig.HeroImageKey = publication.HeroSourceKey
		}
		if publication.AvatarSourceKey != "" {
			publishedConfig.AvatarKey = publication.AvatarSourceKey
		}
		if publishedConfig.Style == "" {
			publishedConfig.Style = "classic"
		}
		if publishedConfig.Illustration == "" {
			publishedConfig.Illustration = "none"
		}
	}
	draftComparable, publishedComparable := draft, publishedConfig
	draftComparable.HeroImagePrompt, draftComparable.AvatarImagePrompt, draftComparable.AboutConversation = "", "", nil
	publishedComparable.HeroImagePrompt, publishedComparable.AvatarImagePrompt, publishedComparable.AboutConversation = "", "", nil
	state.HasUnpublishedChanges = !reflect.DeepEqual(draftComparable, publishedComparable)
	if !state.HasUnpublishedChanges && publication.Config.Layout != "" {
		byID := make(map[string]showcaseWork, len(works))
		for _, work := range works {
			byID[work.ID] = work
		}
		current := make([]showcaseWork, 0, len(draft.SelectedWorkIDs))
		for _, id := range draft.SelectedWorkIDs {
			if work, ok := byID[id]; ok {
				current = append(current, work)
			}
		}
		currentBlob, _ := json.Marshal(current)
		publishedWorksBlob, _ := json.Marshal(publication.Works)
		state.HasUnpublishedChanges = !bytes.Equal(currentBlob, publishedWorksBlob)
	}
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
	if err := json.NewDecoder(io.LimitReader(r.Body, 128<<10)).Decode(&in); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_json", "请求内容无效", nil))
		return
	}
	c, err := normalizeShowcase(in.Draft)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_showcase", err.Error(), nil))
		return
	}
	if !ownShowcaseImage(u.ID, c.AvatarKey) || !ownShowcaseImage(u.ID, c.HeroImageKey) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "图片不属于当前账号", nil))
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
	if err := tx.QueryRow(r.Context(), `SELECT 1 FROM pbl_showcase WHERE user_id=$1 FOR UPDATE`, u.ID).Scan(new(int)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
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
	if !ownShowcaseImage(u.ID, c.AvatarKey) || !ownShowcaseImage(u.ID, c.HeroImageKey) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "图片不属于当前账号", nil))
		return
	}
	works, err := q.ListShowcaseWorks(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	allowed := map[string]showcaseWork{}
	for _, x := range works {
		id := pbl.SiteItemID(u.ID.String(), x.AtomID.String())
		work := showcaseWork{ID: id, Kind: x.Kind, Title: x.Title, Summary: trimShowcaseRunes(x.Summary, 240), Date: x.Date}
		if x.Kind != "project" {
			work.PublicPath = publicWorkPath(x.ShareToken)
		}
		allowed[id] = work
	}
	selected := make([]showcaseWork, 0, len(c.SelectedWorkIDs))
	for _, id := range c.SelectedWorkIDs {
		work, ok := allowed[id]
		if !ok {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_work", "选择的作品不存在", nil))
			return
		}
		selected = append(selected, work)
	}
	var interestTree *showcaseInterestTree
	if c.InterestTreeMode != "none" {
		snapshot, treeErr := a.showcaseInterestTree(r.Context(), u.ID)
		if treeErr != nil {
			httpx.WriteError(w, r, treeErr)
			return
		}
		snapshot.Mode = c.InterestTreeMode
		if c.InterestTreeMode == "keywords" {
			snapshot.Branches = nil
		}
		interestTree = &snapshot
	}
	publicConfig, publicKeys, err := a.publishShowcaseImages(r.Context(), u.ID, c)
	committed := false
	defer func() {
		if !committed && a.d.PublicAssets != nil {
			for _, key := range publicKeys {
				_ = a.d.PublicAssets.DeleteObject(context.Background(), key)
			}
		}
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	publicConfig.HeroImagePrompt, publicConfig.AvatarImagePrompt = "", ""
	publicConfig.AboutConversation = nil
	var previous showcasePublication
	_ = json.Unmarshal(row.PublishedConfig, &previous)
	publicationBlob, _ := json.Marshal(showcasePublication{Config: publicConfig, Works: selected, InterestTree: interestTree, HeroSourceKey: c.HeroImageKey, AvatarSourceKey: c.AvatarKey})
	if _, err = q.PublishPblShowcase(r.Context(), sqlc.PublishPblShowcaseParams{UserID: u.ID, Revision: in.ExpectedRevision, PublishedConfig: publicationBlob}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrConflict("主页已在其他位置修改，请刷新后重试"))
			return
		}
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
	committed = true
	if a.d.PublicAssets != nil {
		for _, key := range []string{previous.Config.HeroImageKey, previous.Config.AvatarKey} {
			if strings.HasPrefix(key, "showcase/users/"+u.ID.String()+"/") {
				_ = a.d.PublicAssets.DeleteObject(context.Background(), key)
			}
		}
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
	row, rowErr := q.GetPblShowcase(r.Context(), u.ID)
	if rowErr != nil && !errors.Is(rowErr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, rowErr)
		return
	}
	var previous showcasePublication
	_ = json.Unmarshal(row.PublishedConfig, &previous)
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
	if a.d.PublicAssets != nil {
		for _, key := range []string{previous.Config.HeroImageKey, previous.Config.AvatarKey} {
			if strings.HasPrefix(key, "showcase/users/"+u.ID.String()+"/") {
				_ = a.d.PublicAssets.DeleteObject(context.Background(), key)
			}
		}
	}
	a.getPblShowcase(w, r)
}

func (a *API) publicShowcase(r *http.Request, userID uuid.UUID) (*showcaseConfig, []showcaseWork, *showcaseInterestTree, error) {
	row, err := a.d.Queries.GetPblShowcase(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if !row.PublishedAt.Valid {
		return nil, nil, nil, nil
	}
	var publication showcasePublication
	if err := json.Unmarshal(row.PublishedConfig, &publication); err != nil || publication.Config.Layout == "" {
		return nil, nil, nil, errors.New("invalid published showcase")
	}
	// Drafting prompts and coaching history are private. Public pages receive only
	// the resulting presentation fields and selected images.
	c := redactShowcaseForPublic(publication.Config)
	if c.Style == "" {
		c.Style = "classic"
	}
	if c.Illustration == "" {
		c.Illustration = "none"
	}
	if c.AboutLayout == "" {
		c.AboutLayout = "classic"
	}
	if c.PortfolioLayout == "" {
		c.PortfolioLayout = "sections"
	}
	all, err := a.showcaseWorks(r, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	byID := map[string]showcaseWork{}
	for _, x := range all {
		if x.Kind == "project" {
			byID[x.ID] = x
		} else if x.PublicPath != "" {
			byID[x.ID+"\x00"+x.PublicPath] = x
		}
	}
	selected := make([]showcaseWork, 0, len(c.SelectedWorkIDs))
	visibleIDs := make([]string, 0, len(c.SelectedWorkIDs))
	for _, snap := range publication.Works {
		key := snap.ID
		if snap.Kind != "project" {
			key += "\x00" + snap.PublicPath
		}
		if _, ok := byID[key]; ok {
			selected = append(selected, snap)
			visibleIDs = append(visibleIDs, snap.ID)
		}
	}
	c.SelectedWorkIDs = visibleIDs
	return &c, selected, publication.InterestTree, nil
}

func (a *API) generatePblShowcaseImage(w http.ResponseWriter, r *http.Request) {
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
	var in struct {
		Prompt  string `json:"prompt"`
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	in.Prompt = strings.TrimSpace(in.Prompt)
	if len([]rune(in.Prompt)) < 3 || len([]rune(in.Prompt)) > 2000 || !oneOf(in.Purpose, "hero", "avatar") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image_request", "图片描述或用途无效", nil))
		return
	}
	prompt, size := showcaseImageRequest(in.Purpose, in.Prompt)
	key, err := a.drawAndStoreSized(r.Context(), u.ID, uuid.Nil, "showcase-"+in.Purpose, prompt, size)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"objectKey": key, "url": a.signedShowcaseImageOrEmpty(key)})
}

func (a *API) resolvePblShowcaseImage(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var in struct {
		ObjectKey string `json:"objectKey"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.ObjectKey == "" || !ownShowcaseImage(u.ID, in.ObjectKey) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "图片不属于当前账号", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"objectKey": in.ObjectKey, "url": a.signedShowcaseImageOrEmpty(in.ObjectKey)})
}

func (a *API) uploadPblShowcaseImage(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (10<<20)+(1<<20))
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "图片超过 10 MB 或上传内容无效", nil))
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "缺少图片文件", nil))
		return
	}
	defer f.Close()
	blob, err := io.ReadAll(io.LimitReader(f, (10<<20)+1))
	if err != nil || len(blob) == 0 || len(blob) > 10<<20 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "图片超过 10 MB 或上传内容无效", nil))
		return
	}
	contentType, ext := http.DetectContentType(blob), ""
	switch contentType {
	case "image/png":
		ext = "png"
	case "image/jpeg":
		ext = "jpg"
	case "image/webp":
		ext = "webp"
	}
	if ext == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "仅支持 PNG、JPEG 或 WebP", nil))
		return
	}
	key, err := generatedImageKey(u.ID, "showcase-upload", ext)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.OSS.PutObject(r.Context(), key, contentType, blob); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"objectKey": key, "url": a.signedShowcaseImageOrEmpty(key)})
}

func (a *API) publishShowcaseImages(ctx context.Context, userID uuid.UUID, c showcaseConfig) (showcaseConfig, []string, error) {
	if c.HeroImageKey == "" && c.AvatarKey == "" {
		return c, nil, nil
	}
	if a.d.OSS == nil || a.d.PublicAssets == nil {
		return c, nil, errors.New("公开图片存储未配置")
	}
	created := []string{}
	copyOne := func(source, purpose string) (string, error) {
		if source == "" {
			return "", nil
		}
		blob, err := a.d.OSS.GetObject(ctx, source)
		if err != nil {
			return "", err
		}
		ct, ext, err := generatedImageFormat(blob)
		if err != nil && http.DetectContentType(blob) == "image/webp" {
			ct, ext, err = "image/webp", "webp", nil
		}
		if err != nil {
			return "", err
		}
		key, err := publicShowcaseImageKey(userID, purpose, ext)
		if err != nil {
			return "", err
		}
		if err := a.d.PublicAssets.PutObject(ctx, key, ct, blob); err != nil {
			return "", err
		}
		created = append(created, key)
		return key, nil
	}
	var err error
	if c.HeroImageKey, err = copyOne(c.HeroImageKey, "hero"); err != nil {
		return c, created, err
	}
	if c.AvatarKey, err = copyOne(c.AvatarKey, "avatar"); err != nil {
		return c, created, err
	}
	return c, created, nil
}

func showcaseImageRequest(purpose, studentPrompt string) (string, string) {
	studentClause := "Student's original request (preserve its subject and intent): " + studentPrompt
	if purpose == "hero" {
		return "Create a wide website hero image with a clear focal area, generous negative space for interface content, and a composition that remains readable when center-cropped. Do not include words, letters, logos, captions, watermarks, or embedded typography. " + studentClause, "1664*928"
	}
	return "Create a square profile illustration with one clear centered subject, a simple background, and safe margins for circular cropping. Do not include words, letters, logos, captions, watermarks, or embedded typography. " + studentClause, "1024*1024"
}

// showcaseWasPublished keeps the retired homepage publishers from silently
// replacing (or appearing to replace) the standalone showcase. A revoked
// showcase retains its snapshot, so reopening the old publisher cannot expose
// old material by accident.
func (a *API) showcaseWasPublished(r *http.Request, userID uuid.UUID) (bool, error) {
	row, err := a.d.Queries.GetPblShowcase(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(row.PublishedConfig) > 0 && string(row.PublishedConfig) != "null", nil
}

func (a *API) rejectLegacyShowcasePublish(w http.ResponseWriter, r *http.Request, userID uuid.UUID) bool {
	published, err := a.showcaseWasPublished(r, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return true
	}
	if published {
		httpx.WriteError(w, r, httpx.ErrConflict("个人主页已使用新版展示页，请在“我的主页”中发布或停止发布"))
		return true
	}
	return false
}
