package server

import (
	"os"
	"runtime"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/version"
	"github.com/gofiber/fiber/v2"
)

var startTime = time.Now()

// HealthHandler returns a handler for /_nguyen/health that reports
// server status, uptime, memory usage, and Go runtime info.
// Useful for load balancers, monitoring, and deployment checks.
func HealthHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		return c.JSON(fiber.Map{
			"status":    "ok",
			"version":   version.Version,
			"uptime_s":  int(time.Since(startTime).Seconds()),
			"go":        runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
			"memory": fiber.Map{
				"alloc_mb":       float64(mem.Alloc) / 1024 / 1024,
				"sys_mb":         float64(mem.Sys) / 1024 / 1024,
				"gc_cycles":      mem.NumGC,
				"gc_pause_ns":    mem.PauseNs[(mem.NumGC+255)%256],
			},
			"env": os.Getenv("NGUYEN_ENV"),
		})
	}
}
