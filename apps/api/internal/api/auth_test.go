package api

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUserContextRoundTrip(t *testing.T) {
	u := User{ID: uuid.New(), SchoolID: uuid.New(), Role: "student", DisplayName: "Phoebe"}
	ctx := WithUser(context.Background(), u)
	got, ok := UserFromContext(ctx)
	if !ok || got.ID != u.ID {
		t.Fatalf("round-trip failed: %v %v", got, ok)
	}

	if _, ok := UserFromContext(context.Background()); ok {
		t.Fatal("empty context must yield ok=false")
	}
}

func TestHasEntitlementStubTrue(t *testing.T) {
	ok, err := HasEntitlement(context.Background(), User{})
	if err != nil || !ok {
		t.Fatalf("stub must be open: ok=%v err=%v", ok, err)
	}
}
