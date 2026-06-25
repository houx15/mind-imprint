package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

var seededClassID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

func TestAuthQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	// Join code resolves to the seeded class.
	cls, err := q.GetClassByJoinCode(ctx, "DEMO-0001")
	if err != nil || cls.ID != seededClassID {
		t.Fatalf("GetClassByJoinCode: %v id=%v", err, cls.ID)
	}

	// Atomic signup: create user + enrollment in a tx.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	qtx := q.WithTx(tx)
	u, err := qtx.CreateUser(ctx, sqlc.CreateUserParams{
		Email:           "newkid@demo.local",
		PasswordHash:    "$argon2id$placeholder",
		Role:            "student",
		SchoolID:        cls.SchoolID,
		DisplayName:     "New Kid",
		AvatarColor:     "#7C9CF0",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := qtx.CreateEnrollment(ctx, sqlc.CreateEnrollmentParams{
		UserID: u.ID, ClassID: cls.ID, RoleInClass: "student",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// GetUserByEmail finds it; classes list it.
	got, err := q.GetUserByEmail(ctx, "newkid@demo.local")
	if err != nil || got.ID != u.ID {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	classes, err := q.ListClassesForUser(ctx, u.ID)
	if err != nil || len(classes) != 1 || classes[0].ID != cls.ID {
		t.Fatalf("ListClassesForUser: %v %+v", err, classes)
	}

	// Sessions: create, resolve, delete.
	sess, err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID: u.ID, TokenHash: "hash-abc", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := q.GetSessionWithUserByHash(ctx, "hash-abc")
	if err != nil || row.ID != u.ID || row.SchoolID != cls.SchoolID {
		t.Fatalf("GetSessionWithUserByHash: %v %+v", err, row)
	}
	if err := q.DeleteSessionByHash(ctx, "hash-abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSessionWithUserByHash(ctx, "hash-abc"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("after delete want ErrNoRows, got %v", err)
	}
	_ = sess

	// Expired session is not resolved.
	if _, err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID: u.ID, TokenHash: "hash-expired", ExpiresAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSessionWithUserByHash(ctx, "hash-expired"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired want ErrNoRows, got %v", err)
	}

	// Verification token: active lookup, consume, then inactive.
	evt, err := q.CreateEmailVerificationToken(ctx, sqlc.CreateEmailVerificationTokenParams{
		UserID: u.ID, TokenHash: "vt-hash", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetActiveEmailVerificationToken(ctx, "vt-hash"); err != nil {
		t.Fatalf("active token: %v", err)
	}
	if err := q.ConsumeEmailVerificationToken(ctx, evt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetActiveEmailVerificationToken(ctx, "vt-hash"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("consumed token want ErrNoRows, got %v", err)
	}
	if _, err := q.MarkEmailVerified(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	// GetSchool resolves.
	if _, err := q.GetSchool(ctx, cls.SchoolID); err != nil {
		t.Fatalf("GetSchool: %v", err)
	}
}
