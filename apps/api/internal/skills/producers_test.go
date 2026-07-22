package skills_test

// producers_test.go — N3f Task 11. Every gate item in every skill must have a
// producer. See the plan's Task 11 for why this exists and what it does not
// prove.
//
// What this test is, and is not: it is a tripwire, not a proof. Someone can
// still satisfy it by adding a map entry that names a function which does not
// really record the item. What it makes impossible is the failure that
// actually shipped three times (S1/S2, then S3/S4 + S6): a gate item added to
// a skill's JSON with no producer anywhere, invisible because a seed
// migration hand-writes gate rows for the one demo project. The map entry
// forces the thought and names the producer where the next reader looks; it
// does not verify the named function actually behaves correctly.

import (
	"testing"

	"mindimprint/api/internal/skills"
)

// gateItemProducers names the function that records each non-machine gate
// item. Adding a gate item to a skill's JSON without adding a line here fails
// this test — that is the point. Verified against the real code as of N3f
// Task 11 (some entries also cover machine items produced by the same call,
// kept here for the reader even though the test below only requires
// student_written/human names to be present).
var gateItemProducers = map[string]string{
	// S0 decode_task
	"weakness_prediction": "api.submitOnboarding",
	"milestone_plan":      "api.submitOnboarding",
	// S1 frame_question
	"research_question":  "api.createProject (title node)",
	"provisional_answer": "api.submitFraming",
	"preregistration":    "api.submitFraming",
	"terms_defined":      "api.submitFraming",
	// S2 evaluate_perspectives
	"recon_logged":            "api.attestReconLogged (side effect of api.logSourceOpen)",
	"sources_per_perspective": "api.attestGate (explicit student confirm)",
	// S3 evaluate_sources
	"source_risk_notes":         "api.attestS3S4",
	"source_quality_spot_check": "api.orderSpotCheck",
	// S4 build_argument
	"warrants":                   "api.attestS3S4",
	"steelman":                   "api.attestS3S4",
	"warrant_quality_spot_check": "api.orderSpotCheck",
	// S5 draft_polish
	"citations_matched":  "api.attestGate",
	"whole_draft_review": "api.orderReview",
	// S6 reflect_archive
	"reflection":         "api.submitReflection",
	"declaration_signed": "api.signDeclaration",
}

// machineKinds is the closed set agent/gate.go's evalMachineItem handles. A
// kind outside it falls through to that switch's default and fails closed at
// runtime, which no test would otherwise catch until a student hit it.
// Verified against evalMachineItem's actual case list as of N3f Task 11.
var machineKinds = map[string]bool{
	"node_present":            true,
	"node_count_at_least":     true,
	"no_orphan_evidence":      true,
	"no_unsupported_claim":    true,
	"no_single_sourced_claim": true,
	"every_source_evaluated":  true,
}

func TestEveryGateItemHasAProducer(t *testing.T) {
	all, err := skills.Catalog()
	if err != nil {
		t.Fatalf("load skill catalog: %v", err)
	}
	for _, sk := range all {
		for contractID, c := range sk.Contracts {
			for _, m := range c.Gate.Machine {
				if !machineKinds[m.Kind] {
					t.Errorf("%s/%s: machine kind %q is not handled by evalMachineItem — it will fail closed at runtime",
						sk.ID, contractID, m.Kind)
				}
			}
			for _, name := range append(append([]string{}, c.Gate.StudentWritten...), c.Gate.Human...) {
				if _, ok := gateItemProducers[name]; !ok {
					t.Errorf("%s/%s: gate item %q has NO registered producer — a student can never clear this station. "+
						"Wire a producer, then add it to gateItemProducers in this file.",
						sk.ID, contractID, name)
				}
			}
		}
	}
}
