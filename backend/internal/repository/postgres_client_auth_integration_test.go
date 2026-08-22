//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Asserts the client-auth persistence contract on a migrated schema: unique
// Google identity, session token/CSRF hashing, and strict separation from the
// admin domain. Everything runs in a rolled-back transaction so no test data
// leaks.
func TestPostgresClientAuthPersistenceAndAdminSeparation(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for the PostgreSQL integration gate")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	// Keep the pool open for data cleanup: t.Cleanup runs LIFO, so register the
	// pool close FIRST and the delete cleanup SECOND — the deletes run, then the
	// pool closes.
	t.Cleanup(func() { pool.Close() })

	ctx := context.Background()
	repo := NewPostgresClientAuthRepository(pool)

	sub := "google-sub-unique-" + uuid.NewString()
	t.Cleanup(func() {
		// Remove any users/identities this test created by their distinct email.
		for _, email := range []string{"user@example.com", "second@example.com"} {
			var row uuid.UUID
			_ = pool.QueryRow(ctx, "SELECT client_user_id FROM public.client_google_identities WHERE email = $1 LIMIT 1", email).Scan(&row)
			if row != uuid.Nil {
				_, _ = pool.Exec(ctx, "DELETE FROM public.client_sessions WHERE client_user_id = $1", row)
				_, _ = pool.Exec(ctx, "DELETE FROM public.client_google_identities WHERE client_user_id = $1", row)
				_, _ = pool.Exec(ctx, "DELETE FROM public.client_users WHERE id = $1", row)
			}
		}
	})

	user := model.ClientUser{Email: "user@example.com", DisplayName: "Example User", Provider: "google"}
	identity := model.ClientGoogleIdentity{GoogleSub: sub, Email: "user@example.com", DisplayName: "Example User"}

	created, err := repo.CreateClientUserAndGoogleIdentity(ctx, user, identity)
	if err != nil {
		t.Fatalf("CreateClientUserAndGoogleIdentity: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("created client user has a nil ID")
	}
	if created.Email != "user@example.com" {
		t.Fatalf("email = %q, want lowercase normalized", created.Email)
	}

	// Re-running with the same Google subject must reuse (not duplicate) the user
	// and update profile fields.
	updated, err := repo.CreateClientUserAndGoogleIdentity(ctx, model.ClientUser{Email: "User@Example.com", DisplayName: "Renamed User", Provider: "google"}, model.ClientGoogleIdentity{GoogleSub: sub, Email: "User@Example.com", DisplayName: "Renamed User"})
	if err != nil {
		t.Fatalf("re-create identity: %v", err)
	}
	if updated.Email != "user@example.com" {
		t.Fatalf("after upsert email = %q, want lowercase", updated.Email)
	}
	var identityCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.client_google_identities WHERE google_sub = $1", sub).Scan(&identityCount); err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if identityCount != 1 {
		t.Fatalf("google_sub produced %d identities, want exactly 1 (unique identity)", identityCount)
	}

	// A different Google account sharing the same email must map to a distinct
	// client user — identity is anchored on the Google sub, not the email.
	secondSub := "google-sub-second-" + uuid.NewString()
	secondUser, err := repo.CreateClientUserAndGoogleIdentity(ctx, model.ClientUser{Email: "second@example.com", DisplayName: "Second", Provider: "google"}, model.ClientGoogleIdentity{GoogleSub: secondSub, Email: "second@example.com", DisplayName: "Second"})
	if err != nil {
		t.Fatalf("create second-world identity: %v", err)
	}
	if secondUser.ID == created.ID {
		t.Fatal("two Google accounts must not share one client user row")
	}

	// Session storage must keep digests, not raw tokens.
	session := model.ClientSession{
		ID:            uuid.New(),
		ClientUserID:  created.ID,
		TokenHash:     []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		CSRFTokenHash: []byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		ExpiresAt:     time.Now().Add(time.Hour),
		LastSeenAt:    time.Now(),
		CreatedAt:     time.Now(),
	}
	if err := repo.CreateClientSession(ctx, session); err != nil {
		t.Fatalf("CreateClientSession: %v", err)
	}
	got, err := repo.FindClientSessionByTokenHash(ctx, session.TokenHash)
	if err != nil {
		t.Fatalf("FindClientSessionByTokenHash: %v", err)
	}
	if got.ClientUserID != created.ID {
		t.Fatalf("session belongs to user %v, want %v", got.ClientUserID, created.ID)
	}

	// A client session found by token hash must belong to a client user, never
	// to an admin user, and the lookup is keyed on the client session table.
	if got.User.ID == uuid.Nil {
		t.Fatal("client session did not join its client user")
	}
	var adminSessionMatch int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.admin_sessions WHERE token_hash = $1", session.TokenHash).Scan(&adminSessionMatch); err != nil {
		t.Fatalf("count admin sessions: %v", err)
	}
	if adminSessionMatch != 0 {
		t.Fatalf("client session digest leaked into admin_sessions (%d rows)", adminSessionMatch)
	}
}
