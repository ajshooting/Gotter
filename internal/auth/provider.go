package auth

import (
	"context"
	"errors"
)

var ErrNotAllowedTeam = errors.New("not a member of the allowed esa team")

type Provider interface {
	AuthCodeURL(state string) string
	FetchProfile(ctx context.Context, code string) (Profile, error)
}

type Profile struct {
	Provider       string
	ProviderUserID string
	ScreenName     string
	Email          string
	DisplayName    string
	AvatarURL      string
}

func (p Profile) normalized() Profile {
	if p.DisplayName == "" {
		p.DisplayName = p.ScreenName
	}
	if p.ProviderUserID == "" {
		p.ProviderUserID = p.ScreenName
	}
	return p
}
