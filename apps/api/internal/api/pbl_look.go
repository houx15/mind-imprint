package api

// pbl_look.go — 主页项目第三关：给网站定调子。
//
// 产品负责人 2026-09-03：「what is color palette and your choice? style? hero
// image, do you need? can generate it here - your website is becoming real.」
//
// 三件事，三个端点，因为它们的代价差得很远：配色是一次几秒的 compose 调用，
// 头图是一次 69 秒的生成，而「定下来」是一次纯写库。混成一个端点，她改一次配色
// 就要重画一次头图。

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// generatePblPalettes —— 从她第一关留下的关键词派生三组配色。
//
// 🚨 输入是**她的关键词**，不是一排预设色卡。「挑一个你喜欢的颜色」是一道和这个
// 项目无关的题：她凭直觉点一个，页面就多了一个她说不出理由的决定。这一关问的是
// 「哪一组颜色配得上你说的那个读者」。
func (a *API) generatePblPalettes(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	row, err := a.ensureSite(r, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !row.AtomID.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_project", "生成失败：主页项目还没开始", nil))
		return
	}
	atomID := uuid.UUID(row.AtomID.Bytes)

	keywords, feeling, err := a.chosenPersonaKeywords(r, atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(keywords) == 0 {
		// 说清楚差的是哪一步，而不是给一组随便的颜色。
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_keywords",
			"生成失败：请先在「受众画像」里定下读者和关键词，配色是从那些词派生的", nil))
		return
	}

	resolved, rerr := a.routeE(r.Context(), gateway.ClassCompose)
	if rerr != nil {
		httpx.WriteError(w, r, rerr)
		return
	}
	out, usage, gerr := pbl.GeneratePalettes(r.Context(), a.d.Provider, resolved, keywords, feeling)
	a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_palettes", resolved, usage)
	if gerr != nil {
		slog.Warn("pbl look: palette generation failed", "err", gerr,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrBadRequest("generate_failed", "生成失败："+gerr.Error(), nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// drawPblSiteHero —— 画一张头图。
//
// 头图是可选的：三个版式里有两个本来就没有放头图的位置，而「不要头图」是一个
// 合法的选择，不是一件没做完的事。
func (a *API) drawPblSiteHero(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	row, err := a.ensureSite(r, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !row.AtomID.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_project", "生成失败：主页项目还没开始", nil))
		return
	}
	atomID := uuid.UUID(row.AtomID.Bytes)

	keywords, feeling, err := a.chosenPersonaKeywords(r, atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(keywords) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_keywords",
			"生成失败：请先在「受众画像」里定下关键词，头图是照着那些词画的", nil))
		return
	}

	key, derr := a.drawAndStore(r.Context(), u.ID, atomID, "hero", pbl.HeroPrompt(keywords, feeling))
	if derr != nil {
		slog.Warn("pbl look: hero failed", "err", derr,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrBadRequest("draw_failed", "生成失败："+derr.Error(), nil))
		return
	}
	updated, err := a.d.Queries.SetPblSiteHero(r.Context(),
		sqlc.SetPblSiteHeroParams{UserID: u.ID, HeroKey: key})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.siteDTO(r, u, updated)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// clearPblSiteHero —— 不要头图了。
func (a *API) clearPblSiteHero(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	if _, err := a.ensureSite(r, u.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.SetPblSiteHero(r.Context(),
		sqlc.SetPblSiteHeroParams{UserID: u.ID, HeroKey: ""})
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

// chosenPersonaKeywords 读她第一关留下的读者：关键词，和那一页该给他的感觉。
func (a *API) chosenPersonaKeywords(r *http.Request, atomID uuid.UUID) ([]string, string, error) {
	rows, err := a.d.Queries.ListPblPersonas(r.Context(), atomID)
	if err != nil {
		return nil, "", err
	}
	for _, p := range rows {
		if !p.Chosen {
			continue
		}
		var kws []string
		if len(p.Keywords) > 0 {
			_ = json.Unmarshal(p.Keywords, &kws)
		}
		kept := make([]string, 0, len(kws))
		for _, k := range kws {
			if t := strings.TrimSpace(k); t != "" {
				kept = append(kept, t)
			}
		}
		return kept, p.Feeling, nil
	}
	return nil, "", nil
}
