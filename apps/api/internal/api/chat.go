package api

// chat.go — Task 5 (Slice 11): the Chat surface's HTTP endpoints. Chat is
// project-free (agent/chat_step.go's RunChatStep runs project-agnostic, one
// thread at a time) — every handler here is keyed by {id} = thread id, never
// a project id. Thread list/create and message history are plain JSON;
// postChatTurn is SSE, reusing studioturn.go's studioEmitter/startHeartbeat
// scaffolding verbatim rather than standing up a second emitter type; card
// submit/skip are thin JSON endpoints (no refeed, no graph_effects — Chat's
// keystone card moment is a one-shot CRAAP offer, not the Studio card loop).
import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// getChatThreads returns the caller's chat threads, most recent first
// (ListThreadsByUser's own ORDER BY).
func (a *API) getChatThreads(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListThreadsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]ChatThreadDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, toChatThreadDTO(t))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// createChatThread starts a new standalone thread (title optional/blank).
func (a *API) createChatThread(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	th, err := a.d.Queries.CreateStandaloneThread(r.Context(), sqlc.CreateStandaloneThreadParams{
		UserID: u.ID, Title: body.Title,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toChatThreadDTO(th))
}

// loadOwnedThread resolves {id} and 404s (not 403) unless it belongs to the
// caller — same ownership-hidden-as-not-found convention as
// projects.go's loadOwnedProject, over chat_thread instead of project.
func (a *API) loadOwnedThread(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, false
	}
	u, _ := UserFromContext(r.Context())
	th, err := a.d.Queries.GetThread(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return uuid.UUID{}, false
	}
	if th.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, false
	}
	return id, true
}

// loadOwnedThreadCard scopes {cid} to the owned {id} thread (404-no-leak) —
// mirrors projectcards.go's loadOwnedProjectCard over chat_thread/card_instance
// instead of project/card_instance.
func (a *API) loadOwnedThreadCard(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	cis, err := a.d.Queries.ListCardInstancesByThread(r.Context(), pgtype.UUID{Bytes: threadID, Valid: true})
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, uuid.UUID{}, false
	}
	for _, ci := range cis {
		if ci.ID == cid {
			return threadID, cid, true
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
	return uuid.UUID{}, uuid.UUID{}, false
}

// getChatMessages returns the thread's full message history, oldest first
// (ListMessagesByThread's own ORDER BY).
func (a *API) getChatMessages(w http.ResponseWriter, r *http.Request) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListMessagesByThread(r.Context(), threadID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]ChatMessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toChatMessageDTO(m))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// postChatTurn drives one RunChatStep for a thread and streams the result
// over SSE: an assistant text delta, then (if the moment fires) a fresh card
// offer, then done. Mirrors postProjectTurn (studioturn.go) exactly for the
// entitlement/decode/SSE-commit scaffolding — the only differences are the
// project→thread substitution and RunChatStep replacing RunAgentStep (no
// refeed, no graph_effects — the Chat policy is coach-alone, planner off).
func (a *API) postChatTurn(w http.ResponseWriter, r *http.Request) {
	threadID, ok := a.loadOwnedThread(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — JSON error BEFORE committing to the stream.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body struct {
		UserInput string `json:"user_input"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.UserInput) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
		return
	}

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	// Commit to streaming. After this, errors are SSE error events, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}

	// Heartbeat until the turn returns or the client disconnects.
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() {
		close(stop)
		<-hbDone
	}()

	if _, err := a.d.Queries.CreateChatMessage(r.Context(), sqlc.CreateChatMessageParams{
		ThreadID: threadID, Role: "user", Content: body.UserInput, Modality: "text",
	}); err != nil {
		slog.Error("chat turn: persist student message",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}
	if _, err := a.d.Queries.AppendEvent(r.Context(), sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: u.ID,
		Surface: "chat", Type: "prompt_sent", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("chat turn: append prompt_sent event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	deps := agent.ChatDeps{
		Store:    agent.NewSqlcChatStore(a.d.Queries),
		Provider: a.d.Provider,
		Resolved: resolved,
		UserID:   u.ID,
		ThreadID: threadID,
	}
	res, err := agent.RunChatStep(r.Context(), deps, body.UserInput)
	if err != nil {
		slog.Error("chat turn: RunChatStep",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}

	_ = em.Text(res.Reply)
	if res.Offer != nil {
		_ = em.Card(res.Offer.CardInstanceID.String(), res.Offer.CardID, "", []byte("[]"), res.Offer.MaterialID.String())
	}
	_ = em.Done()
}

// submitChatCard persists a filled thread-scoped card envelope and marks it
// completed. Thin by design (agent-spec §5.4's Chat policy): no refeed, no
// graph_effects/CompleteCard — the chat card moment is a one-shot CRAAP
// offer, not part of the Studio evidence-graph loop.
func (a *API) submitChatCard(w http.ResponseWriter, r *http.Request) {
	threadID, cid, ok := a.loadOwnedThreadCard(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	var body struct {
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
		Anchors     json.RawMessage `json:"anchors"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateFieldValues(body.FieldValues); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateAnchors(body.Anchors); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if _, err := a.d.Queries.SubmitThreadCardInstance(r.Context(), sqlc.SubmitThreadCardInstanceParams{
		ID: cid, ThreadID: pgtype.UUID{Bytes: threadID, Valid: true},
		FieldValues: body.FieldValues, EventTrace: body.EventTrace, Status: "completed",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.AppendEvent(r.Context(), sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: u.ID,
		Surface: "chat", Type: "card_completed", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("chat card submit: append card_completed event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"card_status": "completed"})
}

// skipChatCard records a skipped thread-scoped card — process-is-data (design's
// 「过程即数据」), same posture as projectcards.go's skipProjectCard, minus the
// event_trace body (thin by design — see submitChatCard's doc comment).
func (a *API) skipChatCard(w http.ResponseWriter, r *http.Request) {
	threadID, cid, ok := a.loadOwnedThreadCard(w, r)
	if !ok {
		return
	}
	if _, err := a.d.Queries.SetThreadCardInstanceStatus(r.Context(), sqlc.SetThreadCardInstanceStatusParams{
		ID: cid, ThreadID: pgtype.UUID{Bytes: threadID, Valid: true}, Status: "skipped",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"card_status": "skipped"})
}
