package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gotter/internal/auth"
	"gotter/internal/config"
	"gotter/internal/post"
)

const (
	sessionUserIDKey = "user_id"
	oauthStateKey    = "oauth_state"
	csrfTokenKey     = "csrf_token"
	flashErrorKey    = "flash_error"
)

type App struct {
	cfg          config.Config
	sessions     *scs.SessionManager
	authProvider auth.Provider
	authStore    *auth.Store
	posts        *post.Repository
	templates    fs.FS
	static       fs.FS
}

type viewData struct {
	AppName         string
	CurrentUser     *auth.User
	AllowedTeam     string
	CSRFToken       string
	FlashError      string
	ErrorTitle      string
	ErrorMessage    string
	MaxBodyLength   int
	ProfileUser     *auth.User
	CurrentUserPath string
	PagePath        string
	EmptyMessage    string
	Posts           []post.Post
	HasNextPosts    bool
	NextBeforeID    int64
	HasLatestLink   bool
}

type currentUserContextKey struct{}

func New(
	cfg config.Config,
	sessions *scs.SessionManager,
	authProvider auth.Provider,
	authStore *auth.Store,
	posts *post.Repository,
	templates fs.FS,
	static fs.FS,
) *App {
	return &App{
		cfg:          cfg,
		sessions:     sessions,
		authProvider: authProvider,
		authStore:    authStore,
		posts:        posts,
		templates:    templates,
		static:       static,
	}
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(a.sessions.LoadAndSave)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(a.static))))

	r.Get("/login", a.handleLogin)
	r.Get("/auth/esa/start", a.handleOAuthStart)
	r.Get("/auth/esa/callback", a.handleOAuthCallback)

	r.Group(func(r chi.Router) {
		r.Use(a.requireAuth)
		r.Get("/", a.handleTimeline)
		r.Get("/users/{screen}", a.handleUser)
		r.Post("/posts", a.handleCreatePost)
		r.Post("/posts/{id}/delete", a.handleDeletePost)
		r.Post("/logout", a.handleLogout)
	})

	return r
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if a.sessions.GetInt64(r.Context(), sessionUserIDKey) != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	a.render(w, r, http.StatusOK, "login.html", viewData{
		AllowedTeam: a.cfg.ESAAllowedTeam,
		FlashError:  a.sessions.PopString(r.Context(), flashErrorKey),
	})
}

func (a *App) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	a.sessions.Put(r.Context(), oauthStateKey, state)
	http.Redirect(w, r, a.authProvider.AuthCodeURL(state), http.StatusFound)
}

func (a *App) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		a.renderError(w, r, http.StatusBadRequest, "Login canceled", oauthErr)
		return
	}

	expectedState := a.sessions.PopString(r.Context(), oauthStateKey)
	actualState := r.URL.Query().Get("state")
	if expectedState == "" || actualState == "" || !secureEqual(expectedState, actualState) {
		a.renderError(w, r, http.StatusBadRequest, "Invalid login state", "Please try signing in again.")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		a.renderError(w, r, http.StatusBadRequest, "Missing authorization code", "Please try signing in again.")
		return
	}

	profile, err := a.authProvider.FetchProfile(r.Context(), code)
	if errors.Is(err, auth.ErrNotAllowedTeam) {
		_ = a.sessions.Destroy(r.Context())
		a.renderError(w, r, http.StatusForbidden, "Access denied", "This account is not a member of the configured esa team.")
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	user, err := a.authStore.UpsertProfile(r.Context(), profile)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	if err := a.sessions.RenewToken(r.Context()); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.sessions.Put(r.Context(), sessionUserIDKey, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleTimeline(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := a.csrfToken(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	beforeID, err := parseBeforeID(r)
	if err != nil {
		a.renderError(w, r, http.StatusBadRequest, "Invalid page", "The requested timeline page could not be found.")
		return
	}

	page, err := a.posts.List(r.Context(), 50, beforeID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	a.render(w, r, http.StatusOK, "timeline.html", viewData{
		CurrentUser:   currentUser(r),
		AllowedTeam:   a.cfg.ESAAllowedTeam,
		CSRFToken:     csrfToken,
		FlashError:    a.sessions.PopString(r.Context(), flashErrorKey),
		MaxBodyLength: post.MaxBodyLength,
		PagePath:      "/",
		EmptyMessage:  "No posts yet.",
		Posts:         page.Posts,
		HasNextPosts:  page.HasNext,
		NextBeforeID:  page.NextBeforeID,
		HasLatestLink: beforeID > 0,
	})
}

func (a *App) handleUser(w http.ResponseWriter, r *http.Request) {
	csrfToken, err := a.csrfToken(r.Context())
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	beforeID, err := parseBeforeID(r)
	if err != nil {
		a.renderError(w, r, http.StatusBadRequest, "Invalid page", "The requested profile page could not be found.")
		return
	}

	screenName := chi.URLParam(r, "screen")
	profileUser, err := a.authStore.GetUserByScreenName(r.Context(), screenName)
	if errors.Is(err, sql.ErrNoRows) {
		a.renderError(w, r, http.StatusNotFound, "User not found", "The requested user could not be found.")
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	page, err := a.posts.ListByUser(r.Context(), profileUser.ID, 50, beforeID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	a.render(w, r, http.StatusOK, "user.html", viewData{
		CurrentUser:   currentUser(r),
		AllowedTeam:   a.cfg.ESAAllowedTeam,
		CSRFToken:     csrfToken,
		ProfileUser:   &profileUser,
		PagePath:      userPath(profileUser.ScreenName),
		EmptyMessage:  "No posts yet.",
		Posts:         page.Posts,
		HasNextPosts:  page.HasNext,
		NextBeforeID:  page.NextBeforeID,
		HasLatestLink: beforeID > 0,
	})
}

func (a *App) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	if !a.validCSRF(w, r) {
		return
	}

	body := r.PostForm.Get("body")
	if err := a.posts.Create(r.Context(), currentUser(r).ID, body); err != nil {
		if errors.Is(err, post.ErrEmptyBody) || errors.Is(err, post.ErrBodyTooLong) {
			a.sessions.Put(r.Context(), flashErrorKey, "Posts must be between 1 and 200 characters.")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		a.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	if !a.validCSRF(w, r) {
		return
	}

	postID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		a.renderError(w, r, http.StatusBadRequest, "Invalid post", "The requested post could not be found.")
		return
	}

	if err := a.posts.DeleteOwn(r.Context(), currentUser(r).ID, postID); err != nil {
		a.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !a.validCSRF(w, r) {
		return
	}

	if err := a.sessions.Destroy(r.Context()); err != nil {
		a.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := a.sessions.GetInt64(r.Context(), sessionUserIDKey)
		if userID == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := a.authStore.GetUser(r.Context(), userID)
		if errors.Is(err, sql.ErrNoRows) {
			_ = a.sessions.Destroy(r.Context())
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			a.serverError(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), currentUserContextKey{}, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUser(r *http.Request) *auth.User {
	user, _ := r.Context().Value(currentUserContextKey{}).(*auth.User)
	return user
}

func parseBeforeID(r *http.Request) (int64, error) {
	raw := r.URL.Query().Get("before")
	if raw == "" {
		return 0, nil
	}
	beforeID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || beforeID <= 0 {
		return 0, errors.New("before must be a positive integer")
	}
	return beforeID, nil
}

func (a *App) csrfToken(ctx context.Context) (string, error) {
	token := a.sessions.GetString(ctx, csrfTokenKey)
	if token != "" {
		return token, nil
	}

	token, err := randomToken()
	if err != nil {
		return "", err
	}
	a.sessions.Put(ctx, csrfTokenKey, token)
	return token, nil
}

func (a *App) validCSRF(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, http.StatusBadRequest, "Invalid form", "The submitted form could not be read.")
		return false
	}

	expected := a.sessions.GetString(r.Context(), csrfTokenKey)
	actual := r.PostForm.Get("csrf_token")
	if expected == "" || actual == "" || !secureEqual(expected, actual) {
		a.renderError(w, r, http.StatusForbidden, "Invalid form token", "Please reload the page and try again.")
		return false
	}
	return true
}

func (a *App) render(w http.ResponseWriter, r *http.Request, status int, page string, data viewData) {
	data.AppName = a.cfg.AppName
	if data.CurrentUser != nil && data.CurrentUser.ScreenName != "" {
		data.CurrentUserPath = userPath(data.CurrentUser.ScreenName)
	}

	tmpl, err := template.ParseFS(a.templates, "layout.html", "posts.html", page)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		a.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func userPath(screenName string) string {
	return "/users/" + url.PathEscape(screenName)
}

func (a *App) renderError(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	data := viewData{
		CurrentUser:  currentUser(r),
		AllowedTeam:  a.cfg.ESAAllowedTeam,
		ErrorTitle:   title,
		ErrorMessage: message,
	}
	if data.CurrentUser != nil {
		token, err := a.csrfToken(r.Context())
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		data.CSRFToken = token
	}

	a.render(w, r, status, "error.html", data)
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "request failed", "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
