package api

// lite_create_helpers.go — creating a reading, a writing or a project, without
// an HTTP request.
//
// Two callers share these: the student's own create buttons (createReading,
// startLibraryReading, createWriting, createPblProject) and 开始 on a practice
// her teacher assigned. Both must produce the same rows, so the rows are
// written in one place.
//
// Every helper runs its own transaction. None checks HasEntitlement or the
// homepage gate (siteGateOpen): those are the caller's decisions, and an
// assigned project deliberately skips the gate.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// readingTitle is the name a reading goes by in 我的阅读: trimmed, 未命名阅读
// when blank, cut to 200 runes.
func readingTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "未命名阅读"
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	return title
}

// ---- library readings ------------------------------------------------------

// createLibraryReadingFor 从库里开一篇：建 atom + reading + reading_source，一个事务。
//
// 已经开着同一篇的同一档时，返回那一篇（resumed=true），不再建一个新的。她换一档
// 是另一回事 —— 那是一次真的选择（同一件事换一种写法），值得一条自己的记录。
func (a *API) createLibraryReadingFor(ctx context.Context, userID uuid.UUID, slug string, tier int) (uuid.UUID, bool, error) {
	art, lvl, err := libraryLevel(slug, tier)
	if err != nil {
		return uuid.Nil, false, err
	}
	existing, err := a.d.Queries.ListLibraryReadingsByUser(ctx, userID)
	if err != nil {
		return uuid.Nil, false, err
	}
	for _, row := range existing {
		if row.LibrarySlug == slug && int(row.LibraryTier) == tier && row.Status != "finished" {
			return row.AtomID, true, nil
		}
	}
	id, err := a.insertLibraryReading(ctx, userID, art, lvl, slug, tier)
	return id, false, err
}

// createLibraryReadingFreshFor is createLibraryReadingFor without the dedupe:
// it always opens a new reading, even when an unfinished one of the same
// article and tier exists.
func (a *API) createLibraryReadingFreshFor(ctx context.Context, userID uuid.UUID, slug string, tier int) (uuid.UUID, error) {
	art, lvl, err := libraryLevel(slug, tier)
	if err != nil {
		return uuid.Nil, err
	}
	return a.insertLibraryReading(ctx, userID, art, lvl, slug, tier)
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

func (a *API) insertLibraryReading(ctx context.Context, userID uuid.UUID, art library.Article, lvl library.Level, slug string, tier int) (uuid.UUID, error) {
	figures, err := json.Marshal(lvl.Figures)
	if err != nil {
		return uuid.Nil, err
	}
	headings, err := json.Marshal(lvl.Headings)
	if err != nil {
		return uuid.Nil, err
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

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
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return at.ID, nil
}

// suggestLibraryTierFor is the default tier her shelf opens at, from what she
// has read in the library. See library.SuggestTier.
func (a *API) suggestLibraryTierFor(ctx context.Context, userID uuid.UUID) (int, error) {
	rows, err := a.d.Queries.ListLibraryReadingsByUser(ctx, userID)
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
// atom + reading + reading_source in one transaction.
//
// The article is text when text is non-blank; otherwise srcURL is fetched
// server-side, the same way putReadingSourceLite fetches. A non-blank srcURL is
// stored as the source link either way, so it must be http/https: it is later
// rendered as a link on the teacher's item page, where a `javascript:` URL
// would run in the teacher's session. It is checked before any fetch or write.
//
// A blank title takes the fetched page's title, then the defaults the two
// endpoints use (未命名阅读 for the reading, 未命名文章 for the source).
func (a *API) createReadingWithSourceFor(ctx context.Context, userID uuid.UUID, title, lang, srcURL, text string) (uuid.UUID, error) {
	title = strings.TrimSpace(title)
	body := strings.TrimSpace(text)
	srcURL = strings.TrimSpace(srcURL)

	if srcURL != "" && !isHTTPURL(srcURL) {
		return uuid.Nil, errInvalidSourceURL
	}
	if body == "" && srcURL != "" {
		if a.d.Fetcher == nil {
			return uuid.Nil, errFetchUnavailable
		}
		fetchedTitle, fetched, _, err := a.d.Fetcher.FetchReadable(ctx, srcURL)
		if err != nil {
			return uuid.Nil, fmt.Errorf("%w: %v", errFetchFailed, err)
		}
		body = strings.TrimSpace(fetched)
		if title == "" {
			title = strings.TrimSpace(fetchedTitle)
		}
	}
	if len(SplitBlocks(body)) == 0 {
		return uuid.Nil, errMissingSourceText
	}
	sourceTitle := title
	if sourceTitle == "" {
		sourceTitle = "未命名文章"
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	id, err := createReadingInTx(ctx, qtx, userID, readingTitle(title), liteLang(lang))
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.UpsertReadingSource(ctx, sqlc.UpsertReadingSourceParams{
		AtomID: id, Title: sourceTitle, Body: body, SourceUrl: nullableText(srcURL),
	}); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
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

	id, err := createWritingInTx(ctx, qtx, userID, idea, lang)
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
func createWritingInTx(ctx context.Context, qtx *sqlc.Queries, userID uuid.UUID, idea, lang string) (uuid.UUID, error) {
	idea = strings.TrimSpace(idea)
	if idea == "" {
		return uuid.Nil, errEmptyIdea
	}
	title := idea
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "writing", UserID: userID})
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := qtx.CreateWriting(ctx, sqlc.CreateWritingParams{
		AtomID: at.ID, Title: title, Lang: liteLang(lang),
	}); err != nil {
		return uuid.Nil, err
	}
	// seq=1 literal, not NextAtomMessageSeq: this atom_id was just minted
	// inside this same transaction, so it is unconditionally the first
	// message — no concurrent writer can have raced it.
	if _, err := qtx.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: 1, Role: "student", Content: idea,
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
	idea = strings.TrimSpace(idea)
	if idea == "" {
		return sqlc.Atom{}, sqlc.PblProject{}, errEmptyIdea
	}
	if len([]rune(idea)) > maxPblIdeaRunes {
		idea = string([]rune(idea)[:maxPblIdeaRunes])
	}

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

	// atom + pbl_project in ONE transaction: an atom with no project row is an
	// identity nothing can render, exactly as in createReading.
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "project", UserID: userID})
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	p, err := qtx.CreatePblProject(ctx, sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: idea, Kind: kind,
		// 先给个名字，她随时能改。整句原文顶在页头上会把房间挤没。
		Name: pbl.DefaultProjectName(idea),
	})
	if err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.Atom{}, sqlc.PblProject{}, err
	}
	return at, p, nil
}
