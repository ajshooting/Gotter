package post

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"gotter/assets"
	dbpkg "gotter/internal/database"
)

func TestNormalizeBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "trims whitespace", input: "  hello  ", want: "hello"},
		{name: "empty", input: "   ", wantErr: ErrEmptyBody},
		{name: "too long", input: strings.Repeat("あ", MaxBodyLength+1), wantErr: ErrBodyTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeBody(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepositoryListPaginatesByCursor(t *testing.T) {
	t.Parallel()

	ctx, repo, _, userID := newTestRepository(t)
	for i := 1; i <= 5; i++ {
		if err := repo.Create(ctx, userID, fmt.Sprintf("post %d", i)); err != nil {
			t.Fatal(err)
		}
	}

	first, err := repo.List(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertPage(t, first, []string{"post 5", "post 4"}, true)

	second, err := repo.List(ctx, 2, first.NextBeforeID)
	if err != nil {
		t.Fatal(err)
	}
	assertPage(t, second, []string{"post 3", "post 2"}, true)

	third, err := repo.List(ctx, 2, second.NextBeforeID)
	if err != nil {
		t.Fatal(err)
	}
	assertPage(t, third, []string{"post 1"}, false)
}

func TestRepositoryDeleteOwnSoftDeletes(t *testing.T) {
	t.Parallel()

	ctx, repo, db, userID := newTestRepository(t)
	if err := repo.Create(ctx, userID, "keep this row"); err != nil {
		t.Fatal(err)
	}

	page, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Posts) != 1 {
		t.Fatalf("len(posts) = %d, want 1", len(page.Posts))
	}
	postID := page.Posts[0].ID

	if err := repo.DeleteOwn(ctx, userID, postID); err != nil {
		t.Fatal(err)
	}

	page, err = repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Posts) != 0 {
		t.Fatalf("len(posts) after delete = %d, want 0", len(page.Posts))
	}

	var deletedAt sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT deleted_at FROM posts WHERE id = ?", postID).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if !deletedAt.Valid || deletedAt.String == "" {
		t.Fatal("deleted post row was not retained with deleted_at")
	}
}

func TestFormatCreatedAtJST(t *testing.T) {
	t.Parallel()

	got := formatCreatedAtJST("2026-07-06 00:30:00")
	want := "2026-07-06 09:30 JST"
	if got != want {
		t.Fatalf("formatCreatedAtJST() = %q, want %q", got, want)
	}
}

func newTestRepository(t *testing.T) (context.Context, *Repository, *sql.DB, int64) {
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

	result, err := db.ExecContext(ctx, "INSERT INTO users (display_name, avatar_url) VALUES (?, ?)", "Tester", "")
	if err != nil {
		t.Fatal(err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO auth_identities (
  user_id,
  provider,
  provider_user_id,
  screen_name,
  email,
  display_name,
  avatar_url
) VALUES (?, 'esa', 'esa-1', 'tester', '', 'Tester', '')
`, userID); err != nil {
		t.Fatal(err)
	}

	return ctx, NewRepository(db), db, userID
}

func assertPage(t *testing.T, page Page, wantBodies []string, wantNext bool) {
	t.Helper()

	if len(page.Posts) != len(wantBodies) {
		t.Fatalf("len(posts) = %d, want %d", len(page.Posts), len(wantBodies))
	}
	for i, want := range wantBodies {
		if page.Posts[i].Body != want {
			t.Fatalf("posts[%d].Body = %q, want %q", i, page.Posts[i].Body, want)
		}
	}
	if page.HasNext != wantNext {
		t.Fatalf("HasNext = %v, want %v", page.HasNext, wantNext)
	}
	if wantNext && page.NextBeforeID == 0 {
		t.Fatal("NextBeforeID = 0, want cursor")
	}
}
