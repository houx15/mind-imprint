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
