package main

import (
	"hello-nguyen/api"
	"github.com/dev2k6/Nguyen.go/pkg/dev"

	"github.com/gofiber/fiber/v2"
)

func main() {
	dev.Run(dev.Config{
		Setup: func(app *fiber.App) {
			api.Register(app)
		},
	})
}
