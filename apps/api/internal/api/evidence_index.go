package api

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/store/sqlc"
)

// evidence_index.go — G5 · the DB-backed evidence-candidate index builder. The
// (deferred) report generator is fed this compact, id-indexed set of every
// citable record in a project and told to cite ONLY ids that appear in it; at
// store time evalreport.ValidateRefs drops any id that slipped through. The
// pure index type + validator live in evalreport (DB-free, unit-tested); this
// is the sqlc-coupled aggregation, exported so the generator can call it too.

const evidenceLabelRunes = 60

// BuildEvidenceIndex gathers a project's citable records across all five
// streams (chat message, event, reference, card, exploration lead) into an
// id-indexed EvidenceIndex. Each candidate carries a short human label so the
// generator can pick the right id. A failure in any single stream fails the
// whole build (the generator wants a complete candidate set or none).
func BuildEvidenceIndex(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) (evalreport.EvidenceIndex, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	var cands []evalreport.Candidate

	msgs, err := q.ListChatMessagesByProject(ctx, pg)
	if err != nil {
		return evalreport.EvidenceIndex{}, err
	}
	for _, m := range msgs {
		who := "答"
		if m.Role == "user" {
			who = "问"
		}
		cands = append(cands, evalreport.Candidate{
			ID: m.ID.String(), Kind: evalreport.KindChat,
			Label: evalreport.Label(who+"："+m.Content, evidenceLabelRunes),
		})
	}

	events, err := q.ListEventsByProject(ctx, pg)
	if err != nil {
		return evalreport.EvidenceIndex{}, err
	}
	for _, e := range events {
		cands = append(cands, evalreport.Candidate{
			ID: e.ID.String(), Kind: evalreport.KindEvent,
			Label: evalreport.Label(e.Type, evidenceLabelRunes),
		})
	}

	refs, err := q.ListReferences(ctx, projectID)
	if err != nil {
		return evalreport.EvidenceIndex{}, err
	}
	for _, ref := range refs {
		cands = append(cands, evalreport.Candidate{
			ID: ref.ID.String(), Kind: evalreport.KindReference,
			Label: evalreport.Label(ref.Title, evidenceLabelRunes),
		})
	}

	cards, err := q.ListCardInstancesByProject(ctx, pg)
	if err != nil {
		return evalreport.EvidenceIndex{}, err
	}
	for _, c := range cards {
		cands = append(cands, evalreport.Candidate{
			ID: c.ID.String(), Kind: evalreport.KindCard,
			Label: evalreport.Label(c.CardID, evidenceLabelRunes),
		})
	}

	leads, err := q.ListExplorationLeads(ctx, projectID)
	if err != nil {
		return evalreport.EvidenceIndex{}, err
	}
	for _, l := range leads {
		cands = append(cands, evalreport.Candidate{
			ID: l.ID.String(), Kind: evalreport.KindLead,
			Label: evalreport.Label(l.Text, evidenceLabelRunes),
		})
	}

	return evalreport.NewEvidenceIndex(cands), nil
}
