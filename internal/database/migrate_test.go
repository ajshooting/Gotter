package database

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"gotter/assets"
)

func TestMigrateRemovesStoredDisplayNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	allMigrations := assets.Migrations()
	legacyMigrations := fstest.MapFS{}
	for _, name := range []string{"001_init.sql", "002_post_likes.sql"} {
		body, err := fs.ReadFile(allMigrations, name)
		if err != nil {
			t.Fatal(err)
		}
		legacyMigrations[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, db, legacyMigrations); err != nil {
		t.Fatal(err)
	}

	result, err := db.ExecContext(ctx, `
INSERT INTO users (display_name, avatar_url)
VALUES ('Real Name', 'https://example.com/avatar.png')
`)
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
) VALUES (?, 'esa', 'esa-1', 'tester', '', 'Real Name', 'https://example.com/avatar.png')
`, userID); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, db, allMigrations); err != nil {
		t.Fatal(err)
	}

	assertColumnAbsent(t, ctx, db, "users", "display_name")
	assertColumnAbsent(t, ctx, db, "auth_identities", "display_name")

	var screenName string
	if err := db.QueryRowContext(ctx, `
SELECT screen_name
FROM auth_identities
WHERE user_id = ?
`, userID).Scan(&screenName); err != nil {
		t.Fatal(err)
	}
	if screenName != "tester" {
		t.Fatalf("screen_name = %q, want tester", screenName)
	}
}

func assertColumnAbsent(t *testing.T, ctx context.Context, db queryer, table, column string) {
	t.Helper()

	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			position   int
			name       string
			dataType   string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&position, &name, &dataType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == column {
			t.Fatalf("%s.%s still exists", table, column)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
