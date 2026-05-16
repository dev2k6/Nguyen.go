package nguyen

import (
	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/live"
	"github.com/dev2k6/Nguyen.go/internal/server"
	"github.com/gofiber/fiber/v2"
)

type Option func(*App)

func WithPort(port int) Option {
	return func(a *App) { a.port = port }
}

func WithHost(host string) Option {
	return func(a *App) { a.host = host }
}

func WithPages(dir string) Option {
	return func(a *App) { a.pagesDir = dir }
}

func WithPublic(dir string) Option {
	return func(a *App) { a.publicDir = dir }
}

func WithStyles(dir string) Option {
	return func(a *App) { a.stylesDir = dir }
}

func WithBuildDir(dir string) Option {
	return func(a *App) { a.buildDir = dir }
}

func WithSetup(fn func(*fiber.App)) Option {
	return func(a *App) { a.setup = fn }
}

func WithDatabase(cfg config.DatabaseConfig) Option {
	return func(a *App) { a.dbConfig = &cfg }
}

func WithAuth(cfg config.AuthConfig) Option {
	return func(a *App) { a.authConfig = &cfg }
}

func WithCSRF(cfg config.CSRFConfig) Option {
	return func(a *App) { a.csrfConfig = &cfg }
}

func WithUpload(cfg config.UploadConfig) Option {
	return func(a *App) { a.uploadConfig = &cfg }
}

func WithWebSocket(cfg config.WSConfig) Option {
	return func(a *App) { a.wsConfig = &cfg }
}

func WithI18n(cfg config.I18nConfig) Option {
	return func(a *App) { a.i18nConfig = &cfg }
}

func WithMail(cfg config.MailConfig) Option {
	return func(a *App) { a.mailConfig = &cfg }
}

// WithLiveOptions configures the Live Mode WebSocket endpoint.
//
// AllowedOrigins controls CSWSH protection. An empty slice (default)
// allows only same-origin connections. Pass []string{"*"} in
// development to allow all origins.
//
// AuthFunc is an optional hook called before a session is spawned.
// Return a non-nil error to reject the connection with 403.
//
//	app := nguyen.New(
//	    nguyen.WithLiveOptions(server.LiveOptions{
//	        AllowedOrigins: []string{"https://example.com"},
//	        AuthFunc: func(c *fiber.Ctx) error {
//	            _, err := myAuth.ValidateSession(c)
//	            return err
//	        },
//	    }),
//	)
func WithLiveOptions(opts server.LiveOptions) Option {
	return func(a *App) { a.liveOpts = opts }
}

// WithHubOptions configures the Live Mode session hub (MaxSessions,
// IdleTimeout, HeartbeatInterval, OutboundBuffer).
func WithHubOptions(opts live.Options) Option {
	return func(a *App) { a.hubOpts = opts }
}
