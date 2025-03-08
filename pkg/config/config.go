package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	// Server settings
	APIPort       int
	MetricsPort   int
	LogLevel      string

	// Database settings
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis settings
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Worker settings
	WorkerCount int
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		APIPort:       8080,
		MetricsPort:   9090,
		LogLevel:      "info",
		DBHost:        "localhost",
		DBPort:        5432,
		DBUser:        "taskuser",
		DBPassword:    "password",
		DBName:        "taskdb",
		DBSSLMode:     "disable",
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		WorkerCount:   5,
	}
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() *Config {
	config := DefaultConfig()

	// Server settings
	if port := os.Getenv("API_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.APIPort = p
		}
	}
	if port := os.Getenv("METRICS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.MetricsPort = p
		}
	}
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.LogLevel = level
	}

	// Database settings
	if host := os.Getenv("DB_HOST"); host != "" {
		config.DBHost = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.DBPort = p
		}
	}
	if user := os.Getenv("DB_USER"); user != "" {
		config.DBUser = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		config.DBPassword = password
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		config.DBName = name
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		config.DBSSLMode = sslMode
	}

	// Redis settings
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		config.RedisAddr = addr
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.RedisPassword = password
	}
	if db := os.Getenv("REDIS_DB"); db != "" {
		if d, err := strconv.Atoi(db); err == nil {
			config.RedisDB = d
		}
	}

	// Worker settings
	if count := os.Getenv("WORKER_COUNT"); count != "" {
		if c, err := strconv.Atoi(count); err == nil {
			config.WorkerCount = c
		}
	}

	return config
}

// IsProduction checks if the application is running in production mode
func IsProduction() bool {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	return env == "production" || env == "prod"
} 