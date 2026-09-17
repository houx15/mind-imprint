package api

// reading_block.go — 段落工具：点开一段，把它拆给她看。
//
// 产品的原话：「for lite-level students, they first need to be taught about the
// paragraphs. for english reading materials, they need — after click a
// paragraph, they can click 翻译、关键单词讲解、语法讲解、写作解析; chinese
// paragraph — 成语/修辞运用、案例、结构解析。then we can guide students to
// focus on information/subject lens then.」
//
// 那个 then 是重点：**先能读懂一段，才谈得上用透镜读一篇**。学科透镜这个房间
// 一直有，缺的是它下面那一级台阶。
//
// ## 铁律 检查
//
// 这些工具是**讲解**，这正是它们安全的原因。铁律① 禁的是 AI 替学生写**她自己的**
// 文字。讲解一段**别人已经发表的**文章，是老师干的事；不给，只会让房间更没用，
// 不会让它更诚实。
//
// 真正要守的那条线：每个工具都锚在**文章的** blockId 上，并且这个文件里没有
// 任何一条写入学生自己字段（摘要 / 批注 / takeaway）的路径。是结构上没有，不是
// prompt 里承诺没有。
//
// ## 缓存
//
// 一段的翻译不会变。所以按 (blockId, tool) 存一份重放：第二次点开是瞬时的，
// 也不会重复付钱。这也是它读起来像工具而不像聊天的原因之一。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingBlockContextRunes bounds how much surrounding article rides along.
// A paragraph's JOB ("这一段在整篇里在干什么") cannot be judged from the
// paragraph alone — 结构解析 and 写作解析 are both about position — so the
// neighbours travel with it, bounded.
const readingBlockContextRunes = 1200

// readingShapedSuffix is appended for the two tools whose output is a
// STRUCTURE rather than prose. The structure IS the guarantee: a sample
// paragraph has no field to live in, so 铁律① is enforced by the schema rather
// than by asking the model to restrain itself.
var readingShapedSuffix = map[string]string{
	"questions": "\n\n只输出一个 JSON 对象：{\"questions\":[\"...\",\"...\"]}。每条都必须以问号结尾。不要输出对象以外的任何文字或代码块标记。",
	"imitate":   "\n\n只输出一个 JSON 对象：{\"move\":\"这一段在写法上做了什么，一句话\",\"tryThis\":[\"一个可以用同样写法去写的话题\",\"另一个\"]}。\n\n**绝对不要写出任何一段示范文字。** 你只说写法和话题，段落由她自己写。tryThis 里每一条是一个话题或情境，不是一句范文。不要输出对象以外的任何文字或代码块标记。",
	"words": "\n\n只输出一个 JSON 对象：" +
		`{"words":[{"term":"","pos":"","meaning":"","note":"","example":"","exampleZh":""}]}` + "\n\n" +
		"- term：这个词**在这一段里的原样**，一个字母都不许改 —— 不要还原成原形、不要改大小写、" +
		"不要把词组拆开。**系统会拿它回段落里逐字核对，对不上的整张卡片丢掉。**\n" +
		"- pos：词性，用中文（名词 / 动词 / 形容词 / 副词 / 介词短语 / 动词短语 …）。\n" +
		"- meaning：它**在这一句里**的意思，一句中文，不超过 20 字。不是词典里的第一条释义。\n" +
		"- note：为什么这个词值得学 —— 它的词根、它和近义词的差别、它常和哪些词搭配、" +
		"或者它在这里的用法特别在哪。两句以内。\n" +
		"- example：一个**新造的**英文例句，用上这个词，不要抄原文那一句。一行，不超过 20 个词。\n" +
		"- exampleZh：上面那句的中文翻译。\n" +
		"不要输出对象以外的任何文字或代码块标记。",
}

// readingWord 是一张词卡。
//
// 🚨 Term 是**这一段里的原样**，不是词典形。荧光笔就是拿它回正文里找的：
// 还原成原形的 term（scrambling → scramble）在正文里一个字都找不到，那个词
// 就标不出来。所以 parseWordCards 拿 term 回段落里逐字核对，核不上的丢掉 ——
// 这条判据同时守着两件事：卡片说的是这一段里真有的词，以及荧光笔一定落得下去。
type readingWord struct {
	Term      string `json:"term"`
	Pos       string `json:"pos"`
	Meaning   string `json:"meaning"`
	Note      string `json:"note"`
	Example   string `json:"example"`
	ExampleZh string `json:"exampleZh"`
}

// readingWordsMax 是一段最多留几张词卡。
//
// 五张：prompt 要的是 3–5 个，而「真正值得学的」本来就没那么多。多出来的那些
// 十有八九是模型在凑数（最长的那几个词），留着只会让她把注意力花在词典干的事上。
const readingWordsMax = 5

// parseWordCards 读关键单词那份回话，并丢掉一切核对不上的。
//
// 丢弃规则：
//
//  1. term 为空 → 丢。
//  2. **term 在这一段里找不到 → 丢。** 大小写不敏感地找（模型很爱把句首那个词
//     还原成小写），但找到之后用的是**段落里的那一份写法**，因为荧光笔要按它
//     去标。
//  3. meaning 为空 → 丢。一张只有词没有意思的卡片，她不必点开就知道没用。
//  4. 同一个词重复 → 只留第一张。
//  5. 一张都不剩 → 整件工具算失败，绝不返回一组空卡片。
func parseWordCards(body, paragraph string) ([]readingWord, bool) {
	var got struct {
		Words []readingWord `json:"words"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		return nil, false
	}
	lowerPara := strings.ToLower(paragraph)
	out := make([]readingWord, 0, readingWordsMax)
	seen := map[string]bool{}
	for _, w := range got.Words {
		term := strings.TrimSpace(w.Term)
		meaning := strings.TrimSpace(w.Meaning)
		if term == "" || meaning == "" {
			continue
		}
		i := strings.Index(lowerPara, strings.ToLower(term))
		if i < 0 {
			// 这个词不在这一段里。卡片说的就不是这一段，荧光笔也无处可落。
			continue
		}
		// 用段落里的那一份写法 —— 荧光笔按它去标，两边必须是同一串字符。
		term = paragraph[i : i+len(term)]
		key := strings.ToLower(term)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, readingWord{
			Term: term, Pos: strings.TrimSpace(w.Pos), Meaning: meaning,
			Note:      strings.TrimSpace(w.Note),
			Example:   strings.TrimSpace(w.Example),
			ExampleZh: strings.TrimSpace(w.ExampleZh),
		})
		if len(out) == readingWordsMax {
			break
		}
	}
	return out, len(out) > 0
}

// wordCardsAsProse 把一组词卡写成 body 那一列里的纯文字。
//
// 🚨 它不是拿来渲染的（界面渲染的是卡片本身）。它存在是为了让这一行在任何一个
// **不带解析器**的地方仍然读得懂 —— 日后的报告、教师端、一次 psql 查。一行
// 只有 JSON 的记录在那些地方就是一段乱码。
func wordCardsAsProse(words []readingWord) string {
	var b strings.Builder
	for _, w := range words {
		b.WriteString("- **" + w.Term + "**")
		if w.Pos != "" {
			b.WriteString("（" + w.Pos + "）")
		}
		b.WriteString(" " + w.Meaning + "\n")
		if w.Note != "" {
			b.WriteString("  " + w.Note + "\n")
		}
		if w.Example != "" {
			b.WriteString("  " + w.Example + "\n")
		}
		if w.ExampleZh != "" {
			b.WriteString("  " + w.ExampleZh + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// parseShapedBlockReply turns a structured reply into the markdown the panel
// renders, and DROPS anything that does not fit the shape.
//
// For "questions" that means every entry must end in a question mark — the
// same filter the writing room's guiding box uses, for the same reason: a
// declarative sentence slipped into the list is a sentence she could paste,
// which is exactly what these two shapes exist to make impossible.
// sliceBlockJSON 把模型回话里那个 JSON 对象切出来。模型很爱在前后加一句
// 「好的，这是结果：」或者用 ```json 围起来 —— 与其在每个 prompt 里再劝一次
// （劝告不可验），不如在解析这一侧把这件事变成不重要的。
func sliceBlockJSON(text string) string {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	return strings.TrimSpace(c)
}

func parseShapedBlockReply(shape, text string) (string, bool) {
	c := sliceBlockJSON(text)

	switch shape {
	case "questions":
		var got struct {
			Questions []string `json:"questions"`
		}
		if err := json.Unmarshal([]byte(c), &got); err != nil {
			return "", false
		}
		kept := make([]string, 0, len(got.Questions))
		for _, q := range got.Questions {
			q = strings.TrimSpace(q)
			if q == "" || (!strings.HasSuffix(q, "？") && !strings.HasSuffix(q, "?")) {
				continue
			}
			kept = append(kept, "- "+q)
			if len(kept) == 4 {
				break
			}
		}
		if len(kept) == 0 {
			return "", false
		}
		return strings.Join(kept, "\n"), true

	case "imitate":
		var got struct {
			Move    string   `json:"move"`
			TryThis []string `json:"tryThis"`
		}
		if err := json.Unmarshal([]byte(c), &got); err != nil {
			return "", false
		}
		move := strings.TrimSpace(got.Move)
		if move == "" {
			return "", false
		}
		var b strings.Builder
		b.WriteString("**这一段的写法**：" + move + "\n\n换个话题，你也这样写一段：\n")
		n := 0
		for _, t := range got.TryThis {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			b.WriteString("- " + t + "\n")
			n++
			if n == 3 {
				break
			}
		}
		if n == 0 {
			return "", false
		}
		return strings.TrimRight(b.String(), "\n"), true
	}
	return "", false
}

// readingBlockSystemFor 把字数上限填进去。
//
// 🚨 上限**按工具**给，不再是一个写死的 200。2026-09-17 把「把握度」并进
// 「写作解析」之后，那一件要在同一段话里讲两件事（这一段在干什么 + 作者说得
// 有多满），200 字装不下；而别的工具一个字都不该多写 —— 讲解越长她越不读。
func readingBlockSystemFor(t readingBlockTool) string {
	cap := t.MaxRunes
	if cap <= 0 {
		cap = readingBlockDefaultMaxRunes
	}
	return fmt.Sprintf(readingBlockSystem, cap) + t.Instruction + readingShapedSuffix[t.Shape]
}

// readingBlockDefaultMaxRunes 是一件工具默认最多写多少字。
const readingBlockDefaultMaxRunes = 200

const readingBlockSystem = `你是「印记」，正在给一个中学生讲解她点开的**这一段**。

你只讲这一段。上下文给你，是为了让你知道这一段在整篇里的位置，不是让你去讲整篇。

共同的规矩：
- 直接讲，不要「好的」「让我们来看看」这类开场白。
- 讲给一个中学生听：把话说清楚，不要用他没学过的术语；非用不可就顺手解释一句。
- 不超过 %d 个字。讲不满不要凑。
- 用中文讲解（哪怕原文是英文）。

这一次要做的是：`

// sentence 非空时，讲的是这一段里的**那一句**（语法那件工具）。段落仍然给，
// 因为一个代词指的是谁、一个省略省掉了什么，只有把上一句读了才说得清 ——
// 但要讲的是哪一句必须写死，否则模型会顺手把整段都讲一遍。
func buildReadingBlockPrompt(title string, blocks []Block, idx int, sentence string) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}
	if sentence != "" {
		b.WriteString("\n【要讲解的这一句】\n" + sentence + "\n")
		b.WriteString("\n【它所在的那一段（只作参考，不要讲解整段）】\n" +
			strings.TrimSpace(blocks[idx].Text) + "\n")
		return b.String()
	}
	b.WriteString("\n【要讲解的这一段】\n" + strings.TrimSpace(blocks[idx].Text) + "\n")

	// Neighbours, nearest first, until the budget runs out. Position is what
	// 结构解析 / 写作解析 are ABOUT, so a paragraph with no neighbours would
	// make those two tools guess.
	var before, after []string
	budget := readingBlockContextRunes
	for step := 1; budget > 0 && (idx-step >= 0 || idx+step < len(blocks)); step++ {
		if i := idx - step; i >= 0 {
			t := strings.TrimSpace(blocks[i].Text)
			if n := len([]rune(t)); n > 0 && n <= budget {
				before = append([]string{t}, before...)
				budget -= n
			}
		}
		if i := idx + step; i < len(blocks) {
			t := strings.TrimSpace(blocks[i].Text)
			if n := len([]rune(t)); n > 0 && n <= budget {
				after = append(after, t)
				budget -= n
			}
		}
	}
	if len(before) > 0 {
		b.WriteString("\n【它前面的内容（只作参考，不要讲解）】\n" + strings.Join(before, "\n") + "\n")
	} else {
		b.WriteString("\n（这是文章的第一段。）\n")
	}
	if len(after) > 0 {
		b.WriteString("\n【它后面的内容（只作参考，不要讲解）】\n" + strings.Join(after, "\n") + "\n")
	} else {
		b.WriteString("\n（这是文章的最后一段。）\n")
	}
	return b.String()
}

// listReadingBlockTools is GET /api/v1/readings/{id}/blocks/tools.
//
// Which tools exist depends on the ARTICLE's language, not on a setting: the
// student already said what she wants to read by pasting it. Serving the list
// (rather than hardcoding it in the frontend) is what keeps the buttons she
// sees and the ids the server accepts from drifting apart.
func (a *API) listReadingBlockTools(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	lang := readingLangOf(src.Body)
	tools := readingBlockToolsFor(lang)
	out := make([]map[string]string, 0, len(tools))
	for _, t := range tools {
		// subject 要发出去：界面凭它决定「点这件工具之后先请她点一句，还是
		// 直接开讲」。写死在前端会和服务端漂开 —— 和这个端点本来就存在的理由
		// 是同一条。
		out = append(out, map[string]string{
			"id": t.ID, "label": t.Label, "subject": t.Subject,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"lang": lang, "tools": out})
}

// blockNoteDTO 是一份已经开过的讲解。
type blockNoteDTO struct {
	BlockID string `json:"blockId"`
	Tool    string `json:"tool"`
	Body    string `json:"body"`
	// Subject 是「讲的是哪一句」。空串 = 整段。
	Subject string `json:"subject,omitempty"`
	// Words 是关键单词那件工具的词卡。别的工具没有这一项。
	Words []readingWord `json:"words,omitempty"`
}

// blockNoteDTOFrom 把一行 reading_block_note 变成发出去的那份。
//
// data 那一列读不动就当它没有：一份坏掉的 JSON 不该让整个阅读室打不开，而
// body 那一段文字仍然是可读的（wordCardsAsProse 存的就是它）。
func blockNoteDTOFrom(row sqlc.ReadingBlockNote) blockNoteDTO {
	dto := blockNoteDTO{
		BlockID: row.BlockID, Tool: row.Tool, Body: row.Body, Subject: row.Subject,
	}
	if len(row.Data) > 0 {
		var payload struct {
			Words []readingWord `json:"words"`
		}
		if err := json.Unmarshal(row.Data, &payload); err == nil {
			dto.Words = payload.Words
		}
	}
	return dto
}

// listReadingBlockNotes is GET /api/v1/readings/{id}/blocks/notes — every
// explanation she has already opened, so a reload restores them instead of
// making her pay for them twice.
func (a *API) listReadingBlockNotes(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListReadingBlockNotes(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]blockNoteDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, blockNoteDTOFrom(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"notes": out})
}

// explainReadingBlock is POST /api/v1/readings/{id}/blocks/{bid}/explain.
//
// Cached by (blockId, tool, subject): a replay costs nothing and returns
// instantly, so the entitlement gate and the model call are both SKIPPED on
// that path — gating a replay would charge her for reading something she
// already has.
//
// subject 是「讲的是哪一句」，只有语法那件工具用（2026-09-16）。其余工具永远
// 传空串，于是缓存键和改这件事之前完全一样。
func (a *API) explainReadingBlock(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	blockID := r.PathValue("bid")
	var req struct {
		Tool string `json:"tool"`
		// Sentence 是她在这一段里点的那一句。只有 Subject == "sentence" 的
		// 工具要它；服务端会拿它回段落里逐字核对。
		Sentence string `json:"sentence"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tool, found := findReadingBlockTool(strings.TrimSpace(req.Tool))
	if !found {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_tool", "这不是可用的段落工具。", nil))
		return
	}
	sentence := strings.TrimSpace(req.Sentence)
	if tool.Subject != "sentence" {
		// 别的工具讲的是整段。带了句子也当没带 —— 否则同一段会按她随手划到
		// 哪儿缓存出好几份一模一样的讲解。
		sentence = ""
	} else if sentence == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_sentence", "请先在这一段里点一个句子。", nil))
		return
	}

	// Replay first — before the entitlement gate, before any model call.
	if row, err := a.d.Queries.GetReadingBlockNote(r.Context(), sqlc.GetReadingBlockNoteParams{
		AtomID: at.ID, BlockID: blockID, Tool: tool.ID, Subject: sentence,
	}); err == nil {
		dto := blockNoteDTOFrom(row)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"blockId": dto.BlockID, "tool": dto.Tool, "body": dto.Body,
			"subject": dto.Subject, "words": dto.Words, "cached": true,
		})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks := SplitBlocks(src.Body)
	idx := -1
	for i, blk := range blocks {
		if blk.ID == blockID {
			idx = i
			break
		}
	}
	if idx < 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	// 🚨 她点的那一句必须真的在这一段里。前端是按 segmentSentences 切出来的，
	// 所以正常情况下一定对得上；这条挡的是没有界面的调用 —— 一个编出来的句子
	// 会让模型去讲一句这篇文章里不存在的话，而她看到的讲解一个字都验不了。
	if sentence != "" && !strings.Contains(blocks[idx].Text, sentence) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("sentence_not_in_block", "这句话不在这一段里。", nil))
		return
	}
	// A tool from the other language set is a mis-click, not a preference:
	// 语法讲解 on a Chinese paragraph would produce something confidently
	// useless. Refuse rather than spend.
	//
	// Lang == "" means the tool is language-independent (想一想 / 仿写 — what a
	// paragraph DOES is not a language-specific question), so it is never a
	// mismatch.
	if tool.Lang != "" && tool.Lang != readingLangOf(src.Body) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("tool_language_mismatch", "这个工具不适用于这篇文章。", nil))
		return
	}

	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// §model-routing · dialogue. Explaining a paragraph is explanation, not
	// judgement, and it is the most frequently-clicked call in the room —
	// which is exactly what dialogue's 4-second budget is for.
	resolved, rerr := a.routeE(turnCtx, gateway.ClassDialogue)
	if rerr != nil {
		slog.Warn("reading block explain: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingBlockSystemFor(tool)},
			{Role: gateway.RoleUser, Content: buildReadingBlockPrompt(src.Title, blocks, idx, sentence)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_"+tool.ID, resolved, res.Usage)
	body := strings.TrimSpace(res.Text)
	var words []readingWord
	var data []byte
	switch {
	case cerr != nil || body == "":
		// 下面那道统一的失败分支会处理。
	case tool.Shape == "words":
		// 🚨 每个 term 都要回这一段里逐字核对。核不上的丢光了，这件工具就算
		// 失败 —— 绝不返回一组空卡片，也绝不把原始回话当散文渲染出去：
		// 一张指着这一段里没有的词的卡片，荧光笔无处可落，讲解也验不了。
		got, okWords := parseWordCards(sliceBlockJSON(body), blocks[idx].Text)
		if !okWords {
			slog.Warn("reading block explain: no word card survived the verbatim check",
				"atom_id", at.ID, "tool", tool.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		words = got
		// body 存的是这组卡片的纯文字形态。见 wordCardsAsProse。
		body = wordCardsAsProse(words)
		if encoded, merr := json.Marshal(map[string]any{"words": words}); merr == nil {
			data = encoded
		}
	case tool.Shape != "prose":
		shaped, okShape := parseShapedBlockReply(tool.Shape, body)
		if !okShape {
			// A shaped reply that will not parse is a FAILURE, never rendered
			// raw — rendering it raw is exactly how a sample paragraph would
			// reach her through the one tool built to prevent that.
			slog.Warn("reading block explain: shaped reply unparseable",
				"atom_id", at.ID, "tool", tool.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		body = shaped
	}
	if cerr != nil || body == "" {
		slog.Warn("reading block explain: model call failed", "err", cerr,
			"atom_id", at.ID, "tool", tool.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	if _, err := a.d.Queries.InsertReadingBlockNote(turnCtx, sqlc.InsertReadingBlockNoteParams{
		AtomID: at.ID, BlockID: blockID, Tool: tool.ID, Body: body,
		Data: data, Subject: sentence,
	}); err != nil {
		// Failing to CACHE must not cost her the explanation she just paid
		// for — it is already in hand, so serve it and let the next click pay
		// again rather than turning a storage hiccup into a 500.
		slog.Warn("reading block explain: cache write failed", "err", err, "atom_id", at.ID)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"blockId": blockID, "tool": tool.ID, "body": body,
		"subject": sentence, "words": words, "cached": false,
	})
}
