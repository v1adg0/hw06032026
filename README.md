# Weather Service

A gRPC weather data collection service that fetches weather from WeatherAPI.com and stores it in PostgreSQL.

## Prerequisites

- Docker and Docker Compose
- WeatherAPI.com API key (free tier: https://www.weatherapi.com/)
- grpcurl or Postman (for testing)

## Quick Start

1. Set your API key:
```bash
export WEATHER_API_KEY=your_api_key_here
```

2. Start the services:
```bash
docker-compose up
```

3. Test with grpcurl:

Add a city:
```bash
grpcurl -plaintext -d '{"name":"London","country":"UK"}' localhost:50051 weather.WeatherService/AddCity
```

Get current weather:
```bash
grpcurl -plaintext -d '{"city":"London","country":"UK"}' localhost:50051 weather.WeatherService/GetCurrentWeather
```

Stream weather history:
```bash
grpcurl -plaintext -d '{"city":"London","country":"UK"}' localhost:50051 weather.WeatherService/StreamHistory
```

List all cities:
```bash
grpcurl -plaintext localhost:50051 weather.WeatherService/ListCities
```

Delete a city:
```bash
grpcurl -plaintext -d '{"name":"London","country":"UK"}' localhost:50051 weather.WeatherService/DeleteCity
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| DATABASE_URL | postgres://postgres:postgres@localhost:5432/weather?sslmode=disable | PostgreSQL connection string |
| GRPC_PORT | 50051 | gRPC server port |
| WEATHER_API_KEY | (required) | WeatherAPI.com API key |
| FETCH_INTERVAL | 15m | Background fetch interval |

## Architecture

```
cmd/server/          - Entry point
internal/
  domain/            - Entities (City, WeatherObservation)
  usecase/           - Business logic + interfaces
  adapter/
    grpc/            - gRPC handlers
    repository/      - PostgreSQL implementation
    weatherapi/      - WeatherAPI.com client
api/proto/           - gRPC definitions
migrations/          - SQL migrations
infrastructure/      - setup, config, server
```

## Assumptions

- **Weather data freshness**: Cached observations less than 5 minutes old are returned for `GetCurrentWeather` to reduce API calls
- **Background fetcher**: Runs at configurable intervals (default 15m) to keep weather data current for all registered cities
- **Streaming**: `StreamHistory` returns all observations within the time range; if no range is specified, returns all history
- **City uniqueness**: Cities are uniquely identified by (name, country) combination

## Testing

Run tests:
```bash
go test ./...
```

## Future Improvements

- Add circuit breaker for external API resilience
- Add pagination for large city lists
- Add metrics and distributed tracing (OpenTelemetry)
- Support forecast data from WeatherAPI.com
