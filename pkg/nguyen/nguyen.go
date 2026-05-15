package nguyen

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/auth"
	"github.com/dev2k6/Nguyen.go/internal/cache"
	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/csrf"
	"github.com/dev2k6/Nguyen.go/internal/database"
	"github.com/dev2k6/Nguyen.go/internal/event"
	"github.com/dev2k6/Nguyen.go/internal/geo"
	"github.com/dev2k6/Nguyen.go/internal/i18n"
	"github.com/dev2k6/Nguyen.go/internal/live"
	internallivepage "github.com/dev2k6/Nguyen.go/internal/livepage"
	"github.com/dev2k6/Nguyen.go/internal/mail"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"
	"github.com/dev2k6/Nguyen.go/internal/server"
	"github.com/dev2k6/Nguyen.go/internal/upload"
	"github.com/dev2k6/Nguyen.go/internal/ws"
	"github.com/dev2k6/Nguyen.go/pkg/livepage"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type App struct {
	fiber        *fiber.App
	config       *config.NguyenConfig
	routes       []router.Route
	isrCache     *cache.ISR
	pagesDir     string
	publicDir    string
	stylesDir    string
	buildDir     string
	port         int
	host         string
	setup        func(*fiber.App)
	dbConfig     *config.DatabaseConfig
	authConfig   *config.AuthConfig
	csrfConfig   *config.CSRFConfig
	uploadConfig *config.UploadConfig
	wsConfig     *config.WSConfig
	i18nConfig   *config.I18nConfig
	mailConfig   *config.MailConfig
	db           *database.DB
	auth         *auth.Auth
	csrf         *csrf.CSRF
	uploader     *upload.Uploader
	wsHub        *ws.Hub
	i18n         *i18n.I18n
	mailer       *mail.Mailer
	events       *event.Bus
	liveHub      *live.Hub
	livePages    *internallivepage.Registry
	liveOpts     server.LiveOptions
}

func New(opts ...Option) *App {
	app := &App{
		pagesDir:  "pages",
		publicDir: "public",
		stylesDir: "styles",
		buildDir:  ".nguyen",
		port:      3000,
		host:      "0.0.0.0",
	}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

func (a *App) Fiber() *fiber.App {
	return a.fiber
}

func (a *App) Listen(addr string) error {
	if addr != "" {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			a.host = parts[0]
			fmt.Sscanf(parts[1], "%d", &a.port)
		}
	}
	return a.serve()
}

func (a *App) serve() error {
	cfg, err := config.Load("config/nguyen.config.yml")
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.config = cfg

	if a.port > 0 {
		cfg.Server.Port = a.port
	}
	if a.host != "" {
		cfg.Server.Host = a.host
	}

	pagesAbs, _ := filepath.Abs(a.pagesDir)
	routes, err := router.Discover(pagesAbs)
	if err != nil {
		return fmt.Errorf("nguyen: route discovery failed: %w", err)
	}
	a.routes = routes

	a.fiber = fiber.New(fiber.Config{
		AppName:               cfg.Name,
		ServerHeader:          "Nguyen.go",
		DisableStartupMessage: true,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           120 * time.Second,
		BodyLimit:             4 * 1024 * 1024,
	})

	a.fiber.Use(func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("  PANIC recovered: %v — %s %s", r, c.Method(), c.Path())
				err = c.Status(500).JSON(fiber.Map{"error": "Internal Server Error"})
			}
		}()
		return c.Next()
	})

	a.fiber.Use(func(c *fiber.Ctx) error {
		c.Set("X-Powered-By", fmt.Sprintf("Nguyen.go V%s", cfg.Version))
		c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		return c.Next()
	})

	a.fiber.Use(server.ContextMiddleware())
	a.fiber.Use(server.AccessLog())
	a.fiber.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	a.fiber.Use(etag.New())
	a.fiber.Use(limiter.New(limiter.Config{
		Max:          600,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Too Many Requests"})
		},
	}))

	a.fiber.Get("/_nguyen/image", server.ImageHandler(cfg))
	a.fiber.Get("/_nguyen/runtime", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"version":       cfg.Version,
			"liveMode":      a.livePages != nil && len(a.livePages.Routes()) > 0,
			"liveSessionTo": "/_nguyen/live",
		})
	})

	// Live Mode wiring — bridge JS + WebSocket endpoint. The hub and
	// registry are lazily created so applications that do not register
	// any live pages pay no overhead.
	a.liveHub = live.NewHub(live.Options{})
	a.livePages = internallivepage.NewRegistry()
	a.fiber.Get("/_nguyen/live.js", server.LiveBridgeHandler())
	a.fiber.Get("/_nguyen/live/+", server.LiveUpgradeMiddleware(a.liveOpts), server.LiveHandler(a.liveHub, a.livePages))
	go func() {
		_ = a.liveHub.Run(context.Background())
	}()

	a.fiber.Static("/styles", a.stylesDir)
	a.fiber.Static("/public", a.publicDir)

	if err := a.initModules(cfg); err != nil {
		return fmt.Errorf("nguyen: module init failed: %w", err)
	}

	if a.setup != nil {
		a.setup(a.fiber)
	}

	geoHandler := geo.NewHandler(&cfg.GEO, routes)
	a.fiber.Get("/sitemap.xml", geoHandler.Sitemap)
	a.fiber.Get("/robots.txt", geoHandler.Robots)
	if cfg.GEO.LLMTxt.Enabled {
		a.fiber.Get("/llms.txt", geoHandler.LLMTxt)
	}
	a.fiber.Get("/.well-known/ai-bot.txt", geoHandler.AIBot)

	a.isrCache = cache.NewISR(filepath.Join(a.buildDir, "cache/pages"))
	a.mountRoutes(pagesAbs)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	a.printBanner(addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if err := a.fiber.Listen(addr); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-quit:
		fmt.Println()
		log.Println("  Shutting down gracefully...")
		a.shutdown()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.fiber.ShutdownWithContext(ctx); err != nil {
			return fmt.Errorf("nguyen: forced shutdown: %w", err)
		}
		log.Println("  Server stopped")
	case err := <-errCh:
		a.shutdown()
		msg := err.Error()
		if strings.Contains(msg, "address already in use") ||
			strings.Contains(msg, "Only one usage of each socket address") {
			return fmt.Errorf("nguyen: port %d is already in use", cfg.Server.Port)
		}
		return fmt.Errorf("nguyen: %w", err)
	}
	return nil
}

func (a *App) shutdown() {
	if a.db != nil {
		a.db.Close()
		log.Println("  ✓ Database connection closed")
	}
}

func (a *App) mountRoutes(pagesAbs string) {
	cfg := a.config
	layouts, _ := router.FindLayouts(pagesAbs)

	for _, route := range a.routes {
		if route.Is404 {
			continue
		}
		r := route
		a.fiber.Get(r.Pattern, func(c *fiber.Ctx) error {
			return a.renderRoute(c, r, cfg, layouts)
		})
	}

	a.fiber.Use(func(c *fiber.Ctx) error {
		c.Status(404)
		for _, r := range a.routes {
			if r.Is404 {
				return a.renderRoute(c, r, cfg, layouts)
			}
		}
		return c.JSON(fiber.Map{"error": "Not Found", "path": c.Path()})
	})
}

func (a *App) renderRoute(c *fiber.Ctx, route router.Route, cfg *config.NguyenConfig, layouts []router.LayoutInfo) error {
	renderMode := cfg.Render.Mode

	if renderMode == "isr" {
		html, isStale, err := a.isrCache.Get(c.Path())
		if err == nil {
			if isStale {
				c.Set("Cache-Control", "s-maxage=60, stale-while-revalidate=3600")
				c.Set("X-Nguyen-ISR", "stale")
				go a.isrCache.BackgroundRevalidate(c.Path(), func(path string) (string, int, []string) {
					ngFile, parseErr := parser.Parse(route.FilePath)
					if parseErr != nil {
						return "", 0, nil
					}
					result := render.RenderSSR(ngFile, cfg.Version, layouts...)
					return render.InjectMetaTags(result.HTML, result), result.Revalidate, nil
				})
			} else {
				c.Set("Cache-Control", "s-maxage=3600, stale-while-revalidate=86400")
				c.Set("X-Nguyen-ISR", "fresh")
			}
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.SendString(html)
		}
	}

	ngFile, err := parser.Parse(route.FilePath)
	if err != nil {
		log.Printf("  Parse error for %s: %v", route.Pattern, err)
		return c.Status(500).SendString("Internal Server Error")
	}

	result := render.RenderSSR(ngFile, cfg.Version, layouts...)
	html := render.InjectMetaTags(result.HTML, result)

	if renderMode == "isr" && result.Revalidate > 0 {
		a.isrCache.Set(c.Path(), html, result.Revalidate)
		c.Set("Cache-Control", fmt.Sprintf("s-maxage=%d, stale-while-revalidate=86400", result.Revalidate))
		c.Set("X-Nguyen-ISR", "miss")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
}

func (a *App) printBanner(addr string) {
	fmt.Println()
	fmt.Printf("  ⬡ Nguyen.go — Production\n")
	fmt.Println()
	fmt.Printf("  ► http://%s\n", addr)
	fmt.Printf("  ► %d route(s) loaded\n", len(a.routes))
	fmt.Println()
}

func (a *App) initModules(cfg *config.NguyenConfig) error {
	dbCfg := a.resolveDBConfig(cfg)
	if dbCfg != nil {
		db, err := database.New(database.Config{
			Driver:      dbCfg.Driver,
			DSN:         dbCfg.DSN,
			MaxOpenConn: dbCfg.MaxOpenConn,
			MaxIdleConn: dbCfg.MaxIdleConn,
			MaxLifetime: dbCfg.MaxLifetime,
		})
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		a.db = db
		log.Println("  ✓ Database connected")
	}

	authCfg := a.resolveAuthConfig(cfg)
	if authCfg != nil {
		a.auth = auth.New(auth.Config{
			JWTSecret:      authCfg.JWTSecret,
			JWTExpiry:      authCfg.JWTExpiry,
			RefreshExpiry:  authCfg.RefreshExpiry,
			SessionTTL:     authCfg.SessionTTL,
			BcryptCost:     authCfg.BcryptCost,
			TokenHeader:    authCfg.TokenHeader,
			CookieName:     authCfg.CookieName,
			CookieSecure:   authCfg.CookieSecure,
			CookieHTTPOnly: authCfg.CookieHTTPOnly,
		}, a.db)
		log.Println("  ✓ Auth initialized")
	}

	csrfCfg := a.resolveCSRFConfig(cfg)
	if csrfCfg != nil {
		a.csrf = csrf.New(csrf.Config{
			TokenLength: csrfCfg.TokenLength,
			CookieName:  csrfCfg.CookieName,
			HeaderName:  csrfCfg.HeaderName,
			FormField:   csrfCfg.FormField,
			Expiry:      csrfCfg.Expiry,
			Secure:      csrfCfg.Secure,
			SameSite:    csrfCfg.SameSite,
			SkipPaths:   csrfCfg.SkipPaths,
		})
		a.fiber.Use(a.csrf.Middleware())
		log.Println("  ✓ CSRF protection enabled")
	}

	uploadCfg := a.resolveUploadConfig(cfg)
	if uploadCfg != nil {
		uploader, err := upload.New(upload.Config{
			MaxSize:      uploadCfg.MaxSize,
			AllowedTypes: uploadCfg.AllowedTypes,
			StorageType:  uploadCfg.StorageType,
			LocalDir:     uploadCfg.LocalDir,
			S3Bucket:     uploadCfg.S3Bucket,
			S3Region:     uploadCfg.S3Region,
			S3Endpoint:   uploadCfg.S3Endpoint,
			S3AccessKey:  uploadCfg.S3AccessKey,
			S3SecretKey:  uploadCfg.S3SecretKey,
			BaseURL:      uploadCfg.BaseURL,
		})
		if err != nil {
			return fmt.Errorf("upload: %w", err)
		}
		a.uploader = uploader
		if uploadCfg.StorageType == "local" {
			a.fiber.Static("/uploads", uploadCfg.LocalDir)
		}
		log.Println("  ✓ File upload ready")
	}

	wsCfg := a.resolveWSConfig(cfg)
	if wsCfg != nil {
		a.wsHub = ws.NewHub(ws.Config{
			Enabled:        wsCfg.Enabled,
			Path:           wsCfg.Path,
			MaxMessageSize: wsCfg.MaxMessageSize,
			PingInterval:   wsCfg.PingInterval,
		})
		a.fiber.Use(wsCfg.Path, a.wsHub.UpgradeMiddleware())
		a.fiber.Get(wsCfg.Path, a.wsHub.Upgrade())
		log.Println("  ✓ WebSocket enabled at", wsCfg.Path)
	}

	i18nCfg := a.resolveI18nConfig(cfg)
	if i18nCfg != nil {
		i, err := i18n.New(i18n.Config{
			DefaultLocale:   i18nCfg.DefaultLocale,
			Locales:         i18nCfg.Locales,
			TranslationsDir: i18nCfg.TranslationsDir,
			URLPrefix:       i18nCfg.URLPrefix,
			CookieName:      i18nCfg.CookieName,
			QueryParam:      i18nCfg.QueryParam,
		})
		if err != nil {
			return fmt.Errorf("i18n: %w", err)
		}
		a.i18n = i
		a.fiber.Use(a.i18n.Middleware())
		log.Println("  ✓ i18n loaded:", i18nCfg.Locales)
	}

	mailCfg := a.resolveMailConfig(cfg)
	if mailCfg != nil {
		m, err := mail.New(mail.Config{
			Host:         mailCfg.Host,
			Port:         mailCfg.Port,
			Username:     mailCfg.Username,
			Password:     mailCfg.Password,
			FromName:     mailCfg.FromName,
			FromAddress:  mailCfg.FromAddress,
			TLS:          mailCfg.TLS,
			TemplatesDir: mailCfg.TemplatesDir,
		})
		if err != nil {
			return fmt.Errorf("mail: %w", err)
		}
		a.mailer = m
		log.Println("  ✓ Mail sender ready")
	}

	a.events = event.NewBus()
	log.Println("  ✓ Event bus initialized")

	return nil
}

func (a *App) resolveDBConfig(cfg *config.NguyenConfig) *config.DatabaseConfig {
	if a.dbConfig != nil {
		return a.dbConfig
	}
	if cfg.Database.Driver != "" {
		return &cfg.Database
	}
	return nil
}

func (a *App) resolveAuthConfig(cfg *config.NguyenConfig) *config.AuthConfig {
	if a.authConfig != nil {
		return a.authConfig
	}
	if cfg.Auth.Enabled {
		return &cfg.Auth
	}
	return nil
}

func (a *App) resolveCSRFConfig(cfg *config.NguyenConfig) *config.CSRFConfig {
	if a.csrfConfig != nil {
		return a.csrfConfig
	}
	if cfg.CSRF.Enabled {
		return &cfg.CSRF
	}
	return nil
}

func (a *App) resolveUploadConfig(cfg *config.NguyenConfig) *config.UploadConfig {
	if a.uploadConfig != nil {
		return a.uploadConfig
	}
	if cfg.Upload.Enabled {
		return &cfg.Upload
	}
	return nil
}

func (a *App) resolveWSConfig(cfg *config.NguyenConfig) *config.WSConfig {
	if a.wsConfig != nil {
		return a.wsConfig
	}
	if cfg.WS.Enabled {
		return &cfg.WS
	}
	return nil
}

func (a *App) resolveI18nConfig(cfg *config.NguyenConfig) *config.I18nConfig {
	if a.i18nConfig != nil {
		return a.i18nConfig
	}
	if cfg.I18n.Enabled {
		return &cfg.I18n
	}
	return nil
}

func (a *App) resolveMailConfig(cfg *config.NguyenConfig) *config.MailConfig {
	if a.mailConfig != nil {
		return a.mailConfig
	}
	if cfg.Mail.Enabled {
		return &cfg.Mail
	}
	return nil
}

func (a *App) DB() *database.DB {
	return a.db
}

func (a *App) Auth() *auth.Auth {
	return a.auth
}

func (a *App) CSRF() *csrf.CSRF {
	return a.csrf
}

func (a *App) Uploader() *upload.Uploader {
	return a.uploader
}

func (a *App) WSHub() *ws.Hub {
	return a.wsHub
}

func (a *App) I18n() *i18n.I18n {
	return a.i18n
}

func (a *App) Mailer() *mail.Mailer {
	return a.mailer
}

func (a *App) Events() *event.Bus {
	return a.events
}

// LiveHub returns the Live Mode session hub. nil before serve() runs.
// Useful for tests and for tooling that wants to introspect active
// sessions.
func (a *App) LiveHub() *live.Hub {
	return a.liveHub
}

// RegisterLivePage binds a Live Mode page to a route pattern. Call it
// from your WithSetup hook so the page is registered before the server
// starts handling requests. Routes registered here are reachable at
// /_nguyen/live<pattern> over WebSocket.
//
//	app := nguyen.New(nguyen.WithSetup(func(f *fiber.App) {
//	    app.RegisterLivePage("/counter", &CounterPage{})
//	}))
func (a *App) RegisterLivePage(pattern string, page livepage.Page) error {
	if a.livePages == nil {
		a.livePages = internallivepage.NewRegistry()
	}
	return a.livePages.Register(pattern, page)
}
