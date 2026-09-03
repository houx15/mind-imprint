package api

// writing_title.go — giving the finished piece a NAME.
//
// ## The bug this exists for
//
// A writing's title is born as her raw idea sentence: createWriting
// (writings.go) takes the one line she typed into 「我想写：…」 and uses it,
// truncated to 200 runes, as `writing.title`. That was always meant to be a
// placeholder — EditableTitle exists precisely so she can replace it — but
// nothing ever ASKED her to, so most pieces are still carrying it at 完成这篇.
//
// It stops being a private placeholder at that moment. The report puts the
// title in display type at the top of the hero, the exported poster prints
// it, and the share link opens it for someone with no account at all. So a
// parent scanning a QR code was reading 「我想写中国是不是真的让地球更可持续
// 了，因为我们地理课上…」 where the piece's name should be.
//
// ## The shape: offered at 完成这篇, never imposed
//
// One endpoint, called when she presses 完成这篇:
//
//   - `needsName` false — she already renamed it herself. No model call, no
//     dialog, nothing at all: 完成这篇 goes straight through, exactly as
//     before. Anyone who has named their piece must not be asked again.
//   - `needsName` true — the title is still, character for character, the
//     truncated idea sentence. The client opens one small dialog with these
//     `ideas` as tappable suggestions and a free text box, plus a way to keep
//     what she has.
//
// The server NEVER writes a title here. It returns candidates and the client
// saves whichever one she chose (or her own words) through the existing
// PATCH /writings/{id}. That separation is the whole reason this is an
// offer: there is no code path in which a generated title reaches the
// database without her having picked it.
//
// ## 铁律① — why a title is not 正文, said out loud
//
// AGENTS.md draws this boundary explicitly: 「AI 绝不代写正文」 is about her
// BODY TEXT, and it does not extend to deterministic or supporting system
// steps. A title is not a sentence of her essay — nothing here is written
// into the draft, the snippets or the outline, and this file has no write
// path to any of them. What it does have is the same posture as every other
// offer in this room (GuideBox's methods, VocabExamples, the 语文 term card):
// named, explained, tappable, and inert until she chooses.
//
// **Honest limit, in the manner of writing_comment.go's own:** unlike a
// comment's quote, a title CANNOT be validated as a literal substring of her
// writing — naming a thing is precisely the act of finding words that are
// not already in it. So `ideas` is free model prose, held only by the
// prompt's instructions and by the length cap in validateTitleIdeas. That is
// accepted deliberately, and it is bounded by the two structural facts
// above: she picks, and nothing is written without her.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// titleRuneCap is createWriting's own truncation, repeated here because
// `needsName` compares against exactly what that function stored. If one
// changes, both must — a mismatch would make every writing look renamed
// (and silently switch this whole feature off), which is the failure mode
// worth naming rather than the one worth being clever about.
const titleRuneCap = 200

// suggestedTitleRuneCap bounds a candidate. A "title" of 60 characters is
// the very thing this file exists to get rid of, so an over-long suggestion
// is dropped rather than truncated — cutting one mid-clause would produce a
// worse title than the placeholder it replaces.
const suggestedTitleRuneCap = 40

// titleIsStillTheIdea reports whether `title` is untouched since
// createWriting — i.e. it is still her idea sentence, truncated the same way
// that function truncates it.
//
// Equality, not a heuristic. A length threshold ("titles are short") would
// pester a student who deliberately chose a long title and would miss a
// short idea sentence entirely; comparing against the actual stored opening
// turn is exact, and it costs nothing because the caller has already loaded
// the messages.
//
// Whitespace is trimmed on both sides before comparing: createWriting stores
// the idea already trimmed, and renameWriting trims too, so a difference in
// surrounding space alone never means she renamed anything.
func titleIsStillTheIdea(title, idea string) bool {
	t := strings.TrimSpace(title)
	i := strings.TrimSpace(idea)
	if i == "" {
		// No opening turn to compare against (should not happen — createWriting
		// writes it in the same transaction as the writing row). Treat the
		// title as hers rather than interrupting her over a row we cannot
		// account for.
		return false
	}
	if len([]rune(i)) > titleRuneCap {
		i = string([]rune(i)[:titleRuneCap])
	}
	return t == strings.TrimSpace(i)
}

// firstStudentMessage returns the verbatim opening turn — her idea sentence,
// untruncated. createWriting writes it as seq=1, role='student', in the same
// transaction as the writing itself, so it is always the first row;
// ListAtomMessages already orders by seq and already filters out block
// threads in SQL (see writing_deepen.go), so no caller of this can pick up a
// 深入一层 message by accident.
func firstStudentMessage(msgs []sqlc.AtomMessage) string {
	for _, m := range msgs {
		if m.Role == "student" {
			return m.Content
		}
	}
	return ""
}

// validateTitleIdeas keeps the candidates that could actually serve as a
// title: non-empty, not absurdly long, and distinct from one another.
//
// Deduping is on the trimmed string, and it also drops anything equal to the
// placeholder she is replacing — offering her current title back as a fresh
// suggestion would read as the model not having looked at anything.
func validateTitleIdeas(ideas []string, current string) []string {
	out := make([]string, 0, len(ideas))
	seen := map[string]bool{strings.TrimSpace(current): true}
	for _, raw := range ideas {
		// Models like to wrap a title in quotes or 书名号; those are packaging,
		// not part of the name she'd type.
		t := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), `"'“”「」《》`))
		if t == "" || len([]rune(t)) > suggestedTitleRuneCap || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) == 4 {
			break
		}
	}
	return out
}

// writingTitleSystem asks for names for a finished piece — and nothing else.
//
// The prompt is deliberately narrow: it never sees a question it could
// answer with a sentence for her essay, because the only thing it is asked
// to produce is a list of names. writingGuideTeachingRules is NOT included
// here — that doctrine is about how 印记 talks to her, and this reply is
// never shown as speech; it becomes four chips she taps.
const writingTitleSystem = `你是「印记」。学生刚写完一篇文章，现在要给它起个名字。她原来那一栏里放的是她最开始随手写下的一句「我想写…」，那是给她自己看的备忘，不是标题。

请你读她写完的这篇文章，给出 4 个可以当标题的候选。要求：
- 每个标题都要短。中文一般不超过 20 个字，英文不超过 8 个词。
- 必须来自她这篇文章本身——说的是她真正写了的那件事、她真正持的那个立场，不要写成一个泛泛的作文题。
- 4 个之间要有区别：可以有的直白概括，有的用她文章里的一个具体形象或一组对比，有的带一点问句。不要 4 个都是同一个句式。
- 只给名字。不要解释，不要在标题后面加副标题、破折号说明或者任何点评。

输出 JSON：{"ideas":["…","…","…","…"]}

只输出一个 JSON 对象，不要输出对象以外的任何文字或代码块标记。`

// buildWritingTitlePrompt gives the model her finished piece and the note she
// started from, labelled for what each one is. The idea sentence is included
// because it carries what she SET OUT to argue, which a draft that wanders
// may not state as plainly — but it is labelled 「不是标题」 so it is never
// echoed straight back as a candidate (validateTitleIdeas drops it anyway if
// it is).
func buildWritingTitlePrompt(idea, draft string) string {
	var b strings.Builder
	if i := strings.TrimSpace(idea); i != "" {
		b.WriteString("她最开始说想写的（这是备忘，不是标题）：\n" + i + "\n\n")
	}
	b.WriteString("她写完的文章：\n" + strings.TrimSpace(draft) + "\n")
	return b.String()
}

type writingTitleResult struct {
	Ideas []string `json:"ideas"`
}

// parseWritingTitleIdeas decodes the model's reply. Reuses
// extractWritingJSONObject (writing_snippets.go) — the same "strip fences,
// clamp to the outermost {..}" extraction every JSON-replying prompt in this
// package shares.
func parseWritingTitleIdeas(text string) (writingTitleResult, bool) {
	c := extractWritingJSONObject(text)
	if c == "" {
		return writingTitleResult{}, false
	}
	var got writingTitleResult
	if err := json.Unmarshal([]byte(c), &got); err != nil {
		return writingTitleResult{}, false
	}
	return got, true
}

// suggestWritingTitles is POST /api/v1/writings/{id}/title-ideas.
//
// A spend endpoint, but a CONDITIONAL one: it makes a model call only when
// the title is still the placeholder. A student who named her piece herself
// pays nothing and sees nothing, which is what keeps this from becoming a
// toll on 完成这篇.
//
// Metered as purpose="title_ideas". Follows commentOnSnippet's shape:
// loadOwnedWritingAtom → HasEntitlement → 150s detached timeout →
// resolveEval → gateway.Collect → recordLiteLLMCall → parse → validate.
// It persists NOTHING — see this file's header for why that is the point.
func (a *API) suggestWritingTitles(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	idea := firstStudentMessage(msgs)

	// She has already named it. Answer honestly and spend nothing — this is
	// the common case for anyone who used EditableTitle, and it must stay
	// free and instant.
	if !titleIsStillTheIdea(wr.Title, idea) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"needsName": false, "ideas": []string{}})
		return
	}

	// Nothing written yet means nothing to name from. Not an error: the
	// client simply shows the box with no suggestions in it, and she can
	// still type a title herself.
	draft, derr := a.d.Queries.GetWritingDraft(r.Context(), at.ID)
	body := ""
	if derr == nil {
		body = strings.TrimSpace(draft.Body)
	}
	if body == "" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"needsName": true, "ideas": []string{}})
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

	// §model-routing · compose. Naming a piece is a reading-comprehension task
	// over her whole draft, and a model with nothing left to spend produces
	// generic 作文题 titles — exactly the failure this endpoint exists to avoid.
	// It derives a name from a draft she has already written, so it is compose,
	// not review; what it needs is a reasoning budget, not the reviewer tier.
	resolved, ok2 := a.route(turnCtx, gateway.ClassCompose)
	if !ok2 {
		slog.Warn("writing title ideas: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingTitleSystem},
			{Role: gateway.RoleUser, Content: buildWritingTitlePrompt(idea, body)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "title_ideas", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing title ideas: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	parsed, okParse := parseWritingTitleIdeas(res.Text)
	if !okParse {
		slog.Warn("writing title ideas: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"needsName": true,
		"ideas":     validateTitleIdeas(parsed.Ideas, wr.Title),
	})
}
