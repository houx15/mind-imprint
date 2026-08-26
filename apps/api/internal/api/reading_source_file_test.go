package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "mindimprint/api/internal/api"
)

// reading_source_file_test.go — Task 15: POST /api/v1/readings/{id}/source/file
// accepts an uploaded DOCX/PDF and must land the extracted text through the
// EXACT same storage path as paste/URL (UpsertReadingSource + SplitBlocks),
// returning the same sourceDTO shape.

// multipartFileRequest builds a multipart/form-data POST with a single "file"
// part carrying filename/data.
func multipartFileRequest(t *testing.T, url, filename string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func readTestFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestReadingSourceFile_DOCXUploadExtractsAndSplitsBlocks(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	docx := readTestFixture(t, "reading.docx")
	req := multipartFileRequest(t, "/api/v1/readings/"+id+"/source/file", "reading.docx", docx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload docx = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Title     string  `json:"title"`
		SourceURL string  `json:"sourceUrl"`
		Blocks    []Block `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Blocks) == 0 {
		t.Fatalf("want non-empty blocks, got none; body=%s", rec.Body)
	}
	if out.Title == "" {
		t.Fatalf("want a non-empty title; body=%s", rec.Body)
	}

	// The same storage path as PUT .../source: GET must now return the same
	// blocks (byte-identical shape — sourceDTO either way).
	getReq := httptest.NewRequest("GET", "/api/v1/readings/"+id+"/source", nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, withCookie(getReq, cookie))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get source after upload = %d; body=%s", getRec.Code, getRec.Body)
	}
	var got struct {
		Blocks []Block `json:"blocks"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if len(got.Blocks) != len(out.Blocks) {
		t.Fatalf("GET blocks = %d, want %d (upload response)", len(got.Blocks), len(out.Blocks))
	}
}

func TestReadingSourceFile_PDFUploadExtractsAndSplitsBlocks(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	pdf := readTestFixture(t, "reading.pdf")
	req := multipartFileRequest(t, "/api/v1/readings/"+id+"/source/file", "reading.pdf", pdf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload pdf = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Blocks []Block `json:"blocks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Blocks) == 0 {
		t.Fatalf("want non-empty blocks, got none; body=%s", rec.Body)
	}
}

func TestReadingSourceFile_DisallowedTypeRejected(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	req := multipartFileRequest(t, "/api/v1/readings/"+id+"/source/file", "malware.exe", []byte("MZ\x90\x00fake exe"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upload .exe = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

func TestReadingSourceFile_OversizeRejected(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	// One byte over pro's user_doc cap (30 MB) — must be rejected before the
	// whole file is buffered into memory.
	oversized := bytes.Repeat([]byte("a"), 30<<20+1)
	req := multipartFileRequest(t, "/api/v1/readings/"+id+"/source/file", "big.pdf", oversized)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversize upload = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// emptyDOCX is a structurally valid .docx with a document.xml that has no
// paragraph text at all — extraction succeeds but yields "".
func emptyDOCX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body></w:body></w:document>`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadingSourceFile_EmptyExtractedTextRejected(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	req := multipartFileRequest(t, "/api/v1/readings/"+id+"/source/file", "empty.docx", emptyDOCX(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty-text upload = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error.Code != "missing_text" {
		t.Fatalf("error code = %q, want missing_text (same as PUT .../source's empty-body code); body=%s", body.Error.Code, rec.Body)
	}
}

// TestReadingSourceFile_ForeignAtomIs404 mirrors
// TestPutSource_ForeignAtomIs404 in reading_source_test.go: an atom id that
// exists nowhere (never created, or belonging to nobody the caller is) must
// 404 flatly — loadOwnedReadingAtom never leaks existence via 403.
func TestReadingSourceFile_ForeignAtomIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	docx := readTestFixture(t, "reading.docx")
	req := multipartFileRequest(t, "/api/v1/readings/00000000-0000-0000-0000-0000000009ff/source/file", "reading.docx", docx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign atom upload = %d, want 404 (never 403); body=%s", rec.Code, rec.Body)
	}
}
