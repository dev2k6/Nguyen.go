package nguyen

import "github.com/gofiber/fiber/v2"

type Option func(*App)

func WithPort(port int) Option {
	return func(a *App) {
		a.port = port
	}
}

func WithHost(host string) Option {
	return func(a *App) {
		a.host = host
	}
}

func WithPages(dir string) Option {
	return func(a *App) {
		a.pagesDir = dir
	}
}

func WithPublic(dir string) Option {
	return func(a *App) {
		a.publicDir = dir
	}
}

func WithStyles(dir string) Option {
	return func(a *App) {
		a.stylesDir = dir
	}
}

func WithBuildDir(dir string) Option {
	return func(a *App) {
		a.buildDir = dir
	}
}

func WithSetup(fn func(*fiber.App)) Option {
	return func(a *App) {
		a.setup = fn
	}
}
