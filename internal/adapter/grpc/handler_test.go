package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "weather-service/api/proto"
	"weather-service/internal/domain"
)

type mockUseCase struct {
	cities        []domain.City
	addCityResult *domain.City
	addCityErr    error
	weatherResult *domain.WeatherObservation
	weatherErr    error
	deleteCityErr error
	streamObs     []*domain.WeatherObservation
	streamErr     error
}

func (m *mockUseCase) AddCity(ctx context.Context, name, country string) (*domain.City, error) {
	if m.addCityErr != nil {
		return nil, m.addCityErr
	}
	return m.addCityResult, nil
}

func (m *mockUseCase) GetCurrentWeather(ctx context.Context, cityName, country string) (*domain.WeatherObservation, error) {
	if m.weatherErr != nil {
		return nil, m.weatherErr
	}
	return m.weatherResult, nil
}

func (m *mockUseCase) ListCities(ctx context.Context) ([]domain.City, error) {
	return m.cities, nil
}

func (m *mockUseCase) DeleteCity(ctx context.Context, name, country string) error {
	return m.deleteCityErr
}

func (m *mockUseCase) StreamHistory(ctx context.Context, cityName, country string, start, end time.Time) (<-chan *domain.WeatherObservation, <-chan error) {
	obsChan := make(chan *domain.WeatherObservation)
	errChan := make(chan error, 1)
	go func() {
		defer close(obsChan)
		defer close(errChan)
		for _, obs := range m.streamObs {
			select {
			case <-ctx.Done():
				return
			case obsChan <- obs:
			}
		}
		if m.streamErr != nil {
			errChan <- m.streamErr
		}
	}()
	return obsChan, errChan
}

func (m *mockUseCase) StartBackgroundFetcher(ctx context.Context, interval time.Duration) {}

type useCaseInterface interface {
	AddCity(ctx context.Context, name, country string) (*domain.City, error)
	GetCurrentWeather(ctx context.Context, cityName, country string) (*domain.WeatherObservation, error)
	ListCities(ctx context.Context) ([]domain.City, error)
	DeleteCity(ctx context.Context, name, country string) error
	StreamHistory(ctx context.Context, cityName, country string, start, end time.Time) (<-chan *domain.WeatherObservation, <-chan error)
}

type testableHandler struct {
	pb.UnimplementedWeatherServiceServer
	uc useCaseInterface
}

func newTestableHandler(uc useCaseInterface) *testableHandler {
	return &testableHandler{uc: uc}
}

func (h *testableHandler) AddCity(ctx context.Context, req *pb.AddCityRequest) (*pb.AddCityResponse, error) {
	city, err := h.uc.AddCity(ctx, req.Name, req.Country)
	if err != nil {
		return nil, err
	}
	return &pb.AddCityResponse{Id: city.ID, Name: city.Name, Country: city.Country}, nil
}

func (h *testableHandler) GetCurrentWeather(ctx context.Context, req *pb.GetWeatherRequest) (*pb.WeatherResponse, error) {
	obs, err := h.uc.GetCurrentWeather(ctx, req.City, req.Country)
	if err != nil {
		return nil, err
	}
	return observationToProto(obs), nil
}

func (h *testableHandler) ListCities(ctx context.Context, req *pb.ListCitiesRequest) (*pb.ListCitiesResponse, error) {
	cities, err := h.uc.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	result := &pb.ListCitiesResponse{Cities: make([]*pb.CityInfo, len(cities))}
	for i, city := range cities {
		result.Cities[i] = &pb.CityInfo{Id: city.ID, Name: city.Name, Country: city.Country}
	}
	return result, nil
}

func (h *testableHandler) DeleteCity(ctx context.Context, req *pb.DeleteCityRequest) (*pb.DeleteCityResponse, error) {
	if err := h.uc.DeleteCity(ctx, req.Name, req.Country); err != nil {
		return nil, err
	}
	return &pb.DeleteCityResponse{}, nil
}

func (h *testableHandler) StreamHistory(req *pb.StreamHistoryRequest, stream pb.WeatherService_StreamHistoryServer) error {
	start := time.Unix(0, 0)
	end := time.Now()
	if req.StartTime != nil {
		start = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		end = req.EndTime.AsTime()
	}
	obsChan, errChan := h.uc.StreamHistory(stream.Context(), req.City, req.Country, start, end)
	for obs := range obsChan {
		if err := stream.Send(observationToProto(obs)); err != nil {
			return err
		}
	}
	return <-errChan
}

func TestHandler_AddCity(t *testing.T) {
	mock := &mockUseCase{
		addCityResult: &domain.City{ID: 1, Name: "London", Country: "UK"},
	}
	handler := newTestableHandler(mock)

	resp, err := handler.AddCity(context.Background(), &pb.AddCityRequest{Name: "London", Country: "UK"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "London" || resp.Country != "UK" {
		t.Errorf("expected London, UK got %s, %s", resp.Name, resp.Country)
	}
}

func TestHandler_AddCity_Error(t *testing.T) {
	mock := &mockUseCase{addCityErr: errors.New("db error")}
	handler := newTestableHandler(mock)

	_, err := handler.AddCity(context.Background(), &pb.AddCityRequest{Name: "Test", Country: "Test"})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestHandler_GetCurrentWeather(t *testing.T) {
	mock := &mockUseCase{
		weatherResult: &domain.WeatherObservation{
			ID:          1,
			CityName:    "London",
			Temperature: 15.5,
			Humidity:    80,
			ObservedAt:  time.Now(),
		},
	}
	handler := newTestableHandler(mock)

	resp, err := handler.GetCurrentWeather(context.Background(), &pb.GetWeatherRequest{City: "London", Country: "UK"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Temperature != 15.5 {
		t.Errorf("expected temperature 15.5, got %.1f", resp.Temperature)
	}
}

func TestHandler_ListCities(t *testing.T) {
	mock := &mockUseCase{
		cities: []domain.City{
			{ID: 1, Name: "London", Country: "UK"},
			{ID: 2, Name: "Paris", Country: "France"},
		},
	}
	handler := newTestableHandler(mock)

	resp, err := handler.ListCities(context.Background(), &pb.ListCitiesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Cities) != 2 {
		t.Errorf("expected 2 cities, got %d", len(resp.Cities))
	}
}

func TestHandler_DeleteCity(t *testing.T) {
	mock := &mockUseCase{}
	handler := newTestableHandler(mock)

	_, err := handler.DeleteCity(context.Background(), &pb.DeleteCityRequest{Name: "London", Country: "UK"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type mockStreamServer struct {
	grpc.ServerStream
	ctx  context.Context
	sent []*pb.WeatherResponse
}

func (m *mockStreamServer) Send(resp *pb.WeatherResponse) error {
	m.sent = append(m.sent, resp)
	return nil
}

func (m *mockStreamServer) Context() context.Context {
	return m.ctx
}

func TestHandler_StreamHistory(t *testing.T) {
	now := time.Now()
	mock := &mockUseCase{
		streamObs: []*domain.WeatherObservation{
			{ID: 1, CityName: "London", Temperature: 15.0, ObservedAt: now.Add(-1 * time.Hour)},
			{ID: 2, CityName: "London", Temperature: 16.0, ObservedAt: now},
		},
	}
	handler := newTestableHandler(mock)
	stream := &mockStreamServer{ctx: context.Background(), sent: make([]*pb.WeatherResponse, 0)}

	err := handler.StreamHistory(&pb.StreamHistoryRequest{City: "London", Country: "UK"}, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 2 {
		t.Errorf("expected 2 messages, got %d", len(stream.sent))
	}
}
