package post

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

const MaxBodyLength = 200

var (
	ErrEmptyBody   = errors.New("post body is empty")
	ErrBodyTooLong = errors.New("post body is too long")
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

func (r *Repository) List(ctx context.Context, limit int) ([]Post, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := r.db.QueryContext(ctx, `
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
ORDER BY p.created_at DESC, p.id DESC
LIMIT ?
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var p Post
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Body,
			&p.CreatedAt,
			&p.AuthorScreen,
			&p.AuthorAvatar,
		); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
