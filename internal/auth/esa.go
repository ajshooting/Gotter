package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

const esaProviderName = "esa"

type ESAProvider struct {
	config      oauth2.Config
	allowedTeam string
	httpClient  *http.Client
}

func NewESAProvider(clientID, clientSecret, redirectURL, allowedTeam string) *ESAProvider {
	return &ESAProvider{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"read"},
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://api.esa.io/oauth/authorize",
				TokenURL:  "https://api.esa.io/oauth/token",
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
		allowedTeam: allowedTeam,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *ESAProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *ESAProvider) FetchProfile(ctx context.Context, code string) (Profile, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return Profile{}, fmt.Errorf("exchange esa oauth code: %w", err)
	}

	client := p.config.Client(ctx, token)

	tokenInfo, err := p.fetchTokenInfo(ctx, client)
	if err != nil {
		return Profile{}, err
	}

	allowed, err := p.belongsToAllowedTeam(ctx, client)
	if err != nil {
		return Profile{}, err
	}
	if !allowed {
		return Profile{}, ErrNotAllowedTeam
	}

	member, err := p.fetchMember(ctx, client)
	if err != nil {
		return Profile{}, err
	}

	providerUserID := strconv.FormatInt(tokenInfo.UserID(), 10)
	if providerUserID == "0" {
		providerUserID = member.ScreenName
	}

	return Profile{
		Provider:       esaProviderName,
		ProviderUserID: providerUserID,
		ScreenName:     member.ScreenName,
		Email:          member.Email,
		DisplayName:    member.Name,
		AvatarURL:      member.Icon,
	}.normalized(), nil
}

func (p *ESAProvider) fetchTokenInfo(ctx context.Context, client *http.Client) (esaTokenInfo, error) {
	var result esaTokenInfo
	if err := getJSON(ctx, client, "https://api.esa.io/oauth/token/info", &result); err != nil {
		return esaTokenInfo{}, fmt.Errorf("fetch esa token info: %w", err)
	}
	return result, nil
}

func (p *ESAProvider) belongsToAllowedTeam(ctx context.Context, client *http.Client) (bool, error) {
	for page := 1; ; page++ {
		u, err := url.Parse("https://api.esa.io/v1/teams")
		if err != nil {
			return false, err
		}
		q := u.Query()
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", "100")
		u.RawQuery = q.Encode()

		var result esaTeamsResponse
		if err := getJSON(ctx, client, u.String(), &result); err != nil {
			return false, fmt.Errorf("fetch esa teams: %w", err)
		}

		for _, team := range result.Teams {
			if team.Name == p.allowedTeam {
				return true, nil
			}
		}

		if result.NextPage == nil {
			return false, nil
		}
	}
}

func (p *ESAProvider) fetchMember(ctx context.Context, client *http.Client) (esaMember, error) {
	endpoint := fmt.Sprintf(
		"https://api.esa.io/v1/teams/%s/members/me",
		url.PathEscape(p.allowedTeam),
	)
	var member esaMember
	if err := getJSON(ctx, client, endpoint, &member); err != nil {
		return esaMember{}, fmt.Errorf("fetch esa member profile: %w", err)
	}
	return member, nil
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("GET %s: %s: %s", endpoint, resp.Status, string(body))
	}

	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(target)
}

type esaTokenInfo struct {
	ResourceOwnerID int64 `json:"resource_owner_id"`
	User            struct {
		ID int64 `json:"id"`
	} `json:"user"`
}

func (i esaTokenInfo) UserID() int64 {
	if i.User.ID != 0 {
		return i.User.ID
	}
	return i.ResourceOwnerID
}

type esaTeamsResponse struct {
	Teams    []esaTeam `json:"teams"`
	NextPage *int      `json:"next_page"`
}

type esaTeam struct {
	Name string `json:"name"`
}

type esaMember struct {
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
	Icon       string `json:"icon"`
	Email      string `json:"email"`
}
