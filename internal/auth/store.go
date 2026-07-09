package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID          int64
	DisplayName string
	AvatarURL   string
	ScreenName  string
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
SELECT u.id, u.display_name, u.avatar_url, COALESCE(ai.screen_name, '')
FROM users u
LEFT JOIN auth_identities ai ON ai.user_id = u.id AND ai.provider = ?
WHERE u.id = ?
`, esaProviderName, id).Scan(&user.ID, &user.DisplayName, &user.AvatarURL, &user.ScreenName)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) GetUserByScreenName(ctx context.Context, screenName string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
SELECT u.id, u.display_name, u.avatar_url, ai.screen_name
FROM auth_identities ai
JOIN users u ON u.id = ai.user_id
WHERE ai.provider = ? AND ai.screen_name = ?
`, esaProviderName, screenName).Scan(&user.ID, &user.DisplayName, &user.AvatarURL, &user.ScreenName)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) UpsertProfile(ctx context.Context, profile Profile) (User, error) {
	profile = profile.normalized()
	if profile.Provider == "" || profile.ProviderUserID == "" {
		return User{}, errors.New("auth profile requires provider and provider user id")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	userID, exists, err := findIdentityUserID(ctx, tx, profile.Provider, profile.ProviderUserID)
	if err != nil {
		return User{}, err
	}

	if exists {
		if err := updateExistingProfile(ctx, tx, userID, profile); err != nil {
			return User{}, err
		}
	} else {
		userID, err = insertProfile(ctx, tx, profile)
		if err != nil {
			return User{}, err
		}
	}

	user, err := getUserTx(ctx, tx, userID)
	if err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func findIdentityUserID(ctx context.Context, tx *sql.Tx, provider, providerUserID string) (int64, bool, error) {
	var userID int64
	err := tx.QueryRowContext(ctx, `
SELECT user_id
FROM auth_identities
WHERE provider = ? AND provider_user_id = ?
`, provider, providerUserID).Scan(&userID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	return userID, err == nil, err
}

func updateExistingProfile(ctx context.Context, tx *sql.Tx, userID int64, profile Profile) error {
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET display_name = ?, avatar_url = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
`, profile.DisplayName, profile.AvatarURL, userID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE auth_identities
SET screen_name = ?, email = ?, display_name = ?, avatar_url = ?, updated_at = CURRENT_TIMESTAMP
WHERE provider = ? AND provider_user_id = ?
`,
		profile.ScreenName,
		profile.Email,
		profile.DisplayName,
		profile.AvatarURL,
		profile.Provider,
		profile.ProviderUserID,
	); err != nil {
		return err
	}

	return nil
}

func insertProfile(ctx context.Context, tx *sql.Tx, profile Profile) (int64, error) {
	result, err := tx.ExecContext(ctx, `
INSERT INTO users (display_name, avatar_url)
VALUES (?, ?)
`, profile.DisplayName, profile.AvatarURL)
	if err != nil {
		return 0, err
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO auth_identities (
  user_id,
  provider,
  provider_user_id,
  screen_name,
  email,
  display_name,
  avatar_url
) VALUES (?, ?, ?, ?, ?, ?, ?)
`,
		userID,
		profile.Provider,
		profile.ProviderUserID,
		profile.ScreenName,
		profile.Email,
		profile.DisplayName,
		profile.AvatarURL,
	); err != nil {
		return 0, fmt.Errorf("insert auth identity: %w", err)
	}

	return userID, nil
}

func getUserTx(ctx context.Context, tx *sql.Tx, id int64) (User, error) {
	var user User
	err := tx.QueryRowContext(ctx, `
SELECT u.id, u.display_name, u.avatar_url, COALESCE(ai.screen_name, '')
FROM users u
LEFT JOIN auth_identities ai ON ai.user_id = u.id AND ai.provider = ?
WHERE u.id = ?
`, esaProviderName, id).Scan(&user.ID, &user.DisplayName, &user.AvatarURL, &user.ScreenName)
	if err != nil {
		return User{}, err
	}
	return user, nil
}
