package usecase

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"weather-service/internal/domain"
)

type WeatherUseCase struct {
	repo      WeatherRepository
	apiClient WeatherAPIClient
}

func NewWeatherUseCase(repo WeatherRepository, apiClient WeatherAPIClient) *WeatherUseCase {
	return &WeatherUseCase{
		repo:      repo,
		apiClient: apiClient,
	}
}

func (uc *WeatherUseCase) AddCity(ctx context.Context, name, country string) (*domain.City, error) {
	city, err := uc.repo.GetCity(ctx, name, country)
	if err == nil {
		return city, nil
	}

	return uc.repo.AddCity(ctx, name, country)
}

func (uc *WeatherUseCase) GetCurrentWeather(ctx context.Context, cityName, country string) (*domain.WeatherObservation, error) {
	latest, err := uc.repo.GetLatestObservation(ctx, cityName, country)
	if err == nil && time.Since(latest.ObservedAt) < 5*time.Minute {
		return latest, nil
	}

	obs, err := uc.apiClient.GetCurrentWeather(ctx, cityName, country)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}

	city, err := uc.repo.GetCity(ctx, cityName, country)
	if err != nil {
		return nil, fmt.Errorf("city not found: %w", err)
	}

	obs.CityID = city.ID
	obs.CityName = city.Name

	if err := uc.repo.SaveObservation(ctx, obs); err != nil {
		log.Warnf("failed to save observation: %v", err)
	}

	return obs, nil
}

func (uc *WeatherUseCase) StreamHistory(ctx context.Context, cityName, country string, start, end time.Time) (<-chan *domain.WeatherObservation, <-chan error) {
	return uc.repo.StreamObservations(ctx, cityName, country, start, end)
}

func (uc *WeatherUseCase) DeleteCity(ctx context.Context, name, country string) error {
	return uc.repo.DeleteCity(ctx, name, country)
}

func (uc *WeatherUseCase) ListCities(ctx context.Context) ([]domain.City, error) {
	return uc.repo.ListCities(ctx)
}

func (uc *WeatherUseCase) StartBackgroundFetcher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Info("background fetcher stopped")
				return
			case <-ticker.C:
				uc.fetchAllCities(ctx)
			}
		}
	}()
	log.Infof("background fetcher started with interval %v", interval)
}

func (uc *WeatherUseCase) fetchAllCities(ctx context.Context) {
	cities, err := uc.repo.ListCities(ctx)
	if err != nil {
		log.Errorf("failed to list cities: %v", err)
		return
	}

	for _, city := range cities {
		obs, err := uc.apiClient.GetCurrentWeather(ctx, city.Name, city.Country)
		if err != nil {
			log.Warnf("failed to fetch weather for %s, %s: %v", city.Name, city.Country, err)
			continue
		}

		obs.CityID = city.ID
		obs.CityName = city.Name

		if err := uc.repo.SaveObservation(ctx, obs); err != nil {
			log.Warnf("failed to save observation for %s, %s: %v", city.Name, city.Country, err)
			continue
		}

		log.Infof("fetched weather for %s, %s: temperature=%.2f", city.Name, city.Country, obs.Temperature)
	}
}
