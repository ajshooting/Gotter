package auth

import (
	"context"
	"path/filepath"
	"testing"

	"gotter/assets"
	dbpkg "gotter/internal/database"
)

func TestStoreGetsUserWithScreenName(t *testing.T) {
	t.Parallel()

	ctx, store := newTestStore(t)
	user, err := store.UpsertProfile(ctx, Profile{
		Provider:       esaProviderName,
		ProviderUserID: "esa-1",
		ScreenName:     "tester",
		DisplayName:    "Tester",
		AvatarURL:      "https://example.com/avatar.png",
	})
	if err != nil {
		t.Fatal(err)
	}

	byID, err := store.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if byID.ScreenName != "tester" {
		t.Fatalf("GetUser ScreenName = %q, want tester", byID.ScreenName)
	}

	byScreenName, err := store.GetUserByScreenName(ctx, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if byScreenName.ID != user.ID {
		t.Fatalf("GetUserByScreenName ID = %d, want %d", byScreenName.ID, user.ID)
	}
	if byScreenName.DisplayName != "Tester" {
		t.Fatalf("GetUserByScreenName DisplayName = %q, want Tester", byScreenName.DisplayName)
	}
}

func newTestStore(t *testing.T) (context.Context, *Store) {
	t.Helper()

	ctx := context.Background()
	db, err := dbpkg.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := dbpkg.Migrate(ctx, db, assets.Migrations()); err != nil {
		t.Fatal(err)
	}

	return ctx, NewStore(db)
}
