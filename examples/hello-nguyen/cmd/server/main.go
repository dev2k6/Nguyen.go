package main

import (
	"hello-nguyen/api"
	"hello-nguyen/livedemo"

	"github.com/dev2k6/Nguyen.go/pkg/nguyen"

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := nguyen.New(
		nguyen.WithHost("0.0.0.0"),
		nguyen.WithPort(3000),
		nguyen.WithPages("./pages"),
		nguyen.WithPublic("./public"),
		nguyen.WithStyles("./styles"),
		nguyen.WithSetup(func(f *fiber.App) {
			api.Register(f)
		}),
	)

	if err := app.RegisterLivePage("/counter", livedemo.CounterPage{}); err != nil {
		panic(err)
	}

	app.Listen(":3000")
}
