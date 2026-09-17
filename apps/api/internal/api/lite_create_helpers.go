package api

// lite_create_helpers.go — creating a reading, a writing or a project, without
// an HTTP request.
//
// Two callers share these: the student's own create buttons (createReading,
// startLibraryReading, createWriting, createPblProject) and 开始 on a practice
// her teacher assigned. Both must produce the same rows, so the rows are
// written in one place.
//
// Each kind has an *InTx variant that writes through the caller's transaction
// and a *For wrapper that opens its own. 开始 uses the InTx variants: it holds
// row locks on one connection and must not take a second one from the pool
// while it waits (see startLiteAssignment). None of them makes a network call
// except resolveReadingSource, which writes nothing.
//
// None checks HasEntitlement or the homepage gate (siteGateOpen): those are the
// caller's decisions, and an assigned project deliberately skips the gate.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"mindimprint/api/internal/library"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// Sentinel errors. Each handler maps them back to the response it has always
// written; a caller without a handler of its own picks its own wording.
var (
	errLibraryArticleNotFound = errors.New("library article not found")
	errLibraryTierInvalid     = errors.New("library article has no such tier")
	errInvalidSourceURL       = errors.New("source url is not http or https")
	errFetchUnavailable       = errors.New("no fetcher configured")
	errFetchFailed            = errors.New("fetch failed")
	errMissingSourceText      = errors.New("source has no text")
	errEmptyIdea              = errors.New("idea is empty")
)

// liteLang keeps a room's language to the two the rooms support; anything else
// is zh.
func liteLang(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang != "zh" && lang != "en" {
		lang = "zh"
	}
	return lang
}

// guessWritingLang picks the language of a writing from its own text when the
// client did not say.
//
// 🚨 2026-09-18 写作入口走查：从阅读「去写一写」、从兴趣树「去写」、从写作页
// 直接打一道托福题、带一篇英文作文进来 —— 这四条路都不传语言，于是一律落成
// 中文，设定弹窗也就预选「中文」。一个没留意那两颗按钮的学生，英文作文会被
// 按中文字数、中文方法库、中文症状表教一整篇。
// 判据只看字：拉丁字母明显多于汉字就是英文。拿不准的归中文（原来的默认）。
func guessWritingLang(text string) string {
	han, latin := 0, 0
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
		case r < unicode.MaxASCII && unicode.IsLetter(r):
			latin++
		}
	}
	if latin >= 12 && latin > han*4 {
		return "en"
	}
	return "zh"
}

// readingTitle is the name a reading goes by in 我的阅读: trimmed, 未命名阅读
// when blank, cut to 200 runes.
func readingTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "未命名阅读"
	}
	return cutRunes(title, 200)
}

func cutRunes(s string, n int) string {
	if len([]rune(s)) > n {
		return string([]rune(s)[:n])
	}
	return s
}

// ---- library readings ------------------------------------------------------

// createLibraryReadingFor 从库里开一篇：建 atom + reading + reading_source，一个事务。
//
// 已经开着同一篇的同一档时，返回那一篇（resumed=true），不再建一个新的。她换一档
// 是另一回事 —— 那是一次真的选择（同一件事换一种写法），值得一条自己的记录。
func (a *API) createLibraryReadingFor(ctx context.Context, userID uuid.UUID, slug string, tier int) (uuid.UUID, bool, error) {
	// An unknown article or tier is refused before a connection is taken.
	if _, _, err := libraryLevel(slug, tier); err != nil {
		return uuid.Nil, false, err
	}
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, resumed, err := createLibraryReadingInTx(ctx, a.d.Queries.WithTx(tx), userID, slug, tier)
	if err != nil {
		return uuid.Nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, false, err
	}
	return id, resumed, nil
}

// createLibraryReadingInTx is createLibraryReadingFor inside the caller's
// transaction.
func createLibraryReadingInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, slug string, tier int) (uuid.UUID, bool, error) {
	art, lvl, err := libraryLevel(slug, tier)
	if err != nil {
		return uuid.Nil, false, err
	}
	existing, err := qtx.ListLibraryReadingsByUser(ctx, userID)
	if err != nil {
		return uuid.Nil, false, err
	}
	for _, row := range existing {
		if row.LibrarySlug == slug && int(row.LibraryTier) == tier && row.Status != "finished" {
			return row.AtomID, true, nil
		}
	}
	id, err := insertLibraryReadingInTx(ctx, qtx, userID, art, lvl, slug, tier)
	return id, false, err
}

// createLibraryReadingFreshFor is createLibraryReadingFor without the dedupe:
// it always opens a new reading, even when an unfinished one of the same
// article and tier exists.
func (a *API) createLibraryReadingFreshFor(ctx context.Context, userID uuid.UUID, slug string, tier int) (uuid.UUID, error) {
	if _, _, err := libraryLevel(slug, tier); err != nil {
		return uuid.Nil, err
	}
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := createLibraryReadingFreshInTx(ctx, a.d.Queries.WithTx(tx), userID, slug, tier)
	if err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// createLibraryReadingFreshInTx is createLibraryReadingFreshFor inside the
// caller's transaction.
func createLibraryReadingFreshInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, slug string, tier int) (uuid.UUID, error) {
	art, lvl, err := libraryLevel(slug, tier)
	if err != nil {
		return uuid.Nil, err
	}
	return insertLibraryReadingInTx(ctx, qtx, userID, art, lvl, slug, tier)
}

func libraryLevel(slug string, tier int) (library.Article, library.Level, error) {
	art, ok := library.BySlug(slug)
	if !ok {
		return library.Article{}, library.Level{}, errLibraryArticleNotFound
	}
	lvl, ok := art.LevelAt(tier)
	if !ok {
		return library.Article{}, library.Level{}, errLibraryTierInvalid
	}
	return art, lvl, nil
}

// insertLibraryReadingInTx writes atom + reading + reading_source. The caller's
// transaction keeps them together.
func insertLibraryReadingInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, art library.Article, lvl library.Level, slug string, tier int) (uuid.UUID, error) {
	figures, err := json.Marshal(lvl.Figures)
	if err != nil {
		return uuid.Nil, err
	}
	headings, err := json.Marshal(lvl.Headings)
	if err != nil {
		return uuid.Nil, err
	}

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: userID})
	if err != nil {
		return uuid.Nil, err
	}
	// 阅读的名字用中文标题：我的阅读那一列里，二十条英文长标题分不出彼此。
	if _, err := qtx.CreateLibraryReading(ctx, sqlc.CreateLibraryReadingParams{
		AtomID: at.ID, Title: art.ZhTitle, Lang: art.Lang,
		LibrarySlug: slug, LibraryTier: int16(tier),
	}); err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.UpsertLibraryReadingSource(ctx, sqlc.UpsertLibraryReadingSourceParams{
		AtomID: at.ID, Title: lvl.Title, Body: lvl.Body,
		// 库里的文章没有可以打开的原文链接 —— 它们是我们自己排好的版本。
		// 与其给一条打不开的链接，不如不给。
		SourceUrl: nil, Figures: figures, Headings: headings,
	}); err != nil {
		return uuid.Nil, err
	}
	return at.ID, nil
}

// suggestLibraryTierFor is the default tier her shelf opens at, from what she
// has read in the library. See library.SuggestTier.
func (a *API) suggestLibraryTierFor(ctx context.Context, userID uuid.UUID) (int, error) {
	return suggestLibraryTierInTx(ctx, a.d.Queries, userID)
}

// suggestLibraryTierInTx reads through q, which may be the caller's
// transaction.
func suggestLibraryTierInTx(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (int, error) {
	rows, err := q.ListLibraryReadingsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return suggestLibraryTierFromRows(rows), nil
}

// suggestLibraryTierFromRows: the highest tier she finished and the highest
// tier she opened and left unfinished decide the next default.
func suggestLibraryTierFromRows(rows []sqlc.ListLibraryReadingsByUserRow) int {
	finishedTop, abandonedTop := 0, 0
	for _, row := range rows {
		tier := int(row.LibraryTier)
		if row.Status == "finished" {
			if tier > finishedTop {
				finishedTop = tier
			}
		} else if tier > abandonedTop {
			abandonedTop = tier
		}
	}
	return library.SuggestTier(finishedTop, abandonedTop)
}

// ---- readings with a source ------------------------------------------------

// createReadingInTx writes atom + reading. They go in ONE transaction: an atom
// with no reading row would be an identity nothing can render. title and lang
// are stored as given.
func createReadingInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, title, lang string) (uuid.UUID, error) {
	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: userID})
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.CreateReading(ctx, sqlc.CreateReadingParams{
		AtomID: at.ID, Title: title, Lang: lang,
	}); err != nil {
		return uuid.Nil, err
	}
	return at.ID, nil
}

// createReadingWithSourceFor creates a reading that already has its article:
// atom + reading + reading_source in one transaction. The fetch, if any,
// happens before the transaction opens.
func (a *API) createReadingWithSourceFor(ctx context.Context, userID uuid.UUID, title, lang, srcURL, text string) (uuid.UUID, error) {
	title, body, err := a.resolveReadingSource(ctx, title, srcURL, text)
	if err != nil {
		return uuid.Nil, err
	}
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := createReadingWithSourceInTx(ctx, a.d.Queries.WithTx(tx), userID, title, lang, srcURL, body)
	if err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// resolveReadingSource returns the article's title and body. It writes
// nothing, and it is the only helper here that may make a network call.
//
// The article is text when text is non-blank; otherwise srcURL is fetched
// server-side, the same way putReadingSourceLite fetches. A non-blank srcURL is
// stored as the source link either way, so it must be http/https: it is later
// rendered as a link on the teacher's item page, where a `javascript:` URL
// would run in the teacher's session. It is checked before any fetch.
//
// A blank title takes the fetched page's title.
func (a *API) resolveReadingSource(ctx context.Context, title, srcURL, text string) (string, string, error) {
	title = strings.TrimSpace(title)
	body := strings.TrimSpace(text)
	srcURL = strings.TrimSpace(srcURL)

	if srcURL != "" && !isHTTPURL(srcURL) {
		return "", "", errInvalidSourceURL
	}
	if body == "" && srcURL != "" {
		if a.d.Fetcher == nil {
			return "", "", errFetchUnavailable
		}
		fetchedTitle, fetched, _, err := a.d.Fetcher.FetchReadable(ctx, srcURL)
		if err != nil {
			return "", "", fmt.Errorf("%w: %v", errFetchFailed, err)
		}
		body = strings.TrimSpace(fetched)
		if title == "" {
			title = strings.TrimSpace(fetchedTitle)
		}
	}
	if len(SplitBlocks(body)) == 0 {
		return "", "", errMissingSourceText
	}
	return title, body, nil
}

// createReadingWithSourceInTx writes atom + reading + reading_source for an
// article resolveReadingSource already resolved. A blank title takes the
// defaults the two endpoints use (未命名阅读 for the reading, 未命名文章 for
// the source).
func createReadingWithSourceInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, title, lang, srcURL, body string) (uuid.UUID, error) {
	srcURL = strings.TrimSpace(srcURL)
	sourceTitle := strings.TrimSpace(title)
	if sourceTitle == "" {
		sourceTitle = "未命名文章"
	}
	id, err := createReadingInTx(ctx, qtx, userID, readingTitle(title), liteLang(lang))
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.UpsertReadingSource(ctx, sqlc.UpsertReadingSourceParams{
		AtomID: id, Title: sourceTitle, Body: body, SourceUrl: nullableText(srcURL),
	}); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ---- writings --------------------------------------------------------------

// createWritingFor creates a writing from the one sentence she starts with:
// atom + writing + that sentence as atom_message seq 1, in one transaction.
// A non-nil targetWords is set inside the same transaction.
func (a *API) createWritingFor(ctx context.Context, userID uuid.UUID, idea, lang string, targetWords *int32) (uuid.UUID, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	id, err := createWritingInTx(ctx, qtx, userID, idea, lang, true)
	if err != nil {
		return uuid.Nil, err
	}
	if targetWords != nil {
		if err := qtx.SetWritingTargetWords(ctx, sqlc.SetWritingTargetWordsParams{
			AtomID: id, TargetWords: targetWords,
		}); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// createWritingInTx writes atom + writing + the first atom_message inside the
// caller's transaction. createWriting uses it directly so a brought body lands
// in the same transaction.
//
// The idea does double duty: cut to 200 runes it becomes the writing's initial
// title, and verbatim (uncut) it becomes the first atom_message
// (role='student') — because 先聊's first line really is the one she just
// said, and it must not vanish from the transcript.
//
// sayIdea=false skips that message: a brought piece's idea is its title (often
// the file name), which she never said to 印记 — as a student row it showed up
// as her first chat bubble and fed every downstream prompt as her words.
func createWritingInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, idea, lang string, sayIdea bool) (uuid.UUID, error) {
	idea = strings.TrimSpace(idea)
	if idea == "" {
		return uuid.Nil, errEmptyIdea
	}

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: userID})
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.CreateWriting(ctx, sqlc.CreateWritingParams{
		AtomID: at.ID, Title: cutRunes(idea, 200), Lang: liteLang(lang),
	}); err != nil {
		return uuid.Nil, err
	}
	// seq=1 literal, not NextAtomMessageSeq: this atom_id was just minted
	// inside this same transaction, so it is unconditionally the first
	// message — no concurrent writer can have raced it.
	if !sayIdea {
		return at.ID, nil
	}
	if _, err := qtx.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: 1, Role: "student", Content: idea,
	}); err != nil {
		return uuid.Nil, err
	}
	return at.ID, nil
}

// createAssignedWritingInTx creates the writing her teacher assigned. The
// title is the assignment's title and the prompt is stored as
// writing.assigned_prompt. No atom_message is written: the prompt is the
// teacher's text, and a transcript row would put it in her mouth.
func createAssignedWritingInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, title, prompt, lang string, targetWords int32) (uuid.UUID, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return uuid.Nil, errEmptyIdea
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = prompt
	}

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: userID})
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.CreateWriting(ctx, sqlc.CreateWritingParams{
		AtomID: at.ID, Title: cutRunes(title, 200), Lang: liteLang(lang),
	}); err != nil {
		return uuid.Nil, err
	}
	if err := qtx.SetWritingTargetWords(ctx, sqlc.SetWritingTargetWordsParams{
		AtomID: at.ID, TargetWords: &targetWords,
	}); err != nil {
		return uuid.Nil, err
	}
	if err := qtx.SetWritingAssignedPrompt(ctx, sqlc.SetWritingAssignedPromptParams{
		AtomID: at.ID, AssignedPrompt: &prompt,
	}); err != nil {
		return uuid.Nil, err
	}
	return at.ID, nil
}

// ---- projects --------------------------------------------------------------

// createPblProjectFor creates a project from her opening sentence. It does not
// check the homepage gate; createPblProject does, an assigned project does not.
func (a *API) createPblProjectFor(ctx context.Context, userID uuid.UUID, idea string) (sqlc.PblProject, error) {
	_, p, err := a.createPblProjectWithAtomFor(ctx, userID, idea)
	return p, err
}

// createPblProjectWithAtomFor also returns the atom, whose created_at the
// handler's response carries.
func (a *API) createPblProjectWithAtomFor(ctx context.Context, userID uuid.UUID, idea string) (sqlc.Atom, sqlc.PblProject, error) {
	if strings.TrimSpace(idea) == "" {
		return sqlc.Atom{}, sqlc.PblProject{}, errEmptyIdea
	}
	// atom + pbl_project in ONE transaction: an atom with no project row is an
	// identity nothing can render, exactly as in createReading.
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	at, p, err := createPblProjectInTx(ctx, a.d.Queries.WithTx(tx), userID, idea, "")
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	return at, p, nil
}

// createPblProjectInTx writes atom + pbl_project inside the caller's
// transaction. A blank name takes pbl.DefaultProjectName(idea).
func createPblProjectInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, idea, name string) (sqlc.Atom, sqlc.PblProject, error) {
	idea = strings.TrimSpace(idea)
	if idea == "" {
		return sqlc.Atom{}, sqlc.PblProject{}, errEmptyIdea
	}
	idea = cutRunes(idea, maxPblIdeaRunes)

	// 🚨 建项目不再判类别，也不再调模型。
	//
	// 产品负责人 2026-09-02：「neither should we decide the category of a project
	// then.」——她刚写下一句话，自己都还没想清楚要做什么；机器先替她归好类，
	// 是把一个还没有答案的问题伪造成有答案。类别默认空着（迁移 0112），等她
	// 自己定。
	//
	// 顺带修掉一个真 bug：原来这里的分类调用 MaxTokens=200，推理模型光是想事情
	// 就超了，于是她建项目时经常直接撞上一句"接口错误"。现在这条路径一次模型
	// 调用都没有，建项目不可能因为模型而失败。
	kind := ""

	name = strings.TrimSpace(name)
	if name == "" {
		// 先给个名字，她随时能改。整句原文顶在页头上会把房间挤没。
		name = pbl.DefaultProjectName(idea)
	}

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "project", UserID: userID})
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	p, err := qtx.CreatePblProject(ctx, sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: idea, Kind: kind, Name: name,
	})
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	return at, p, nil
}

// createAssignedPblProjectInTx creates the project her teacher assigned: name
// is the assignment's title, idea is the driving question, and assigned marks
// idea as the teacher's text. brief is the teacher's 补充说明, NULL when blank.
func createAssignedPblProjectInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, title, drivingQuestion, brief string) (uuid.UUID, error) {
	_, p, err := createPblProjectInTx(ctx, qtx, userID, drivingQuestion, cutRunes(strings.TrimSpace(title), maxPblNameRunes))
	if err != nil {
		return uuid.Nil, err
	}
	if err := qtx.MarkPblProjectAssigned(ctx, sqlc.MarkPblProjectAssignedParams{
		AtomID: p.AtomID, AssignedBrief: nullableText(strings.TrimSpace(brief)),
	}); err != nil {
		return uuid.Nil, err
	}
	return p.AtomID, nil
}
