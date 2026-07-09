package post

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxBodyLength         = 200
	sqliteTimestampLayout = "2006-01-02 15:04:05"
	jstTimestampLayout    = "2006-01-02 15:04 MST"
)

var (
	ErrEmptyBody   = errors.New("post body is empty")
	ErrBodyTooLong = errors.New("post body is too long")
	jst            = time.FixedZone("JST", 9*60*60)
)

type Repository struct {
	db *sql.DB
}

type Post struct {
	ID           int64
	UserID       int64
	Body         string
	CreatedAt    string
	AuthorScreen string
	AuthorAvatar string
}

type Page struct {
	Posts        []Post
	HasNext      bool
	NextBeforeID int64
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func NormalizeBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	switch {
	case body == "":
		return "", ErrEmptyBody
	case utf8.RuneCountInString(body) > MaxBodyLength:
		return "", ErrBodyTooLong
	default:
		return body, nil
	}
}

func (r *Repository) Create(ctx context.Context, userID int64, body string) error {
	body, err := NormalizeBody(body)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, "INSERT INTO posts (user_id, body) VALUES (?, ?)", userID, body)
	return err
}

func (r *Repository) DeleteOwn(ctx context.Context, userID, postID int64) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE posts
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = ? AND user_id = ? AND deleted_at IS NULL
`, postID, userID)
	return err
}

func (r *Repository) List(ctx context.Context, limit int, beforeID int64) (Page, error) {
	return r.list(ctx, limit, beforeID, 0)
}

func (r *Repository) ListByUser(ctx context.Context, userID int64, limit int, beforeID int64) (Page, error) {
	return r.list(ctx, limit, beforeID, userID)
}

func (r *Repository) list(ctx context.Context, limit int, beforeID int64, userID int64) (Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
SELECT
  p.id,
  p.user_id,
  p.body,
  p.created_at,
  COALESCE(ai.screen_name, ''),
  u.avatar_url
FROM posts p
JOIN users u ON u.id = p.user_id
LEFT JOIN auth_identities ai ON ai.user_id = u.id AND ai.provider = 'esa'
WHERE p.deleted_at IS NULL
`
	args := []any{}
	if userID > 0 {
		query += "AND p.user_id = ?\n"
		args = append(args, userID)
	}
	if beforeID > 0 {
		query += "AND p.id < ?\n"
		args = append(args, beforeID)
	}
	query += `
ORDER BY p.id DESC
LIMIT ?
`
	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var p Post
		var createdAt string
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Body,
			&createdAt,
			&p.AuthorScreen,
			&p.AuthorAvatar,
		); err != nil {
			return Page{}, err
		}
		p.CreatedAt = formatCreatedAtJST(createdAt)
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}

	page := Page{Posts: posts}
	if len(posts) > limit {
		page.Posts = posts[:limit]
		page.HasNext = true
		page.NextBeforeID = page.Posts[len(page.Posts)-1].ID
	}
	return page, nil
}

func formatCreatedAtJST(value string) string {
	t, err := time.Parse(sqliteTimestampLayout, value)
	if err != nil {
		return value
	}
	return t.In(jst).Format(jstTimestampLayout)
}
