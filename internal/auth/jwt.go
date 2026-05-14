package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	UserID    string                 `json:"user_id"`
	Role      string                 `json:"role,omitempty"`
	Email     string                 `json:"email,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
	IssuedAt  int64                  `json:"iat"`
	ExpiresAt int64                  `json:"exp"`
	Subject   string                 `json:"sub,omitempty"`
	Issuer    string                 `json:"iss,omitempty"`
}

func (c *Claims) Valid() error {
	now := time.Now().Unix()
	if c.ExpiresAt > 0 && now > c.ExpiresAt {
		return fmt.Errorf("auth: token expired")
	}
	if c.IssuedAt > now+60 {
		return fmt.Errorf("auth: token issued in the future")
	}
	return nil
}

func (a *Auth) GenerateToken(claims *Claims) (string, error) {
	if claims.IssuedAt == 0 {
		claims.IssuedAt = time.Now().Unix()
	}
	if claims.ExpiresAt == 0 {
		claims.ExpiresAt = time.Now().Add(a.config.JWTExpiry).Unix()
	}
	if claims.Subject == "" {
		claims.Subject = claims.UserID
	}
	claims.Issuer = "nguyen.go"

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("auth: failed to marshal header: %w", err)
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("auth: failed to marshal claims: %w", err)
	}

	headerB64 := base64URLEncode(headerJSON)
	payloadB64 := base64URLEncode(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	signature := signHS256([]byte(signingInput), []byte(a.config.JWTSecret))
	signatureB64 := base64URLEncode(signature)

	return signingInput + "." + signatureB64, nil
}

func (a *Auth) GenerateRefreshToken(claims *Claims) (string, error) {
	claims.ExpiresAt = time.Now().Add(a.config.RefreshExpiry).Unix()
	return a.GenerateToken(claims)
}

func (a *Auth) ValidateToken(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("auth: invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("auth: invalid signature encoding: %w", err)
	}

	expected := signHS256([]byte(signingInput), []byte(a.config.JWTSecret))
	if !hmac.Equal(signature, expected) {
		return nil, fmt.Errorf("auth: invalid signature")
	}

	payloadJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("auth: invalid payload encoding: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("auth: invalid payload: %w", err)
	}

	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return &claims, nil
}

func (a *Auth) RefreshToken(refreshTokenStr string) (string, string, error) {
	claims, err := a.ValidateToken(refreshTokenStr)
	if err != nil {
		return "", "", fmt.Errorf("auth: invalid refresh token: %w", err)
	}

	newClaims := &Claims{
		UserID: claims.UserID,
		Role:   claims.Role,
		Email:  claims.Email,
		Extra:  claims.Extra,
	}

	accessToken, err := a.GenerateToken(newClaims)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := a.GenerateRefreshToken(newClaims)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func signHS256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
