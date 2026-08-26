package api

// reading_source_file.go — Task 15: POST /api/v1/readings/{id}/source/file.
// The lite edition's third way to get an article into a reading, alongside
// paste and URL (reading_source.go): the student uploads a PDF/DOCX and the
// server extracts it with the SAME pure-Go parser pro uses
// (internal/docextract), then lands the text through the EXACT same storage
// call as paste/URL — UpsertReadingSource + SplitBlocks — so the reading room
// cannot tell how the article arrived and the frontend needs no branching.
//
// Size cap, content-type allowlist, and error messages are lifted verbatim
// from pro's user_doc OSS scope (oss.go) so a student hitting the limit on
// this direct-upload surface sees the same behavior as pro's presigned-OSS
// upload path.

import (
	"io"
	"net/http"
	"strings"

	"mindimprint/api/internal/docextract"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// liteSourceFileMaxBytes mirrors pro's "user_doc" OSS scope maxBytes (oss.go).
const liteSourceFileMaxBytes = 30 << 20 // 30 MB

// liteSourceFileMultipartMemory bounds how much of the multipart body
// ParseMultipartForm buffers in memory before spilling the rest to a temp
// file on disk. Set to the same 30 MB cap: a within-limit upload never
// touches disk, and MaxBytesReader below refuses anything larger before
// ParseMultipartForm ever runs, so this ceiling is never actually reached —
// it exists only so a future cap increase can't accidentally buffer an
// unbounded amount in memory.
const liteSourceFileMultipartMemory = liteSourceFileMaxBytes

// postReadingSourceFileLite accepts a multipart upload ("file" field), an
// uploaded PDF or DOCX, and stores its extracted text as the reading's
// article — the same UpsertReadingSource/SplitBlocks path putReadingSourceLite
// uses for paste and URL.
func (a *API) postReadingSourceFileLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	// Same guard as PUT .../source (refuseIfAnchored): this path lands through
	// the identical UpsertReadingSource call, so it can re-point anchors into
	// unrelated prose in exactly the same way. Checked before the upload is
	// read, so an oversize replacement is refused for the honest reason.
	if a.refuseIfAnchored(w, r, at.ID) {
		return
	}

	// Bound the read BEFORE any parsing touches the body — an oversize upload
	// must be rejected without ever buffering the whole file into memory.
	r.Body = http.MaxBytesReader(w, r.Body, liteSourceFileMaxBytes)
	if err := r.ParseMultipartForm(liteSourceFileMultipartMemory); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("file_too_large", "文件太大，最多 30MB。", nil))
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_file", "请选择要上传的文件。", nil))
		return
	}
	defer file.Close()

	// Extension allowlist BEFORE reading the bytes: an unsupported type is
	// rejected without ever pulling the file into memory.
	ext := strings.ToLower(fileExt(header.Filename))
	if ext != ".pdf" && ext != ".docx" {
		// Same allowlist and message as pro's user_doc OSS scope (oss.go /
		// ingest_file.go): only PDF and Word documents.
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_type", "只支持 PDF 或 Word 文档。", nil))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		// Distinct from the oversize case above: ParseMultipartForm already
		// accepted the body (so it was within the size cap) — this is a read
		// failure on the already-parsed part itself, e.g. a truncated/corrupt
		// multipart stream, not "too large".
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_file", "文件读取失败，请重新上传。", nil))
		return
	}

	var title, text string
	if ext == ".pdf" {
		title, text, err = docextract.PDF(data)
	} else {
		title, text, err = docextract.DOCX(data)
	}
	if err != nil {
		// Extraction failed (corrupt / password-protected / unsupported
		// internals) — 400 so the reading room can offer the paste fallback.
		httpx.WriteError(w, r, httpx.ErrBadRequest("extract_failed", "这个文件没能解析出正文——直接把正文粘进来就能逐句共读。", nil))
		return
	}

	body := strings.TrimSpace(text)
	if len(SplitBlocks(body)) == 0 {
		// Same code/message the paste path (PUT .../source) uses for an empty
		// body — a scanned/image-only PDF or a blank document degrades to the
		// identical "put the text in" prompt, not a distinct file-upload error.
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先把文章正文放进来。", nil))
		return
	}

	docTitle := strings.TrimSpace(title)
	if docTitle == "" {
		docTitle = "未命名文章"
	}

	// No SourceUrl: an uploaded file has no fetchable link (its filename is
	// not a URL and would render as a broken "查看原文" href) — same as a
	// pasted body, which also carries none.
	row, err := a.d.Queries.UpsertReadingSource(r.Context(), sqlc.UpsertReadingSourceParams{
		AtomID: at.ID, Title: docTitle, Body: body, SourceUrl: nil,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: derefOr(row.SourceUrl, ""), Blocks: SplitBlocks(row.Body),
	})
}

// fileExt returns the lowercase extension (including the dot) of filename,
// or "" if it has none. Only used to route to the right extractor — never
// used as a path segment or trusted for anything security-sensitive.
func fileExt(filename string) string {
	i := strings.LastIndex(filename, ".")
	if i < 0 {
		return ""
	}
	return filename[i:]
}
