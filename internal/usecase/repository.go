package usecase

import (
	"context"
	"time"

	"weather-service/internal/domain"
)

type WeatherRepository interface {
	AddCity(ctx context.Context, name, country string) (*domain.City, error)
	GetCity(ctx context.Context, name, country string) (*domain.City, error)
	DeleteCity(ctx context.Context, name, country string) error
	ListCities(ctx context.Context) ([]domain.City, error)
	SaveObservation(ctx context.Context, obs *domain.WeatherObservation) error
	GetLatestObservation(ctx context.Context, cityName, country string) (*domain.WeatherObservation, error)
	StreamObservations(ctx context.Context, cityName, country string, start, end time.Time) (<-chan *domain.WeatherObservation, <-chan error)
}

type WeatherAPIClient interface {
	GetCurrentWeather(ctx context.Context, city, country string) (*domain.WeatherObservation, error)
}
