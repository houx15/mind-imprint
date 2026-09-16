package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

// LiteWorkspaceBrokenSurfaceForTest names a surface that exists only in the
// test binary. It resolves ownership like the assignment surface and then
// fails to build, so a test can check that a build failure is written before
// any model call is made.
const LiteWorkspaceBrokenSurfaceForTest = "brokenSurfaceForTest"

// Registered from init, before any test runs: liteWorkspaceSurfaces is read
// without a lock, so it must never change while a request is in flight.
func init() {
	liteWorkspaceSurfaces[LiteWorkspaceBrokenSurfaceForTest] = liteWorkspaceSurfaceSpec{
		resolve: liteWorkspaceSubjectFromClass,
		build: func(*API, liteWorkspaceSurfaceInput) (liteWorkspaceSurface, error) {
			return nil, &httpx.APIError{
				Status: http.StatusConflict, Code: "surface_unavailable", Message: "加载失败：测试用的画布",
			}
		},
	}
}
