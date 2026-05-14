package config

import (
	"fmt"
	"os"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/version"
	"gopkg.in/yaml.v3"
)

// NguyenConfig represents the full configuration loaded from nguyen.config.yml
type NguyenConfig struct {
	Name     string         `yaml:"name"`
	Version  string         `yaml:"version"`
	Server   ServerConfig   `yaml:"server"`
	Render   RenderConfig   `yaml:"render"`
	WASM     WASMConfig     `yaml:"wasm"`
	HMR      HMRConfig      `yaml:"hmr"`
	Images   ImageConfig    `yaml:"images"`
	Head     HeadConfig     `yaml:"head"`
	GEO      GEOConfig      `yaml:"geo"`
	PWA      PWAConfig      `yaml:"pwa"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	CSRF     CSRFConfig     `yaml:"csrf"`
	Upload   UploadConfig   `yaml:"upload"`
	WS       WSConfig       `yaml:"websocket"`
	I18n     I18nConfig     `yaml:"i18n"`
	Mail     MailConfig     `yaml:"mail"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	StrictRouting bool   `yaml:"strict_routing"`
	CaseSensitive bool   `yaml:"case_sensitive"`
}

// Address returns the full listen address
func (s ServerConfig) Address() string {
	if s.Port == 0 {
		s.Port = 3000
	}
	if s.Host == "" {
		s.Host = "localhost"
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// RenderConfig controls rendering behavior
type RenderConfig struct {
	Mode               string `yaml:"mode"`   // "csr", "ssr", or "isr"
	Stream             bool   `yaml:"stream"` // enable Transfer-Encoding: chunked SSR
	ExtractCriticalCSS bool   `yaml:"extract_critical_css"`
}

// WASMConfig controls WebAssembly compilation
type WASMConfig struct {
	Entry    string `yaml:"entry"`  // e.g. "app/main.go"
	Output   string `yaml:"output"` // e.g. ".nguyen/app.wasm"
	Target   string `yaml:"target"` // "wasm"
	Optimize bool   `yaml:"optimize"`
}

// HMRConfig controls Hot Module Replacement
type HMRConfig struct {
	Enabled     bool     `yaml:"enabled"`
	SSEEndpoint string   `yaml:"sse"`
	Watch       []string `yaml:"watch"`
}

// ImageConfig controls the built-in image optimizer
type ImageConfig struct {
	Formats   []string `yaml:"formats"`
	Sizes     []int    `yaml:"sizes"`
	Quality   int      `yaml:"quality"`
	CacheTTL  int      `yaml:"cache_ttl"`  // seconds
	CacheDir  string   `yaml:"cache_dir"`  // default ".nguyen/cache/images"
	PublicDir string   `yaml:"public_dir"` // default "public"
	BlurSize  int      `yaml:"blur_size"`  // default 10
}

// HeadConfig controls default HTML head metadata
type HeadConfig struct {
	TitleTemplate     string            `yaml:"title_template"`
	DefaultTitle      string            `yaml:"default_title"`
	Meta              map[string]string `yaml:"meta"`
	OpenGraph         map[string]string `yaml:"open_graph"`
	PreconnectOrigins []string          `yaml:"preconnect_origins"`
	PreloadWASM       bool              `yaml:"preload_wasm"`
}

// GEOConfig controls Generative Engine Optimization settings
type GEOConfig struct {
	Enabled          bool          `yaml:"enabled"`
	SiteName         string        `yaml:"site_name"`
	SiteURL          string        `yaml:"site_url"`
	OrganizationName string        `yaml:"organization_name"`
	OrganizationURL  string        `yaml:"organization_url"`
	SameAs           []string      `yaml:"same_as"`
	AuthorType       string        `yaml:"author_type"`
	DefaultPageType  string        `yaml:"default_page_type"`
	Sitemap          SitemapConfig `yaml:"sitemap"`
	Robots           RobotsConfig  `yaml:"robots"`
	LLMTxt           LLMTxtConfig  `yaml:"llms_txt"`
	CacheTTL         int           `yaml:"cache_ttl"`
}

// SitemapConfig controls sitemap.xml generation
type SitemapConfig struct {
	Enabled           bool    `yaml:"enabled"`
	DefaultChangefreq string  `yaml:"default_changefreq"`
	DefaultPriority   float64 `yaml:"default_priority"`
}

// RobotsConfig controls robots.txt generation
type RobotsConfig struct {
	Enabled     bool `yaml:"enabled"`
	AllowAIBots bool `yaml:"allow_ai_bots"`
}

// LLMTxtConfig controls llms.txt generation
type LLMTxtConfig struct {
	Enabled  bool `yaml:"enabled"`
	MaxChars int  `yaml:"max_chars_per_page"`
}

// PWAConfig controls Progressive Web App settings
type PWAConfig struct {
	Enabled         bool      `yaml:"enabled"`
	ShortName       string    `yaml:"short_name"`
	Description     string    `yaml:"description"`
	ThemeColor      string    `yaml:"theme_color"`
	BackgroundColor string    `yaml:"background_color"`
	Display         string    `yaml:"display"`
	Orientation     string    `yaml:"orientation"`
	Scope           string    `yaml:"scope"`
	StartURL        string    `yaml:"start_url"`
	Icons           []PWAIcon `yaml:"icons"`
}

// PWAIcon defines a PWA icon entry
type PWAIcon struct {
	Src   string `yaml:"src"`
	Sizes string `yaml:"sizes"`
	Type  string `yaml:"type"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	MaxOpenConn int    `yaml:"max_open_conn"`
	MaxIdleConn int    `yaml:"max_idle_conn"`
	MaxLifetime int    `yaml:"max_lifetime"`
	Migrations  string `yaml:"migrations"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	Enabled        bool                    `yaml:"enabled"`
	JWTSecret      string                  `yaml:"jwt_secret"`
	JWTExpiry      time.Duration           `yaml:"jwt_expiry"`
	RefreshExpiry  time.Duration           `yaml:"refresh_expiry"`
	SessionTTL     time.Duration           `yaml:"session_ttl"`
	BcryptCost     int                     `yaml:"bcrypt_cost"`
	TokenHeader    string                  `yaml:"token_header"`
	CookieName     string                  `yaml:"cookie_name"`
	CookieSecure   bool                    `yaml:"cookie_secure"`
	CookieHTTPOnly bool                    `yaml:"cookie_httponly"`
	OAuth          map[string]OAuthConfig  `yaml:"oauth"`
}

// OAuthConfig holds OAuth provider settings
type OAuthConfig struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
	AuthURL      string   `yaml:"auth_url"`
	TokenURL     string   `yaml:"token_url"`
	UserInfoURL  string   `yaml:"user_info_url"`
}

// CSRFConfig holds CSRF protection settings
type CSRFConfig struct {
	Enabled     bool          `yaml:"enabled"`
	TokenLength int           `yaml:"token_length"`
	CookieName  string        `yaml:"cookie_name"`
	HeaderName  string        `yaml:"header_name"`
	FormField   string        `yaml:"form_field"`
	Expiry      time.Duration `yaml:"expiry"`
	Secure      bool          `yaml:"secure"`
	SameSite    string        `yaml:"same_site"`
	SkipPaths   []string      `yaml:"skip_paths"`
}

// UploadConfig holds file upload settings
type UploadConfig struct {
	Enabled      bool     `yaml:"enabled"`
	MaxSize      int64    `yaml:"max_size"`
	AllowedTypes []string `yaml:"allowed_types"`
	StorageType  string   `yaml:"storage_type"`
	LocalDir     string   `yaml:"local_dir"`
	S3Bucket     string   `yaml:"s3_bucket"`
	S3Region     string   `yaml:"s3_region"`
	S3Endpoint   string   `yaml:"s3_endpoint"`
	S3AccessKey  string   `yaml:"s3_access_key"`
	S3SecretKey  string   `yaml:"s3_secret_key"`
	BaseURL      string   `yaml:"base_url"`
}

// WSConfig holds WebSocket settings
type WSConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Path           string `yaml:"path"`
	MaxMessageSize int64  `yaml:"max_message_size"`
	PingInterval   int    `yaml:"ping_interval"`
}

// I18nConfig holds internationalization settings
type I18nConfig struct {
	Enabled         bool     `yaml:"enabled"`
	DefaultLocale   string   `yaml:"default_locale"`
	Locales         []string `yaml:"locales"`
	TranslationsDir string   `yaml:"translations_dir"`
	URLPrefix       bool     `yaml:"url_prefix"`
	CookieName      string   `yaml:"cookie_name"`
	QueryParam      string   `yaml:"query_param"`
}

// MailConfig holds email sending settings
type MailConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	FromName     string `yaml:"from_name"`
	FromAddress  string `yaml:"from_address"`
	TLS          bool   `yaml:"tls"`
	TemplatesDir string `yaml:"templates_dir"`
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *NguyenConfig {
	return &NguyenConfig{
		Name:    "nguyen-app",
		Version: version.Version,
		Server: ServerConfig{
			Host:          "localhost",
			Port:          3000,
			StrictRouting: false,
			CaseSensitive: false,
		},
		Render: RenderConfig{
			Mode:   "csr",
			Stream: false,
		},
		WASM: WASMConfig{
			Entry:    "app/main.go",
			Output:   ".nguyen/app.wasm",
			Target:   "wasm",
			Optimize: true,
		},
		HMR: HMRConfig{
			Enabled:     true,
			SSEEndpoint: "/_nguyen/hmr",
			Watch:       []string{"pages/", "components/", "styles/"},
		},
		Images: ImageConfig{
			Formats:   []string{"webp", "avif"},
			Sizes:     []int{640, 750, 1080, 1920},
			Quality:   80,
			CacheTTL:  86400,
			CacheDir:  ".nguyen/cache/images",
			PublicDir: "public",
			BlurSize:  10,
		},
		Head: HeadConfig{
			TitleTemplate:     "%s — Nguyen.go",
			DefaultTitle:      "Nguyen.go App",
			PreconnectOrigins: []string{"https://fonts.gstatic.com"},
			PreloadWASM:       true,
		},
		GEO: GEOConfig{
			Enabled:          true,
			SiteName:         "Nguyen.go Site",
			SiteURL:          "https://example.com",
			OrganizationName: "Organization",
			OrganizationURL:  "https://example.com",
			SameAs:           []string{},
			AuthorType:       "Organization",
			DefaultPageType:  "Article",
			Sitemap: SitemapConfig{
				Enabled:           true,
				DefaultChangefreq: "weekly",
				DefaultPriority:   0.7,
			},
			Robots: RobotsConfig{
				Enabled:     true,
				AllowAIBots: false,
			},
			LLMTxt: LLMTxtConfig{
				Enabled:  true,
				MaxChars: 5000,
			},
			CacheTTL: 3600,
		},
		PWA: PWAConfig{
			Enabled:         true,
			ShortName:       "Nguyen",
			Description:     "Nguyen.go Progressive Web App",
			ThemeColor:      "#6366f1",
			BackgroundColor: "#ffffff",
			Display:         "standalone",
			Orientation:     "portrait",
			Scope:           "/",
			StartURL:        "/",
			Icons: []PWAIcon{
				{Src: "/icon-192.png", Sizes: "192x192", Type: "image/png"},
				{Src: "/icon-512.png", Sizes: "512x512", Type: "image/png"},
			},
		},
	}
}

// Load reads and parses a YAML config file
func Load(path string) (*NguyenConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: invalid YAML in %s: %w", path, err)
	}

	// Validate
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return nil, fmt.Errorf("config: invalid server.port %d", cfg.Server.Port)
	}
	if cfg.Render.Mode != "" && cfg.Render.Mode != "csr" && cfg.Render.Mode != "ssr" && cfg.Render.Mode != "isr" {
		return nil, fmt.Errorf("config: invalid render.mode %q (must be csr, ssr, or isr)", cfg.Render.Mode)
	}

	return cfg, nil
}
