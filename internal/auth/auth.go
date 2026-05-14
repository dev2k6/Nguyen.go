package auth

import (
	"time"

	"github.com/dev2k6/Nguyen.go/internal/database"
)

type Config struct {
	JWTSecret        string                  `yaml:"jwt_secret"`
	JWTExpiry        time.Duration           `yaml:"jwt_expiry"`
	RefreshExpiry    time.Duration           `yaml:"refresh_expiry"`
	SessionTTL       time.Duration           `yaml:"session_ttl"`
	BcryptCost       int                     `yaml:"bcrypt_cost"`
	TokenHeader      string                  `yaml:"token_header"`
	CookieName       string                  `yaml:"cookie_name"`
	CookieSecure     bool                    `yaml:"cookie_secure"`
	CookieHTTPOnly   bool                    `yaml:"cookie_httponly"`
	OAuth            map[string]OAuthProvider `yaml:"oauth"`
}

type OAuthProvider struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
	AuthURL      string   `yaml:"auth_url"`
	TokenURL     string   `yaml:"token_url"`
	UserInfoURL  string   `yaml:"user_info_url"`
}

type Auth struct {
	config   Config
	db       *database.DB
	sessions *SessionStore
}

func New(cfg Config, db *database.DB) *Auth {
	if cfg.JWTExpiry == 0 {
		cfg.JWTExpiry = 24 * time.Hour
	}
	if cfg.RefreshExpiry == 0 {
		cfg.RefreshExpiry = 7 * 24 * time.Hour
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 24 * time.Hour
	}
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = 12
	}
	if cfg.TokenHeader == "" {
		cfg.TokenHeader = "Authorization"
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "nguyen_session"
	}

	a := &Auth{
		config:   cfg,
		db:       db,
		sessions: NewSessionStore(cfg.SessionTTL),
	}
	return a
}

func (a *Auth) Config() Config {
	return a.config
}

func (a *Auth) Sessions() *SessionStore {
	return a.sessions
}
