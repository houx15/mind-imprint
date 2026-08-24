package api

// course_definition_admin.go — Task 3 of the course authoring & publish
// lifecycle: the course generator's create/modify endpoint. PUT
// /api/v1/admin/courses/{slug}/definition (OSS_ADMIN_KEY bearer, same gate as
// course_admin.go's legacy publish route) upserts a CourseDefinition 2.0
// course. Border-validation only: schemaVersion == "2.0", course.id/title
// present and course.id == {slug}, every cardId resolves in the registry —
// deep structural validation is the generator's own job (the TS
// course-contract Zod schema), never re-done here. The definition body is
// stored verbatim as jsonb (agent.UpsertCourseDefinitionInput.Definition);
// this handler never interprets its inner shape beyond the border slice
// below. A new course lands status='preview'; re-PUTting an existing course
// preserves whatever status it already has — that's the Task-1
// UpsertCourseDefinition query's ON CONFLICT contract (ignores EXCLUDED on
// status), not something this handler decides.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
)

// validCourseCategories mirrors COURSE_CATEGORIES in packages/contracts —
// the closed 7-slug vocabulary. Kept as a Go set for border validation; the
// contract remains the source of truth for the labels.
var validCourseCategories = map[string]bool{
	"stance-value": true, "source-check": true, "media-literacy": true,
	"self-knowledge": true, "data-literacy": true, "research-process": true,
	"argument-writing": true,
}

// putCourseDefinitionReq is the upload envelope: Definition travels as
// json.RawMessage (stored verbatim), Blurb/CardIDs are the catalog-card
// fields the definition document itself doesn't carry.
type putCourseDefinitionReq struct {
	Definition   json.RawMessage `json:"definition"` // the whole { schemaVersion, course } document
	Blurb        string          `json:"blurb"`
	CardIDs      []string        `json:"cardIds"`
	Category     string          `json:"category"`     // one of the 7 slugs, or "" to leave unset
	Introduction json.RawMessage `json:"introduction"` // schema-driven intro object, or absent
}

// putCourseDefinitionDoc is the border slice of `definition` this handler
// validates. The document's full shape (objectives/interactions/…) is the
// generator's concern, never this handler's — but `parts[].slices[]` is parsed
// (shallow, just to COUNT) because a slice is a course "step": the count feeds
// the catalog's `step_count` (the "N 步" label + progress math). See
// courseStepCount below.
type putCourseDefinitionDoc struct {
	SchemaVersion string `json:"schemaVersion"`
	Course        struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		EstimatedMinutes int    `json:"estimatedMinutes"`
		Parts            []struct {
			Slices []struct {
				// Blocks is walked ONLY for the richText security check below;
				// every other authored block field is ignored here.
				Blocks []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
					HTML string `json:"html"`
				} `json:"blocks"`
			} `json:"slices"`
		} `json:"parts"`
	} `json:"course"`
}

// richTextForbidden are the constructs a `richText` block's inline HTML may
// never contain. The renderer already isolates that HTML in an iframe with no
// `allow-scripts` and no `allow-same-origin`, so this is NOT the security
// boundary — it is the server refusing to STORE authored markup whose only
// purpose would be to execute, so a future renderer change (or a preview host
// that mounts the HTML some other way) can never turn stored content into a
// live script in a student's session. Mirrors RICH_TEXT_FORBIDDEN in
// packages/course-contract/src/blocks.ts; the contract is the authority on the
// full rule set (this side deliberately checks only the executable subset).
var richTextForbidden = []struct {
	re   *regexp.Regexp
	what string
}{
	{regexp.MustCompile(`(?i)<\s*script\b`), "<script>"},
	{regexp.MustCompile(`(?i)<\s*(iframe|object|embed)\b`), "<iframe>/<object>/<embed>"},
	{regexp.MustCompile(`(?i)<[a-z][^>]*\son[a-z]+\s*=`), "内联事件处理器 (onclick=…)"},
	{regexp.MustCompile(`(?i)javascript\s*:`), "javascript: URL"},
}

// validateRichTextBlocks border-walks the definition for richText blocks and
// rejects any whose inline HTML carries an executable construct. Returns the
// first offending (blockID, construct) — one clear error beats a list the
// generator has to page through.
func validateRichTextBlocks(doc putCourseDefinitionDoc) (blockID, construct string, ok bool) {
	for _, part := range doc.Course.Parts {
		for _, slice := range part.Slices {
			for _, block := range slice.Blocks {
				if block.Type != "richText" || block.HTML == "" {
					continue
				}
				for _, f := range richTextForbidden {
					if f.re.MatchString(block.HTML) {
						return block.ID, f.what, false
					}
				}
			}
		}
	}
	return "", "", true
}

// courseStepCount is the authored step count of a 2.0 course: the total number
// of slices across every part. One slice = one "step" (the runtime's own
// progress bar reads `第 X / Y 步` off the same slice sequence), so this is the
// denominator the catalog surfaces divide by and the label they render. Empty
// parts contribute 0; a course with no parts is 0 steps.
func courseStepCount(doc putCourseDefinitionDoc) int {
	n := 0
	for _, part := range doc.Course.Parts {
		n += len(part.Slices)
	}
	return n
}

// putCourseDefinition lets the course generator push a CourseDefinition 2.0
// document behind the OSS_ADMIN_KEY bearer. Border-validates the envelope,
// then upserts by slug (course.id) — agent.UpsertCourseDefinition's own
// upsert-by-slug contract lands a brand-new course as 'preview' and leaves an
// existing course's status untouched.
func (a *API) putCourseDefinition(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) { // constant-time compare (oss.go) — same gate as the OSS admin routes
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥")) // 401, no key echoed
		return
	}

	slug := r.PathValue("slug")

	var body putCourseDefinitionReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	var doc putCourseDefinitionDoc
	if err := json.Unmarshal(body.Definition, &doc); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "definition 不是合法 JSON", nil))
		return
	}
	if doc.SchemaVersion != "2.0" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "invalid_course_definition",
			Message: "schemaVersion 必须为 2.0",
		})
		return
	}
	if doc.Course.ID == "" || doc.Course.Title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course.id / course.title 不能为空", nil))
		return
	}
	if doc.Course.ID != slug {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course.id 必须与 slug 一致", nil))
		return
	}
	// Security border (see richTextForbidden): never STORE authored HTML whose
	// only purpose is to execute, even though the renderer's sandbox already
	// makes it inert.
	if blockID, construct, ok := validateRichTextBlocks(doc); !ok {
		httpx.WriteError(w, r, &httpx.APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "invalid_course_definition",
			Message: "richText 块 '" + blockID + "' 的 html 含有 " + construct + "，静态图文卡片不执行任何代码。",
		})
		return
	}
	for _, id := range body.CardIDs {
		if _, ok := cards.ByID(id); !ok {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "未知的工具卡: "+id, nil))
			return
		}
	}
	cardIDs := body.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}

	// Category: empty means "leave unset" (NULL); a non-empty value must be one
	// of the 7 controlled slugs — never a free-text 8th (spec Global Constraints).
	var categoryPtr *string
	if body.Category != "" {
		if !validCourseCategories[body.Category] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "未知的课程分类: "+body.Category, nil))
			return
		}
		categoryPtr = &body.Category
	}

	// Introduction: border-validate only that it is a JSON object (the deep
	// shape is the generator's Zod contract, per the border-validation rule).
	// JSON null unmarshals into a nil map without erroring — treat that as
	// "leave unset" too, not as a stored literal null.
	var introBytes []byte
	if len(body.Introduction) > 0 {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(body.Introduction, &probe); err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "introduction 必须是一个对象", nil))
			return
		}
		if probe != nil {
			introBytes = body.Introduction
		}
	}

	timeLabel := ""
	if doc.Course.EstimatedMinutes > 0 {
		timeLabel = fmt.Sprintf("约 %d 分钟", doc.Course.EstimatedMinutes)
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	status, err := store.UpsertCourseDefinition(r.Context(), agent.UpsertCourseDefinitionInput{
		Slug:         slug,
		Branch:       "Runtime",
		Title:        doc.Course.Title,
		Blurb:        body.Blurb,
		TimeLabel:    timeLabel,
		CardIDs:      cardIDs,
		Definition:   body.Definition,
		StepCount:    courseStepCount(doc),
		Category:     categoryPtr,
		Introduction: introBytes,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": slug, "status": status, "step_count": courseStepCount(doc)})
}
