package grpc

import (
	"context"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "weather-service/api/proto"
	"weather-service/internal/domain"
	"weather-service/internal/usecase"
)

const (
	// streamRateLimit is the max number of messages per second
	streamRateLimit = 100
	// streamBurstSize is the max burst size for rate limiting
	streamBurstSize = 20
)

type WeatherHandler struct {
	pb.UnimplementedWeatherServiceServer
	uc *usecase.WeatherUseCase
}

func NewWeatherHandler(uc *usecase.WeatherUseCase) *WeatherHandler {
	return &WeatherHandler{uc: uc}
}

// TODO: add maxmindDB for more precise city selection before adding
func (h *WeatherHandler) AddCity(ctx context.Context, req *pb.AddCityRequest) (*pb.AddCityResponse, error) {
	city, err := h.uc.AddCity(ctx, req.Name, req.Country)
	if err != nil {
		return nil, err
	}
	return &pb.AddCityResponse{
		Id:      city.ID,
		Name:    city.Name,
		Country: city.Country,
	}, nil
}

func (h *WeatherHandler) GetCurrentWeather(ctx context.Context, req *pb.GetWeatherRequest) (*pb.WeatherResponse, error) {
	obs, err := h.uc.GetCurrentWeather(ctx, req.City, req.Country)
	if err != nil {
		return nil, err
	}
	return observationToProto(obs), nil
}

func (h *WeatherHandler) StreamHistory(req *pb.StreamHistoryRequest, stream pb.WeatherService_StreamHistoryServer) error {
	start := time.Unix(0, 0)
	end := time.Now()

	if req.StartTime != nil {
		start = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		end = req.EndTime.AsTime()
	}

	ctx := stream.Context()

	// rate limiter for backpressure control
	limiter := rate.NewLimiter(streamRateLimit, streamBurstSize)

	obsChan, errChan := h.uc.StreamHistory(ctx, req.City, req.Country, start, end)
	for obs := range obsChan {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := stream.Send(observationToProto(obs)); err != nil {
			return err
		}
	}

	if err := <-errChan; err != nil {
		return err
	}

	return nil
}

// TODO: add filtering by country, zip code etc

func (h *WeatherHandler) ListCities(ctx context.Context, req *pb.ListCitiesRequest) (*pb.ListCitiesResponse, error) {
	cities, err := h.uc.ListCities(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: add pagination for large city lists
	result := &pb.ListCitiesResponse{
		Cities: make([]*pb.CityInfo, len(cities)),
	}
	for i, city := range cities {
		result.Cities[i] = &pb.CityInfo{
			Id:      city.ID,
			Name:    city.Name,
			Country: city.Country,
		}
	}
	return result, nil
}

// TODO make delete more specific, e.g. by city ID from DB
func (h *WeatherHandler) DeleteCity(ctx context.Context, req *pb.DeleteCityRequest) (*pb.DeleteCityResponse, error) {
	if err := h.uc.DeleteCity(ctx, req.Name, req.Country); err != nil {
		return nil, err
	}
	return &pb.DeleteCityResponse{}, nil
}

func observationToProto(obs *domain.WeatherObservation) *pb.WeatherResponse {
	return &pb.WeatherResponse{
		Id:          obs.ID,
		City:        obs.CityName,
		Temperature: obs.Temperature,
		Humidity:    int32(obs.Humidity),
		Description: obs.Description,
		WindSpeed:   obs.WindSpeed,
		ObservedAt:  timestamppb.New(obs.ObservedAt),
	}
}
