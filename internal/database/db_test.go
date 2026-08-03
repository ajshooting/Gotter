package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gotter/assets"
)

func TestOpenUsesPrivateDatabasePermissions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "gotter.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(ctx, db, assets.Migrations()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (avatar_url) VALUES ('')"); err != nil {
		t.Fatal(err)
	}

	assertPermissions(t, dir, 0o700)
	assertPermissions(t, path, 0o600)
	assertPermissions(t, path+"-wal", 0o600)
	assertPermissions(t, path+"-shm", 0o600)
}

func TestSecureDatabaseFilesRestrictsExistingSidecars(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "gotter.db")
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.WriteFile(candidate, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := secureDatabaseFiles(path); err != nil {
		t.Fatal(err)
	}

	assertPermissions(t, path, 0o600)
	assertPermissions(t, path+"-wal", 0o600)
	assertPermissions(t, path+"-shm", 0o600)
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s permissions = %04o, want %04o", path, got, want)
	}
}
