package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

func TestSeedAdminCanLogIn(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	u, err := q.GetUserByEmail(context.Background(), "admin@demo.mindimprint.local")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if u.Role != "admin" {
		t.Fatalf("role = %q, want admin", u.Role)
	}
	ok, err := auth.VerifyPassword("admin-dev-pass", u.PasswordHash)
	if err != nil || !ok {
		t.Fatalf("seeded admin password must verify: ok=%v err=%v", ok, err)
	}
	// Admin maps to no class.
	var enrollments int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM enrollments WHERE user_id=$1`, u.ID).Scan(&enrollments); err != nil {
		t.Fatalf("count enrollments: %v", err)
	}
	if enrollments != 0 {
		t.Fatalf("admin enrollments = %d, want 0", enrollments)
	}
}
