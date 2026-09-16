package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
	"strings"
)

func (a *API) listPblCodeVersions(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblCodeVersions(r.Context(), atom)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var direction pbl.CreativeDirection
	saved, draftErr := a.d.Queries.GetPblCreativeDirection(r.Context(), atom)
	if draftErr == nil {
		if err := json.Unmarshal(saved.Document, &direction); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	} else if !errors.Is(draftErr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, draftErr)
		return
	}
	result := make([]map[string]any, 0, len(rows))
	for _, version := range rows {
		var brief struct {
			Hero *pbl.HeroBrief `json:"hero"`
		}
		if err := json.Unmarshal(version.Brief, &brief); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		missing := []string{}
		if _, reason := publicationPage(version.Brief); reason != "" {
			missing = append(missing, reason)
		}
		if direction.Trial == nil || direction.Trial.VersionID != version.ID.String() || strings.TrimSpace(direction.Trial.Observation) == "" {
			missing = append(missing, "请先试用并保留这一版")
		}
		item := map[string]any{"id": version.ID, "brief_revision": version.BriefRevision, "created_at": version.CreatedAt, "parent_version_id": version.ParentVersionID, "feedback": version.Feedback, "publicationMissing": missing}
		if brief.Hero != nil {
			item["mode"] = brief.Hero.Mode
		}
		item["comparison"] = comparisonSummary(version.Brief)
		result = append(result, item)
	}
	httpx.WriteJSON(w, 200, result)
}
func (a *API) generatePblHeroCode(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	account, _ := UserFromContext(r.Context())
	entitled, entitlementErr := HasEntitlement(r.Context(), account)
	if entitlementErr != nil {
		httpx.WriteError(w, r, entitlementErr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var in struct {
		IncludeContent bool   `json:"includeContent"`
		RedrawImage    bool   `json:"redrawImage"`
		Revision       *int32 `json:"revision"`
		BaseVersion    string `json:"baseVersion"`
		Feedback       string `json:"feedback"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 20000)).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.Revision == nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_revision", "缺少创作构思版本", nil))
		return
	}
	row, err := a.d.Queries.GetPblCreativeDirection(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存创作构思"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.Revision != *in.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("创作构思已修改，请核对最新版本"))
		return
	}
	var doc pbl.CreativeDirection
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if doc, err = pbl.NormalizeCreativeDirection(doc, true); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete_hero", err.Error(), nil))
		return
	}
	var imageAsset *heroImageAsset
	var previous string
	var page *pbl.SiteContent
	var comparison *pbl.ProcessComparison
	var parent pgtype.UUID
	if (in.BaseVersion == "") != (strings.TrimSpace(in.Feedback) == "") || len([]rune(in.Feedback)) > 2000 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision_feedback", "请指定要修改的版本并填写2000字以内的修改意见", nil))
		return
	}
	if in.BaseVersion != "" {
		base, parseErr := uuid.Parse(in.BaseVersion)
		if parseErr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_base_version", "作品版本无效", nil))
			return
		}
		version, loadErr := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom, ID: base})
		if errors.Is(loadErr, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("要修改的作品版本不存在"))
			return
		}
		if loadErr != nil {
			httpx.WriteError(w, r, loadErr)
			return
		}
		var baseBrief struct {
			Hero  *pbl.HeroBrief  `json:"hero"`
			Image *heroImageAsset `json:"heroImage"`
		}
		if err := json.Unmarshal(version.Brief, &baseBrief); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if baseBrief.Hero != nil && baseBrief.Hero.Mode != doc.Hero.Mode {
			httpx.WriteError(w, r, httpx.ErrConflict("呈现方式已改变，请生成新的第一幕后再修改"))
			return
		}
		imageAsset = baseBrief.Image
		if !in.IncludeContent {
			var snapshot struct {
				Page       *pbl.SiteContent       `json:"pageContent"`
				Comparison *pbl.ProcessComparison `json:"processComparison"`
			}
			if err := json.Unmarshal(version.Brief, &snapshot); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			page = snapshot.Page
			comparison = snapshot.Comparison
		}
		previous = version.Html
		parent = pgtype.UUID{Bytes: base, Valid: true}
	}
	var siteSnapshot []byte
	briefSnapshot := row.Document
	if in.IncludeContent {
		if previous == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("missing_base_version", "请先选择一个第一幕版本", nil))
			return
		}
		site, loadErr := a.d.Queries.GetPblSite(r.Context(), account.ID)
		if loadErr != nil {
			httpx.WriteError(w, r, loadErr)
			return
		}
		if !site.AtomID.Valid || uuid.UUID(site.AtomID.Bytes) != atom {
			httpx.WriteError(w, r, httpx.ErrNotFound("主页项目不存在"))
			return
		}
		content, loadErr := a.loadSiteContent(r, account.ID, account.DisplayName, site)
		if loadErr != nil {
			httpx.WriteError(w, r, loadErr)
			return
		}
		if doc.IncludeProcess {
			if doc.Trial == nil {
				httpx.WriteError(w, r, httpx.ErrBadRequest("missing_trial", "请先保留一个试用版本", nil))
				return
			}
			trialID, parseErr := uuid.Parse(doc.Trial.VersionID)
			if parseErr != nil {
				httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_trial", "试用版本无效", nil))
				return
			}
			version, versionErr := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom, ID: trialID})
			if versionErr != nil {
				httpx.WriteError(w, r, versionErr)
				return
			}
			content.Sections = append(content.Sections, pbl.ProcessRecordSection(version.Feedback, doc.Trial.Observation))
			if doc.IncludeComparison {
				if !version.ParentVersionID.Valid {
					httpx.WriteError(w, r, httpx.ErrBadRequest("missing_comparison_parent", "这一版没有修改前版本，请选择一次修改后的试用版本", nil))
					return
				}
				before, err := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom, ID: uuid.UUID(version.ParentVersionID.Bytes)})
				if err != nil {
					httpx.WriteError(w, r, err)
					return
				}
				comparison = &pbl.ProcessComparison{BeforeVersionID: before.ID.String(), AfterVersionID: version.ID.String(), Feedback: version.Feedback, Observation: doc.Trial.Observation}
			}
		}
		hasContent := len(content.About) > 0
		for _, section := range content.Sections {
			if strings.TrimSpace(section.Body) != "" {
				hasContent = true
			}
		}
		if !hasContent {
			httpx.WriteError(w, r, httpx.ErrBadRequest("missing_page_content", "请先保存自我介绍或作品内容", nil))
			return
		}
		page = &content
		siteSnapshot = site.Content
	}
	var html string
	if in.RedrawImage && doc.Hero.Mode == "code" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_redraw", "代码模式没有需要重画的图片", nil))
		return
	}
	if doc.Hero.Mode != "code" {
		if imageAsset != nil && !ownedHeroImage(imageAsset.Key, account.ID) {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "版本图片无效", nil))
			return
		}
		if imageAsset == nil || in.RedrawImage || (doc.Hero.Mode == "image" && previous != "" && !in.IncludeContent) {
			prompt := "为学生个人主页制作第一幕图片。遵循以下学生已确定的构思，不添加虚构作品、成绩或个人经历。只绘制静态画面，互动由网页另行实现。\n风格：" + doc.Feeling + "\n意象：" + strings.Join(doc.Motifs, "、") + "\n画面：" + doc.Hero.Scene + "\n制作说明：" + doc.Hero.Prompt
			if previous != "" {
				prompt += "\n本轮修改意见：" + in.Feedback
			}
			key, drawErr := a.drawAndStore(r.Context(), account.ID, atom, "hero-version", prompt)
			if drawErr != nil {
				httpx.WriteError(w, r, httpx.ErrBadRequest("image_generation_failed", "生成图片失败："+drawErr.Error(), nil))
				return
			}
			imageAsset = &heroImageAsset{Key: key, Prompt: prompt}
		}
	}
	if doc.Hero.Mode == "image" && page == nil {
		html = pbl.ImageHeroHTML(account.DisplayName, doc.Hero.Scene)
	} else {
		resolved, routeErr := a.routeE(r.Context(), gateway.ClassCompose)
		if routeErr != nil {
			httpx.WriteError(w, r, routeErr)
			return
		}
		source := ""
		if imageAsset != nil {
			source = pbl.HeroImageSource
		}
		var usage gateway.ChatUsage
		html, usage, err = pbl.GenerateHeroCode(r.Context(), a.d.Provider, resolved, doc, account.DisplayName, previous, in.Feedback, page, source)
		a.recordLiteLLMCall(r.Context(), account.ID, atom, "pbl_hero_code", resolved, usage)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("code_generation_failed", "生成失败："+err.Error(), nil))
			return
		}
	}
	var snapshot map[string]any
	if err := json.Unmarshal(row.Document, &snapshot); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if page != nil {
		snapshot["pageContent"] = page
	}
	if comparison != nil {
		snapshot["processComparison"] = comparison
	}
	if imageAsset != nil {
		snapshot["heroImage"] = imageAsset
	}
	briefSnapshot, _ = json.Marshal(snapshot)
	// Serialize with draft updates. A completed generation cannot silently represent a newer brief.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var revision int32
	if err = tx.QueryRow(r.Context(), "SELECT revision FROM pbl_creative_direction WHERE atom_id=$1 FOR UPDATE", atom).Scan(&revision); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if revision != row.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("生成期间构思已修改，旧版本不会覆盖当前构思，请重新生成"))
		return
	}
	if in.IncludeContent {
		var current []byte
		if err = tx.QueryRow(r.Context(), "SELECT content FROM pbl_site WHERE user_id=$1 FOR UPDATE", account.ID).Scan(&current); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !bytes.Equal(current, siteSnapshot) {
			httpx.WriteError(w, r, httpx.ErrConflict("生成期间主页内容已修改，请基于最新内容重新生成"))
			return
		}
	}
	created, err := a.d.Queries.WithTx(tx).CreatePblCodeVersion(r.Context(), sqlc.CreatePblCodeVersionParams{AtomID: atom, BriefRevision: row.Revision, Brief: briefSnapshot, Html: html, ParentVersionID: parent, Feedback: in.Feedback})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 201, map[string]any{"id": created.ID, "brief_revision": created.BriefRevision, "created_at": created.CreatedAt, "parent_version_id": created.ParentVersionID, "feedback": created.Feedback})
}
func (a *API) previewPblCodeVersion(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(r.PathValue("version"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("版本不存在"))
		return
	}
	row, err := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrNotFound("版本不存在"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	account, _ := UserFromContext(r.Context())
	row, err = a.comparisonVersion(r, row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.writePblCodeVersion(w, r, row, account.ID)
}

func (a *API) writePblCodeVersion(w http.ResponseWriter, r *http.Request, row sqlc.PblCodeVersion, owner uuid.UUID) {
	var snapshot struct {
		Image *heroImageAsset `json:"heroImage"`
	}
	if err := json.Unmarshal(row.Brief, &snapshot); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if snapshot.Image != nil {
		if strings.Count(row.Html, pbl.HeroImageSource) > 3 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "版本图片引用过多", nil))
			return
		}
		if !ownedHeroImage(snapshot.Image.Key, owner) || a.d.OSS == nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("版本图片不可用"))
			return
		}
		blob, loadErr := a.d.OSS.GetObjectLimited(r.Context(), snapshot.Image.Key, maxGeneratedImageBytes)
		if loadErr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("image_preview_failed", "读取版本图片失败", nil))
			return
		}
		mime, _, imageErr := generatedImageFormat(blob)
		if imageErr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("image_preview_failed", "版本图片格式无效", nil))
			return
		}
		row.Html = strings.ReplaceAll(row.Html, pbl.HeroImageSource, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(blob))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", pbl.CodePreviewCSP)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
	_, _ = w.Write([]byte(row.Html))
}

// Asset provenance is read from an owned version, never from client-supplied keys.
type heroImageAsset struct {
	Key    string `json:"key"`
	Prompt string `json:"prompt"`
}

func ownedHeroImage(key string, user uuid.UUID) bool {
	prefix := fmt.Sprintf("users/%s/generated/hero-version-", user)
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(key, prefix)
	return !strings.ContainsAny(suffix, "/\\") && (strings.HasSuffix(suffix, ".png") || strings.HasSuffix(suffix, ".jpg"))
}
