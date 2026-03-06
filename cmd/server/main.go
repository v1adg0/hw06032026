package main

import (
	"context"

	log "github.com/sirupsen/logrus"

	"weather-service/infrastructure/config"
	"weather-service/infrastructure/database"
	"weather-service/infrastructure/server"
	"weather-service/internal/adapter/repository"
	"weather-service/internal/adapter/weatherapi"
	"weather-service/internal/usecase"
)

func main() {
	cfg := config.Load()

	if cfg.WeatherAPIKey == "" {
		log.Fatal("WEATHER_API_KEY environment variable is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Info("connected to database")

	repo := repository.NewPostgresRepository(pool)
	apiClient := weatherapi.NewClient(cfg.WeatherAPIKey)
	uc := usecase.NewWeatherUseCase(repo, apiClient)

	uc.StartBackgroundFetcher(ctx, cfg.FetchInterval)

	// TODO: consider adding metrics/tracing

	grpcServer, err := server.NewGRPCServer(cfg, uc)
	if err != nil {
		log.Fatalf("failed to create gRPC server: %v", err)
	}

	if err := grpcServer.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
