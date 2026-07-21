package agent

import "testing"

func TestMomentCardTableIsComplete(t *testing.T) {
	for _, m := range AllMoments {
		e, ok := momentCard[m]
		if !ok {
			t.Fatalf("moment %q has no momentCard entry", m)
		}
		if e.CardID == "" || e.Desc == "" || e.Flag == "" || e.Reason == "" || e.Criterion == "" {
			t.Fatalf("moment %q has an incomplete entry: %+v", m, e)
		}
	}
	if len(momentCard) != len(AllMoments) {
		t.Fatalf("momentCard has %d entries, AllMoments has %d", len(momentCard), len(AllMoments))
	}
}

func TestEligibleMomentsDropsAnyStatus(t *testing.T) {
	// An offer is never a wall: a card_instance in ANY status — including
	// skipped — makes its moment permanently ineligible.
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		got := EligibleMoments([]CardInstanceView{{CardID: "steelman", Status: status}})
		for _, m := range got {
			if m == MomentOneSided {
				t.Fatalf("status %q: one_sided still eligible after an offer", status)
			}
		}
		if len(got) != len(AllMoments)-1 {
			t.Fatalf("status %q: got %d eligible, want %d", status, len(got), len(AllMoments)-1)
		}
	}
}

func TestEligibleMomentsAllWhenNoCards(t *testing.T) {
	if got := EligibleMoments(nil); len(got) != len(AllMoments) {
		t.Fatalf("got %d eligible moments, want %d", len(got), len(AllMoments))
	}
}
