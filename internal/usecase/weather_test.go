package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"weather-service/internal/domain"
)

type mockRepository struct {
	cities       map[string]*domain.City
	observations []*domain.WeatherObservation
	addCityErr   error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		cities:       make(map[string]*domain.City),
		observations: make([]*domain.WeatherObservation, 0),
	}
}

func cityKey(name, country string) string {
	return name + "|" + country
}

func (m *mockRepository) AddCity(ctx context.Context, name, country string) (*domain.City, error) {
	if m.addCityErr != nil {
		return nil, m.addCityErr
	}
	city := &domain.City{ID: int64(len(m.cities) + 1), Name: name, Country: country}
	m.cities[cityKey(name, country)] = city
	return city, nil
}

func (m *mockRepository) GetCity(ctx context.Context, name, country string) (*domain.City, error) {
	city, ok := m.cities[cityKey(name, country)]
	if !ok {
		return nil, errors.New("city not found")
	}
	return city, nil
}

func (m *mockRepository) DeleteCity(ctx context.Context, name, country string) error {
	key := cityKey(name, country)
	if _, ok := m.cities[key]; !ok {
		return errors.New("city not found")
	}
	delete(m.cities, key)
	return nil
}

func (m *mockRepository) ListCities(ctx context.Context) ([]domain.City, error) {
	cities := make([]domain.City, 0, len(m.cities))
	for _, c := range m.cities {
		cities = append(cities, *c)
	}
	return cities, nil
}

func (m *mockRepository) SaveObservation(ctx context.Context, obs *domain.WeatherObservation) error {
	obs.ID = int64(len(m.observations) + 1)
	m.observations = append(m.observations, obs)
	return nil
}

func (m *mockRepository) GetLatestObservation(ctx context.Context, cityName, country string) (*domain.WeatherObservation, error) {
	for i := len(m.observations) - 1; i >= 0; i-- {
		if m.observations[i].CityName == cityName {
			return m.observations[i], nil
		}
	}
	return nil, errors.New("no observations")
}

func (m *mockRepository) StreamObservations(ctx context.Context, cityName, country string, start, end time.Time) (<-chan *domain.WeatherObservation, <-chan error) {
	obsChan := make(chan *domain.WeatherObservation)
	errChan := make(chan error, 1)
	go func() {
		defer close(obsChan)
		defer close(errChan)
		for _, obs := range m.observations {
			if obs.CityName == cityName && !obs.ObservedAt.Before(start) && !obs.ObservedAt.After(end) {
				obsChan <- obs
			}
		}
	}()
	return obsChan, errChan
}

type mockAPIClient struct {
	weather    *domain.WeatherObservation
	weatherErr error
}

func (m *mockAPIClient) GetCurrentWeather(ctx context.Context, city, country string) (*domain.WeatherObservation, error) {
	if m.weatherErr != nil {
		return nil, m.weatherErr
	}
	return m.weather, nil
}

func TestWeatherUseCase_AddCity(t *testing.T) {
	repo := newMockRepository()
	uc := NewWeatherUseCase(repo, nil)

	city, err := uc.AddCity(context.Background(), "London", "UK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if city.Name != "London" || city.Country != "UK" {
		t.Errorf("expected London, UK got %s, %s", city.Name, city.Country)
	}
}

func TestWeatherUseCase_AddCity_AlreadyExists(t *testing.T) {
	repo := newMockRepository()
	repo.cities[cityKey("Paris", "France")] = &domain.City{ID: 1, Name: "Paris", Country: "France"}

	uc := NewWeatherUseCase(repo, nil)
	city, err := uc.AddCity(context.Background(), "Paris", "France")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if city.Name != "Paris" {
		t.Errorf("expected Paris, got %s", city.Name)
	}
}

func TestWeatherUseCase_GetCurrentWeather(t *testing.T) {
	repo := newMockRepository()
	repo.cities[cityKey("London", "UK")] = &domain.City{ID: 1, Name: "London", Country: "UK"}

	apiClient := &mockAPIClient{
		weather: &domain.WeatherObservation{
			CityName:    "London",
			Temperature: 15.5,
			Humidity:    80,
			Description: "Cloudy",
			ObservedAt:  time.Now(),
		},
	}

	uc := NewWeatherUseCase(repo, apiClient)
	obs, err := uc.GetCurrentWeather(context.Background(), "London", "UK")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Temperature != 15.5 {
		t.Errorf("expected temperature 15.5, got %.1f", obs.Temperature)
	}
}

func TestWeatherUseCase_GetCurrentWeather_UseCache(t *testing.T) {
	repo := newMockRepository()
	repo.cities[cityKey("Paris", "France")] = &domain.City{ID: 1, Name: "Paris", Country: "France"}
	repo.observations = append(repo.observations, &domain.WeatherObservation{
		ID:          1,
		CityID:      1,
		CityName:    "Paris",
		Temperature: 20.0,
		ObservedAt:  time.Now().Add(-2 * time.Minute), // recent, should use cache
	})

	uc := NewWeatherUseCase(repo, nil)
	obs, err := uc.GetCurrentWeather(context.Background(), "Paris", "France")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Temperature != 20.0 {
		t.Errorf("expected cached temperature 20.0, got %.1f", obs.Temperature)
	}
}

func TestWeatherUseCase_ListCities(t *testing.T) {
	repo := newMockRepository()
	repo.cities[cityKey("London", "UK")] = &domain.City{ID: 1, Name: "London", Country: "UK"}
	repo.cities[cityKey("Paris", "France")] = &domain.City{ID: 2, Name: "Paris", Country: "France"}

	uc := NewWeatherUseCase(repo, nil)
	cities, err := uc.ListCities(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cities) != 2 {
		t.Errorf("expected 2 cities, got %d", len(cities))
	}
}

func TestWeatherUseCase_DeleteCity(t *testing.T) {
	repo := newMockRepository()
	repo.cities[cityKey("London", "UK")] = &domain.City{ID: 1, Name: "London", Country: "UK"}

	uc := NewWeatherUseCase(repo, nil)
	err := uc.DeleteCity(context.Background(), "London", "UK")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = repo.GetCity(context.Background(), "London", "UK")
	if err == nil {
		t.Error("expected city to be deleted")
	}
}
