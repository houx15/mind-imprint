package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestPblRefeedTracksExactPublishedVersion(t *testing.T) {
	pool := NewTestDB(t)
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{AtomID: atom.ID, Kind: "website", Name: "fixture", Idea: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(t.Context(), "INSERT INTO pbl_site(user_id,atom_id) VALUES($1,$2)", SeedUserID, atom.ID); err != nil {
		t.Fatal(err)
	}
	brief, _ := json.Marshal(map[string]any{"hero": map[string]string{"mode": "mixed"}, "heroImage": map[string]string{"key": "private-asset-key", "prompt": "private-image-provider-prompt"}, "pageContent": map[string]any{"about": []string{"snapshot body"}}})
	version, err := q.CreatePblCodeVersion(t.Context(), sqlc.CreatePblCodeVersionParams{AtomID: atom.ID, BriefRevision: 1, Brief: brief, Html: "<html><body>snapshot body</body></html>"})
	if err != nil {
		t.Fatal(err)
	}
	a := &API{d: Deps{Pool: pool, Queries: q}}
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(WithUser(req.Context(), User{ID: SeedUserID, DisplayName: "fixture"}))
	gather := func() string { return strings.Join(a.gatherPblToolWork(req, atom.ID, nil, nil), "\n") }
	initial := gather()
	for _, want := range []string{version.ID.String(), `"hasImage":true`, `"mode":"mixed"`, `"includesSavedContent":true`, "未公开或已撤回"} {
		if !strings.Contains(initial, want) {
			t.Fatalf("missing %s in %s", want, initial)
		}
	}
	for _, secret := range []string{"private-asset-key", "private-image-provider-prompt", "主页没有头图"} {
		if strings.Contains(initial, secret) {
			t.Fatal("incorrect generation context", secret)
		}
	}
	if err = q.SetPblSitePublication(t.Context(), sqlc.SetPblSitePublicationParams{UserID: SeedUserID, AtomID: atom.ID, VersionID: version.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(t.Context(), "UPDATE pbl_site SET share_token='fixture-only-token' WHERE user_id=$1", SeedUserID); err != nil {
		t.Fatal(err)
	}
	published := gather()
	if !strings.Contains(published, "主页当前已公开的生成版本："+version.ID.String()) || strings.Contains(published, "主页当前未公开或已撤回") {
		t.Fatal("lost publication state", published)
	}
	if _, err = pool.Exec(t.Context(), "UPDATE pbl_site SET share_token=NULL WHERE user_id=$1", SeedUserID); err != nil {
		t.Fatal(err)
	}
	revoked := gather()
	if !strings.Contains(revoked, "未公开或已撤回") || strings.Contains(revoked, "主页当前已公开的生成版本") {
		t.Fatal("revoked publication reported public", revoked)
	}
}
