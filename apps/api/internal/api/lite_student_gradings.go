package api

// lite_student_gradings.go — what a student sees of AI 批改: sent gradings of
// her own writing, and marking one seen from the inbox. Only status = 'sent'
// rows are ever read here (the queries filter it), and the DTO has no field
// for ai, error or status.

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

type studentGradingDTO struct {
	ID            string          `json:"id"`
	VersionNumber int32           `json:"versionNumber"`
	Rubric        json.RawMessage `json:"rubric"`
	Content       json.RawMessage `json:"content"`
	SentAt        string          `json:"sentAt"`
	Seen          bool            `json:"seen"`
	// Source is "ai" when the model drafted it, "teacher" for a 人工批改.
	Source string `json:"source"`
}

// InboxGradingDTO is a sent grading in the student's inbox.
type InboxGradingDTO struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	AtomID       string `json:"atomId"`
	WritingTitle string `json:"writingTitle"`
	SentAt       string `json:"sentAt"`
	Unread       bool   `json:"unread"`
}

func inboxGradingItems(rows []sqlc.ListLiteInboxGradingsRow) ([]InboxGradingDTO, int) {
	out := make([]InboxGradingDTO, 0, len(rows))
	unread := 0
	for _, g := range rows {
		if !g.StudentSeenAt.Valid {
			unread++
		}
		out = append(out, InboxGradingDTO{
			Type: "grading", ID: g.ID.String(), AtomID: g.AtomID.String(), WritingTitle: g.Title,
			SentAt: g.SentAt.Time.Format(time.RFC3339), Unread: !g.StudentSeenAt.Valid,
		})
	}
	return out, unread
}

// listWritingGradingsHandler handles GET /api/v1/writings/{id}/gradings.
func (a *API) listWritingGradingsHandler(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListSentLiteGradingsForAtom(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]studentGradingDTO, 0, len(rows))
	for _, g := range rows {
		out = append(out, studentGradingDTO{
			ID: g.ID.String(), VersionNumber: g.VersionNumber,
			Rubric: json.RawMessage(g.Rubric), Content: json.RawMessage(g.Content),
			SentAt: g.SentAt.Time.Format(time.RFC3339), Seen: g.StudentSeenAt.Valid,
			Source: studentGradingSource(g.AiDrafted),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"gradings": out})
}

// markLiteGradingSeen handles POST /api/v1/lite/inbox/gradings/{gid}/seen.
func (a *API) markLiteGradingSeen(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	gid, err := uuid.Parse(r.PathValue("gid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	n, err := a.d.Queries.MarkLiteGradingSeen(r.Context(), sqlc.MarkLiteGradingSeenParams{ID: gid, UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if n == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func studentGradingSource(aiDrafted bool) string {
	if aiDrafted {
		return litegrade.SourceAI
	}
	return litegrade.SourceTeacher
}
