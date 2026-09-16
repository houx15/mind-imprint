package api_test

import (
	"context"
	"fmt"
	"mindimprint/api/internal/gateway"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

// AI text analysis is only persisted when its quotations match fetched content.
func TestSiteRefQuality_OnlyVerifiedAnalysisIsPersisted(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<title>摄影记录</title><p>校园树木：观察叶片的变化。</p>`)
	}))
	defer page.Close()
	for _, tc := range []struct {
		name, reply string
		want        int
	}{
		{"valid", `{"readable":true,"evidence":["校园树木：观察叶片的变化。"],"what":"摄影观察记录","structure":"标题和观察记录","best":"以观察问题介绍作品"}`, http.StatusCreated},
		{"loading", `{"readable":false,"reason":"only loading"}`, http.StatusBadRequest},
		{"invented", `{"readable":true,"evidence":["并不存在的照片展览"],"what":"摄影记录","structure":"作品列表","best":"有展览"}`, http.StatusBadRequest},
		{"missing evidence", `{"what":"摄影记录","structure":"作品列表","best":"有展览"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c, q, pool := liteHandler(t)
			h := New(Deps{Queries: q, Pool: pool, Provider: pblCoachSaying(tc.reply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), Fetcher: fakeFetcher{}}).Handler()
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/sites", fmt.Sprintf(`{"url":%q}`, page.URL))
			if rec.Code != tc.want {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
			}
			var count int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_site_ref WHERE atom_id=$1`, id).Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 0
			if tc.want == http.StatusCreated {
				want = 1
			}
			if count != want {
				t.Fatalf("stored %d, want %d", count, want)
			}
		})
	}
}

// A failed extraction may be repaired once; both model calls remain billable.
type siteRefRepairProvider struct {
	calls    int
	repaired bool
	feedback bool
}

func (p *siteRefRepairProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.calls++
	reply := `{"readable":true,"evidence":["不存在的原文"],"what":"摄影记录","structure":"标题和记录","best":"展示观察"}`
	if p.calls == 2 {
		p.feedback = strings.Contains(req.Messages[len(req.Messages)-1].Content, "上一次分析未通过原文引用核验")
		if p.repaired {
			reply = `{"readable":true,"evidence":["校园树木：观察叶片的变化。"],"what":"摄影记录","structure":"标题和记录","best":"展示观察"}`
		}
	}
	return pblCoachSaying(reply).Stream(ctx, r, req)
}
func TestSiteRefQuality_RepairsOnceAndMetersBothCalls(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<title>摄影记录</title><p>校园树木：观察叶片的变化。</p>`)
	}))
	defer page.Close()
	for _, repaired := range []bool{true, false} {
		t.Run(fmt.Sprint(repaired), func(t *testing.T) {
			_, c, q, pool := liteHandler(t)
			provider := &siteRefRepairProvider{repaired: repaired}
			h := New(Deps{Queries: q, Pool: pool, Provider: provider, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), Fetcher: fakeFetcher{}}).Handler()
			id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			rec := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/sites", fmt.Sprintf(`{"url":%q}`, page.URL))
			want := http.StatusBadRequest
			if repaired {
				want = http.StatusCreated
			}
			if rec.Code != want || provider.calls != 2 || !provider.feedback {
				t.Fatalf("status=%d calls=%d feedback=%v body=%s", rec.Code, provider.calls, provider.feedback, rec.Body)
			}
			var count int
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM llm_call WHERE atom_id=$1 AND purpose IN ('pbl_site_ref','pbl_site_ref_retry')`, id).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatalf("metered calls=%d", count)
			}
			if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pbl_site_ref WHERE atom_id=$1`, id).Scan(&count); err != nil {
				t.Fatal(err)
			}
			expected := 0
			if repaired {
				expected = 1
			}
			if count != expected {
				t.Fatalf("saved=%d", count)
			}
		})
	}
}
