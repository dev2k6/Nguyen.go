package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	DefaultLocale   string   `yaml:"default_locale"`
	Locales         []string `yaml:"locales"`
	TranslationsDir string   `yaml:"translations_dir"`
	URLPrefix       bool     `yaml:"url_prefix"`
	CookieName      string   `yaml:"cookie_name"`
	QueryParam      string   `yaml:"query_param"`
}

type I18n struct {
	config       Config
	translations map[string]map[string]string
	mu           sync.RWMutex
}

func New(cfg Config) (*I18n, error) {
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "en"
	}
	if cfg.TranslationsDir == "" {
		cfg.TranslationsDir = "locales"
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "nguyen_locale"
	}
	if cfg.QueryParam == "" {
		cfg.QueryParam = "lang"
	}

	i := &I18n{
		config:       cfg,
		translations: make(map[string]map[string]string),
	}

	if err := i.loadTranslations(); err != nil {
		return nil, err
	}

	return i, nil
}

func (i *I18n) T(locale, key string, args ...interface{}) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if trans, ok := i.translations[locale]; ok {
		if val, ok := trans[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(val, args...)
			}
			return val
		}
	}

	if locale != i.config.DefaultLocale {
		if trans, ok := i.translations[i.config.DefaultLocale]; ok {
			if val, ok := trans[key]; ok {
				if len(args) > 0 {
					return fmt.Sprintf(val, args...)
				}
				return val
			}
		}
	}

	return key
}

func (i *I18n) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		locale := i.detectLocale(c)
		c.Locals("locale", locale)
		c.Locals("i18n", i)
		return c.Next()
	}
}

func (i *I18n) LocaleFromContext(c *fiber.Ctx) string {
	if locale, ok := c.Locals("locale").(string); ok {
		return locale
	}
	return i.config.DefaultLocale
}

func (i *I18n) Locales() []string {
	return i.config.Locales
}

func (i *I18n) HasLocale(locale string) bool {
	for _, l := range i.config.Locales {
		if l == locale {
			return true
		}
	}
	return false
}

func (i *I18n) Reload() error {
	return i.loadTranslations()
}

func (i *I18n) detectLocale(c *fiber.Ctx) string {
	if i.config.URLPrefix {
		path := c.Path()
		parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
		if len(parts) > 0 && i.HasLocale(parts[0]) {
			return parts[0]
		}
	}

	if q := c.Query(i.config.QueryParam); q != "" && i.HasLocale(q) {
		return q
	}

	if cookie := c.Cookies(i.config.CookieName); cookie != "" && i.HasLocale(cookie) {
		return cookie
	}

	accept := c.Get("Accept-Language")
	if accept != "" {
		locale := i.parseAcceptLanguage(accept)
		if locale != "" {
			return locale
		}
	}

	return i.config.DefaultLocale
}

func (i *I18n) parseAcceptLanguage(header string) string {
	parts := strings.Split(header, ",")
	for _, part := range parts {
		lang := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		lang = strings.SplitN(lang, "-", 2)[0]
		if i.HasLocale(lang) {
			return lang
		}
	}
	return ""
}

func (i *I18n) loadTranslations() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	dir := i.config.TranslationsDir
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("i18n: cannot read translations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		locale := strings.TrimSuffix(name, ".json")
		filePath := filepath.Join(dir, name)

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("i18n: cannot read %s: %w", filePath, err)
		}

		var nested map[string]interface{}
		if err := json.Unmarshal(data, &nested); err != nil {
			return fmt.Errorf("i18n: invalid JSON in %s: %w", filePath, err)
		}

		flat := make(map[string]string)
		flatten("", nested, flat)
		i.translations[locale] = flat

		if !i.HasLocale(locale) {
			i.config.Locales = append(i.config.Locales, locale)
		}
	}

	return nil
}

func flatten(prefix string, data map[string]interface{}, result map[string]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]interface{}:
			flatten(fullKey, v, result)
		default:
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}
}
