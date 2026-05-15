package csrf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	TokenLength int           `yaml:"token_length"`
	CookieName  string        `yaml:"cookie_name"`
	HeaderName  string        `yaml:"header_name"`
	FormField   string        `yaml:"form_field"`
	Expiry      time.Duration `yaml:"expiry"`
	Secure      bool          `yaml:"secure"`
	SameSite    string        `yaml:"same_site"`
	SkipPaths   []string      `yaml:"skip_paths"`
}

type CSRF struct {
	config Config
}

func New(cfg Config) *CSRF {
	if cfg.TokenLength == 0 {
		cfg.TokenLength = 32
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "_nguyen_csrf"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	if cfg.FormField == "" {
		cfg.FormField = "_csrf"
	}
	if cfg.Expiry == 0 {
		cfg.Expiry = 12 * time.Hour
	}
	if cfg.SameSite == "" {
		cfg.SameSite = "Lax"
	}

	return &CSRF{config: cfg}
}

func (cs *CSRF) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cs.shouldSkip(c) {
			return c.Next()
		}

		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			cs.ensureToken(c)
			return c.Next()
		}

		cookieToken := c.Cookies(cs.config.CookieName)
		if cookieToken == "" {
			return c.Status(403).JSON(fiber.Map{"error": "CSRF token missing"})
		}

		requestToken := c.Get(cs.config.HeaderName)
		if requestToken == "" {
			requestToken = c.FormValue(cs.config.FormField)
		}
		if requestToken == "" {
			return c.Status(403).JSON(fiber.Map{"error": "CSRF token not provided"})
		}

		if !tokensMatch(cookieToken, requestToken) {
			return c.Status(403).JSON(fiber.Map{"error": "CSRF token mismatch"})
		}

		cs.rotateToken(c)
		return c.Next()
	}
}

func (cs *CSRF) Token(c *fiber.Ctx) string {
	token := c.Cookies(cs.config.CookieName)
	if token == "" {
		token = cs.generateAndSet(c)
	}
	return token
}

func (cs *CSRF) ensureToken(c *fiber.Ctx) {
	if c.Cookies(cs.config.CookieName) == "" {
		cs.generateAndSet(c)
	}
}

func (cs *CSRF) rotateToken(c *fiber.Ctx) {
	cs.generateAndSet(c)
}

func (cs *CSRF) generateAndSet(c *fiber.Ctx) string {
	token := generateToken(cs.config.TokenLength)

	sameSite := cs.parseSameSite()
	// HTTPOnly must be false so JS can read the token for the
	// double-submit pattern. SameSite=Lax (default) limits the
	// exposure: the cookie is not sent on cross-site sub-resource
	// requests, only on top-level navigations.
	c.Cookie(&fiber.Cookie{
		Name:     cs.config.CookieName,
		Value:    token,
		HTTPOnly: false,
		Secure:   cs.config.Secure,
		SameSite: sameSite,
		MaxAge:   int(cs.config.Expiry.Seconds()),
		Path:     "/",
	})

	return token
}

func (cs *CSRF) shouldSkip(c *fiber.Ctx) bool {
	path := c.Path()
	for _, skip := range cs.config.SkipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

func (cs *CSRF) parseSameSite() string {
	switch strings.ToLower(cs.config.SameSite) {
	case "strict":
		return "Strict"
	case "none":
		return "None"
	default:
		return "Lax"
	}
}

func generateToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func tokensMatch(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
