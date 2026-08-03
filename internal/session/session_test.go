package session

import (
	"database/sql"
	"testing"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	_ "github.com/mattn/go-sqlite3"
)

func TestNewManagerUsesSecureSessionDefaults(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	lifetime := 24 * time.Hour
	idleTimeout := 8 * time.Hour
	manager := NewManager(db, true, lifetime, idleTimeout)
	store := manager.Store.(*sqlite3store.SQLite3Store)
	t.Cleanup(store.StopCleanup)

	if !manager.HashTokenInStore {
		t.Fatal("HashTokenInStore = false, want true")
	}
	if manager.Lifetime != lifetime {
		t.Fatalf("Lifetime = %v, want %v", manager.Lifetime, lifetime)
	}
	if manager.IdleTimeout != idleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v", manager.IdleTimeout, idleTimeout)
	}
	if !manager.Cookie.HttpOnly || !manager.Cookie.Secure {
		t.Fatal("session cookie is missing HttpOnly or Secure")
	}
}
