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
	// ResolveDOI turns a DOI into bibliographic metadata (title/author/year/
	// journal/abstract) via Crossref, without fetching full text. Backs the
	// add-source path so pasting a DOI populates the reference. Best-effort:
	// nil on any failure.
	ResolveDOI(ctx context.Context, doi string) *materialize.DOIMeta
}

// Deps are everything the handlers need, wired once at startup.
type Deps struct {
	Queries          *sqlc.Queries
	Provider         gateway.Provider    // the MuxProvider
	ChatResolver     gateway.KeyResolver // chaperone (turn)
	FastChatResolver gateway.KeyResolver // fast chaperone (per-status studio router); falls back to ChatResolver when nil
	EvalResolver     gateway.KeyResolver // flagship (course step render)
	Catalog          []cards.Spec
	SpecByID         func(id string) (cards.Spec, bool)
	Pool             TxBeginner   // for multi-statement transactions (signup)
	CookieSecure     bool         // Secure flag on the session cookie
	Voice            VoiceService // TTS/ASR seam; nil disables voice routes (503)
	CORSOrigins      []string     // allowlisted SPA origins, used for WS OriginPatterns
	Fetcher          Fetcher      // URL→readable-text seam for student material ingestion (Slice 6b Task 4)
	OSS              *oss.Service // presigned-URL signer; nil disables /oss/* routes (503)
	OSSAdminKey      string       // static bearer secret authorizing the admin upload routes
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
	// 站点分流：pro 学校的账号看不见 /readings，lite 学校的账号看不见
	// /projects —— 双向都是 404，不泄漏另一侧的存在。
	proOnly := func(h http.HandlerFunc) http.Handler { return a.requireEdition("pro", http.HandlerFunc(h)) }
	liteOnly := func(h http.HandlerFunc) http.Handler { return a.requireEdition("lite", http.HandlerFunc(h)) }
	mux.Handle("GET /api/v1/auth/me", protected(a.me))
	mux.Handle("GET /api/v1/growth/cards", protected(a.getGrowthCards))
	mux.Handle("GET /api/v1/cards/catalog", protected(a.getCardsCatalog))
	mux.Handle("GET /api/v1/project-covers", protected(a.getProjectCovers))
	mux.Handle("PUT /api/v1/cards/theme", protected(a.putCardTheme))
	mux.Handle("PUT /api/v1/users/me/accent", protected(a.putUserAccent))
	mux.Handle("PUT /api/v1/users/me/background", protected(a.putUserBackground)) // page background colorway
	mux.Handle("PUT /api/v1/users/me/onboarding", protected(a.putUserOnboarding)) // guided-tour first-run flag
	mux.Handle("POST /api/v1/feedback", protected(a.postFeedback))                // nav 反馈 button
	mux.Handle("GET /api/v1/projects", proOnly(a.listProjects))
	mux.Handle("POST /api/v1/projects", proOnly(a.createProject))
	mux.Handle("POST /api/v1/projects/{id}/onboarding", proOnly(a.submitOnboarding))
	mux.Handle("POST /api/v1/projects/{id}/framing", proOnly(a.submitFraming))
	mux.Handle("POST /api/v1/projects/{id}/perspectives", proOnly(a.submitPerspectives))
	mux.Handle("POST /api/v1/projects/{id}/journey/reopen/{code}", proOnly(a.reopenStation))
	mux.Handle("GET /api/v1/projects/{id}", proOnly(a.getProject))
	mux.Handle("PATCH /api/v1/projects/{id}", proOnly(a.renameProject))
	mux.Handle("PUT /api/v1/projects/{id}/proposal", proOnly(a.putProposal))
	mux.Handle("GET /api/v1/projects/{id}/plan", proOnly(a.listPlan))
	mux.Handle("POST /api/v1/projects/{id}/plan/generate", proOnly(a.postPlanGenerate))
	mux.Handle("POST /api/v1/projects/{id}/plan/items", proOnly(a.createPlanItem))
	mux.Handle("PATCH /api/v1/projects/{id}/plan/items/{iid}", proOnly(a.patchPlanItem))
	mux.Handle("DELETE /api/v1/projects/{id}/plan/items/{iid}", proOnly(a.deletePlanItem))
	mux.Handle("GET /api/v1/projects/{id}/log", proOnly(a.listLog))
	mux.Handle("POST /api/v1/projects/{id}/log", proOnly(a.postLog))
	mux.Handle("GET /api/v1/projects/{id}/library", proOnly(a.getLibrary))
	mux.Handle("POST /api/v1/projects/{id}/collections", proOnly(a.createCollection))
	mux.Handle("PATCH /api/v1/projects/{id}/collections/{cid}", proOnly(a.patchCollection))
	mux.Handle("DELETE /api/v1/projects/{id}/collections/{cid}", proOnly(a.deleteCollection))
	mux.Handle("POST /api/v1/projects/{id}/references", proOnly(a.createReference))
	mux.Handle("PATCH /api/v1/projects/{id}/references/{rid}", proOnly(a.patchReference))
	mux.Handle("DELETE /api/v1/projects/{id}/references/{rid}", proOnly(a.deleteReference))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/enter-reading", proOnly(a.enterReading))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/paste-content", proOnly(a.pasteContent))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/ingest-file", proOnly(a.ingestReferenceFile))
	mux.Handle("PUT /api/v1/projects/{id}/references/{rid}/reading-brief", proOnly(a.putReadingBrief))
	mux.Handle("GET /api/v1/projects/{id}/references/{rid}/takeaway-draft", proOnly(a.getTakeawayDraft))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/finalize-reading", proOnly(a.postFinalizeReading))
	mux.Handle("GET /api/v1/projects/{id}/exploration", proOnly(a.getExploration))
	mux.Handle("GET /api/v1/projects/{id}/revision-checkpoints", proOnly(a.listRevisionCheckpoints))
	mux.Handle("POST /api/v1/projects/{id}/revision/card-edit", proOnly(a.postCardRevision))
	mux.Handle("POST /api/v1/projects/{id}/exploration/leads", proOnly(a.createExplorationLead))
	mux.Handle("PATCH /api/v1/projects/{id}/exploration/leads/{lid}", proOnly(a.patchExplorationLead))
	mux.Handle("DELETE /api/v1/projects/{id}/exploration/leads/{lid}", proOnly(a.deleteExplorationLead))
	mux.Handle("POST /api/v1/projects/{id}/exploration/guide", proOnly(a.postExplorationGuide))
	mux.Handle("POST /api/v1/projects/{id}/exploration/dig", proOnly(a.digExploration))
	mux.Handle("POST /api/v1/projects/{id}/exploration/adopt", proOnly(a.adoptExploration))
	mux.Handle("POST /api/v1/projects/{id}/exploration/attach", proOnly(a.attachExploration))
	mux.Handle("POST /api/v1/projects/{id}/exploration/suggest-placement", proOnly(a.postSuggestPlacement))
	mux.Handle("POST /api/v1/projects/{id}/exploration/edges", proOnly(a.createQuestionEdge))
	mux.Handle("PATCH /api/v1/projects/{id}/exploration/edges/{eid}", proOnly(a.patchQuestionEdge))
	mux.Handle("DELETE /api/v1/projects/{id}/exploration/edges/{eid}", proOnly(a.deleteQuestionEdge))
	mux.Handle("POST /api/v1/projects/{id}/exploration/edges/propose", proOnly(a.proposeQuestionEdges))
	mux.Handle("POST /api/v1/projects/{id}/cards/persist", proOnly(a.postPersistProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/reflect", proOnly(a.postReflectProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/dismiss-proposal", proOnly(a.postDismissProposal))
	mux.Handle("POST /api/v1/projects/{id}/coach", proOnly(a.postCoach))
	mux.Handle("POST /api/v1/projects/{id}/coach/opening", proOnly(a.postCoachOpening))
	mux.Handle("POST /api/v1/projects/{id}/coach/start", proOnly(a.postCoachStart))
	mux.Handle("POST /api/v1/projects/{id}/coach/advance", proOnly(a.postCoachAdvance))
	mux.Handle("GET /api/v1/projects/{id}/coach/history", proOnly(a.getCoachHistory))
	mux.Handle("GET /api/v1/projects/{id}/studio-state", proOnly(a.getStudioState))
	mux.Handle("GET /api/v1/projects/{id}/proposal-track", proOnly(a.getProposalTrack))
	mux.Handle("POST /api/v1/projects/{id}/proposal-track/mode", proOnly(a.setProposalMode))
	mux.Handle("POST /api/v1/projects/{id}/proposal-track/start", proOnly(a.startProposalGuide))
	mux.Handle("POST /api/v1/projects/{id}/proposal-track/subquestions", proOnly(a.setProposalSubQuestions))
	mux.Handle("POST /api/v1/projects/{id}/proposal-track/advance", proOnly(a.advanceProposalStep))
	mux.Handle("POST /api/v1/projects/{id}/proposal-track/review", proOnly(a.reviewProposalPart))
	mux.Handle("GET /api/v1/projects/{id}/cards/question-card", proOnly(a.getQuestionCardState))
	mux.Handle("POST /api/v1/projects/{id}/cards/question-card/turn", proOnly(a.postQuestionCardTurn))
	mux.Handle("POST /api/v1/projects/{id}/cards/question-card/commit", proOnly(a.postQuestionCardCommit))
	mux.Handle("POST /api/v1/projects/{id}/framework/waive-counterpoints", proOnly(a.waiveCounterpoints))
	mux.Handle("GET /api/v1/projects/{id}/proposal-annotations", proOnly(a.getProposalAnnotations))
	mux.Handle("POST /api/v1/projects/{id}/proposal-annotations/review", proOnly(a.reviewProposalAnnotations))
	mux.Handle("POST /api/v1/projects/{id}/annotations/open", proOnly(a.postAnnotationOpen))
	mux.Handle("POST /api/v1/projects/{id}/citations", proOnly(a.postCitation))
	mux.Handle("GET /api/v1/projects/{id}/evidence-map", proOnly(a.getEvidenceMap))
	mux.Handle("PATCH /api/v1/projects/{id}/references/{rid}/evidence", proOnly(a.patchReferenceEvidence))
	mux.Handle("PATCH /api/v1/projects/{id}/references/{rid}/triage", proOnly(a.patchReferenceTriage))
	mux.Handle("POST /api/v1/projects/{id}/references/{rid}/archive", proOnly(a.archiveReference))
	mux.Handle("POST /api/v1/projects/{id}/evidence-map/subquestions/{sqId}/review", proOnly(a.reviewSubQuestionSaturation))
	mux.Handle("GET /api/v1/projects/{id}/essay-track", proOnly(a.getEssayTrack))
	mux.Handle("POST /api/v1/projects/{id}/essay-track/advance-stage", proOnly(a.advanceEssayStage))
	mux.Handle("GET /api/v1/projects/{id}/essay-statement", proOnly(a.getEssayStatement))
	mux.Handle("POST /api/v1/projects/{id}/essay-statement/start", proOnly(a.startEssayStatement))
	mux.Handle("POST /api/v1/projects/{id}/essay-statement/advance", proOnly(a.advanceEssayStatement))
	mux.Handle("POST /api/v1/projects/{id}/essay-statement/review", proOnly(a.reviewEssayStatement))
	mux.Handle("POST /api/v1/projects/{id}/essay-statement/revise-claim", proOnly(a.reviseEssayClaim))
	mux.Handle("GET /api/v1/projects/{id}/essay-submission", proOnly(a.getEssaySubmission))
	mux.Handle("POST /api/v1/projects/{id}/essay-submission/start", proOnly(a.startEssaySubmission))
	mux.Handle("POST /api/v1/projects/{id}/essay-submission/advance", proOnly(a.advanceEssaySubmission))
	mux.Handle("GET /api/v1/projects/{id}/resource-needs", proOnly(a.getResourceNeeds))
	mux.Handle("PUT /api/v1/projects/{id}/resource-needs", proOnly(a.putResourceNeeds))
	mux.Handle("POST /api/v1/projects/{id}/search-guidance", proOnly(a.postSearchGuidance))
	mux.Handle("POST /api/v1/projects/{id}/exploration/review", proOnly(a.postExplorationReview))
	mux.Handle("GET /api/v1/projects/{id}/card-tags", proOnly(a.getCardTags))
	mux.Handle("PUT /api/v1/projects/{id}/card-tags", proOnly(a.putCardTag))
	mux.Handle("POST /api/v1/projects/{id}/materials", proOnly(a.ingestMaterial))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/open", proOnly(a.logSourceOpen))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/annotate", proOnly(a.prepareSourceAnnotation))
	mux.Handle("GET /api/v1/projects/{id}/outline", proOnly(a.listOutline))
	mux.Handle("PUT /api/v1/projects/{id}/outline", proOnly(a.putOutline))
	mux.Handle("GET /api/v1/projects/{id}/snippets", proOnly(a.listSnippets))
	mux.Handle("PUT /api/v1/projects/{id}/snippets", proOnly(a.putSnippets))
	mux.Handle("GET /api/v1/projects/{id}/draft", proOnly(a.getDraft))
	mux.Handle("PUT /api/v1/projects/{id}/buffer", proOnly(a.putEditBuffer))
	mux.Handle("POST /api/v1/projects/{id}/snapshots", proOnly(a.commitSnapshot))
	mux.Handle("POST /api/v1/projects/{id}/gate/{contractId}/attest", proOnly(a.attestGate))
	mux.Handle("POST /api/v1/projects/{id}/contracts/{contractId}/spot-check", proOnly(a.orderSpotCheck))
	mux.Handle("POST /api/v1/projects/{id}/snapshots/{sid}/review", proOnly(a.orderReview))
	mux.Handle("GET /api/v1/projects/{id}/annotations", proOnly(a.getAnnotations))
	mux.Handle("POST /api/v1/projects/{id}/self-score", proOnly(a.submitSelfScore))
	mux.Handle("POST /api/v1/projects/{id}/reflection", proOnly(a.submitReflection))
	mux.Handle("GET /api/v1/projects/{id}/ai-use-draft", proOnly(a.getAIUseDraft))
	mux.Handle("GET /api/v1/projects/{id}/ai-use", proOnly(a.getAIUse))
	mux.Handle("POST /api/v1/projects/{id}/ai-use", proOnly(a.postAIUse))
	mux.Handle("POST /api/v1/projects/{id}/declaration/sign", proOnly(a.signDeclaration))
	mux.Handle("GET /api/v1/projects/{id}/reflection-doc", proOnly(a.getReflection))
	mux.Handle("PUT /api/v1/projects/{id}/reflection-doc", proOnly(a.putReflection))
	mux.Handle("GET /api/v1/projects/{id}/summary", proOnly(a.getProjectSummary))
	mux.Handle("POST /api/v1/projects/{id}/summary", proOnly(a.postProjectSummary))
	mux.Handle("GET /api/v1/projects/{id}/evaluation-report", proOnly(a.getEvaluationReport))
	mux.Handle("POST /api/v1/projects/{id}/evaluation-report/generate", proOnly(a.postGenerateEvaluationReport))
	mux.Handle("GET /api/v1/evaluation-reports", protected(a.listEvaluationReports))
	mux.Handle("POST /api/v1/projects/{id}/finish-writing", proOnly(a.finishWriting))
	mux.Handle("POST /api/v1/projects/{id}/reopen-writing", proOnly(a.reopenWriting))
	mux.Handle("POST /api/v1/projects/{id}/finish", proOnly(a.finishProject))
	mux.Handle("POST /api/v1/projects/{id}/turn", proOnly(a.postProjectTurn))
	mux.Handle("POST /api/v1/projects/{id}/interventions/{iid}/disposition", proOnly(a.postInterventionDisposition))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/activate", proOnly(a.activateProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/skip", proOnly(a.skipProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/submit", proOnly(a.submitProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/read-turn", proOnly(a.postReadingTurn))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/summon-card", proOnly(a.summonProjectCard))
	mux.Handle("GET /api/v1/projects/{id}/materials/{mid}/open-card", proOnly(a.getOpenReadingCard))
	mux.Handle("GET /api/v1/projects/{id}/materials/{mid}/source", proOnly(a.getMaterialSource)) // read-only MaterialSource projection (guided-tour P6)
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/evaluate", proOnly(a.evaluateProjectCard))

	// 轻量版（lite edition）· 阅读原子。{id} 一律是 atom id。
	mux.Handle("GET /api/v1/readings", liteOnly(a.listReadings))
	mux.Handle("POST /api/v1/readings", liteOnly(a.createReading))
	mux.Handle("GET /api/v1/readings/{id}", liteOnly(a.getReading))
	mux.Handle("PATCH /api/v1/readings/{id}", liteOnly(a.renameReading))

	mux.Handle("GET /api/v1/courses", protected(a.listCourses))
	mux.Handle("GET /api/v1/courses/{slug}", protected(a.getCourse))
	mux.Handle("GET /api/v1/courses/history", protected(a.getCourseHistory)) // learning history (touched courses)
	mux.Handle("GET /api/v1/courses/{slug}/progress", protected(a.getCourseProgress))
	mux.Handle("PUT /api/v1/courses/{slug}/progress", protected(a.putCourseProgress))
	mux.Handle("POST /api/v1/courses/{slug}/restart", protected(a.postCourseRestart)) // wipe progress → start over
	mux.Handle("POST /api/v1/courses/{slug}/quiz-answer", protected(a.postCourseQuizAnswer))
	mux.Handle("POST /api/v1/courses/{slug}/ask", protected(a.postCourseAsk))              // Task 6
	mux.Handle("POST /api/v1/courses/{slug}/scene", protected(a.sceneForCourse))           // Course Runtime Slice 7
	mux.Handle("GET /api/v1/courses/{slug}/definition", protected(a.getCourseDefinition))  // Course Runtime Slice 8
	mux.Handle("POST /api/v1/courses/{slug}/session", protected(a.postCourseSession))      // Course Runtime Slice 8
	mux.Handle("PUT /api/v1/courses/{slug}/session", protected(a.putCourseSession))        // Course Runtime Slice 8
	mux.Handle("POST /api/v1/courses/{slug}/asset-urls", protected(a.postCourseAssetURLs)) // OSS CDN URL鉴权
	mux.Handle("GET /api/v1/courses/{slug}/report", protected(a.getCourseReport))
	mux.Handle("GET /api/v1/courses/{slug}/report/answers", protected(a.getCourseAnswerReport))                    // per-attempt recorded answers + time
	mux.Handle("POST /api/v1/admin/courses", http.HandlerFunc(a.postAdminUploadCourse))                            // Task 7 (admin-key gate inside)
	mux.Handle("GET /api/v1/admin/courses", http.HandlerFunc(a.listCoursesAdmin))                                  // course generator draft discovery (incl. preview)
	mux.Handle("PUT /api/v1/admin/courses/{slug}/definition", http.HandlerFunc(a.putCourseDefinition))             // course generator create/modify
	mux.Handle("GET /api/v1/admin/courses/{slug}/definition", http.HandlerFunc(a.getCourseDefinitionAdmin))        // course generator draft readback
	mux.Handle("POST /api/v1/admin/courses/{slug}/asset-upload-url", http.HandlerFunc(a.postCourseAssetUploadURL)) // course generator media upload
	mux.Handle("POST /api/v1/admin/courses/{slug}/ship", http.HandlerFunc(a.postCourseShip))                       // publish a preview course
	mux.Handle("POST /api/v1/admin/courses/{slug}/unpublish", http.HandlerFunc(a.postCourseUnpublish))             // 下线: published -> preview (never a delete)
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
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/evaluation-report/{projectId}", teacherOrAdmin(a.getStudentEvaluationReport))
	mux.Handle("GET /api/v1/classes/{id}/weekly-report", teacherOrAdmin(a.getClassWeeklyReport))
	mux.Handle("POST /api/v1/classes/{id}/weekly-report/prose", teacherOrAdmin(a.postClassWeeklyProse))

	return SessionAuth(a.d.Queries)(mux)
}
