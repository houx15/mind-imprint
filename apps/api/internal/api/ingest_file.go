package api

// ingest_file.go — POST /projects/{id}/references/{rid}/ingest-file. The student
// uploaded a PDF/DOCX to OSS (scope user_doc) and now asks the server to turn it
// into readable material: download the bytes, extract text (pure-Go, no CGO),
// segment into blocks, and link a material to the reference — the same tx shape
// fetchMaterialForReference uses for fetched URLs. After this the reference has
// a material, so 进入阅读室 opens normally instead of the no-readable-content 422.

import (
	"net/http"
	"strings"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/docextract"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
)

// ingestReferenceFile downloads an uploaded document from OSS and attaches an
// extracted material to the reference.
func (a *API) ingestReferenceFile(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body struct {
		ObjectKey string `json:"objectKey"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	key := strings.TrimSpace(body.ObjectKey)
	// IDOR: the object must live under THIS user's own docs prefix — never read
	// another user's object (or an arbitrary key) into this project.
	if key == "" || !strings.HasPrefix(key, "users/"+u.ID.String()+"/docs/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_object", "无效的文件。", nil))
		return
	}

	ref, err := a.d.Queries.GetReference(r.Context(), sqlc.GetReferenceParams{ID: rid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	data, err := a.d.OSS.GetObject(r.Context(), key)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	var title, text string
	switch {
	case strings.HasSuffix(key, ".pdf"):
		title, text, err = docextract.PDF(data)
	case strings.HasSuffix(key, ".docx"):
		title, text, err = docextract.DOCX(data)
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_type", "只支持 PDF 或 Word 文档。", nil))
		return
	}
	if err != nil {
		// Extraction failed (corrupt / password-protected / unsupported internals)
		// — 422 so the reading room offers the paste-body fallback.
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "extract_failed",
			Message: "这个文件没能解析出正文——直接把正文粘进来就能逐句共读。",
		})
		return
	}

	blocks := materialize.Segment(text)
	if len(blocks) == 0 {
		// A scanned/image-only PDF has no text layer → nothing extracted.
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "empty_body",
			Message: "没从文件里解析到正文（可能是扫描件）——把正文粘进来就能逐句共读。",
		})
		return
	}

	// Title precedence: the document's own title, else the reference title, else
	// a generic label.
	docTitle := strings.TrimSpace(title)
	if docTitle == "" {
		docTitle = strings.TrimSpace(ref.Title)
	}
	if docTitle == "" {
		docTitle = "上传文档"
	}

	rawBlocks, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	src := key
	mat, err := qtx.CreateProjectMaterial(r.Context(), sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{},
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Kind:      "article",
		Source:    "uploaded",
		Title:     docTitle,
		SourceUrl: &src,
		Blocks:    rawBlocks,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateSourceLogEntry(r.Context(), sqlc.CreateSourceLogEntryParams{
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
		Url:        src,
		Title:      docTitle,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.SetReferenceMaterial(r.Context(), sqlc.SetReferenceMaterialParams{
		ID:         ref.ID,
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"materialId": mat.ID.String(), "title": docTitle})
}
