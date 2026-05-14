package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
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

type Storage interface {
	Put(ctx context.Context, path string, reader io.Reader, size int64) (string, error)
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	URL(path string) string
}

type UploadResult struct {
	Path     string `json:"path"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

type Uploader struct {
	config  Config
	storage Storage
}

func New(cfg Config) (*Uploader, error) {
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 10 * 1024 * 1024
	}
	if cfg.StorageType == "" {
		cfg.StorageType = "local"
	}
	if cfg.LocalDir == "" {
		cfg.LocalDir = "uploads"
	}

	var storage Storage
	switch cfg.StorageType {
	case "local":
		storage = NewLocalStorage(cfg.LocalDir, cfg.BaseURL)
	case "s3":
		s3, err := NewS3Storage(cfg)
		if err != nil {
			return nil, err
		}
		storage = s3
	default:
		return nil, fmt.Errorf("upload: unsupported storage type %q", cfg.StorageType)
	}

	return &Uploader{config: cfg, storage: storage}, nil
}

func (u *Uploader) Storage() Storage {
	return u.storage
}

func (u *Uploader) HandleSingle(fieldName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		file, err := c.FormFile(fieldName)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "file is required"})
		}

		result, err := u.processFile(c.Context(), file)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"data": result})
	}
}

func (u *Uploader) HandleMultiple(fieldName string, maxFiles int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid multipart form"})
		}

		files := form.File[fieldName]
		if len(files) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "no files provided"})
		}
		if maxFiles > 0 && len(files) > maxFiles {
			return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("max %d files allowed", maxFiles)})
		}

		var results []UploadResult
		for _, file := range files {
			result, err := u.processFile(c.Context(), file)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": err.Error()})
			}
			results = append(results, *result)
		}

		return c.JSON(fiber.Map{"data": results})
	}
}

func (u *Uploader) Upload(ctx context.Context, file *multipart.FileHeader) (*UploadResult, error) {
	return u.processFile(ctx, file)
}

func (u *Uploader) Delete(ctx context.Context, path string) error {
	return u.storage.Delete(ctx, path)
}

func (u *Uploader) processFile(ctx context.Context, file *multipart.FileHeader) (*UploadResult, error) {
	if file.Size > u.config.MaxSize {
		return nil, fmt.Errorf("file too large (max %d bytes)", u.config.MaxSize)
	}

	mimeType := file.Header.Get("Content-Type")
	if !u.isAllowedType(mimeType, file.Filename) {
		return nil, fmt.Errorf("file type %q not allowed", mimeType)
	}

	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("upload: cannot open file: %w", err)
	}
	defer src.Close()

	ext := filepath.Ext(file.Filename)
	storagePath := generatePath(ext)

	url, err := u.storage.Put(ctx, storagePath, src, file.Size)
	if err != nil {
		return nil, fmt.Errorf("upload: storage failed: %w", err)
	}

	return &UploadResult{
		Path:     storagePath,
		URL:      url,
		Filename: file.Filename,
		Size:     file.Size,
		MimeType: mimeType,
	}, nil
}

func (u *Uploader) isAllowedType(mimeType, filename string) bool {
	if len(u.config.AllowedTypes) == 0 {
		return true
	}

	for _, allowed := range u.config.AllowedTypes {
		if strings.HasPrefix(allowed, ".") {
			if strings.EqualFold(filepath.Ext(filename), allowed) {
				return true
			}
		} else if strings.Contains(allowed, "*") {
			parts := strings.Split(allowed, "/")
			mimeParts := strings.Split(mimeType, "/")
			if len(parts) == 2 && len(mimeParts) == 2 {
				if parts[0] == mimeParts[0] && parts[1] == "*" {
					return true
				}
			}
		} else if mimeType == allowed {
			return true
		}
	}
	return false
}

func generatePath(ext string) string {
	now := time.Now()
	dir := now.Format("2006/01/02")
	name := fmt.Sprintf("%d%s", now.UnixNano(), ext)
	return filepath.ToSlash(filepath.Join(dir, name))
}
