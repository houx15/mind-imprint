package api_test

// writing_structure_test.go — the 2026-08-27 ruling, pinned as tests:
// **AI never directly generates an outline.** It may only pick one skeleton
// out of a fixed, generic library and say why; every word of content is the
// student's.
//
// Reuses writing_turn_test.go / writings_test.go's harness
// (liteHandlerWithProvider, createWritingAtomHTTP, writingTextStubProvider).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type structureBlockDTO struct {
	Role string `json:"role"`
	Hint string `json:"hint"`
}

type structureDTO struct {
	Key    string              `json:"key"`
	Lang   string              `json:"lang"`
	Name   string              `json:"name"`
	Blurb  string              `json:"blurb"`
	Blocks []structureBlockDTO `json:"blocks"`
}

func listStructures(t *testing.T, h http.Handler, cookie *http.Cookie, lang string) []structureDTO {
	t.Helper()
	url := "/api/v1/writings/structures"
	if lang != "" {
		url += "?lang=" + lang
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", url, nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET structures = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Structures []structureDTO `json:"structures"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode structures: %v — body=%s", err, rec.Body)
	}
	return out.Structures
}

// TestWritingStructures_LibraryIsLangScopedAndNonEmpty — the library must
// always offer the student something to pick, in her own language.
func TestWritingStructures_LibraryIsLangScopedAndNonEmpty(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))

	zh := listStructures(t, h, cookie, "zh")
	if len(zh) == 0 {
		t.Fatalf("zh library is empty — a student with no idea has nothing to choose")
	}
	for _, s := range zh {
		if s.Lang != "zh" {
			t.Fatalf("?lang=zh returned a %q skeleton (%s)", s.Lang, s.Key)
		}
		if len(s.Blocks) == 0 {
			t.Fatalf("skeleton %s has no blocks", s.Key)
		}
	}

	en := listStructures(t, h, cookie, "en")
	if len(en) == 0 {
		t.Fatalf("en library is empty")
	}
	for _, s := range en {
		if s.Lang != "en" {
			t.Fatalf("?lang=en returned a %q skeleton (%s)", s.Lang, s.Key)
		}
	}

	// An unrecognised lang must degrade to a usable library, never to none.
	if got := listStructures(t, h, cookie, "kl"); len(got) == 0 {
		t.Fatalf("unknown lang returned an empty library — must fall back, not strand her")
	}
}

// TestApplyWritingStructure_LaysOutRolesWithEmptyText is THE 铁律 assertion of
// this phase, and the reason the skeleton is data rather than a model call:
// applying a structure must produce blocks that are LABELLED but CONTENTLESS.
// If any block came back with text pre-filled, the product would be writing
// her essay for her.
func TestApplyWritingStructure_LaysOutRolesWithEmptyText(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "我想写该不该禁止学生带手机进校园。")

	rec := postWritingStructure(t, h, cookie, id, `{"structureKey":"zh-argument-concession"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply structure = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		StructureKey string               `json:"structureKey"`
		Outline      []writingOutlineItem `json:"outline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode apply: %v — body=%s", err, rec.Body)
	}
	if out.StructureKey != "zh-argument-concession" {
		t.Fatalf("structureKey = %q, want zh-argument-concession", out.StructureKey)
	}
	if len(out.Outline) == 0 {
		t.Fatalf("applying a structure produced no blocks")
	}
	for _, b := range out.Outline {
		if strings.TrimSpace(b.Text) != "" {
			t.Fatalf("block %q arrived with text %q — the AI must never pre-fill a block's content",
				b.Role, b.Text)
		}
		if strings.TrimSpace(b.Role) == "" {
			t.Fatalf("block at position %d has no role label — the skeleton contributed nothing", b.Position)
		}
	}

	// And it must be persisted, not merely echoed.
	got := getWritingOutlineHTTP(t, h, cookie, id)
	if len(got) != len(out.Outline) {
		t.Fatalf("GET outline returned %d blocks, apply returned %d", len(got), len(out.Outline))
	}
	if got[0].Role == "" {
		t.Fatalf("role did not persist — GET returned %+v", got[0])
	}
}

// TestApplyWritingStructure_RefusesToDiscardWrittenWork — swapping skeleton
// wipes the outline. That must never happen silently on top of text she has
// written; it takes an explicit force.
func TestApplyWritingStructure_RefusesToDiscardWrittenWork(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "手机该不该禁。")

	if rec := postWritingStructure(t, h, cookie, id, `{"structureKey":"zh-argument-stance"}`); rec.Code != http.StatusOK {
		t.Fatalf("first apply = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	// She writes her own point into the first block.
	if rec := putWritingOutlineHTTP(t, h, cookie, id,
		`{"outline":[{"text":"我反对一刀切禁止","role":"你的立场","depth":0}]}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT outline = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec := postWritingStructure(t, h, cookie, id, `{"structureKey":"zh-narrative"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("switching structure over written work = %d, want 409; body=%s", rec.Code, rec.Body)
	}
	// Her sentence must still be there.
	if got := getWritingOutlineHTTP(t, h, cookie, id); len(got) != 1 || got[0].Text != "我反对一刀切禁止" {
		t.Fatalf("her text did not survive the refused switch: %+v", got)
	}

	// With force, the switch goes through — her decision, made explicitly.
	if rec := postWritingStructure(t, h, cookie, id, `{"structureKey":"zh-narrative","force":true}`); rec.Code != http.StatusOK {
		t.Fatalf("forced switch = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestApplyWritingStructure_UnknownKeyIs400 — fail closed. A key that does not
// resolve must never lay out an empty outline and call it a structure.
func TestApplyWritingStructure_UnknownKeyIs400(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "随便写点什么。")
	rec := postWritingStructure(t, h, cookie, id, `{"structureKey":"not-a-real-skeleton"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown structureKey = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func postStructureRecommend(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/structure/recommend", nil), cookie))
	return rec
}

// TestRecommendWritingStructure_ReturnsALibraryKey — the model SELECTS; the
// endpoint persists nothing, so the student still has to accept it.
func TestRecommendWritingStructure_ReturnsALibraryKey(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(
		`{"structureKey":"zh-argument-concession","reason":"你两边都有理由，让步式放得下反方。"}`))
	id := createWritingAtomHTTP(t, h, cookie, "该不该禁止学生带手机。")

	rec := postStructureRecommend(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("recommend = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		StructureKey string `json:"structureKey"`
		Reason       string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode recommend: %v — body=%s", err, rec.Body)
	}
	if out.StructureKey != "zh-argument-concession" {
		t.Fatalf("structureKey = %q, want the recommended library key", out.StructureKey)
	}
	if out.Reason == "" {
		t.Fatalf("recommendation carried no reason")
	}

	// PERSISTS NOTHING: recommending must not lay out an outline behind her
	// back. She accepts by calling apply.
	if got := getWritingOutlineHTTP(t, h, cookie, id); len(got) != 0 {
		t.Fatalf("recommend wrote %d outline rows — it must only suggest", len(got))
	}
}

// TestRecommendWritingStructure_OutOfLibraryKeyIsAnError — a model that
// invents a skeleton (or picks the other language's) gets no pass-through and
// no silent default substitution: she is told it failed and still has the
// whole library to choose from herself.
func TestRecommendWritingStructure_OutOfLibraryKeyIsAnError(t *testing.T) {
	for _, reply := range []string{
		`{"structureKey":"zh-invented-by-the-model","reason":"x"}`,
		`{"structureKey":"en-personal-essay","reason":"x"}`, // right shape, wrong language
		`不是 JSON`,
	} {
		h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
		id := createWritingAtomHTTP(t, h, cookie, "中文写作。")
		rec := postStructureRecommend(t, h, cookie, id)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("reply %q → %d, want 502 ai_dialogue_failed; body=%s", reply, rec.Code, rec.Body)
		}
	}
}

// TestSetWritingStage_RetiredIdeateIsRejected — the map collapsed to three
// steps; 'ideate' no longer names a page, so the API must not accept it.
func TestSetWritingStage_RetiredIdeateIsRejected(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写点什么。")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings/"+id+"/stage",
		strings.NewReader(`{"stage":"ideate"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("stage=ideate = %d, want 400 — the stage was retired; body=%s", rec.Code, rec.Body)
	}
}
