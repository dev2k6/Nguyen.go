package validator

import (
	"github.com/gofiber/fiber/v2"
)

func Middleware(rules map[string]string) fiber.Handler {
	v := New()
	return func(c *fiber.Ctx) error {
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		errors := v.ValidateMap(body, rules)
		if errors.HasErrors() {
			return c.Status(422).JSON(fiber.Map{
				"error":  "Validation failed",
				"fields": errors,
			})
		}

		c.Locals("validated_body", body)
		return c.Next()
	}
}

func MiddlewareFor(data interface{}) fiber.Handler {
	v := New()
	return func(c *fiber.Ctx) error {
		if err := c.BodyParser(data); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		errors := v.Validate(data)
		if errors.HasErrors() {
			return c.Status(422).JSON(fiber.Map{
				"error":  "Validation failed",
				"fields": errors,
			})
		}

		c.Locals("validated_body", data)
		return c.Next()
	}
}
