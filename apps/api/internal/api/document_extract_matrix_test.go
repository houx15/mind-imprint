package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The teacher's upload field calls this endpoint before publishing a reading
// assignment. Exercise the real multipart handler for every advertised type.
func TestTeacherDocumentExtractFormatMatrix(t *testing.T) {
	h, _, teacher, _, _ := liteTeacherFixture(t)
	cases := []struct {
		name   string
		data   []byte
		status int
		code   string
	}{
		{"article.txt", []byte("文章标题\n\n第一段文字。"), http.StatusOK, ""},
		{"article.md", []byte("# 文章标题\n\n第一段文字。"), http.StatusOK, ""},
		{"reading.docx", readTestFixture(t, "reading.docx"), http.StatusOK, ""},
		{"reading.pdf", readTestFixture(t, "reading.pdf"), http.StatusOK, ""},
		{"image.png", []byte("not a supported text document"), http.StatusBadRequest, "unsupported_type"},
		{"empty.docx", emptyDOCX(t), http.StatusBadRequest, "missing_text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, withCookie(multipartFileRequest(t, "/api/v1/documents/extract", tc.name, tc.data), teacher))
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if tc.status == http.StatusOK {
				var out struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || strings.TrimSpace(out.Text) == "" {
					t.Fatalf("extracted text: %s (%v)", rec.Body, err)
				}
			} else if !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("error code %s missing: %s", tc.code, rec.Body)
			}
		})
	}
}
