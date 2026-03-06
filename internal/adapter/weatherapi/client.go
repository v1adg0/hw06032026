package weatherapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"

	"weather-service/internal/domain"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.weatherapi.com/v1",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type weatherAPIResponse struct {
	Current struct {
		TempC     float64 `json:"temp_c"`
		Humidity  int     `json:"humidity"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
		WindKph float64 `json:"wind_kph"`
	} `json:"current"`
}

// TODO: add circuit breaker for API resilience
func (c *Client) GetCurrentWeather(ctx context.Context, city, country string) (*domain.WeatherObservation, error) {
	var obs *domain.WeatherObservation

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 500 * time.Millisecond
	bo.MaxElapsedTime = 10 * time.Second

	operation := func() error {
		var err error
		obs, err = c.doRequest(ctx, city, country)
		if err != nil {
			if !isRetryable(err) {
				return backoff.Permanent(err)
			}
			return err
		}
		return nil
	}

	if err := backoff.Retry(operation, backoff.WithContext(bo, ctx)); err != nil {
		return nil, err
	}

	return obs, nil
}

func (c *Client) doRequest(ctx context.Context, city, country string) (*domain.WeatherObservation, error) {
	query := city
	if country != "" {
		query = city + ", " + country
	}
	endpoint := fmt.Sprintf("%s/current.json?key=%s&q=%s", c.baseURL, c.apiKey, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &retryableError{err: fmt.Errorf("execute request: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, &retryableError{err: fmt.Errorf("server error: %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp weatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &domain.WeatherObservation{
		CityName:    city,
		Temperature: apiResp.Current.TempC,
		Humidity:    apiResp.Current.Humidity,
		Description: apiResp.Current.Condition.Text,
		WindSpeed:   apiResp.Current.WindKph,
		ObservedAt:  time.Now(),
	}, nil
}

type retryableError struct {
	err error
}

func (e *retryableError) Error() string {
	return e.err.Error()
}

func (e *retryableError) Unwrap() error {
	return e.err
}

func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}
