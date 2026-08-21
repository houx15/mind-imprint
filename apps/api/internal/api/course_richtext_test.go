package api_test

// course_richtext_test.go — the PUT-definition security border for `richText`
// blocks: the server refuses to STORE authored HTML whose only purpose is to
// execute. The renderer's sandbox (no allow-scripts) is the actual boundary;
// this keeps stored content honest so no future host can turn it into a live
// script.

import (
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// richTextCourseDoc is a border-valid definition with one richText block whose
// html is the given fragment (JSON-escaped by the caller).
func richTextCourseDoc(id, htmlJSON string) string {
	return `{"schemaVersion":"2.0","course":{"id":"` + id + `","title":"Rich Text Course","language":"zh","estimatedMinutes":5,` +
		`"objectives":[],"parts":[{"slices":[{"blocks":[{"id":"card","type":"richText","html":` + htmlJSON + `}]}]}]}}`
}

func TestPutDefinitionAcceptsAStyledRichTextCard(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "richtext-ok"
	html := `"<style>.k{color:#b0463a}</style><h2>三种来源</h2><table><tr><th>类型</th></tr></table>"`
	rec := putDefinition(h, slug, putCourseDefinitionBody(richTextCourseDoc(slug, html), nil, "styled card"))
	if rec.Code != http.StatusOK {
		t.Fatalf("styled richText card: want 200, got %d %s", rec.Code, rec.Body)
	}
}

func TestPutDefinitionRejectsExecutableRichTextHtml(t *testing.T) {
	cases := []struct {
		name string
		html string
	}{
		{"script tag", `"<p>hi</p><script>alert(1)</script>"`},
		{"spaced script tag", `"< script >alert(1)< /script >"`},
		{"iframe", `"<iframe src=\"https://evil.example\"></iframe>"`},
		{"inline handler", `"<div onclick=\"steal()\">tap</div>"`},
		{"javascript url", `"<a href=\"javascript:alert(1)\">go</a>"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := newAPITestPool(t)
			h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

			slug := "richtext-reject"
			rec := putDefinition(h, slug, putCourseDefinitionBody(richTextCourseDoc(slug, tc.html), nil, "bad card"))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d %s", rec.Code, rec.Body)
			}
			// The error names the offending block, so the generator can fix it
			// without diffing the whole document.
			if !strings.Contains(rec.Body.String(), "card") {
				t.Fatalf("error does not name the block: %s", rec.Body)
			}
		})
	}
}

// The guard is scoped to richText: an interactiveHtml block legitimately ships
// scripts (in its own OSS asset), and other blocks have no html field at all.
func TestPutDefinitionGuardDoesNotTouchOtherBlockTypes(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "richtext-scoped"
	doc := `{"schemaVersion":"2.0","course":{"id":"` + slug + `","title":"Mixed","language":"zh","estimatedMinutes":5,` +
		`"objectives":[],"parts":[{"slices":[{"blocks":[` +
		`{"id":"sim","type":"interactiveHtml","source":"interactions/html/sim.html"},` +
		`{"id":"note","type":"text","content":"<script>not markup, just prose</script>"}` +
		`]}]}]}}`
	rec := putDefinition(h, slug, putCourseDefinitionBody(doc, nil, "mixed blocks"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body)
	}
}
