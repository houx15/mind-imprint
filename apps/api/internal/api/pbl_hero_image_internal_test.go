package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/oss"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

func TestHeroImageVersionsReuseRedrawAndPreview(t *testing.T) {
	for _, mode := range []string{"image", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			pool := NewTestDB(t)
			q := sqlc.New(pool)
			atom, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{AtomID: atom.ID, Kind: "website", Idea: "test", Name: "test"}); err != nil {
				t.Fatal(err)
			}
			doc := pbl.CreativeDirection{Stage: "hero", Feeling: "space", Motifs: []string{"garden"}, Hero: &pbl.HeroBrief{Mode: mode, Scene: "space garden", Prompt: "draw garden"}}
			brief, _ := json.Marshal(doc)
			if _, err = pool.Exec(t.Context(), "INSERT INTO pbl_creative_direction(atom_id,document,revision) VALUES($1,$2,1)", atom.ID, brief); err != nil {
				t.Fatal(err)
			}
			var pixels bytes.Buffer
			_ = png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 2)))
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(pixels.Bytes()) }))
			defer upstream.Close()
			objects := map[string][]byte{}
			var mu sync.Mutex
			storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.Method == "PUT" {
					objects[r.URL.Path], _ = io.ReadAll(r.Body)
				} else {
					data, ok := objects[r.URL.Path]
					if !ok {
						http.NotFound(w, r)
						return
					}
					_, _ = w.Write(data)
				}
			}))
			defer storage.Close()
			svc, err := oss.New(config.Config{OSSEndpoint: storage.URL, OSSCDNDomain: strings.TrimPrefix(storage.URL, "http://"), OSSBucket: "test", OSSAccessKeyID: "test", OSSAccessSecret: "test"})
			if err != nil {
				t.Fatal(err)
			}
			response, _ := json.Marshal(map[string]string{"html": pbl.ImageHeroHTML("test", "garden")})
			provider := gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: string(response)}, {Kind: gateway.EventDone}})
			draws := 0
			a := &API{d: Deps{Pool: pool, Queries: q, OSS: svc, Provider: provider, Route: func(class string) gateway.KeyResolver {
				return func(context.Context) (gateway.Resolved, error) {
					return gateway.Resolved{Provider: "dashscope_image", Model: "qwen-image-3.0", Tier: "chaperone"}, nil
				}
			}, Drawer: trialDrawer(func(context.Context, gateway.Resolved, gateway.DrawRequest) (gateway.DrawResult, error) {
				draws++
				return gateway.DrawResult{URL: upstream.URL}, nil
			})}}
			request := func(body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("POST", "/", strings.NewReader(body))
				r.SetPathValue("id", atom.ID.String())
				r = r.WithContext(WithUser(r.Context(), User{ID: SeedUserID, DisplayName: "test"}))
				w := httptest.NewRecorder()
				a.generatePblHeroCode(w, r)
				return w
			}
			create := func(base string, redraw bool) uuid.UUID {
				input := map[string]any{"revision": 1, "redrawImage": redraw}
				if base != "" {
					input["baseVersion"] = base
					input["feedback"] = "larger flowers"
				}
				body, _ := json.Marshal(input)
				w := request(string(body))
				if w.Code != 201 {
					t.Fatalf("generate %d: %s", w.Code, w.Body)
				}
				var result struct {
					ID uuid.UUID `json:"id"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				return result.ID
			}
			first := create("", false)
			if draws != 1 {
				t.Fatal(draws)
			}
			second := create(first.String(), false)
			expected := 1
			if mode == "image" {
				expected = 2
			}
			if draws != expected {
				t.Fatalf("unexpected redraw: %d", draws)
			}
			firstRow, _ := q.GetPblCodeVersion(t.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom.ID, ID: first})
			secondRow, _ := q.GetPblCodeVersion(t.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom.ID, ID: second})
			var x, y struct {
				Image heroImageAsset `json:"heroImage"`
			}
			_ = json.Unmarshal(firstRow.Brief, &x)
			_ = json.Unmarshal(secondRow.Brief, &y)
			if (x.Image.Key == y.Image.Key) != (mode == "mixed") {
				t.Fatal("wrong asset reuse")
			}
			_ = create(second.String(), true)
			if draws != expected+1 {
				t.Fatal("redraw ignored")
			}
			if !strings.Contains(firstRow.Html, pbl.HeroImageSource) || strings.Contains(string(firstRow.Brief), upstream.URL) {
				t.Fatal("expiring asset persisted")
			}
			for _, user := range []uuid.UUID{SeedUserID, uuid.New()} {
				r := httptest.NewRequest("GET", "/", nil)
				r.SetPathValue("id", atom.ID.String())
				r.SetPathValue("version", first.String())
				r = r.WithContext(WithUser(r.Context(), User{ID: user}))
				w := httptest.NewRecorder()
				a.previewPblCodeVersion(w, r)
				if user != SeedUserID {
					if w.Code != 404 {
						t.Fatal("foreign preview accepted")
					}
					continue
				}
				if w.Code != 200 || !strings.Contains(w.Body.String(), "data:image/png;base64,") || strings.Contains(w.Body.String(), pbl.HeroImageSource) || w.Header().Get("Content-Security-Policy") != pbl.CodePreviewCSP {
					t.Fatalf("preview %d %s", w.Code, w.Body)
				}
			}
			// Stale requests must not spend another draw.
			before := draws
			w := request(`{"revision":0}`)
			if w.Code != 409 || draws != before {
				t.Fatal("stale request generated")
			}
		})
	}
}
