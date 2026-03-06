package weatherapi

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClient_GetCurrentWeather_Integration(t *testing.T) {
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		t.Skip("WEATHER_API_KEY not set, skipping integration test")
	}

	client := NewClient(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	obs, err := client.GetCurrentWeather(ctx, "London", "UK")
	if err != nil {
		t.Fatalf("failed to get weather: %v", err)
	}

	if obs.CityName != "London" {
		t.Errorf("expected city London, got %s", obs.CityName)
	}

	if obs.Temperature < -50 || obs.Temperature > 60 {
		t.Errorf("temperature out of reasonable range: %.1f", obs.Temperature)
	}

	if obs.Humidity < 0 || obs.Humidity > 100 {
		t.Errorf("humidity out of range: %d", obs.Humidity)
	}

	if obs.Description == "" {
		t.Error("description is empty")
	}

	t.Logf("Weather in London: %.1f°C, %d%% humidity, %s, wind %.1f kph",
		obs.Temperature, obs.Humidity, obs.Description, obs.WindSpeed)
}
