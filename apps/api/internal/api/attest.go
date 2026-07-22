package api

// attest.go — N3f Task 1. Three S3/S4 student_written gate items whose
// WRITING already had a producer but whose ATTESTATION did not, so no real
// student could ever clear evaluate_sources or build_argument. Same
// attestation-by-endpoint pattern as N3d (advance.go's attestReconLogged):
// the endpoint that persists the writing records the item — no checkbox.
//
// RL-5: these check that she DID the work, never how well. There is
// deliberately no length floor — the CRAAP card's own
// field_written_by(risk_note, student) completion predicate is already the
// floor, and a second, different threshold on the same text would let a card
// complete while its gate item stayed missing with nothing on screen
// explaining the discrepancy.

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// riskNoteText reads body.source_quality.risk_note off a minted evidence node.
// Mirrors studio/projection.go's riskNote — kept local because the projection
// one is unexported and this package must not depend on projection internals.
func riskNoteText(body []byte) string {
	var b struct {
		SourceQuality map[string]string `json:"source_quality"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return ""
	}
	return b.SourceQuality["risk_note"]
}

// allArticlesHaveRiskNote reports whether EVERY kind:"article" material has an
// evaluated-as edge to a node carrying a non-blank risk_note.
//
// Unlike the every_source_evaluated machine predicate (agent/gate.go:54),
// which passes vacuously over an empty material list by design, this returns
// false when the project has no article materials: an empty dossier has not
// done S3's work, and marking the item solid there would let a student clear
// 信源评估 having evaluated nothing.
func allArticlesHaveRiskNote(materials []sqlc.Material, nodes []sqlc.GraphNode, edges []sqlc.GraphEdge) bool {
	byID := make(map[string]sqlc.GraphNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID.String()] = n
	}
	evidence := map[string]sqlc.GraphNode{}
	for _, e := range edges {
		if e.Type != "evaluated-as" || e.FromKind != "material" {
			continue
		}
		if n, ok := byID[e.ToID.String()]; ok {
			evidence[e.FromID.String()] = n
		}
	}
	articles := 0
	for _, m := range materials {
		if m.Kind != "article" {
			continue
		}
		articles++
		n, ok := evidence[m.ID.String()]
		if !ok || strings.TrimSpace(riskNoteText(n.Body)) == "" {
			return false
		}
	}
	return articles > 0
}

// slotHasText reports whether a Toulmin slot's node exists with non-blank
// body.text. Slot id == node type, and "written" means the same thing the 结构
// pane means by "done" (studio/projection.go:692–703) — one definition of
// written, so the gate and the screen can never disagree.
func slotHasText(nodes []sqlc.GraphNode, nodeType string) bool {
	for _, n := range nodes {
		if n.Type != nodeType {
			continue
		}
		var b struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(n.Body, &b) == nil && strings.TrimSpace(b.Text) != "" {
			return true
		}
	}
	return false
}

// hasCompletedSteelmanCard reports whether the standalone 钢人卡 was completed.
// That card has no graph_effects and mints no node, so without this a student
// who did the steelman card and not the Toulmin counter slot would write a
// steelman and get no credit for it. A SKIPPED card never counts.
func hasCompletedSteelmanCard(cards []sqlc.CardInstance) bool {
	for _, c := range cards {
		if c.CardID == "steelman" && c.Status == "completed" {
			return true
		}
	}
	return false
}

// attestS3S4 recomputes the three S3/S4 student_written items from graph
// state and records them. Like advanceGates, gate state is DERIVED and
// fully recomputable, so a failure here must never fail the student's write
// — the next gate-affecting write recomputes it from scratch. Hence: logs,
// returns nothing.
//
// Both sets AND clears each item (attestGate's set/delete pattern): deleting
// a material or blanking a slot must be able to re-open the gate. A gate
// that can only ever close is a gate that lies after a deletion.
func (a *API) attestS3S4(ctx context.Context, projectID uuid.UUID) {
	materials, err := a.d.Queries.ListMaterialsByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		slog.Warn("attest s3s4: list materials", "err", err, "project_id", projectID.String())
		return
	}
	nodes, err := a.d.Queries.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		slog.Warn("attest s3s4: list nodes", "err", err, "project_id", projectID.String())
		return
	}
	edges, err := a.d.Queries.ListGraphEdgesByProject(ctx, projectID)
	if err != nil {
		slog.Warn("attest s3s4: list edges", "err", err, "project_id", projectID.String())
		return
	}
	cards, err := a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		slog.Warn("attest s3s4: list cards", "err", err, "project_id", projectID.String())
		return
	}

	want := map[string]map[string]bool{
		"evaluate_sources": {
			"source_risk_notes": allArticlesHaveRiskNote(materials, nodes, edges),
		},
		"build_argument": {
			"warrants": slotHasText(nodes, "warrant"),
			"steelman": slotHasText(nodes, "counter") || hasCompletedSteelmanCard(cards),
		},
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(ctx, projectID)
	if err != nil {
		slog.Warn("attest s3s4: list gate states", "err", err, "project_id", projectID.String())
		return
	}
	for contract, items := range want {
		rec := recorded[contract]
		if rec.Items == nil {
			rec.Items = map[string]string{}
		}
		for name, ok := range items {
			if ok {
				rec.Items[name] = "solid"
			} else {
				delete(rec.Items, name)
			}
		}
		if err := store.UpsertGateState(ctx, projectID, contract, rec); err != nil {
			slog.Warn("attest s3s4: upsert gate state", "err", err, "contract", contract, "project_id", projectID.String())
		}
	}
}
