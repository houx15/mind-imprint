package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteweekly"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// lite_class_summary.go — D2 · 一句班级摘要，显示在教师班级列表每张班级卡的上方
// (§12.5)。一个 POST，因为它调模型：本仓库的约定是 GET 绝不调模型（见
// lite_teacher_routes.go 里 weekly 那一段的注释）。
//
// 输入是 loadLiteClassWeek / liteClassWeekStats / liteClassWeekCards 算出的
// 「当前这个进行中的北京周」—— 和卡片（LearningSnapshot 走的 getRoster →
// currentLiteWeek）用的同一周，§12.5「和卡片用同一批数据，不另取」；不是 GET
// …/weekly 默认给的上一个已完整周。不新增查询：这三个函数本来就不假设 weekStart
// 落在过去，作业状态、停滞项这些字段本就按「到 weekEnd 为止」算，一个还没走到的
// weekEnd 读到的就是「到现在为止」。
//
// §6 的两条校验在摘要被缓存或返回之前跑：姓名按花名册校验（只有出现在 praise/
// watch 名单里的学生才算有依据），人数按「数字+人/位/名」的形状校验（依据是
// 喂给模型的统计数字，不含百分比——见 liteClassSummaryFacts 的注释）。校验不过重试
// 一次（同一轮对话里追加一句「上一次未通过校验」），仍不过才是 502「摘要生成
// 失败：{原因}」，不缓存，也绝不用一句兜底话代替。
//
// 缓存是进程内的，键是「班级 + 北京日期 + 花名册指纹」；§10 不新增表，重启后
// 缓存清空，下一次请求重算，这是可以接受的代价。

// liteClassSummaryPurpose is the llm_call purpose for every model call this
// route makes.
const liteClassSummaryPurpose = "lite_class_summary"

// liteClassSummaryCacheMax bounds the process-wide cache: once it holds this
// many entries, the oldest is evicted to make room for a new one.
const liteClassSummaryCacheMax = 512

// errLiteClassSummary is the visible failure of one summary attempt. 动词+失败
// plus the real cause, never a plausible sentence standing in for a summary
// that was never produced (AGENTS.md rule: no fallback sentence, ever).
func errLiteClassSummary(reason string) *httpx.APIError {
	return &httpx.APIError{
		Status: http.StatusBadGateway, Code: "class_summary_failed", Message: "摘要生成失败：" + reason,
	}
}

// liteClassSummaryDTO is the response shape.
type liteClassSummaryDTO struct {
	Summary     string `json:"summary"`
	GeneratedAt string `json:"generatedAt"`
	Cached      bool   `json:"cached"`
}

// postLiteClassSummary handles POST /api/v1/lite/teacher/classes/{id}/summary.
func (a *API) postLiteClassSummary(w http.ResponseWriter, r *http.Request) {
	cls, ok := a.authTeacherClass(w, r)
	if !ok {
		return
	}
	u, ok := requireTeacherEntitled(w, r)
	if !ok {
		return
	}
	if a.d.Provider == nil {
		httpx.WriteError(w, r, errLiteClassSummary("未配置模型通道"))
		return
	}

	ctx := r.Context()
	roster, err := a.liteWorkspaceRoster(ctx, cls.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	now := time.Now()
	key := liteClassSummaryCacheKey(cls.ID, now, roster)

	entry, cached, cerr := liteClassSummaryGlobalStore.resolve(key, func() (liteClassSummaryEntry, error) {
		return a.composeLiteClassSummary(r, u.ID, cls, roster)
	})
	if cerr != nil {
		httpx.WriteError(w, r, cerr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, liteClassSummaryDTO{
		Summary: entry.Summary, GeneratedAt: entry.GeneratedAt.Format(time.RFC3339), Cached: cached,
	})
}

// liteClassSummarySystemPrompt asks for plain 说明文, one to three sentences,
// no invented facts. AGENTS.md 界面文案 rule 10: no metaphor, no 抒情副词, no
// exclamation marks outside a real milestone (this is neither).
const liteClassSummarySystemPrompt = `你在给老师写一句摘要，显示在班级列表的班级卡片上方，帮老师判断这周要不要点进去看这个班。
只使用下面给出的事实，不补充事实；不使用给出事实里没有的数字；不写给出的学生名单之外的姓名。
写一到三句话，说这个班这周（进行中）最值得老师注意的事：整体参与情况，或者哪些学生值得表扬、哪些需要关注。
说明文，不用比喻，不用感叹号，不写标题，不写称呼，只输出摘要正文本身。
` + liteworkspace.PronounRule

// liteClassSummaryMaxAttempts is the first call plus one retry — mirrors
// agent.composeLiteWeekly's retry budget. Only a §6 grounding failure spends
// the retry; a transport error or an empty reply returns immediately, because
// retrying a dead provider call buys nothing and would still be billed for
// the same outage.
const liteClassSummaryMaxAttempts = 2

// composeLiteClassSummary runs at most liteClassSummaryMaxAttempts model
// calls and validates each against §6 before returning. roster is the CURRENT
// class roster (for the name check's closed set); the facts the model reads
// are THIS, in-progress week's — the same week the card above it (§12.5)
// already shows.
//
// Everything here, the week load included, runs on the detached model
// context. This function is the single-flight computation: other requests
// for the same key wait on its result, so it must not fail because the
// first requester navigated away.
func (a *API) composeLiteClassSummary(r *http.Request, userID uuid.UUID, cls sqlc.Class, roster []liteworkspace.Student) (liteClassSummaryEntry, error) {
	mctx, cancel := detachedModelCtx(r)
	defer cancel()

	ws := liteweek.WeekStart(time.Now())
	students, err := a.loadLiteClassWeek(mctx, cls.ID, ws)
	if err != nil {
		return liteClassSummaryEntry{}, err
	}
	stats := liteClassWeekStats(students)
	_, praise, watch := liteClassWeekCards(students)
	prompt, names, counts := liteClassSummaryFacts(cls, liteweek.Label(ws), stats, praise, watch, roster)
	rosterNames := liteWorkspaceRosterNames(roster)

	resolved, rerr := a.routeE(mctx, gateway.ClassDigest)
	if rerr != nil {
		return liteClassSummaryEntry{}, errLiteClassSummary(rerr.Error())
	}

	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: liteClassSummarySystemPrompt},
		{Role: gateway.RoleUser, Content: prompt},
	}

	var groundErr error
	for attempt := 0; attempt < liteClassSummaryMaxAttempts; attempt++ {
		res, cerr := gateway.Collect(mctx, a.d.Provider, resolved, gateway.ChatRequest{Messages: msgs})
		// Metered whether or not the call produced anything usable — every
		// attempt, including a rejected one, was paid for.
		a.recordLiteLLMCall(mctx, userID, uuid.Nil, liteClassSummaryPurpose, resolved, res.Usage)
		if cerr != nil {
			// A transport failure is never retried.
			return liteClassSummaryEntry{}, errLiteClassSummary(cerr.Error())
		}
		summary := strings.TrimSpace(res.Text)
		if summary == "" {
			return liteClassSummaryEntry{}, errLiteClassSummary("模型没有返回内容")
		}

		var reason string
		if bad := liteworkspace.UngroundedNames(summary, rosterNames, names); len(bad) > 0 {
			reason = "摘要里出现了本轮没有依据的学生姓名：" + strings.Join(bad, "、")
		} else if bad := liteworkspace.UngroundedCounts(summary, counts); len(bad) > 0 {
			reason = "摘要里出现了本轮没有依据的人数：" + liteWorkspaceJoinInts(bad)
		}
		if reason == "" {
			return liteClassSummaryEntry{Summary: summary, GeneratedAt: time.Now()}, nil
		}

		// No student name reaches the log: the reason and the summary both
		// name students, and every name the check reports is a roster name.
		slog.Warn("lite class summary: summary failed the grounding check",
			"request_id", httpx.RequestIDFromContext(r.Context()),
			"attempt", attempt+1,
			"reason", liteworkspace.RedactNames(reason, rosterNames),
			"summary", liteworkspace.RedactNames(summary, rosterNames))
		groundErr = errLiteClassSummary(reason)
		if attempt == liteClassSummaryMaxAttempts-1 {
			break
		}
		msgs = append(msgs,
			gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text},
			gateway.ChatMessage{Role: gateway.RoleUser, Content: "上一次输出未通过校验：" + reason + "。请重新输出，只用给出的事实，不要写没有依据的姓名或人数。"},
		)
	}
	return liteClassSummaryEntry{}, groundErr
}

// liteClassSummaryFacts builds the user turn AND, in the same pass, the two
// §6 evidence lists: every student name the facts name (praise + watch) and
// every integer the facts state (so the reply may repeat them back). Building
// all three together is deliberate — a fact added to the prompt without also
// being added to one of these lists would silently fail every summary that
// mentions it.
func liteClassSummaryFacts(cls sqlc.Class, weekLabel string, stats liteweekly.ClassWeekStats, praise, watch []liteClassWeekCardDTO, roster []liteworkspace.Student) (prompt string, names []string, counts []int) {
	var b strings.Builder
	fmt.Fprintf(&b, "班级：%s\n", cls.Name)
	fmt.Fprintf(&b, "周：%s\n", weekLabel)
	fmt.Fprintf(&b, "班级人数：%d\n", stats.ClassSize)
	counts = append(counts, stats.ClassSize)
	fmt.Fprintf(&b, "本周活跃学生数：%d\n", stats.ActiveStudents)
	counts = append(counts, stats.ActiveStudents)
	// Measured 2026-09-17: the summary called these 「完成4项作业」 while one of
	// the four was a reading the student started herself.
	fmt.Fprintf(&b, "完成项数：%d（本周完成的阅读、写作、项目，含学生自己开始的，不全是作业）\n", stats.Finished)
	counts = append(counts, stats.Finished)
	// AssignmentRate is NOT added to counts: it is a percentage, not a head
	// count, and a bare number sitting in the grounded set laundered any reply
	// that happened to restate it followed by 人/位/名 — measured: grounded
	// [30,10,5,42] let 「本周有 42 位学生完成了写作练习」 through, where 42 was
	// the RATE, not a student count. Nothing legitimate is lost: 「N%」 never
	// matches UngroundedCounts's number-plus-person-counter shape, so the rate
	// was never valid evidence for a head count in the first place.
	if stats.AssignmentRate >= 0 {
		fmt.Fprintf(&b, "作业按时完成率：%d%%\n", stats.AssignmentRate)
	} else {
		b.WriteString("作业按时完成率：本周无到期作业\n")
	}

	b.WriteString("值得表扬：")
	if len(praise) == 0 {
		b.WriteString("无")
	}
	for _, c := range praise {
		fmt.Fprintf(&b, "\n- %s（称谓：%s）：%s", c.Name, liteWorkspacePronounOf(roster, c.UserID), c.Evidence)
		names = append(names, c.Name)
	}
	b.WriteString("\n需要关注：")
	if len(watch) == 0 {
		b.WriteString("无")
	}
	for _, c := range watch {
		fmt.Fprintf(&b, "\n- %s（称谓：%s）：%s", c.Name, liteWorkspacePronounOf(roster, c.UserID), c.Evidence)
		names = append(names, c.Name)
	}
	// Each list states a head count by listing its students: 「这两位学生」
	// about the two names under 需要关注 is a fact the model was given.
	// Measured: without these, that sentence failed the summary in 2 of 3 live
	// runs (2026-09-17).
	if len(praise) > 0 {
		counts = append(counts, len(praise))
	}
	if len(watch) > 0 {
		counts = append(counts, len(watch))
	}
	return b.String(), names, counts
}

// liteClassSummaryEntry is what the cache stores and the handler returns.
type liteClassSummaryEntry struct {
	Summary     string
	GeneratedAt time.Time
}

// liteClassSummaryCacheKey is "class + Beijing date + roster fingerprint". A
// pure function of its arguments — no DB, no clock read inside it — so the
// UTC/Beijing midnight boundary can be tested directly with a fixed `now`.
//
// The Beijing offset comes from liteworkspace.BeijingOffset, a fixed
// time.FixedZone, never time.LoadLocation: the distroless runtime image the
// server ships in carries no tzdata, and LoadLocation fails silently into UTC
// there (see the repo's dev-ops-gotchas memory).
func liteClassSummaryCacheKey(classID uuid.UUID, now time.Time, roster []liteworkspace.Student) string {
	date := now.In(liteworkspace.BeijingOffset).Format("2006-01-02")
	return classID.String() + "|" + date + "|" + liteClassSummaryRosterFingerprint(roster)
}

// liteClassSummaryRosterFingerprint hashes the roster rows' activity fields —
// the three fields the workspace's list_students tool reads
// (ActiveDaysThisWeek, OverdueAssignments, WritingsDone), her other counters
// (Progress: readings, projects, turns; production 2026-09-17 kept a summary
// all evening after two students finished the homework reading) — and each
// student's gender, which the summary's pronouns follow. Any change to any
// student's week changes the hash, which is what "roster change → recompute"
// means: the cache does not know WHAT changed, only that something did.
//
// Sorted by student id before hashing so the fingerprint does not depend on
// the roster query's row order.
func liteClassSummaryRosterFingerprint(roster []liteworkspace.Student) string {
	rows := make([]string, len(roster))
	for i, s := range roster {
		rows[i] = fmt.Sprintf("%s:%d:%d:%d:%s:%s", s.ID, s.ActiveDaysThisWeek, s.OverdueAssignments, s.WritingsDone, s.Gender, s.Progress)
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, ";")))
	return hex.EncodeToString(sum[:])
}

// liteClassSummaryGlobalStore is the process-wide cache every request shares.
// In-process only (§10: no new table) — a restart loses it and the next
// request recomputes, which is the accepted trade-off.
var liteClassSummaryGlobalStore = newLiteClassSummaryStore(liteClassSummaryCacheMax)

// liteClassSummaryStore is a bounded, mutex-guarded, single-flight cache.
// "Single-flight" here means: while a computation for a key is running, any
// other caller for the SAME key waits on that one call instead of starting
// its own — two simultaneous requests for a freshly-invalidated class do not
// pay for two model calls.
type liteClassSummaryStore struct {
	mu       sync.Mutex
	max      int
	order    []string // insertion order, oldest first, for eviction
	data     map[string]liteClassSummaryEntry
	inflight map[string]*liteClassSummaryFlight
	// onJoin 在一个调用**加入**别人正在跑的那一次计算、开始等它之前被调用。
	// 生产里永远是 nil；只给测试用 —— 没有它，测试没法知道等待者真的加入了
	// 那一次计算，而不是在它结束之后自己另起了一次（见
	// TestLiteClassSummaryStorePanicRecovers，2026-09-17 之前它 30 次里挂 23 次）。
	onJoin func()
}

// liteClassSummaryFlight is one computation in progress for a key. Every
// caller that joins it (found it already in s.inflight) waits on done and
// reads the same result the caller who started it got.
type liteClassSummaryFlight struct {
	done  chan struct{}
	entry liteClassSummaryEntry
	err   error
}

func newLiteClassSummaryStore(max int) *liteClassSummaryStore {
	return &liteClassSummaryStore{
		max: max, data: map[string]liteClassSummaryEntry{}, inflight: map[string]*liteClassSummaryFlight{},
	}
}

// resolve returns the entry cached under key, or runs fn to produce one.
//
// cached=true means the entry was already in the cache before this call did
// anything — the "second request, same day, same data: cached, stub not
// called again" case. A caller that instead joins an in-flight computation
// gets cached=false: this request is one of the reasons a model call
// happened, even though it did not start that call itself.
//
// A failed fn is NEVER stored: the next resolve for the same key — whether it
// joined this flight or arrived after it finished — tries again, exactly as
// §6 requires ("the failed result is NOT cached").
func (s *liteClassSummaryStore) resolve(key string, fn func() (liteClassSummaryEntry, error)) (liteClassSummaryEntry, bool, error) {
	s.mu.Lock()
	if e, ok := s.data[key]; ok {
		s.mu.Unlock()
		return e, true, nil
	}
	if f, ok := s.inflight[key]; ok {
		hook := s.onJoin
		s.mu.Unlock()
		if hook != nil {
			hook()
		}
		<-f.done
		return f.entry, false, f.err
	}
	f := &liteClassSummaryFlight{done: make(chan struct{})}
	s.inflight[key] = f
	s.mu.Unlock()

	entry, err := s.runGuarded(fn)
	f.entry, f.err = entry, err
	close(f.done)

	s.mu.Lock()
	delete(s.inflight, key)
	if err == nil {
		s.put(key, entry)
	}
	s.mu.Unlock()
	return entry, false, err
}

// runGuarded runs fn and turns a panic into an error. Without this, a panic
// inside fn (the model client, a nil-pointer bug, anything) would unwind
// straight out of resolve and skip BOTH close(f.done) and
// delete(s.inflight, key) above — every other request for the same class and
// day would then block on <-f.done forever, and the key could never be
// recomputed again until the process restarted. net/http recovers a panic
// per request so the crash itself would stay invisible; the stuck key would
// not.
func (s *liteClassSummaryStore) runGuarded(fn func() (liteClassSummaryEntry, error)) (entry liteClassSummaryEntry, err error) {
	defer func() {
		if p := recover(); p != nil {
			slog.Error("lite class summary: compose panicked", "panic", p)
			err = fmt.Errorf("摘要计算发生内部错误：%v", p)
		}
	}()
	return fn()
}

// put stores entry under key and evicts the oldest entry once the store is
// over its cap. Called with s.mu held.
func (s *liteClassSummaryStore) put(key string, entry liteClassSummaryEntry) {
	if _, exists := s.data[key]; !exists {
		s.order = append(s.order, key)
	}
	s.data[key] = entry
	for len(s.order) > s.max {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.data, oldest)
	}
}
