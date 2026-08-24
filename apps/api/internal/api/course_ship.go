package api

// course_ship.go — Task 5 of the course authoring & publish lifecycle: ship a
// preview course. POST /api/v1/admin/courses/{slug}/ship (OSS_ADMIN_KEY
// bearer): generate narration TTS from the stored 2.0 definition, set the
// cover, and flip status preview -> published — the ONE publish transition,
// done in a single SetCourseStatusAndCover call so a course is never left
// published-with-no-cover mid-request.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// courseShipReq is the ship envelope. Cover is a stock catalog cover id (e.g.
// "img:3"). CoverAssetPath is the alternative for a generated course: a
// course-relative WebP the toolkit uploaded (cover/course-cover.webp), bound as
// the visible cover. The two are mutually exclusive; both empty preserves the
// existing cover (idempotent re-ship).
type courseShipReq struct {
	Cover          string `json:"cover"`
	CoverAssetPath string `json:"coverAssetPath"`
}

func (a *API) postCourseShip(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	// Defense-in-depth: the slug is used to derive OSS object keys
	// (courseAssetKey) on the cover path below — never let a traversal-shaped
	// slug through, mirroring course_asset_urls.go's write-path guard. (net/http
	// already cleans `..` before routing; this makes the invariant explicit.)
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slug", "无效的课程标识。", nil))
		return
	}
	var body courseShipReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Rule 1: a stock cover id and a generated-cover asset path cannot both be
	// supplied — request-level validation, before any DB or OSS work.
	if body.Cover != "" && body.CoverAssetPath != "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("ambiguous_cover", "cover 与 coverAssetPath 不能同时提供。", nil))
		return
	}
	// The "asset:" scheme is minted ONLY by validateCoverAsset, after it has
	// confirmed the object exists + is WebP inside this course's namespace. It
	// must never be accepted as raw input via the stock `cover` field, or a
	// caller could publish an unverified/nonexistent asset cover past that gate
	// (rule 4). Asset covers must come through coverAssetPath.
	if strings.HasPrefix(body.Cover, courseCoverAssetPrefix) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_cover", "生成封面请使用 coverAssetPath 字段。", nil))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	def, _, err := store.GetCourseDefinition(r.Context(), slug) // (definition, status, err); 404 if unknown slug
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if len(def) == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("该课程没有 2.0 定义"))
		return
	}

	// Slice 9 Task 1 — pre-publish asset gate (course_ship_assets.go): border-
	// walk every interactiveHtml block's source, confirm it exists in OSS and
	// is self-contained (no external network dependencies). ANY blocking
	// issue refuses the publish. Guarded the same way the audio step below
	// guards a.d.OSS == nil, so unconfigured-OSS environments (including the
	// existing Part-1 CourseShip tests, which run with no OSS configured)
	// keep shipping exactly as before — this stage adds a new blocking
	// reason, it never changes behavior when OSS isn't wired up.
	if a.d.OSS != nil {
		if issues := validateShipAssets(r.Context(), a.d.OSS, slug, def); len(issues) > 0 {
			httpx.WriteError(w, r, &httpx.APIError{
				Status:  http.StatusUnprocessableEntity,
				Code:    "asset_validation_failed",
				Message: "课程存在未通过资产自洽性校验的交互内容，无法发布。",
				Details: map[string]any{"issues": issues},
			})
			return
		}
	}

	// Resolve the cover to persist. A generated course binds a course-relative
	// WebP it uploaded (coverAssetPath) INSTEAD of a stock img:* cover; the
	// object is verified to exist + be WebP inside this course's namespace
	// BEFORE we publish (rule 4). Done before the expensive TTS below so a bad
	// cover fails fast. Both fields empty leaves coverToStore == "" →
	// SetCourseStatusAndCover preserves the existing cover (rule 7).
	coverToStore := body.Cover
	if body.CoverAssetPath != "" {
		if a.d.OSS == nil {
			httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
			return
		}
		stored, apiErr := validateCoverAsset(r.Context(), a.d.OSS, slug, body.CoverAssetPath)
		if apiErr != nil {
			httpx.WriteError(w, r, apiErr)
			return
		}
		coverToStore = stored
	}

	// Nil-guard voice/OSS the same way postAdminUploadCourse (course_admin.go)
	// does: a nil *oss.Service assigned straight into the agent.CourseAudioStore
	// interface parameter would NOT compare equal to nil inside
	// GenerateDefinitionAudio (typed-nil-in-interface), so both are explicitly
	// guarded here rather than passed through raw.
	var synth agent.CourseAudioSynth
	if a.d.Voice != nil {
		synth = a.d.Voice
	}
	var audioStore agent.CourseAudioStore
	if a.d.OSS != nil {
		audioStore = a.d.OSS
	}
	generated, err := agent.GenerateDefinitionAudio(r.Context(), synth, audioStore, slug, def)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if err := store.SetCourseStatusAndCover(r.Context(), slug, "published", coverToStore); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"slug": slug, "status": "published", "narrationsGenerated": generated,
	})
}
