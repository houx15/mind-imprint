package api

import (
	"context"
	"net/http"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/oss"
	"mindimprint/api/internal/store/sqlc"
)

// Fetcher turns a student-supplied URL into readable text. Injected so tests
// can bypass the production SSRF guard (httptest binds loopback, which the
// guard blocks by design). Never used by the agent — only the student's
// explicit POST /projects/{id}/materials reaches it (RL-2, spec §4).
type Fetcher interface {
	FetchReadable(ctx context.Context, rawURL string) (title, text string, meta *materialize.DOIMeta, err error)
	// SearchWorks/RelatedWorks (#A2) back the exploration "深挖" tray: OpenAlex
	// candidates for a keyword or a paper's related works. Best-effort — the
	// real *materialize.HTTPFetcher returns nil on any failure, never an error.
	SearchWorks(ctx context.Context, query string, limit int) []materialize.WorkMeta
	RelatedWorks(ctx context.Context, doi string, limit int) []materialize.WorkMeta
	// ReferencedWorks/CitingWorks back dig's "citation"/"cited" modes — the
	// works a paper cites, and the works that cite it. Same best-effort
	// contract: nil on any failure, never an error.
	ReferencedWorks(ctx context.Context, doi string, limit int) []materialize.WorkMeta
	CitingWorks(ctx context.Context, doi string, limit int) []materialize.WorkMeta
}

// Deps are everything the handlers need, wired once at startup.
type Deps struct {
	Queries      *sqlc.Queries
	Provider     gateway.Provider    // the MuxProvider
	ChatResolver gateway.KeyResolver // chaperone (turn)
	EvalResolver gateway.KeyResolver // flagship (course step render)
	Catalog      []cards.Spec
	SpecByID     func(id string) (cards.Spec, bool)
	Pool         TxBeginner   // for multi-statement transactions (signup)
	CookieSecure bool         // Secure flag on the session cookie
	Voice        VoiceService // TTS/ASR seam; nil disables voice routes (503)
	CORSOrigins  []string     // allowlisted SPA origins, used for WS OriginPatterns
	Fetcher      Fetcher      // URL→readable-text seam for student material ingestion (Slice 6b Task 4)
	OSS          *oss.Service // presigned-URL signer; nil disables /oss/* routes (503)
	OSSAdminKey  string       // static bearer secret authorizing the admin upload routes
}

// API holds the handler dependencies.
type API struct{ d Deps }

// New builds the API handler set.
func New(d Deps) *API { return &API{d: d} }

// Handler returns the /api/v1 mux. Auth routes (except /me) are public; every
// other route requires a resolved session. SessionAuth runs for all requests
// (so signout can read the cookie); RequireUser guards the protected group.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public auth routes.
	mux.HandleFunc("POST /api/v1/auth/signup", a.signup)
	mux.HandleFunc("POST /api/v1/auth/signin", a.signin)
	mux.HandleFunc("POST /api/v1/auth/signout", a.signout)
	mux.HandleFunc("POST /api/v1/auth/verify-email", a.verifyEmail)

	// Protected routes (require a session).
	protected := func(h http.HandlerFunc) http.Handler { return RequireUser(h) }
	mux.Handle("GET /api/v1/auth/me", protected(a.me))
	mux.Handle("GET /api/v1/growth/history", protected(a.getGrowthHistory))
	mux.Handle("GET /api/v1/growth/ability", protected(a.getAbilityModel))
	mux.Handle("GET /api/v1/growth/cards", protected(a.getGrowthCards))
	mux.Handle("GET /api/v1/cards/catalog", protected(a.getCardsCatalog))
	mux.Handle("PUT /api/v1/cards/theme", protected(a.putCardTheme))
	mux.Handle("PUT /api/v1/users/me/accent", protected(a.putUserAccent))
	mux.Handle("GET /api/v1/projects", protected(a.listProjects))
	mux.Handle("POST /api/v1/projects", protected(a.createProject))
	mux.Handle("POST /api/v1/projects/{id}/onboarding", protected(a.submitOnboarding))
	mux.Handle("POST /api/v1/projects/{id}/framing", protected(a.submitFraming))
	mux.Handle("POST /api/v1/projects/{id}/perspectives", protected(a.submitPerspectives))
	mux.Handle("POST /api/v1/projects/{id}/journey/reopen/{code}", protected(a.reopenStation))
	mux.Handle("GET /api/v1/projects/{id}", protected(a.getProject))
	mux.Handle("PUT /api/v1/projects/{id}/proposal", protected(a.putProposal))
	mux.Handle("GET /api/v1/projects/{id}/plan", protected(a.listPlan))
	mux.Handle("POST /api/v1/projects/{id}/plan/generate", protected(a.postPlanGenerate))
	mux.Handle("POST /api/v1/projects/{id}/plan/items", protected(a.createPlanItem))
	mux.Handle("PATCH /api/v1/projects/{id}/plan/items/{iid}", protected(a.patchPlanItem))
	mux.Handle("DELETE /api/v1/projects/{id}/plan/items/{iid}", protected(a.deletePlanItem))
	mux.Handle("GET /api/v1/projects/{id}/log", protected(a.listLog))
	mux.Handle("POST /api/v1/projects/{id}/log", protected(a.postLog))
	mux.Handle("GET /api/v1/projects/{id}/library", protected(a.getLibrary))
	mux.Handle("POST /api/v1/projects/{id}/collections", protected(a.createCollection))
	mux.Handle("PATCH /api/v1/projects/{id}/collections/{cid}", protected(a.patchCollection))
	mux.Handle("DELETE /api/v1/projects/{id}/collections/{cid}", protected(a.deleteCollection))
	mux.Handle("POST /api/v1/projects/{id}/references", protected(a.createReference))
	mux.Handle("PATCH /api/v1/projects/{id}/references/{rid}", protected(a.patchReference))
	mux.Handle("DELETE /api/v1/projects/{id}/references/{rid}", protected(a.deleteReference))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/enter-reading", protected(a.enterReading))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/paste-content", protected(a.pasteContent))
	mux.Handle("PUT /api/v1/projects/{id}/references/{rid}/reading-brief", protected(a.putReadingBrief))
	mux.Handle("GET /api/v1/projects/{id}/references/{rid}/takeaway-draft", protected(a.getTakeawayDraft))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/finalize-reading", protected(a.postFinalizeReading))
	mux.Handle("GET /api/v1/projects/{id}/exploration", protected(a.getExploration))
	mux.Handle("POST /api/v1/projects/{id}/exploration/leads", protected(a.createExplorationLead))
	mux.Handle("PATCH /api/v1/projects/{id}/exploration/leads/{lid}", protected(a.patchExplorationLead))
	mux.Handle("DELETE /api/v1/projects/{id}/exploration/leads/{lid}", protected(a.deleteExplorationLead))
	mux.Handle("POST /api/v1/projects/{id}/exploration/guide", protected(a.postExplorationGuide))
	mux.Handle("POST /api/v1/projects/{id}/exploration/dig", protected(a.digExploration))
	mux.Handle("POST /api/v1/projects/{id}/exploration/adopt", protected(a.adoptExploration))
	mux.Handle("POST /api/v1/projects/{id}/exploration/edges", protected(a.createQuestionEdge))
	mux.Handle("PATCH /api/v1/projects/{id}/exploration/edges/{eid}", protected(a.patchQuestionEdge))
	mux.Handle("DELETE /api/v1/projects/{id}/exploration/edges/{eid}", protected(a.deleteQuestionEdge))
	mux.Handle("POST /api/v1/projects/{id}/exploration/edges/propose", protected(a.proposeQuestionEdges))
	mux.Handle("POST /api/v1/projects/{id}/cards/persist", protected(a.postPersistProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/reflect", protected(a.postReflectProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/dismiss-proposal", protected(a.postDismissProposal))
	mux.Handle("POST /api/v1/projects/{id}/coach", protected(a.postCoach))
	mux.Handle("GET /api/v1/projects/{id}/coach/history", protected(a.getCoachHistory))
	mux.Handle("GET /api/v1/projects/{id}/studio-state", protected(a.getStudioState))
	mux.Handle("POST /api/v1/projects/{id}/materials", protected(a.ingestMaterial))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/open", protected(a.logSourceOpen))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/annotate", protected(a.prepareSourceAnnotation))
	mux.Handle("GET /api/v1/projects/{id}/outline", protected(a.listOutline))
	mux.Handle("PUT /api/v1/projects/{id}/outline", protected(a.putOutline))
	mux.Handle("GET /api/v1/projects/{id}/snippets", protected(a.listSnippets))
	mux.Handle("PUT /api/v1/projects/{id}/snippets", protected(a.putSnippets))
	mux.Handle("GET /api/v1/projects/{id}/draft", protected(a.getDraft))
	mux.Handle("PUT /api/v1/projects/{id}/buffer", protected(a.putEditBuffer))
	mux.Handle("POST /api/v1/projects/{id}/snapshots", protected(a.commitSnapshot))
	mux.Handle("POST /api/v1/projects/{id}/gate/{contractId}/attest", protected(a.attestGate))
	mux.Handle("POST /api/v1/projects/{id}/contracts/{contractId}/spot-check", protected(a.orderSpotCheck))
	mux.Handle("POST /api/v1/projects/{id}/snapshots/{sid}/review", protected(a.orderReview))
	mux.Handle("POST /api/v1/projects/{id}/self-score", protected(a.submitSelfScore))
	mux.Handle("POST /api/v1/projects/{id}/reflection", protected(a.submitReflection))
	mux.Handle("GET /api/v1/projects/{id}/ai-use-draft", protected(a.getAIUseDraft))
	mux.Handle("GET /api/v1/projects/{id}/ai-use", protected(a.getAIUse))
	mux.Handle("POST /api/v1/projects/{id}/ai-use", protected(a.postAIUse))
	mux.Handle("POST /api/v1/projects/{id}/declaration/sign", protected(a.signDeclaration))
	mux.Handle("GET /api/v1/projects/{id}/reflection-doc", protected(a.getReflection))
	mux.Handle("PUT /api/v1/projects/{id}/reflection-doc", protected(a.putReflection))
	mux.Handle("GET /api/v1/projects/{id}/mirror", protected(a.getMirror))
	mux.Handle("POST /api/v1/projects/{id}/mirror", protected(a.postMirror))
	mux.Handle("GET /api/v1/projects/{id}/summary", protected(a.getProjectSummary))
	mux.Handle("POST /api/v1/projects/{id}/summary", protected(a.postProjectSummary))
	mux.Handle("GET /api/v1/projects/{id}/assessment", protected(a.getAssessment))
	mux.Handle("POST /api/v1/projects/{id}/finish-writing", protected(a.finishWriting))
	mux.Handle("POST /api/v1/projects/{id}/reopen-writing", protected(a.reopenWriting))
	mux.Handle("POST /api/v1/projects/{id}/finish", protected(a.finishProject))
	mux.Handle("POST /api/v1/projects/{id}/turn", protected(a.postProjectTurn))
	mux.Handle("POST /api/v1/projects/{id}/interventions/{iid}/disposition", protected(a.postInterventionDisposition))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/activate", protected(a.activateProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/skip", protected(a.skipProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/submit", protected(a.submitProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/read-turn", protected(a.postReadingTurn))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/summon-card", protected(a.summonProjectCard))
	mux.Handle("GET /api/v1/projects/{id}/materials/{mid}/open-card", protected(a.getOpenReadingCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/evaluate", protected(a.evaluateProjectCard))
	mux.Handle("GET /api/v1/courses", protected(a.listCourses))
	mux.Handle("GET /api/v1/courses/{slug}", protected(a.getCourse))
	mux.Handle("GET /api/v1/courses/{slug}/progress", protected(a.getCourseProgress))
	mux.Handle("PUT /api/v1/courses/{slug}/progress", protected(a.putCourseProgress))
	mux.Handle("POST /api/v1/courses/{slug}/quiz-answer", protected(a.postCourseQuizAnswer))
	mux.Handle("POST /api/v1/courses/{slug}/ask", protected(a.postCourseAsk)) // Task 6
	mux.Handle("GET /api/v1/courses/{slug}/report", protected(a.getCourseReport))
	mux.Handle("POST /api/v1/admin/courses", http.HandlerFunc(a.postAdminUploadCourse)) // Task 7 (admin-key gate inside)
	mux.Handle("POST /api/v1/voice/tts", protected(a.postVoiceTTS))
	mux.Handle("GET /api/v1/voice/asr", protected(a.getVoiceASR))
	// OSS storage. Admin upload + resolve are gated by the OSS_ADMIN_KEY bearer
	// (scripts, no session); the user upload is session-gated. SessionAuth has
	// already run for all of them, so resolve can accept a session too.
	mux.Handle("POST /api/v1/oss/admin/upload-url", http.HandlerFunc(a.ossAdminUploadURL))
	mux.Handle("POST /api/v1/oss/upload-url", protected(a.ossUserUploadURL))
	mux.Handle("POST /api/v1/oss/resolve-url", http.HandlerFunc(a.ossResolveURL))
	mux.Handle("GET /api/v1/chat/threads", protected(a.getChatThreads))
	mux.Handle("POST /api/v1/chat/threads", protected(a.createChatThread))
	mux.Handle("GET /api/v1/chat/threads/{id}/messages", protected(a.getChatMessages))
	mux.Handle("POST /api/v1/chat/threads/{id}/turn", protected(a.postChatTurn))
	mux.Handle("POST /api/v1/chat/threads/{id}/cards/{cid}/submit", protected(a.submitChatCard))
	mux.Handle("POST /api/v1/chat/threads/{id}/cards/{cid}/skip", protected(a.skipChatCard))
	mux.Handle("GET /api/v1/chat/threads/{id}/assessment", protected(a.getChatAssessment))
	mux.Handle("POST /api/v1/chat/threads/{id}/assessment", protected(a.generateChatAssessment))

	// Admin-only routes (require a session + admin role).
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/admin/teacher-invites", adminOnly(a.createTeacherInvite))
	mux.Handle("GET /api/v1/admin/teacher-invites", adminOnly(a.listTeacherInvites))
	mux.Handle("POST /api/v1/admin/import", adminOnly(a.adminImport))
	mux.Handle("GET /api/v1/admin/overview", adminOnly(a.adminOverview))
	mux.Handle("GET /api/v1/admin/teachers", adminOnly(a.adminListTeachers))
	mux.Handle("POST /api/v1/classes/{id}/teachers", adminOnly(a.assignClassTeacher))
	mux.Handle("DELETE /api/v1/classes/{id}/teachers/{userId}", adminOnly(a.removeClassTeacher))

	// Teacher-or-admin routes (require a session + teacher or admin role).
	teacherOrAdmin := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("teacher", "admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/classes", teacherOrAdmin(a.createClass))
	mux.Handle("GET /api/v1/classes", teacherOrAdmin(a.listClasses))
	mux.Handle("GET /api/v1/classes/{id}", teacherOrAdmin(a.getClass))
	mux.Handle("PATCH /api/v1/classes/{id}", teacherOrAdmin(a.patchClass))
	mux.Handle("DELETE /api/v1/classes/{id}/enrollments/{userId}", teacherOrAdmin(a.removeEnrollment))
	mux.Handle("GET /api/v1/classes/{id}/roster-report", teacherOrAdmin(a.getClassRosterReport))
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}", teacherOrAdmin(a.getStudentDetail))
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}", teacherOrAdmin(a.getStudentReport))
	mux.Handle("GET /api/v1/classes/{id}/weekly-report", teacherOrAdmin(a.getClassWeeklyReport))
	mux.Handle("POST /api/v1/classes/{id}/weekly-report/prose", teacherOrAdmin(a.postClassWeeklyProse))
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/parent-report/{surface}/{scopeId}", teacherOrAdmin(a.getParentReport))
	mux.Handle("POST /api/v1/classes/{id}/students/{userId}/parent-report/{surface}/{scopeId}/prose", teacherOrAdmin(a.postParentReportProse))
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/parent-stage-report/{weekStart}", teacherOrAdmin(a.getParentStageReport))
	mux.Handle("POST /api/v1/classes/{id}/students/{userId}/parent-stage-report/{weekStart}/prose", teacherOrAdmin(a.postParentStageProse))

	return SessionAuth(a.d.Queries)(mux)
}
