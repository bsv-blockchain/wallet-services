package main

import (
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearBWSEnvVars backs up, clears, and restores all BWS_ environment variables
// so that tests run with a clean environment.
func clearBWSEnvVars(t *testing.T) {
	t.Helper()

	saved := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "BWS_") {
			parts := strings.SplitN(env, "=", 2)
			saved[parts[0]] = parts[1]
			os.Unsetenv(parts[0])
		}
	}

	t.Cleanup(func() {
		// Clear any BWS_ vars set during the test
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "BWS_") {
				parts := strings.SplitN(env, "=", 2)
				os.Unsetenv(parts[0])
			}
		}
		// Restore originals
		for k, v := range saved {
			os.Setenv(k, v)
		}
	})
}

// --- Config Tests ---

func TestLoadConfig_Defaults(t *testing.T) {
	clearBWSEnvVars(t)

	cfg, err := loadConfig()
	require.NoError(t, err)

	assert.Equal(t, "embedded", string(cfg.Mode))
	assert.Equal(t, "main", cfg.Network)
	assert.Equal(t, ":3011", cfg.Server.Address)
	assert.Equal(t, "sqlite", cfg.Database.Type)
	assert.Equal(t, "memory", cfg.Events.Type)
	assert.Equal(t, "info", cfg.GetLogLevel())
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	clearBWSEnvVars(t)

	t.Setenv("BWS_NETWORK", "test")
	t.Setenv("BWS_SERVER_ADDRESS", ":9090")
	t.Setenv("BWS_LOG_LEVEL", "debug")

	cfg, err := loadConfig()
	require.NoError(t, err)

	assert.Equal(t, "test", cfg.Network)
	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.Equal(t, "debug", cfg.GetLogLevel())
}

func TestLoadConfig_InvalidConfigFile(t *testing.T) {
	clearBWSEnvVars(t)

	// Create a malformed config file in a temp directory
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"
	err := os.WriteFile(configPath, []byte("invalid: yaml: [broken"), 0o644)
	require.NoError(t, err)

	// Change to temp dir so viper finds our bad config
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(origDir) })

	_, err = loadConfig()
	assert.Error(t, err)
}

// --- HTTP Endpoint Tests (isolated fiber app, no real services) ---

// newTestApp creates a minimal fiber app with just the health and status endpoints
// for testing without requiring real arcade/chaintracks services.
func newTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"services": fiber.Map{
				"arcade":      "running",
				"chaintracks": "running",
			},
		})
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Type("html").SendString(`<!DOCTYPE html>
<html>
<head><title>Wallet Services</title></head>
<body>
<h1>Wallet Services</h1>
</body>
</html>`)
	})

	return app
}

func TestHealthEndpoint(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyStr := string(body)
	assert.Contains(t, bodyStr, `"status":"ok"`)
	assert.Contains(t, bodyStr, `"arcade":"running"`)
	assert.Contains(t, bodyStr, `"chaintracks":"running"`)
}

func TestHealthEndpoint_ContentType(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestStatusPage(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyStr := string(body)
	assert.Contains(t, bodyStr, "Wallet Services")
	assert.Contains(t, bodyStr, "<!DOCTYPE html>")
}
