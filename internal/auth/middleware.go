package auth

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (a *Auth) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := a.extractClaims(c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Unauthorized", "message": err.Error()})
		}
		c.Locals("user_id", claims.UserID)
		c.Locals("user_role", claims.Role)
		c.Locals("user_email", claims.Email)
		c.Locals("claims", claims)
		return c.Next()
	}
}

func (a *Auth) Role(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := a.extractClaims(c)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Unauthorized", "message": err.Error()})
		}

		allowed := false
		for _, role := range roles {
			if claims.Role == role {
				allowed = true
				break
			}
		}

		if !allowed {
			return c.Status(403).JSON(fiber.Map{"error": "Forbidden", "message": "insufficient permissions"})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("user_role", claims.Role)
		c.Locals("user_email", claims.Email)
		c.Locals("claims", claims)
		return c.Next()
	}
}

func (a *Auth) Optional() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := a.extractClaims(c)
		if err == nil {
			c.Locals("user_id", claims.UserID)
			c.Locals("user_role", claims.Role)
			c.Locals("user_email", claims.Email)
			c.Locals("claims", claims)
		}
		return c.Next()
	}
}

func (a *Auth) extractClaims(c *fiber.Ctx) (*Claims, error) {
	token := ""

	authHeader := c.Get(a.config.TokenHeader)
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			token = parts[1]
		}
	}

	if token == "" {
		token = c.Cookies(a.config.CookieName)
	}

	if token == "" {
		token = c.Query("token")
	}

	if token == "" {
		return nil, fmt.Errorf("auth: no token provided")
	}

	return a.ValidateToken(token)
}

func GetUserID(c *fiber.Ctx) string {
	if id, ok := c.Locals("user_id").(string); ok {
		return id
	}
	return ""
}

func GetUserRole(c *fiber.Ctx) string {
	if role, ok := c.Locals("user_role").(string); ok {
		return role
	}
	return ""
}

func GetClaims(c *fiber.Ctx) *Claims {
	if claims, ok := c.Locals("claims").(*Claims); ok {
		return claims
	}
	return nil
}
