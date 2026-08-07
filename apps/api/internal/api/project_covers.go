package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"mindimprint/api/internal/httpx"
)

// project_covers.json maps a picker index ("1".."15") to the OSS object key of
// a pre-uploaded project cover image (web/<uuid>.webp). Non-secret; resolved to
// a short-lived signed URL at request time (mirrors internal/cards/covers.go).
//
//go:embed project_covers.json
var projectCoversRaw []byte

var projectCoverManifest = func() map[string]string {
	m := map[string]string{}
	if err := json.Unmarshal(projectCoversRaw, &m); err != nil {
		panic("api: bad project_covers.json: " + err.Error())
	}
	return m
}()

// projectCoverCount is the number of pre-uploaded cover images (indices 1..N).
var projectCoverCount = len(projectCoverManifest)

// projectMacarons is the closed set of gradient cover names a project may use
// instead of a photo cover (mirrors apps/web/src/ui/tokens.ts MACARONS).
var projectMacarons = map[string]bool{
	"peach": true, "butter": true, "matcha": true, "lake": true,
	"mist": true, "taro": true, "berry": true,
}

// projectCoverKey looks up the OSS object key for a picker index (1..N).
func projectCoverKey(idx int) (string, bool) {
	key, ok := projectCoverManifest[strconv.Itoa(idx)]
	return key, ok
}

// randomProjectCover picks a uniformly random photo cover — the default when
// a project is created without an explicit choice.
func randomProjectCover() string {
	return fmt.Sprintf("img:%d", rand.Intn(projectCoverCount)+1)
}

// validProjectCover reports whether cover is a well-formed cover value:
// "img:<1..N>" (a picker index that resolves in the manifest) or
// "grad:<macaron>" (one of the seven macaron gradient names).
func validProjectCover(cover string) bool {
	switch {
	case strings.HasPrefix(cover, "img:"):
		idx, err := strconv.Atoi(strings.TrimPrefix(cover, "img:"))
		if err != nil {
			return false
		}
		_, ok := projectCoverKey(idx)
		return ok
	case strings.HasPrefix(cover, "grad:"):
		return projectMacarons[strings.TrimPrefix(cover, "grad:")]
	default:
		return false
	}
}

// resolveCoverURL signs a project's cover value into a short-lived GET URL.
// Only "img:" covers resolve to a URL (gradients are rendered client-side from
// the name); an unresolvable index or a disabled OSS service returns "".
func (a *API) resolveCoverURL(cover string) string {
	if !strings.HasPrefix(cover, "img:") || a.d.OSS == nil {
		return ""
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(cover, "img:"))
	if err != nil {
		return ""
	}
	key, ok := projectCoverKey(idx)
	if !ok {
		return ""
	}
	url, err := a.d.OSS.SignDownload(key, ossDownloadTTL)
	if err != nil {
		return ""
	}
	return url
}

// projectCoverDTO is one entry in the GET /project-covers picker list.
type projectCoverDTO struct {
	Key string `json:"key"`
	URL string `json:"url,omitempty"`
}

// getProjectCovers lists every pre-uploaded photo cover (img:1..N) with a
// signed URL, for the project-creation/settings cover picker. Pure read, no
// model call.
func (a *API) getProjectCovers(w http.ResponseWriter, r *http.Request) {
	if _, ok := UserFromContext(r.Context()); !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	out := make([]projectCoverDTO, 0, projectCoverCount)
	for i := 1; i <= projectCoverCount; i++ {
		key := fmt.Sprintf("img:%d", i)
		out = append(out, projectCoverDTO{Key: key, URL: a.resolveCoverURL(key)})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"covers": out})
}
