package config

import (
	"os"
	"time"
)

type Config struct {
	DatabaseURL   string
	GRPCPort      string
	WeatherAPIKey string
	FetchInterval time.Duration
}

func Load() Config {
	cfg := Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/weather?sslmode=disable"),
		GRPCPort:      getEnv("GRPC_PORT", "50051"),
		WeatherAPIKey: getEnv("WEATHER_API_KEY", ""),
		FetchInterval: 15 * time.Minute,
	}

	if interval := os.Getenv("FETCH_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			cfg.FetchInterval = d
		}
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
