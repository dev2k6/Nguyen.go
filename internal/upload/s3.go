package upload

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type S3Storage struct {
	bucket    string
	region    string
	endpoint  string
	accessKey string
	secretKey string
}

func NewS3Storage(cfg Config) (*S3Storage, error) {
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("upload: s3_bucket is required")
	}
	if cfg.S3Region == "" {
		cfg.S3Region = "us-east-1"
	}
	if cfg.S3Endpoint == "" {
		cfg.S3Endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", cfg.S3Region)
	}

	return &S3Storage{
		bucket:    cfg.S3Bucket,
		region:    cfg.S3Region,
		endpoint:  cfg.S3Endpoint,
		accessKey: cfg.S3AccessKey,
		secretKey: cfg.S3SecretKey,
	}, nil
}

func (s *S3Storage) Put(ctx context.Context, path string, reader io.Reader, size int64) (string, error) {
	// Clamp the read to the declared size (plus 1 byte to detect lies).
	// This prevents a client that sends a small Content-Length but a
	// huge body from exhausting server memory via io.ReadAll.
	maxRead := size
	if maxRead <= 0 {
		maxRead = 32 << 20 // 32 MB default when size is unknown
	}
	limited := io.LimitReader(reader, maxRead+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("upload: failed to read file: %w", err)
	}
	if int64(len(body)) > maxRead {
		return "", fmt.Errorf("upload: file exceeds declared size of %d bytes", maxRead)
	}

	url := fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, path)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("upload: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	s.signRequest(req, body)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload: s3 put failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload: s3 returned %d: %s", resp.StatusCode, string(respBody))
	}

	return s.URL(path), nil
}

func (s *S3Storage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, path)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("upload: failed to create request: %w", err)
	}

	s.signRequest(req, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload: s3 get failed: %w", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("upload: s3 returned %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	url := fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, path)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("upload: failed to create request: %w", err)
	}

	s.signRequest(req, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload: s3 delete failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("upload: s3 delete returned %d", resp.StatusCode)
	}

	return nil
}

func (s *S3Storage) URL(path string) string {
	return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, path)
}

func (s *S3Storage) signRequest(req *http.Request, payload []byte) {
	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", hashSHA256(payload))

	canonicalHeaders, signedHeaders := s.buildCanonicalHeaders(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		hashSHA256(payload),
	}, "\n")

	scope := fmt.Sprintf("%s/%s/s3/aws4_request", datestamp, s.region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hashSHA256([]byte(canonicalRequest)),
	}, "\n")

	signingKey := s.deriveSigningKey(datestamp)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, scope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
}

func (s *S3Storage) buildCanonicalHeaders(req *http.Request) (string, string) {
	headers := make(map[string]string)
	headers["host"] = req.URL.Host
	for key := range req.Header {
		lower := strings.ToLower(key)
		if lower == "host" || strings.HasPrefix(lower, "x-amz-") || lower == "content-type" {
			headers[lower] = strings.TrimSpace(req.Header.Get(key))
		}
	}

	var keys []string
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var canonical strings.Builder
	for _, k := range keys {
		canonical.WriteString(k + ":" + headers[k] + "\n")
	}

	return canonical.String(), strings.Join(keys, ";")
}

func (s *S3Storage) deriveSigningKey(datestamp string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+s.secretKey), []byte(datestamp))
	kRegion := hmacSHA256(kDate, []byte(s.region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
