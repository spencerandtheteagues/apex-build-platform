package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/github"
)

type OAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(code string) (*oauth2.Token, error)
	GetUserInfo(token *oauth2.Token) (*OAuthUserInfo, error)
}

type OAuthUserInfo struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Picture  string `json:"picture"`
	Provider string `json:"provider"`
}

type GoogleOAuth struct {
	config *oauth2.Config
}

type GitHubOAuth struct {
	config *oauth2.Config
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func NewGitHubOAuth(clientID, clientSecret, redirectURL string) *GitHubOAuth {
	return &GitHubOAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}
}

func (g *GoogleOAuth) GetAuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (g *GoogleOAuth) ExchangeCode(code string) (*oauth2.Token, error) {
	return g.config.Exchange(context.Background(), code)
}

func (g *GoogleOAuth) GetUserInfo(token *oauth2.Token) (*OAuthUserInfo, error) {
	client := g.config.Client(context.Background(), token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var googleUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}

	if err := json.Unmarshal(body, &googleUser); err != nil {
		return nil, err
	}

	return &OAuthUserInfo{
		ID:       googleUser.ID,
		Email:    googleUser.Email,
		Name:     googleUser.Name,
		Picture:  googleUser.Picture,
		Provider: "google",
	}, nil
}

func (gh *GitHubOAuth) GetAuthURL(state string) string {
	return gh.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (gh *GitHubOAuth) ExchangeCode(code string) (*oauth2.Token, error) {
	return gh.config.Exchange(context.Background(), code)
}

func (gh *GitHubOAuth) GetUserInfo(token *oauth2.Token) (*OAuthUserInfo, error) {
	client := gh.config.Client(context.Background(), token)

	// Get user info
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var githubUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.Unmarshal(body, &githubUser); err != nil {
		return nil, err
	}

	// If email is null, get primary email
	if githubUser.Email == "" {
		emailResp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer emailResp.Body.Close()
			emailBody, err := io.ReadAll(emailResp.Body)
			if err == nil {
				var emails []struct {
					Email   string `json:"email"`
					Primary bool   `json:"primary"`
				}
				if json.Unmarshal(emailBody, &emails) == nil {
					for _, email := range emails {
						if email.Primary {
							githubUser.Email = email.Email
							break
						}
					}
				}
			}
		}
	}

	return &OAuthUserInfo{
		ID:       fmt.Sprintf("%d", githubUser.ID),
		Email:    githubUser.Email,
		Name:     githubUser.Name,
		Picture:  githubUser.AvatarURL,
		Provider: "github",
	}, nil
}

type OAuthService struct {
	providers map[string]OAuthProvider
}

func NewOAuthService() *OAuthService {
	return &OAuthService{
		providers: make(map[string]OAuthProvider),
	}
}

func (o *OAuthService) RegisterProvider(name string, provider OAuthProvider) {
	o.providers[name] = provider
}

func (o *OAuthService) GetProvider(name string) (OAuthProvider, bool) {
	provider, exists := o.providers[name]
	return provider, exists
}