package radar

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Env reads configuration from the process environment (same variable names as the Node build).
func Env(k string) string { return os.Getenv(k) }

func envNum(k string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
func envInt(k string, def int) int { return int(envNum(k, float64(def))) }
func envMs(k string, def int) time.Duration {
	return time.Duration(envNum(k, float64(def))) * time.Millisecond
}

// Config holds paths + flags resolved once at start-up.
type Config struct {
	DBPath, SeedPath, SecretsPath string
	SeedData                      []byte // sources.json contents; used instead of SeedPath when set (desktop: embedded)
	Desktop                       bool   // AI_RADAR_DESKTOP=1
	Version                       string // AI_RADAR_VERSION
}

// ConfigFromEnv: NEWS_DB (default ./data/news.db), SOURCES_JSON (./sources/sources.json), SECRETS_FILE (next to the DB).
func ConfigFromEnv() Config {
	cwd, _ := os.Getwd()
	c := Config{DBPath: os.Getenv("NEWS_DB"), SeedPath: os.Getenv("SOURCES_JSON"), SecretsPath: os.Getenv("SECRETS_FILE")}
	if c.DBPath == "" {
		c.DBPath = filepath.Join(cwd, "data", "news.db")
	}
	if c.SeedPath == "" {
		c.SeedPath = filepath.Join(cwd, "sources", "sources.json")
	}
	if c.SecretsPath == "" {
		c.SecretsPath = filepath.Join(filepath.Dir(c.DBPath), "secrets.json")
	}
	c.Desktop = os.Getenv("AI_RADAR_DESKTOP") == "1"
	c.Version = os.Getenv("AI_RADAR_VERSION")
	return c
}

func retentionHours() float64 { return envNum("RETENTION_HOURS", 48) }
