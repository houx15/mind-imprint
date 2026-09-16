package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/store/sqlc"
)

// A revision changes the existing shared candidate, never the student's draft.
// Stable option IDs keep the old reasons attached to the same candidate.
func (a *API) reviseDecision(ctx context.Context, atomID uuid.UUID, scope pgtype.UUID, raw json.RawMessage, versions map[string]int32) error {
	var in struct {
		DecisionID string `json:"decisionId"`
		Subject    string `json:"subject"`
		Reason     string `json:"reason"`
		Options    []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"options"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return err
	}
	id, err := uuid.Parse(in.DecisionID)
	if err != nil || strings.TrimSpace(in.Reason) == "" || len(in.Options) == 0 {
		return errors.New("决策修订缺少有效目标、修改原因或选项")
	}
	expectedVersion, presented := versions[in.DecisionID]
	if !presented {
		return errors.New("待修订决策未在本轮读取，请重新打开讨论")
	}
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := a.d.Queries.WithTx(tx)
	d, err := q.LockPblDecisionForRevision(ctx, id)
	if err != nil || d.AtomID != atomID || d.SessionID != scope {
		return errors.New("待修订决策不属于当前项目讨论")
	}
	if d.SettledAt.Valid || d.ContentVersion != expectedVersion {
		return errors.New("决策已确认或版本已变化，请重新读取后修订")
	}
	options, err := q.ListPblDecisionOptions(ctx, id)
	if err != nil {
		return err
	}
	byID := map[string]sqlc.PblDecisionOption{}
	for _, o := range options {
		byID[o.ID.String()] = o
	}
	seen := map[string]bool{}
	changed := strings.TrimSpace(in.Subject) != "" && strings.TrimSpace(in.Subject) != d.Subject
	for _, change := range in.Options {
		o, ok := byID[change.ID]
		if !ok || seen[change.ID] || o.Author != "yinji" || strings.TrimSpace(change.Label) == "" || utf8.RuneCountInString(change.Label) > 300 || utf8.RuneCountInString(change.Description) > 8000 {
			return errors.New("只能修订当前决策中已有的AI选项，且选项不得重复或超长")
		}
		seen[change.ID] = true
		changed = changed || o.Label != strings.TrimSpace(change.Label) || o.Description != strings.TrimSpace(change.Description)
	}
	if !changed {
		return nil
	}
	// Snapshot excludes draft and final reasons; neither belongs to AI revision history.
	snapshot, err := json.Marshal([]any{map[string]any{"version": d.ContentVersion, "subject": d.Subject, "reason": in.Reason, "options": toPblDecisionDTO(d, options, nil).Options}})
	if err != nil {
		return err
	}
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		subject = d.Subject
	}
	if utf8.RuneCountInString(subject) > 1000 || utf8.RuneCountInString(in.Reason) > 2000 {
		return errors.New("决策主题或修改原因过长")
	}
	for _, change := range in.Options {
		err = q.RevisePblDecisionOption(ctx, sqlc.RevisePblDecisionOptionParams{ID: byID[change.ID].ID, Label: strings.TrimSpace(change.Label), Description: strings.TrimSpace(change.Description)})
		if err != nil {
			return err
		}
	}
	if err = q.RevisePblDecision(ctx, sqlc.RevisePblDecisionParams{ID: id, Subject: subject, Column3: snapshot}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
