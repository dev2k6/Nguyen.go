package upload

import (
	"github.com/gofiber/fiber/v2"
)

func (u *Uploader) DeleteHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Query("path")
		if path == "" {
			return c.Status(400).JSON(fiber.Map{"error": "path is required"})
		}

		if err := u.Delete(c.Context(), path); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"message": "file deleted"})
	}
}

func (u *Uploader) Routes(router fiber.Router) {
	router.Post("/upload", u.HandleSingle("file"))
	router.Post("/upload/multiple", u.HandleMultiple("files", 10))
	router.Delete("/upload", u.DeleteHandler())
}
