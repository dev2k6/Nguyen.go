package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type OAuthUser struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Provider string `json:"provider"`
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (a *Auth) OAuthLoginURL(provider string, state string) (string, error) {
	p, ok := a.config.OAuth[provider]
	if !ok {
		return "", fmt.Errorf("auth: unknown oauth provider %q", provider)
	}

	params := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {p.RedirectURL},
		"response_type": {"code"},
		"scope":         {strings.Join(p.Scopes, " ")},
		"state":         {state},
	}

	return p.AuthURL + "?" + params.Encode(), nil
}

func (a *Auth) OAuthCallback(provider, code string) (*OAuthUser, error) {
	p, ok := a.config.OAuth[provider]
	if !ok {
		return nil, fmt.Errorf("auth: unknown oauth provider %q", provider)
	}

	token, err := a.exchangeCode(p, code)
	if err != nil {
		return nil, err
	}

	user, err := a.fetchUserInfo(p, token.AccessToken)
	if err != nil {
		return nil, err
	}
	user.Provider = provider

	return user, nil
}

func (a *Auth) OAuthRedirectHandler(provider string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		state := generateState()
		c.Cookie(&fiber.Cookie{
			Name:     "oauth_state",
			Value:    state,
			HTTPOnly: true,
			Secure:   a.config.CookieSecure,
			MaxAge:   300,
		})

		loginURL, err := a.OAuthLoginURL(provider, state)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Redirect(loginURL)
	}
}

func (a *Auth) OAuthCallbackHandler(provider string, onSuccess func(c *fiber.Ctx, user *OAuthUser) error) fiber.Handler {
	return func(c *fiber.Ctx) error {
		state := c.Query("state")
		savedState := c.Cookies("oauth_state")
		if state == "" || state != savedState {
			return c.Status(400).JSON(fiber.Map{"error": "invalid oauth state"})
		}

		c.Cookie(&fiber.Cookie{
			Name:   "oauth_state",
			Value:  "",
			MaxAge: -1,
		})

		code := c.Query("code")
		if code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing authorization code"})
		}

		user, err := a.OAuthCallback(provider, code)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return onSuccess(c, user)
	}
}

func (a *Auth) exchangeCode(p OAuthProvider, code string) (*oauthTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.RedirectURL},
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("auth: oauth token request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: oauth token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth: oauth token exchange returned %d: %s", resp.StatusCode, string(body))
	}

	var token oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("auth: failed to decode token response: %w", err)
	}

	return &token, nil
}

func (a *Auth) fetchUserInfo(p OAuthProvider, accessToken string) (*OAuthUser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: oauth userinfo request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: oauth userinfo fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth: oauth userinfo returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("auth: failed to decode userinfo: %w", err)
	}

	user := &OAuthUser{}
	if id, ok := raw["id"]; ok {
		user.ID = fmt.Sprintf("%v", id)
	} else if sub, ok := raw["sub"]; ok {
		user.ID = fmt.Sprintf("%v", sub)
	}
	if email, ok := raw["email"].(string); ok {
		user.Email = email
	}
	if name, ok := raw["name"].(string); ok {
		user.Name = name
	}
	if avatar, ok := raw["picture"].(string); ok {
		user.Avatar = avatar
	} else if avatar, ok := raw["avatar_url"].(string); ok {
		user.Avatar = avatar
	}

	return user, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
