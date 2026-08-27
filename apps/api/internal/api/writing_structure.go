package api

// writing_structure.go — 结构这一步的三个端点。
//
//   - GET  /writings/structures                 — 结构库（前端不硬编码第二份）
//   - POST /writings/{id}/structure/recommend   — 模型从库里**挑一副**并说理由
//   - POST /writings/{id}/structure             — 铺开她选中的那副骨架
//
// 这个文件取代了 writing_outline.go 的 POST /outline/generate。那条路是让
// 模型读完她说过的话、直接产出一份提纲；2026-08-27 的裁定推翻了它：
// **AI 永远不直接生成提纲。AI 帮学生思考，给出通用结构，引导学生用自己的
// 想法和经历去填每一块。**
//
// 于是这里把那一次调用从「创作」降级成「挑选」：模型能看到的候选就是
// writing_structures.go 里那张写死的表，它唯一的输出是一个 key 加一句理由。
// 它挑错了，代价是她多点一下换一副；它**不可能**替她写出论点，因为这条链路
// 上根本没有一个字段能装下论点。这不是靠 prompt 求来的克制，是结构上的不可能。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// listWritingStructures is GET /api/v1/writings/structures. Deliberately NOT
// under /{id}: the library is the same for everyone and depends on no
// writing, so keying it by atom would imply a per-writing customisation that
// does not exist (and must not — see the file comment). ?lang= scopes it;
// absent means zh.
//
// The frontend fetches this rather than shipping its own copy, so the labels
// a student sees and the labels stored in writing_outline.role can never
// drift apart.
func (a *API) listWritingStructures(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"structures": writingStructuresFor(r.URL.Query().Get("lang")),
	})
}

// applyWritingStructure is POST /api/v1/writings/{id}/structure. No model
// call — laying out a chosen skeleton is pure data movement.
//
// It REPLACES the outline. That is destructive when she has already written
// block text, so the handler refuses to run silently over existing work:
// unless `force` is set, an outline that already holds any non-empty `text`
// comes back as 409. Switching skeleton after you have written three blocks
// is a real thing to want, but it must be a decision she makes with her eyes
// open, not a side effect of clicking a different card.
func (a *API) applyWritingStructure(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		StructureKey string `json:"structureKey"`
		Force        bool   `json:"force"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	st, found := findWritingStructure(strings.TrimSpace(req.StructureKey))
	if !found {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_structure", "这不是可用的文章结构。", nil))
		return
	}

	existing, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !req.Force {
		for _, row := range existing {
			if strings.TrimSpace(row.Text) != "" {
				httpx.WriteError(w, r, httpx.ErrConflict("换一副结构会清掉你已经在这些块里写下的内容。确认要换吗？"))
				return
			}
		}
	}

	texts := make([]string, 0, len(st.Blocks))
	roles := make([]string, 0, len(st.Blocks))
	depths := make([]int32, 0, len(st.Blocks))
	positions := make([]int32, 0, len(st.Blocks))
	for i, b := range st.Blocks {
		// text is EMPTY on purpose, for every block. The skeleton contributes
		// the role label and nothing else; every word of content is hers.
		// This empty string is the 铁律 line, expressed as data.
		texts = append(texts, "")
		roles = append(roles, b.Role)
		depths = append(depths, 0)
		positions = append(positions, int32(i))
	}

	// structure_key and the rows it lays out must land together — a committed
	// structure_key pointing at an outline that failed to write would render
	// as a chosen-but-empty skeleton with no way back.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if _, err := qtx.SetWritingStructure(r.Context(), sqlc.SetWritingStructureParams{
		AtomID: at.ID, StructureKey: st.Key,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := qtx.ReplaceWritingOutline(r.Context(), sqlc.ReplaceWritingOutlineParams{
		AtomID: at.ID, Texts: texts, Roles: roles, Depths: depths, Positions: positions,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	out := make([]writingOutlineItemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWritingOutlineItemDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"structureKey": st.Key, "outline": out})
}

// writingStructureRecommendSystem constrains the model to a pure selection.
//
// Note what it is NOT allowed to output: no blocks, no outline, no thesis, no
// example. The response schema has exactly two fields, and `reason` is
// explicitly about the SHAPE ("你两边都有话说，让步式最能放下这种题目"),
// never about her content. A reason that starts arguing her topic is the
// failure mode this prompt exists to prevent.
const writingStructureRecommendSystem = `你是「印记」。下面给你一个学生想写的东西，以及一张**固定的文章结构表**。

你的任务只有一件：从表里挑出最适合她这篇的一副结构，并用一句话说明为什么适合。

严格规则：
- structureKey 必须**逐字**是表里给出的某一个 key，不能自造、不能改写。
- reason 只讲**结构层面**的理由（比如「你两边都有理由，让步式能把反方放进来」），不超过 40 个字。
- reason 里绝对不要出现你替她想的论点、立场、例子或提纲。你不知道她要论证什么，也不该猜。
- 不要输出任何结构块、提纲条目或正文。

只输出一个 JSON 对象：{"structureKey":"...","reason":"..."}，不要输出对象以外的任何文字或代码块标记。`

// buildWritingStructureRecommendPrompt renders the candidate table plus her
// own material. The table is rendered from the same slice the endpoint
// serves, so a skeleton added to the library is automatically a candidate
// here with no second edit.
func buildWritingStructureRecommendPrompt(wr sqlc.Writing, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目/想法：" + t + "\n")
	}
	if wr.TargetWords != nil {
		b.WriteString("目标篇幅：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字\n")
	}
	b.WriteString("写作语言：" + wr.Lang + "\n")

	b.WriteString("\n【可选的结构（只能从这里挑一个 key）】\n")
	for _, s := range writingStructuresFor(wr.Lang) {
		b.WriteString("- key=" + s.Key + " · " + s.Name + " · 适合：" + s.Blurb + "\n")
		roles := make([]string, 0, len(s.Blocks))
		for _, blk := range s.Blocks {
			roles = append(roles, blk.Role)
		}
		b.WriteString("  块：" + strings.Join(roles, " → ") + "\n")
	}

	b.WriteString("\n【她自己说过的话】\n")
	any := false
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString("- " + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（只有上面那个题目。）\n")
	}
	return b.String()
}

// parseWritingStructureRecommendation decodes the model's object and
// VALIDATES the key against the library. An unknown key is treated as no
// recommendation at all rather than being passed through — a key that does
// not resolve would render as a recommendation the student cannot accept.
func parseWritingStructureRecommendation(text string, lang string) (string, string, bool) {
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
	var got struct {
		StructureKey string `json:"structureKey"`
		Reason       string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return "", "", false
	}
	st, ok := findWritingStructure(strings.TrimSpace(got.StructureKey))
	if !ok {
		return "", "", false
	}
	// A skeleton from the other language set is a mis-pick, not a
	// preference — an English student handed 记叙文's Chinese block labels
	// would be worse than no recommendation.
	want := lang
	if want != "en" {
		want = "zh"
	}
	if st.Lang != want {
		return "", "", false
	}
	return st.Key, strings.TrimSpace(got.Reason), true
}

// recommendWritingStructure is POST /api/v1/writings/{id}/structure/recommend.
// A spend endpoint (one model call), metered as purpose="structure_pick".
//
// It PERSISTS NOTHING. The recommendation is a suggestion the student accepts
// by clicking, which then calls applyWritingStructure — the same
// propose-then-she-confirms shape 工具卡 use (铁律②: 触发是自动的，「打开」由
// 学生确认).
func (a *API) recommendWritingStructure(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
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

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing: picking one row out of a table is a small judgement, not
	// evaluative work — the chaperone tier is right here, unlike the outline
	// GENERATION this replaced (which resolved the flagship precisely because
	// it was authoring her structure). The downgrade is a direct consequence
	// of the model having less to do.
	resolved, rerr := a.d.ChatResolver(turnCtx)
	if rerr != nil {
		slog.Warn("writing structure recommend: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingStructureRecommendSystem},
			{Role: gateway.RoleUser, Content: buildWritingStructureRecommendPrompt(wr, msgs)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "structure_pick", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing structure recommend: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	key, reason, okRec := parseWritingStructureRecommendation(res.Text, wr.Lang)
	if !okRec {
		// No canned fallback pick. A silently-substituted "default" skeleton
		// would be indistinguishable from a real recommendation, and she would
		// have no way to know the coach never actually looked at her topic.
		// She still has the full library to choose from herself.
		slog.Warn("writing structure recommend: unparseable or out-of-library reply",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"structureKey": key, "reason": reason})
}
