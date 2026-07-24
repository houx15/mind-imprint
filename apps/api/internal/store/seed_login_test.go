package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

// Proves the pasted PHC hash in 0004_seed_password.sql actually verifies against
// the documented dev password — catches a bad paste.
func TestSeedPhoebeCanLogIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	u, err := q.GetUserByEmail(ctx, "phoebe@demo.mindimprint.local")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	ok, err := auth.VerifyPassword("phoebe-dev-pass", u.PasswordHash)
	if err != nil {
		t.Fatalf("VerifyPassword errored — hash malformed in migration: %v", err)
	}
	if !ok {
		t.Fatal("seed password does not verify — regenerate the hash in 0004")
	}
}

// Proves 0030_seed_teacher_password.sql's pasted PHC hash for the seeded
// teacher persona (吴老师, migration 0029) actually verifies against the
// documented dev password — without this, the demo's teacher persona is
// unreachable (0029 leaves password_hash as the non-PHC 'SEED_NO_LOGIN').
func TestSeedTeacherCanLogIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	u, err := q.GetUserByEmail(ctx, "wu.teacher@demo.mindimprint.local")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	ok, err := auth.VerifyPassword("phoebe-dev-pass", u.PasswordHash)
	if err != nil {
		t.Fatalf("VerifyPassword errored — hash malformed in migration: %v", err)
	}
	if !ok {
		t.Fatal("seed teacher password does not verify — regenerate the hash in 0030")
	}
}
