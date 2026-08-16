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

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// courseShipReq is the ship envelope: the stock catalog cover id to attach on
// publish (e.g. "img:3"). Empty is allowed (no cover set).
type courseShipReq struct {
	Cover string `json:"cover"`
}

func (a *API) postCourseShip(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	var body courseShipReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
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

	if err := store.SetCourseStatusAndCover(r.Context(), slug, "published", body.Cover); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"slug": slug, "status": "published", "narrationsGenerated": generated,
	})
}
