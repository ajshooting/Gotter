package session

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

func NewManager(db *sql.DB, secureCookie bool) *scs.SessionManager {
	manager := scs.New()
	manager.Store = sqlite3store.New(db)
	manager.Lifetime = 14 * 24 * time.Hour
	manager.Cookie.Name = "gotter_session"
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = secureCookie
	manager.Cookie.Persist = true
	return manager
}
