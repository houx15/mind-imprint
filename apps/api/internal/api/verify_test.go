package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

func TestVerifyEmail(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	ctx := context.Background()

	// Create an UNVERIFIED user directly (signup auto-verifies, so build one by hand).
	cls, err := q.GetClassByJoinCode(ctx, "DEMO-0001")
	if err != nil {
		t.Fatal(err)
	}
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Email: "unverified@demo.local", PasswordHash: "x", Role: "student",
		SchoolID: cls.SchoolID, DisplayName: "Un", AvatarColor: "#7C9CF0",
		EmailVerifiedAt: pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, hash, _ := auth.NewToken()
	if _, err := q.CreateEmailVerificationToken(ctx, sqlc.CreateEmailVerificationTokenParams{
		UserID: u.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// Verify → 200.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/verify-email",
		strings.NewReader(`{"token":"`+raw+`"}`)))
	if rr.Code != 200 {
		t.Fatalf("verify: want 200, got %d — %s", rr.Code, rr.Body.String())
	}

	// Reuse → 400 token_invalid_or_expired (consumed).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/verify-email",
		strings.NewReader(`{"token":"`+raw+`"}`)))
	if rr.Code != 400 || !strings.Contains(rr.Body.String(), "token_invalid_or_expired") {
		t.Fatalf("reuse: want 400 token_invalid_or_expired, got %d — %s", rr.Code, rr.Body.String())
	}

	// Garbage token → 400.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/verify-email",
		strings.NewReader(`{"token":"`+uuid.NewString()+`"}`)))
	if rr.Code != 400 {
		t.Fatalf("garbage: want 400, got %d", rr.Code)
	}
}
