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
	Queries  *sqlc.Queries
	Provider gateway.Provider // the MuxProvider
	// Route returns the resolver for a capability class — how much INTELLIGENCE
	// this call needs, not which feature made it. See gateway.Class* and
	// docs/superpowers/specs/2026-09-02-llm-routing-taxonomy-design.md.
	//
	// This is the seam call sites should use. The three resolvers below are the
	// pre-class lanes, kept while call sites migrate class by class; each is an
	// alias for one class (chat→dialogue, fastChat→reflex, eval→assess).
	Route            func(class string) gateway.KeyResolver
	ChatResolver     gateway.KeyResolver // legacy alias → dialogue
	FastChatResolver gateway.KeyResolver // legacy alias → reflex
	EvalResolver     gateway.KeyResolver // legacy alias → assess
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
	mux.Handle("POST /api/v1/readings/{id}/finish", liteOnly(a.finishReading))
	mux.Handle("PUT /api/v1/readings/{id}/source", liteOnly(a.putReadingSourceLite))
	mux.Handle("GET /api/v1/readings/{id}/source", liteOnly(a.getReadingSourceLite))
	mux.Handle("POST /api/v1/readings/{id}/source/file", liteOnly(a.postReadingSourceFileLite))
	mux.Handle("GET /api/v1/readings/{id}/brief", liteOnly(a.liteGetBrief))
	mux.Handle("PUT /api/v1/readings/{id}/brief", liteOnly(a.litePutBrief))
	mux.Handle("GET /api/v1/readings/{id}/takeaway", liteOnly(a.liteGetTakeaway))
	mux.Handle("PUT /api/v1/readings/{id}/takeaway", liteOnly(a.litePutTakeaway))
	mux.Handle("GET /api/v1/readings/{id}/annotations", liteOnly(a.liteListAnnotationsFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/annotations", liteOnly(a.liteCreateAnnotationFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/turn", liteOnly(a.postLiteReadingTurn))
	mux.Handle("GET /api/v1/readings/{id}/messages", liteOnly(a.liteListMessagesFor("reading")))
	mux.Handle("GET /api/v1/readings/{id}/cards", liteOnly(a.liteListCardsFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/activate", liteOnly(a.liteActivateCardFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/skip", liteOnly(a.liteSkipCardFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/submit", liteOnly(a.liteSubmitCardFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/cards/{cid}/evaluate", liteOnly(a.liteEvaluateCardSelectionFor("reading")))
	mux.Handle("POST /api/v1/readings/{id}/summon", liteOnly(a.liteSummonCard))
	// 任务清单 + 段落工具（0101）：先能读懂一段，才谈得上用透镜读一篇。
	mux.Handle("GET /api/v1/readings/{id}/plan", liteOnly(a.getReadingPlan))
	mux.Handle("POST /api/v1/readings/{id}/plan", liteOnly(a.generateReadingPlan))
	mux.Handle("POST /api/v1/readings/{id}/coach", liteOnly(a.postReadingCoachTurn))
	mux.Handle("POST /api/v1/readings/{id}/plan/tasks/{tid}", liteOnly(a.setReadingTaskStatus))
	mux.Handle("GET /api/v1/readings/{id}/blocks/tools", liteOnly(a.listReadingBlockTools))
	mux.Handle("GET /api/v1/readings/{id}/blocks/notes", liteOnly(a.listReadingBlockNotes))
	mux.Handle("POST /api/v1/readings/{id}/blocks/{bid}/explain", liteOnly(a.explainReadingBlock))
	mux.Handle("GET /api/v1/readings/{id}/questions", liteOnly(a.getReadingQuestions))
	mux.Handle("POST /api/v1/readings/{id}/heartbeat", liteOnly(a.readingHeartbeat))
	mux.Handle("GET /api/v1/readings/{id}/report", liteOnly(a.getAtomReportFor("reading")))

	// 兴趣模型（0116）：她的关键词树。词由阅读/写作/项目完成时自动采集，
	// 学科由 internal/interest 的三档路由连上，这里只负责读出来。
	mux.Handle("GET /api/v1/interest/tree", liteOnly(a.getInterestTree))
	// 觉醒协议 —— 冷启动的兴趣测试。开与交是两次请求，所以一次中途退出
	// 也留下痕迹（见 interest_quiz.go 顶部）。
	mux.Handle("GET /api/v1/interest/quiz", liteOnly(a.getInterestQuizStatus))
	mux.Handle("POST /api/v1/interest/quiz", liteOnly(a.startInterestQuiz))
	mux.Handle("PUT /api/v1/interest/quiz/{id}", liteOnly(a.finishInterestQuiz))
	mux.Handle("POST /api/v1/readings/{id}/report/share", liteOnly(a.shareReadingReport()))
	mux.Handle("DELETE /api/v1/readings/{id}/report/share", liteOnly(a.revokeReadingReport()))
	mux.Handle("PUT /api/v1/readings/{id}/rating", liteOnly(a.putReadingRating()))

	// 轻量版（lite edition）· PBL 项目原子。{id} 一律是 atom id。
	//
	// 🚨 路径带 /pbl/ 前缀，不是 /api/v1/projects——那条是 pro 的（见上方
	// proOnly 那一段），而且 edition_test.go 有一条测试明确要求轻量版访问它
	// 得到 404。两边同名会让其中一边静默失效。
	mux.Handle("POST /api/v1/pbl/projects", liteOnly(a.createPblProject))
	mux.Handle("GET /api/v1/pbl/projects", liteOnly(a.listPblProjects))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}", liteOnly(a.patchPblProject))
	// 深挖 / 思考模式。{sid} 是 session id。
	mux.Handle("POST /api/v1/pbl/projects/{id}/sessions", liteOnly(a.openPblSession))
	mux.Handle("GET /api/v1/pbl/projects/{id}/sessions", liteOnly(a.listPblSessions))
	mux.Handle("POST /api/v1/pbl/projects/{id}/sessions/{sid}/close", liteOnly(a.closePblSession))
	mux.Handle("GET /api/v1/pbl/projects/{id}/thread", liteOnly(a.getPblThread))
	mux.Handle("POST /api/v1/pbl/projects/{id}/turn", liteOnly(a.postPblTurn))
	// 活的任务清单 + Plan Check。结构性变更只进 changes，进不了 plan。
	mux.Handle("GET /api/v1/pbl/projects/{id}/plan", liteOnly(a.getPblPlan))
	mux.Handle("POST /api/v1/pbl/projects/{id}/plan", liteOnly(a.proposePblPlan))
	mux.Handle("POST /api/v1/pbl/projects/{id}/plan/approve", liteOnly(a.approvePblPlan))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/plan/steps/{sid}", liteOnly(a.setPblStepStatus))
	mux.Handle("POST /api/v1/pbl/projects/{id}/plan/changes", liteOnly(a.stagePblChange))
	mux.Handle("POST /api/v1/pbl/projects/{id}/plan/changes/{cid}/resolve", liteOnly(a.resolvePblChange))
	// 成果：交出来要说清楚猜了什么、哪里不对；落地要说得出理由。
	mux.Handle("GET /api/v1/pbl/projects/{id}/artifacts", liteOnly(a.listPblArtifacts))
	mux.Handle("POST /api/v1/pbl/projects/{id}/artifacts", liteOnly(a.handOverPblArtifact))
	mux.Handle("POST /api/v1/pbl/projects/{id}/artifacts/{aid}/settle", liteOnly(a.settlePblArtifact))
	// 工具：端点留着，交互延后（spec §13）。
	mux.Handle("GET /api/v1/pbl/projects/{id}/notes", liteOnly(a.listPblNotes))
	mux.Handle("POST /api/v1/pbl/projects/{id}/notes", liteOnly(a.createPblNote))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/notes/{nid}", liteOnly(a.updatePblNote))
	mux.Handle("DELETE /api/v1/pbl/projects/{id}/notes/{nid}", liteOnly(a.archivePblNote))
	mux.Handle("POST /api/v1/pbl/projects/{id}/notes/cluster", liteOnly(a.clusterPblNotes))

	mux.Handle("GET /api/v1/pbl/projects/{id}/lookback", liteOnly(a.getPblLookback))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/lookback/{lid}", liteOnly(a.answerPblLookback))

	mux.Handle("GET /api/v1/pbl/projects/{id}/keep", liteOnly(a.listPblKeepEntries))
	mux.Handle("POST /api/v1/pbl/projects/{id}/keep", liteOnly(a.createPblKeepEntry))
	mux.Handle("POST /api/v1/pbl/projects/{id}/keep/{kid}/session", liteOnly(a.openPblKeepSession))

	mux.Handle("GET /api/v1/pbl/projects/{id}/steps/{sid}/substeps", liteOnly(a.listPblSubsteps))
	mux.Handle("POST /api/v1/pbl/projects/{id}/steps/{sid}/substeps", liteOnly(a.proposePblSubsteps))
	mux.Handle("POST /api/v1/pbl/projects/{id}/substeps/{ssid}/reassign", liteOnly(a.reassignPblSubstep))
	mux.Handle("POST /api/v1/pbl/projects/{id}/substeps/{ssid}/confirm", liteOnly(a.confirmPblSubstep))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/substeps/{ssid}", liteOnly(a.setPblSubstepStatus))

	mux.Handle("GET /api/v1/pbl/projects/{id}/tree", liteOnly(a.getPblTree))
	mux.Handle("POST /api/v1/pbl/projects/{id}/tree", liteOnly(a.createPblTreeNode))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/tree/{nid}", liteOnly(a.updatePblTreeNode))
	mux.Handle("POST /api/v1/pbl/projects/{id}/tree/{nid}/move", liteOnly(a.movePblTreeNode))
	mux.Handle("DELETE /api/v1/pbl/projects/{id}/tree/{nid}", liteOnly(a.deletePblTreeNode))
	mux.Handle("POST /api/v1/pbl/projects/{id}/tree-checks", liteOnly(a.answerPblTreeCheck))

	mux.Handle("GET /api/v1/pbl/projects/{id}/decisions", liteOnly(a.listPblDecisions))
	mux.Handle("POST /api/v1/pbl/projects/{id}/decisions", liteOnly(a.openPblDecision))
	mux.Handle("POST /api/v1/pbl/projects/{id}/decisions/{did}/settle", liteOnly(a.settlePblDecision))

	mux.Handle("GET /api/v1/pbl/projects/{id}/artifacts/{aid}/review", liteOnly(a.getPblReview))
	mux.Handle("POST /api/v1/pbl/projects/{id}/artifacts/{aid}/review", liteOnly(a.createPblReviewPlan))
	mux.Handle("POST /api/v1/pbl/projects/{id}/artifacts/{aid}/ask", liteOnly(a.askPblReviewMark))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/marks/{mid}", liteOnly(a.answerPblReviewMark))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/dimensions/{did}", liteOnly(a.answerPblReviewDimension))

	mux.Handle("GET /api/v1/pbl/projects/{id}/reframes", liteOnly(a.listPblReframes))
	mux.Handle("POST /api/v1/pbl/projects/{id}/reframes", liteOnly(a.createPblReframe))
	mux.Handle("PATCH /api/v1/pbl/projects/{id}/reframes/{rid}", liteOnly(a.updatePblReframe))
	mux.Handle("POST /api/v1/pbl/projects/{id}/reframes/{rid}/confirm", liteOnly(a.confirmPblReframe))

	mux.Handle("GET /api/v1/pbl/projects/{id}/tools", liteOnly(a.listPblTools))
	mux.Handle("POST /api/v1/pbl/projects/{id}/tools", liteOnly(a.summonPblTool))
	mux.Handle("POST /api/v1/pbl/projects/{id}/tools/{tid}/accept", liteOnly(a.acceptPblTool))
	mux.Handle("POST /api/v1/pbl/projects/{id}/tools/{tid}/resolve", liteOnly(a.resolvePblTool))

	// 轻量版（lite edition）· 写作原子。{id} 一律是 atom id。
	mux.Handle("GET /api/v1/writings", liteOnly(a.listWritings))
	mux.Handle("POST /api/v1/writings", liteOnly(a.createWriting))
	mux.Handle("GET /api/v1/writings/{id}", liteOnly(a.getWriting))
	mux.Handle("PATCH /api/v1/writings/{id}", liteOnly(a.renameWriting))
	mux.Handle("POST /api/v1/writings/{id}/title-ideas", liteOnly(a.suggestWritingTitles))
	mux.Handle("POST /api/v1/writings/{id}/stage", liteOnly(a.setWritingStage))
	mux.Handle("PUT /api/v1/writings/{id}/target-words", liteOnly(a.setWritingTargetWords))
	mux.Handle("PUT /api/v1/writings/{id}/setup", liteOnly(a.setWritingSetup))
	mux.Handle("POST /api/v1/writings/{id}/opening", liteOnly(a.postWritingOpening))
	mux.Handle("POST /api/v1/writings/{id}/plan/turn", liteOnly(a.postWritingPlanTurn))
	mux.Handle("POST /api/v1/writings/{id}/turn", liteOnly(a.postLiteWritingTurn))
	mux.Handle("GET /api/v1/writings/{id}/messages", liteOnly(a.liteListMessagesFor("writing")))
	mux.Handle("GET /api/v1/writings/{id}/outline", liteOnly(a.getWritingOutline))
	mux.Handle("PUT /api/v1/writings/{id}/outline", liteOnly(a.putWritingOutline))
	mux.Handle("POST /api/v1/writings/{id}/outline/{oid}/guide", liteOnly(a.guideWritingBlock))
	mux.Handle("POST /api/v1/writings/{id}/guide", liteOnly(a.guideWritingBlocks))
	mux.Handle("POST /api/v1/writings/{id}/outline/{oid}/deepen", liteOnly(a.deepenWritingBlock))
	mux.Handle("GET /api/v1/writings/{id}/outline/{oid}/deepen", liteOnly(a.getWritingBlockThread))
	mux.Handle("GET /api/v1/writings/{id}/snippets", liteOnly(a.getWritingSnippets))
	mux.Handle("PUT /api/v1/writings/{id}/snippets", liteOnly(a.putWritingSnippets))
	mux.Handle("POST /api/v1/writings/{id}/snippets/{sid}/comment", liteOnly(a.commentOnSnippet))
	mux.Handle("GET /api/v1/writings/{id}/comments", liteOnly(a.listWritingComments))
	mux.Handle("POST /api/v1/writings/{id}/compose", liteOnly(a.composeWritingDraft))
	mux.Handle("GET /api/v1/writings/{id}/draft", liteOnly(a.getWritingDraft))
	mux.Handle("PUT /api/v1/writings/{id}/draft", liteOnly(a.putWritingDraft))
	mux.Handle("POST /api/v1/writings/{id}/review", liteOnly(a.reviewWritingDraft))
	mux.Handle("POST /api/v1/writings/{id}/finish", liteOnly(a.finishWritingAtom))
	mux.Handle("POST /api/v1/writings/{id}/heartbeat", liteOnly(a.writingHeartbeat))
	mux.Handle("GET /api/v1/writings/{id}/report", liteOnly(a.getAtomReportFor("writing")))
	mux.Handle("POST /api/v1/writings/{id}/report/share", liteOnly(a.shareWritingReport()))
	mux.Handle("DELETE /api/v1/writings/{id}/report/share", liteOnly(a.revokeWritingReport()))
	// 写作房间没有工具卡（2026-08-27 产品裁定）：pro 的写作面本来也几乎不用它们，
	// 学生要的是段落框和引导问题，不是一摞卡。阅读房间的 学科透镜 保持不变——
	// 那里的卡是拿来对着一篇文章用的，有真实的着力点。

	// PUBLIC — deliberately NOT protected/liteOnly (see atom_report_share.go's
	// file comment). This is the one route in the lite edition meant to be
	// reachable with no session at all: a student who opted in shares this
	// link, and anyone with it — no login — can view her report. Do not wrap
	// this in an auth gate; that would defeat the whole feature.
	mux.Handle("GET /api/v1/public/reports/{token}", http.HandlerFunc(a.getPublicReport))

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
