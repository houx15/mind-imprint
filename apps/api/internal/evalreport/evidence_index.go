package evalreport

// G5 — the evidence-candidate index + ref post-validation.
//
// The (future) generation algorithm must cite ONLY real record ids so the
// report's deep-links resolve. A free-writing model hallucinates ids. Two
// pieces of shared, deterministic infra guard against that:
//
//   - EvidenceIndex — a compact, id-indexed set of every citable record in a
//     project (chat message, event, reference, card, exploration lead), each
//     with a short human label. The generator is fed this and told to cite
//     only ids that appear in it.
//   - ValidateRefs — the post-validation belt-and-suspenders at store time:
//     any Ref.id / evidence id NOT in the index is blanked (label kept so the
//     UI still renders), and the drop rate is returned as a quality signal.
//
// This file is intentionally DB-free (no sqlc import) so it unit-tests without
// a database. The api layer builds the []Candidate from sqlc rows and calls
// NewEvidenceIndex; see api/evidence_index.go.

import "unicode/utf8"

// Kind values for a Candidate — which record stream an id came from.
const (
	KindChat      = "chat"
	KindEvent     = "event"
	KindReference = "reference"
	KindCard      = "card"
	KindLead      = "lead"
)

// Candidate is one citable record: a stable id, its stream, and a short label.
type Candidate struct {
	ID    string
	Kind  string
	Label string
}

// EvidenceIndex is an id → Candidate lookup over all of a project's citable
// records. The zero value is not usable; build with NewEvidenceIndex.
type EvidenceIndex struct {
	byID map[string]Candidate
}

// NewEvidenceIndex indexes candidates by id. Later candidates with a duplicate
// id win (harmless — ids are unique across streams in practice); empty ids are
// skipped.
func NewEvidenceIndex(cands []Candidate) EvidenceIndex {
	byID := make(map[string]Candidate, len(cands))
	for _, c := range cands {
		if c.ID == "" {
			continue
		}
		byID[c.ID] = c
	}
	return EvidenceIndex{byID: byID}
}

// Has reports whether id is a real, citable record.
func (ix EvidenceIndex) Has(id string) bool {
	_, ok := ix.byID[id]
	return ok
}

// Get returns the candidate for id.
func (ix EvidenceIndex) Get(id string) (Candidate, bool) {
	c, ok := ix.byID[id]
	return c, ok
}

// Len is the number of indexed candidates.
func (ix EvidenceIndex) Len() int { return len(ix.byID) }

// DropStats is ValidateRefs's quality signal: how many non-empty ref ids were
// examined and how many of those were blanked as unknown.
type DropStats struct {
	Total   int
	Dropped int
}

// ValidateRefs blanks every id in rep that does NOT resolve in idx, in place.
// A *Ref / PromptItem.Ref keeps its Label (the UI still renders text) but loses
// the dead id so no broken deep-link is offered; a bare evidence id string is
// cleared. Returns the examined/dropped counts. Empty ids are neither counted
// nor dropped (nothing to validate).
func ValidateRefs(rep *Report, idx EvidenceIndex) DropStats {
	var st DropStats
	blankPtr := func(ref *Ref) {
		if ref == nil || ref.ID == "" {
			return
		}
		st.Total++
		if !idx.Has(ref.ID) {
			ref.ID = ""
			st.Dropped++
		}
	}
	blankID := func(id string) string {
		if id == "" {
			return id
		}
		st.Total++
		if !idx.Has(id) {
			st.Dropped++
			return ""
		}
		return id
	}

	for i := range rep.Events {
		blankPtr(rep.Events[i].Ref)
	}
	for i := range rep.Materials {
		blankPtr(rep.Materials[i].UsedIn)
	}
	for i := range rep.Depth {
		for j := range rep.Depth[i].Evidence {
			rep.Depth[i].Evidence[j].ID = blankID(rep.Depth[i].Evidence[j].ID)
		}
	}
	for i := range rep.Autonomy {
		for j := range rep.Autonomy[i].Evidence {
			rep.Autonomy[i].Evidence[j].ID = blankID(rep.Autonomy[i].Evidence[j].ID)
		}
	}
	for i := range rep.PromptLens.Prompts {
		blankPtr(&rep.PromptLens.Prompts[i].Ref)
	}
	for i := range rep.Risks {
		blankPtr(rep.Risks[i].Ref)
	}
	return st
}

// Label truncates s to at most n runes for a candidate label, appending "…"
// when it had to cut. Whitespace-collapsed callers pass already-trimmed text.
func Label(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
