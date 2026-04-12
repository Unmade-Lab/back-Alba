package config

import "os"

// Config holds all application configuration loaded from environment variables.
type Config struct {
	ServerPort  string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	GeminiKey   string
	GeminiModel string
	Environment string
	AppURL      string // Base URL for generating invite links, e.g. https://app.alba.io
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ServerPort:  getEnv("PORT", getEnv("SERVER_PORT", "8080")),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		RedisURL:    getEnv("REDIS_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", ""),
		GeminiKey:   getEnv("GEMINI_API_KEY", ""),
		GeminiModel: getEnv("GEMINI_MODEL", ""),
		Environment: getEnv("ENV", "development"),
		AppURL:      getEnv("APP_URL", "http://localhost:3000"),
	}
}

// IsDevelopment returns true when running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func getEnv(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}
