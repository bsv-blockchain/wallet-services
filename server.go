package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/bsv-blockchain/arcade/config"
	arcadeRoutes "github.com/bsv-blockchain/arcade/routes/fiber"
	chaintracksRoutes "github.com/bsv-blockchain/go-chaintracks/routes/fiber"
)

// setupServer creates a fiber app with all routes registered.
func setupServer(ctx context.Context, services *config.Services, log *slog.Logger) *fiber.App {
	arcadeRts := arcadeRoutes.NewRoutes(arcadeRoutes.Config{
		Service:        services.ArcadeService,
		Store:          services.Store,
		EventPublisher: services.EventPublisher,
		Arcade:         services.Arcade,
		Logger:         log,
	})

	chaintracksRts := chaintracksRoutes.NewRoutes(ctx, services.Chaintracks)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(logger.New(logger.Config{
		Format: "${method} ${path} - ${status} (${latency})\n",
	}))
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
		AllowMethods: "GET,POST,OPTIONS",
	}))

	// Arcade transaction routes
	arcadeGroup := app.Group("/arcade")
	arcadeRts.Register(arcadeGroup)

	// Chaintracks v2 header routes
	chaintracksGroup := app.Group("/chaintracks")
	chaintracksV2Group := chaintracksGroup.Group("/v2")
	chaintracksRts.Register(chaintracksV2Group)

	// Chaintracks legacy v1 routes
	chaintracksV1Group := chaintracksGroup.Group("/v1")
	chaintracksRts.RegisterLegacy(chaintracksV1Group)

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"services": fiber.Map{
				"arcade":      "running",
				"chaintracks": "running",
			},
		})
	})

	// Status page
	app.Get("/", func(c *fiber.Ctx) error { //nolint:contextcheck // request context available via c.UserContext()
		tip := services.Chaintracks.GetTip(c.UserContext())
		height := uint32(0)
		if tip != nil {
			height = tip.Height
		}

		return c.Type("html").SendString(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Wallet Services</title></head>
<body>
<h1>Wallet Services</h1>
<h2>Status</h2>
<ul>
<li>Chain Height: %d</li>
</ul>
<h2>Endpoints</h2>
<h3>Arcade (Transaction Broadcast)</h3>
<ul>
<li>POST /arcade/tx - Submit transaction</li>
<li>POST /arcade/txs - Submit multiple transactions</li>
<li>GET /arcade/tx/{txid} - Get transaction status</li>
<li>GET /arcade/events?callbackToken=X - Stream transaction status (SSE)</li>
<li>GET /arcade/policy - Get broadcast policy</li>
</ul>
<h3>Chaintracks v2 (Block Headers)</h3>
<ul>
<li>GET /chaintracks/v2/network - Get network name</li>
<li>GET /chaintracks/v2/height - Get chain height</li>
<li>GET /chaintracks/v2/tip - Get chain tip</li>
<li>GET /chaintracks/v2/tip/stream - Stream tip updates (SSE)</li>
<li>GET /chaintracks/v2/header/height/:height - Get header by height</li>
<li>GET /chaintracks/v2/header/hash/:hash - Get header by hash</li>
<li>GET /chaintracks/v2/headers?height=N&count=M - Get multiple headers</li>
</ul>
<h3>Chaintracks Legacy v1</h3>
<ul>
<li>GET /chaintracks/v1/getChain - Get network name</li>
<li>GET /chaintracks/v1/getPresentHeight - Get chain height</li>
<li>GET /chaintracks/v1/findChainTipHashHex - Get chain tip hash</li>
<li>GET /chaintracks/v1/findChainTipHeaderHex - Get chain tip header</li>
<li>GET /chaintracks/v1/findHeaderHexForHeight?height=N - Get header by height</li>
<li>GET /chaintracks/v1/findHeaderHexForBlockHash?hash=X - Get header by hash</li>
<li>GET /chaintracks/v1/getHeaders?height=N&count=M - Get multiple headers</li>
</ul>
</body>
</html>`, height))
	})

	return app
}
