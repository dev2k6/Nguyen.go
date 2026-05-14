package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	dir     string
	baseURL string
}

func NewLocalStorage(dir, baseURL string) *LocalStorage {
	if baseURL == "" {
		baseURL = "/uploads"
	}
	return &LocalStorage{dir: dir, baseURL: baseURL}
}

func (s *LocalStorage) Put(_ context.Context, path string, reader io.Reader, _ int64) (string, error) {
	fullPath := filepath.Join(s.dir, path)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("upload: cannot create directory: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("upload: cannot create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("upload: write failed: %w", err)
	}

	return s.URL(path), nil
}

func (s *LocalStorage) Get(_ context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.dir, path)

	absDir, _ := filepath.Abs(s.dir)
	absPath, _ := filepath.Abs(fullPath)
	if !filepath.HasPrefix(absPath, absDir) {
		return nil, fmt.Errorf("upload: path traversal detected")
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("upload: file not found: %w", err)
	}
	return file, nil
}

func (s *LocalStorage) Delete(_ context.Context, path string) error {
	fullPath := filepath.Join(s.dir, path)

	absDir, _ := filepath.Abs(s.dir)
	absPath, _ := filepath.Abs(fullPath)
	if !filepath.HasPrefix(absPath, absDir) {
		return fmt.Errorf("upload: path traversal detected")
	}

	return os.Remove(fullPath)
}

func (s *LocalStorage) URL(path string) string {
	return s.baseURL + "/" + filepath.ToSlash(path)
}
