package api

// writing_finish_wiring_test.go — Slice 5 (#20/#21) · PURE unit tests for the
// static wiring the two-stage 写作→回顾 flow depends on. No DB / no docker: these
// assert the package-level allowlists so a future edit can't silently break the
// reflection card shelf or the coach-propose gate. The DB-coupled handler guards
// (finish-writing empty-draft 422, finishProject writing-not-finished 422) live
// in the docker-based api_test suite.

import "testing"

// #21 · the reflection deck cards must be persistable (the shelf's summon +
// reflect turn both gate on persistableCard).
func TestPersistableCard_AcceptsReflectionDeck(t *testing.T) {
	for _, id := range []string{"learning-report", "metacognition", "knower-perspective"} {
		if !persistableCard(id) {
			t.Errorf("persistableCard(%q) = false, want true (reflection deck must persist)", id)
		}
	}
	// A card in no deck must still be refused (the allowlist didn't go open).
	if persistableCard("craap") {
		t.Errorf("persistableCard(craap) = true, want false (not in any deck)")
	}
}
