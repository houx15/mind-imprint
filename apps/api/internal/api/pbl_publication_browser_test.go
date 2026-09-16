package api

// Opt-in browser harness. All records live in the isolated test database, all
// assets are local fixtures, and the server binds to loopback. No model or real
// object storage is used. The finish control closes the harness and its DB.
import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/oss"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

func TestBrowserPublicImagePublication(t *testing.T) {
	if os.Getenv("PBL_PUBLICATION_BROWSER_WALK") != "1" {
		t.Skip("opt-in loopback browser walkthrough")
	}
	pool := NewTestDB(t)
	q := sqlc.New(pool)
	atom, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{AtomID: atom.ID, Kind: "website", Idea: "browser fixture", Name: "browser fixture"}); err != nil {
		t.Fatal(err)
	}
	var pixels bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 320, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			v := color.RGBA{60, 130, 100, 255}
			if (x/40+y/30)%2 == 0 {
				v = color.RGBA{200, 230, 170, 255}
			}
			img.Set(x, y, v)
		}
	}
	if err = png.Encode(&pixels, img); err != nil {
		t.Fatal(err)
	}
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(pixels.Bytes()) }))
	defer storage.Close()
	svc, err := oss.New(config.Config{OSSEndpoint: storage.URL, OSSCDNDomain: strings.TrimPrefix(storage.URL, "http://"), OSSBucket: "test", OSSAccessKeyID: "test", OSSAccessSecret: "test"})
	if err != nil {
		t.Fatal(err)
	}
	key, err := generatedImageKey(SeedUserID, "hero-version", "png")
	if err != nil {
		t.Fatal(err)
	}
	document := `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>body{font:18px system-ui;background:#faf9f6;color:#18382d;margin:0;padding:24px}img{display:block;width:min(100%,640px);height:auto;border-radius:16px}button{font:inherit;padding:12px;margin-top:20px}</style></head><body><h1>公开图片版本走查样本</h1><p>这是一份本地测试作品，不是真实学生成果。</p><img src="` + pbl.HeroImageSource + `" alt="绿色棋盘测试图片"><button onclick="document.getElementById('result').textContent='作品介绍已展开'">展开作品介绍</button><p id="result" aria-live="polite">作品介绍未展开</p></body></html>`
	brief, _ := json.Marshal(map[string]any{"heroImage": heroImageAsset{Key: key, Prompt: "private fixture prompt"}, "pageContent": pbl.SiteContent{About: []string{"这是一份本地测试作品，不是真实学生成果。"}}})
	version, err := q.CreatePblCodeVersion(t.Context(), sqlc.CreatePblCodeVersionParams{AtomID: atom.ID, BriefRevision: 1, Brief: brief, Html: document})
	if err != nil {
		t.Fatal(err)
	}
	token := "local-browser-fixture"
	if _, err = pool.Exec(t.Context(), "INSERT INTO pbl_site(user_id,atom_id,share_token) VALUES($1,$2,$3)", SeedUserID, atom.ID, token); err != nil {
		t.Fatal(err)
	}
	if err = q.SetPblSitePublication(t.Context(), sqlc.SetPblSitePublicationParams{UserID: SeedUserID, AtomID: atom.ID, VersionID: version.ID}); err != nil {
		t.Fatal(err)
	}
	a := &API{d: Deps{Pool: pool, Queries: q, OSS: svc}}
	done := make(chan struct{})
	var once sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/public/sites/{token}", a.getPublicSite)
	mux.HandleFunc("GET /api/v1/public/sites/{token}/render", a.renderPublicCodeSite)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>本地公开页验收</title><body><h2>独立本地测试：无学生数据、无外部生成</h2><iframe title="公开主页预览" sandbox="allow-scripts" referrerpolicy="no-referrer" src="/api/v1/public/sites/local-browser-fixture/render" style="width:100%;height:560px;border:1px solid #aaa"></iframe><form method="post" action="/revoke"><button>撤回本地测试链接</button></form><form method="post" action="/finish"><button>结束本地验收</button></form></body></html>`))
	})
	mux.HandleFunc("POST /revoke", func(w http.ResponseWriter, r *http.Request) {
		if _, err := pool.Exec(r.Context(), "UPDATE pbl_site SET share_token=NULL WHERE user_id=$1", SeedUserID); err != nil {
			http.Error(w, "revoke failed", 500)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /finish", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Local walkthrough finished"))
		once.Do(func() { close(done) })
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	t.Logf("BROWSER_WALK_URL=%s", server.URL)
	select {
	case <-done:
	case <-time.After(8 * time.Minute):
		t.Fatal("browser walkthrough timed out")
	}
}
