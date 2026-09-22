package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/library"
)

type digCandidateProvider struct {
	slugs     []string
	calls     int
	forceSlug string
}

func (p *digCandidateProvider) Stream(_ context.Context, _ gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.calls++
	for _, m := range req.Messages {
		if m.Role != gateway.RoleUser {
			continue
		}
		for _, line := range strings.Split(m.Content, "\n") {
			if !strings.HasPrefix(line, "- ") {
				continue
			}
			slug, _, ok := strings.Cut(strings.TrimPrefix(line, "- "), " ｜ ")
			if ok {
				if _, exists := library.BySlug(slug); exists {
					p.slugs = append(p.slugs, slug)
				}
			}
		}
	}
	slug := p.forceSlug
	if slug == "" && len(p.slugs) > 0 {
		slug = p.slugs[0]
	}
	raw, _ := json.Marshal(map[string]any{"seeds": []map[string]string{{"kind": "think", "text": "哪些条件会影响海洋生态？", "why": "联系当前问题。"}, {"kind": "read", "text": "模型标题会被真实标题覆盖", "why": "进一步了解相关情况。", "slug": slug}}})
	out := make(chan gateway.StreamEvent, 2)
	out <- gateway.StreamEvent{Kind: gateway.EventTextDelta, TextDelta: string(raw)}
	out <- gateway.StreamEvent{Kind: gateway.EventDone, StopReason: gateway.StopStop}
	close(out)
	return out, nil
}

func TestKeywordDigUsesCurrentKeywordShortlistAndReadingTier(t *testing.T) {
	prov := &digCandidateProvider{}
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	id := seedKeyword(t, pool, "climate-ocean")
	if _, err := pool.Exec(t.Context(), `UPDATE interest_keyword SET interest_id='ocean', text_zh='海洋', text_en='Ocean', norm='海洋' WHERE id=$1`, mustUUID(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `DELETE FROM keyword_source WHERE keyword_id=$1`, mustUUID(id)); err != nil {
		t.Fatal(err)
	}
	ocean, _ := interests.ByID("ocean")
	allowed := map[string]bool{}
	for _, a := range library.All() {
		for _, d := range a.Disciplines {
			for _, want := range ocean.Disciplines {
				if d == want {
					allowed[a.Slug] = true
				}
			}
		}
	}
	var readSlug string
	for _, a := range library.All() {
		if allowed[a.Slug] {
			readSlug = a.Slug
			break
		}
	}
	if readSlug == "" {
		t.Fatal("fixture requires one ocean-related article")
	}
	atomID := createReadingAtom(t, h, cookie)
	if _, err := pool.Exec(t.Context(), `UPDATE reading SET library_slug=$1,library_tier=4,status='finished' WHERE atom_id=$2`, readSlug, mustUUID(atomID)); err != nil {
		t.Fatal(err)
	}
	got := decodeDig(t, getDig(h, cookie, id))
	if prov.calls != 1 || len(prov.slugs) == 0 || len(prov.slugs) > 3 {
		t.Fatalf("calls=%d candidates=%v", prov.calls, prov.slugs)
	}
	for _, s := range prov.slugs {
		if !allowed[s] || s == readSlug {
			t.Fatalf("unrelated or read article sent: %s", s)
		}
	}
	var slug string
	var tier int
	if err := pool.QueryRow(t.Context(), `SELECT library_slug,library_tier FROM keyword_dig WHERE keyword_id=$1 AND kind='read'`, mustUUID(id)).Scan(&slug, &tier); err != nil {
		t.Fatal(err)
	}
	if slug != prov.slugs[0] || tier != 4 {
		t.Fatalf("slug/tier=%s/%d", slug, tier)
	}
	article, _ := library.BySlug(slug)
	for _, s := range got.Seeds {
		if s.Kind == "read" && s.Text != article.ZhTitle {
			t.Fatalf("model title was not replaced: %s", s.Text)
		}
	}
	decodeDig(t, getDig(h, cookie, id))
	if prov.calls != 1 {
		t.Fatal("cached seeds generated again")
	}
}

func TestKeywordDigNoTopicMatchDoesNotRecommendGenericLibraryArticle(t *testing.T) {
	prov := &digCandidateProvider{forceSlug: library.All()[0].Slug}
	h, cookie, _, pool := liteHandlerWithProvider(t, prov)
	id := seedKeyword(t, pool, "climate-ocean")
	if _, err := pool.Exec(t.Context(), `UPDATE interest_keyword SET interest_id=NULL,text_zh='未收录的话题XYZ',text_en='',norm='未收录的话题XYZ' WHERE id=$1`, mustUUID(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `DELETE FROM keyword_source WHERE keyword_id=$1`, mustUUID(id)); err != nil {
		t.Fatal(err)
	}
	got := decodeDig(t, getDig(h, cookie, id))
	if prov.calls != 1 || len(prov.slugs) != 0 {
		t.Fatalf("calls=%d candidates=%v", prov.calls, prov.slugs)
	}
	if len(got.Seeds) != 1 || got.Seeds[0].Kind != "think" {
		t.Fatalf("out-of-list reading survived: %+v", got)
	}
}
