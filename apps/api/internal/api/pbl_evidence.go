package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mindimprint/api/internal/httpx"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// Keep the full snapshot for concurrency checks, but avoid repeating database
// identifiers and timestamps in the semantic review of task instructions.
func missionEvidenceSnapshot(snapshot string) (string, error) {
	var rows []sqlc.PblMissionItem
	if err := json.Unmarshal([]byte(snapshot), &rows); err != nil {
		return "", err
	}
	type item struct {
		Prompt        string `json:"prompt"`
		Kind          string `json:"kind"`
		Done          bool   `json:"marked_done"`
		Historical    bool   `json:"historical"`
		StudentEdited bool   `json:"student_edited"`
	}
	items := make([]item, 0, len(rows))
	for _, row := range rows {
		items = append(items, item{row.Prompt, row.WantKind, row.DoneAt.Valid, row.SupersededAt.Valid, row.EditedByStudent})
	}
	raw, err := json.Marshal(items)
	return string(raw), err
}

// Evidence-bearing projects get a fresh-context check before any response or
// produced artifact is persisted. One repair is allowed; failed checks never
// silently become a successful student-facing claim.
func (a *API) guardPblEvidence(r *http.Request, atomID uuid.UUID, in pbl.CoachInput, out pbl.CoachOutput, dialogue gateway.Resolved) (pbl.CoachOutput, error) {
	notes, err := a.d.Queries.ListPblNotes(r.Context(), atomID)
	if err != nil {
		return out, err
	}
	var sources []string
	// Supply the same authoritative pre-turn snapshots used for optimistic
	// revision checks; prior assistant replies are not proof of an update.
	missionIDs := make([]string, 0, len(in.MissionVersions))
	for id := range in.MissionVersions {
		missionIDs = append(missionIDs, id)
	}
	sort.Strings(missionIDs)
	for _, id := range missionIDs {
		snapshot, err := missionEvidenceSnapshot(in.MissionVersions[id])
		if err != nil {
			return out, err
		}
		sources = append(sources, "系统保存的本轮修改前观察清单（含历史项与勾选状态；不是本轮已执行修改的证明），mission_target="+id+"："+snapshot)
	}
	if event := strings.TrimSpace(in.JustHappened); event != "" {
		sources = append(sources, "系统确认的本轮触发事件（优先于历史请求，不是学生新发言）："+event)
	}
	entries, err := a.d.Queries.ListPblKeepEntries(r.Context(), atomID)
	if err != nil {
		return out, err
	}
	for _, entry := range entries {
		raw, _ := json.Marshal(map[string]any{
			"source": "学生保存的迭代记录（thought是想法或计划；feedback是学生转述的反馈，不代表系统独立验证；原文未测试、模拟等限定必须保留）",
			"id":     entry.ID, "kind": entry.Kind, "body": entry.Body, "expect": entry.Expect,
		})
		sources = append(sources, string(raw))
	}
	for _, n := range notes {
		if n.Kind != "observation" && n.Kind != "quote" && n.Kind != "assumption" && n.Kind != "question" {
			continue
		}
		raw, _ := json.Marshal(map[string]string{"author": n.Author, "category": n.Kind, "original": n.Body})
		sources = append(sources, string(raw))
	}
	// Review answers are student input too. Without them, a valid revision can
	// be rejected for using a criterion the student supplied inside the tool.
	artifacts, err := a.d.Queries.ListPblArtifacts(r.Context(), atomID)
	if err != nil {
		return out, err
	}
	for _, artifact := range artifacts {
		verdict := "pending"
		if artifact.SettledAt.Valid && artifact.Verdict != nil {
			verdict = *artifact.Verdict
		}
		state, _ := json.Marshal(map[string]any{
			"source":      "系统记录的成果审核状态（pending待审核、kept通过、revise要求修改、dropped重做；修改完成不等于审核通过）",
			"artifact_id": artifact.ID, "title": artifact.Title, "created_at": artifact.CreatedAt, "verdict": verdict,
		})
		sources = append(sources, string(state))
		addAnswer := func(question, quote, answer string) {
			if strings.TrimSpace(answer) == "" {
				return
			}
			raw, _ := json.Marshal(map[string]any{
				"source":      "学生审核意见（设计判断，不是现场证据）",
				"artifact_id": artifact.ID, "artifact_created_at": artifact.CreatedAt,
				"review_question": question, "review_quote": quote, "student_answer": answer,
			})
			sources = append(sources, string(raw))
		}
		if artifact.SettledAt.Valid && artifact.Verdict != nil {
			addAnswer("学生审核结论："+*artifact.Verdict, "", artifact.Why)
		}
		marks, merr := a.d.Queries.ListPblReviewMarks(r.Context(), artifact.ID)
		if merr != nil {
			return out, merr
		}
		for _, mark := range marks {
			addAnswer(mark.Question, mark.Quote, mark.Answer)
		}
		dimensions, derr := a.d.Queries.ListPblReviewDimensions(r.Context(), artifact.ID)
		if derr != nil {
			return out, derr
		}
		for _, dimension := range dimensions {
			addAnswer(dimension.Prompt, "", dimension.Answer)
		}
	}
	// Creative homepage work also needs attribution checks: prior AI replies
	// must not become invented student biography or proof of a completed test.
	if in.Kind == "website" {
		if creative, loadErr := a.d.Queries.GetPblCreativeDirection(r.Context(), atomID); loadErr == nil {
			if context := creativeContext(creative.Document); context != "" {
				sources = append(sources, "已保存的学生创作构思与试用判断："+context)
			}
			for _, work := range in.ToolWork {
				sources = append(sources, "系统当前记录（其中AI建议或结构标题不等于学生原话，不能称刚才写过）："+work)
			}
		}
	}
	// A confirmed card is student input even when no chat message repeats it.
	// Exclude private drafts and AI candidates: a decision proves a choice,
	// never that a proposed investigation has already happened.
	decisions, err := a.d.Queries.ListPblDecisions(r.Context(), atomID)
	if err != nil {
		return out, err
	}
	for _, decision := range decisions {
		if !decision.SettledAt.Valid {
			continue
		}
		raw, _ := json.Marshal(map[string]any{
			"source":  "学生在理性决策卡中已确认的选择（不是已完成调查的证据）",
			"subject": decision.Subject, "choice": decision.Choice,
			"why": decision.Why, "why_not": decision.WhyNot,
			"reconsider_if": decision.Flip,
		})
		sources = append(sources, string(raw))
	}
	// Plans and choices must respect constraints and evidence limits even
	// before any observation notes or artifact reviews exist.
	hasPaperLayout := false
	if out.Produce != nil && out.Produce.Kind == "artifact" {
		var payload map[string]json.RawMessage
		if json.Unmarshal(out.Produce.Payload, &payload) == nil {
			hasPaperLayout = (len(payload["paperLayout"]) > 0 && string(payload["paperLayout"]) != "null") ||
				(len(payload["printLayout"]) > 0 && string(payload["printLayout"]) != "null")
		}
	}
	hasNewMission := out.Tool == "observe" && len(out.Mission) > 0
	if len(sources) == 0 && len(in.MissionVersions) == 0 && out.MissionTarget == "" && !hasNewMission && !in.Assigned && !hasPaperLayout && (out.Produce == nil || (out.Produce.Kind != "plan" && out.Produce.Kind != "decision")) {
		return out, nil
	}
	if strings.TrimSpace(in.Idea) != "" {
		origin := "学生项目起点："
		if in.Assigned {
			origin = "老师布置的驱动问题："
		} else if in.Kind == "website" {
			origin = "系统提供的主页起始问题："
		}
		sources = append(sources, origin+in.Idea)
		if in.Assigned && strings.TrimSpace(in.AssignedBrief) != "" {
			sources = append(sources, "老师补充说明："+in.AssignedBrief)
		}
	}
	for _, turn := range in.Recent {
		if turn.Role == "student" {
			sources = append(sources, "学生原话（按时间顺序）："+turn.Content)
		}
	}
	reviewer, err := a.routeE(r.Context(), gateway.ClassReview)
	if err != nil {
		return out, err
	}
	u, _ := UserFromContext(r.Context())
	sourceChars := 0
	for _, source := range sources {
		sourceChars += utf8.RuneCountInString(source)
	}
	for attempt := 0; attempt < 2; attempt++ {
		candidate, _ := json.Marshal(out)
		var issues []string
		repairSource := ""
		if out.MissionTarget != "" {
			if _, ok := in.MissionVersions[out.MissionTarget]; !ok {
				issues = append(issues, "观察清单目标不在本轮可修订范围内。只能使用当前上下文明确标出的mission_target；没有可修订清单时，使用tool=observe和完整mission创建新任务，mission_target留空。不得声称旧清单已更新，历史记录保留。")
			}
		}
		if out.Produce != nil && out.Produce.Kind == "artifact" {
			var paper struct {
				PaperEdits     []pbl.PaperEdit  `json:"paperEdits"`
				PaperLayout    *pbl.PaperLayout `json:"paperLayout"`
				PrintLayout    *pbl.PrintLayout `json:"printLayout"`
				BaseArtifactID string           `json:"baseArtifactId"`
				Edits          []pbl.TextEdit   `json:"edits"`
			}
			_ = json.Unmarshal(out.Produce.Payload, &paper)
			if paper.PrintLayout != nil {
				if err := paper.PrintLayout.Validate(); err != nil {
					issues = append(issues, "打印格式无效："+err.Error()+"。printLayout只支持六面手风琴折页；单张指引卡请删除printLayout，使用paperLayout或普通正文。保留学生选择的形式，不要为通过格式校验改成折页。")
				}
			}
			if paper.PaperLayout != nil {
				if paper.BaseArtifactID != "" || len(paper.Edits) > 0 || len(paper.PaperEdits) > 0 {
					issues = append(issues, "图形完整修订不能同时使用baseArtifactId或edits；请删除这两个字段，保留replacesArtifactId和完整paperLayout。")
				}
				if err := paper.PaperLayout.Validate(); err != nil {
					issues = append(issues, "纸面布局无法打印："+err.Error()+"。请修正完整paperLayout，保留学生要求与未验证说明。")
				}
			}
			if len(issues) == 0 && paper.BaseArtifactID == "" && (paper.PrintLayout != nil || paper.PaperLayout != nil) {
				// New graphical artifacts derive body from the layout at save
				// time. Review the same body, not an unused model-supplied copy.
				var final editedArtifact
				if json.Unmarshal(out.Produce.Payload, &final) == nil {
					if final.PaperLayout != nil {
						final.Body = final.PaperLayout.Markdown()
					} else {
						final.Body = final.PrintLayout.Markdown()
					}
					candidate, _ = json.Marshal(map[string]any{"response": out, "artifactToSave": final})
				}
			}
			var edit struct {
				Kind           string          `json:"kind"`
				BaseArtifactID string          `json:"baseArtifactId"`
				Edits          []pbl.TextEdit  `json:"edits"`
				PaperEdits     []pbl.PaperEdit `json:"paperEdits"`
				Guessed        []string        `json:"guessed"`
				Admits         []string        `json:"admits"`
			}
			if json.Unmarshal(out.Produce.Payload, &edit) == nil && edit.BaseArtifactID != "" && paper.PaperLayout == nil {
				// Validate before asking the semantic reviewer about content that
				// cannot be saved. Never relax matching or expose a foreign source.
				if final, err := a.materializeArtifactEdits(r.Context(), atomID, strings.TrimSpace(edit.Kind), edit.BaseArtifactID, edit.Edits, edit.Guessed, edit.Admits, edit.PaperEdits...); err == nil {
					candidate, _ = json.Marshal(map[string]any{
						"response": out, "artifactToSave": final,
					})
				} else {
					hint := "。请从当前原成果复制唯一匹配的old并提供不同的new；如需同步修改guessed或admits，请在局部修改中直接提供更新后的数组，保留其他正文。"
					if final.PaperLayout != nil {
						hint = "。图形局部修改使用baseArtifactId与paperEdits，old须从原paperLayout完整复制；整体重做请删除baseArtifactId与edits，使用replacesArtifactId和完整paperLayout。"
					}
					issues = append(issues, "成果局部修改无法应用："+err.Error()+hint)
					// materialization returns the owned, immutable source on edit
					// validation failures; authorization failures return no content.
					if final.Body != "" || final.PaperLayout != nil || final.PrintLayout != nil {
						source, _ := json.Marshal(final)
						repairSource = "\n本次替换必须使用的已保存原文（不是上一份失败候选，也不是历史修改对照）：baseArtifactId=" + edit.BaseArtifactID + "\n" + string(source)
					}
				}
			}
		}
		var usage gateway.ChatUsage
		repairKind := "structure"
		categories := make(map[string]int)
		if len(issues) == 0 {
			repairKind = "semantic"
			var check pbl.EvidenceCheck
			var checkErr error
			feedback := ""
			for formatAttempt := 0; formatAttempt < 2; formatAttempt++ {
				started := time.Now()
				var checkUsage gateway.ChatUsage
				check, checkUsage, checkErr = pbl.CheckEvidence(r.Context(), a.d.Provider, reviewer, sources, string(candidate), feedback)
				slog.Info("pbl model stage", "stage", "evidence", "attempt", attempt+1, "format_attempt", formatAttempt+1, "duration_ms", time.Since(started).Milliseconds(), "request_id", httpx.RequestIDFromContext(r.Context()), "failed", checkErr != nil, "supported", check.Supported,
					"source_count", len(sources), "source_chars", sourceChars, "candidate_chars", utf8.RuneCount(candidate), "text_chars", check.TextChars, "reasoning_chars", check.ReasoningChars, "stop_reason", check.StopReason,
					"input_tokens", checkUsage.InputTokens, "output_tokens", checkUsage.OutputTokens, "reasoning_tokens", checkUsage.ReasoningTokens)
				a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_evidence_check", reviewer, checkUsage)
				if !errors.Is(checkErr, pbl.ErrInvalidEvidence) {
					break
				}
				feedback = checkErr.Error()
			}
			if checkErr != nil {
				return out, checkErr
			}
			if check.Supported {
				return out, nil
			}
			for _, issue := range check.Issues {
				issues = append(issues, issue.Quote+"："+issue.Reason)
				categories[issue.Category]++
			}
		}
		slog.Info("pbl evidence rejection", "request_id", httpx.RequestIDFromContext(r.Context()), "attempt", attempt+1, "kind", repairKind, "issue_count", len(issues), "categories", categories)
		if attempt == 1 {
			return out, fmt.Errorf("回复证据核对失败：%s", strings.Join(issues, "；"))
		}
		in.ReviewFeedback = "上一份候选回复尚未发送，学生没有见过它。请基于原始记录重写完整返回，修正以下问题并保留真实性限定；不改变学生的任务。只呈现修正后的回应与产物，不叙述内部核对过程，不把这份未发送候选中的错误说成学生见过的上一轮回复。\n" + strings.Join(issues, "\n") + "\n尚未保存的候选（用于定位并修正，不是学生原话）：\n" + string(candidate)
		if out.Produce != nil {
			in.ReviewFeedback += "\n本轮正在执行学生要求的产物操作。修复必须返回修正后的完整produce对象及payload，核对正文、修改与假设和局限；不能删除produce、只改reply或声称原候选已经保存来避开错误。尚未保存的候选不是可依赖的完成记录。"
		}
		in.ReviewFeedback += repairSource
		started := time.Now()
		out, usage, err = pbl.Coach(r.Context(), a.d.Provider, dialogue, in)
		slog.Info("pbl model stage", "stage", "repair", "duration_ms", time.Since(started).Milliseconds(), "request_id", httpx.RequestIDFromContext(r.Context()), "failed", err != nil)
		a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_evidence_repair", dialogue, usage)
		if err != nil {
			return out, err
		}
	}
	return out, fmt.Errorf("回复证据核对未完成")
}
