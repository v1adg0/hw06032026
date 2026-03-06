package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"weather-service/internal/domain"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) AddCity(ctx context.Context, name, country string) (*domain.City, error) {
	var city domain.City

	err := r.pool.QueryRow(ctx,
		"INSERT INTO cities (name, country) VALUES ($1, $2) RETURNING id, name, country",
		name, country,
	).Scan(&city.ID, &city.Name, &city.Country)
	if err != nil {
		return nil, fmt.Errorf("insert city: %w", err)
	}
	return &city, nil
}

func (r *PostgresRepository) GetCity(ctx context.Context, name, country string) (*domain.City, error) {
	var city domain.City

	err := r.pool.QueryRow(ctx,
		"SELECT id, name, country FROM cities WHERE name = $1 AND country = $2",
		name, country,
	).Scan(&city.ID, &city.Name, &city.Country)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("city not found: %s, %s", name, country)
		}
		return nil, fmt.Errorf("query city: %w", err)
	}

	return &city, nil
}

func (r *PostgresRepository) DeleteCity(ctx context.Context, name, country string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var cityID int64
	err = tx.QueryRow(ctx,
		"SELECT id FROM cities WHERE name = $1 AND country = $2",
		name, country,
	).Scan(&cityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("city not found: %s, %s", name, country)
		}
		return fmt.Errorf("query city: %w", err)
	}

	_, err = tx.Exec(ctx, "DELETE FROM weather_observations WHERE city_id = $1", cityID)
	if err != nil {
		return fmt.Errorf("delete observations: %w", err)
	}

	_, err = tx.Exec(ctx, "DELETE FROM cities WHERE id = $1", cityID)
	if err != nil {
		return fmt.Errorf("delete city: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListCities(ctx context.Context) ([]domain.City, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, name, country FROM cities ORDER BY name, country")
	if err != nil {
		return nil, fmt.Errorf("query cities: %w", err)
	}
	defer rows.Close()

	var cities []domain.City
	for rows.Next() {
		var city domain.City
		if err := rows.Scan(&city.ID, &city.Name, &city.Country); err != nil {
			return nil, fmt.Errorf("scan city: %w", err)
		}
		cities = append(cities, city)
	}
	return cities, nil
}

func (r *PostgresRepository) SaveObservation(ctx context.Context, obs *domain.WeatherObservation) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO weather_observations
    	(city_id, temperature, humidity, description, wind_speed, observed_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		obs.CityID, obs.Temperature, obs.Humidity, obs.Description, obs.WindSpeed, obs.ObservedAt,
	)
	if err != nil {
		return fmt.Errorf("insert observation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetLatestObservation(ctx context.Context, cityName, country string) (*domain.WeatherObservation, error) {
	var obs domain.WeatherObservation
	err := r.pool.QueryRow(ctx,
		`SELECT wo.id, wo.city_id, c.name, wo.temperature, wo.humidity, wo.description, wo.wind_speed, wo.observed_at
		 FROM weather_observations wo
		 JOIN cities c ON c.id = wo.city_id
		 WHERE c.name = $1 AND c.country = $2
		 ORDER BY wo.observed_at DESC
		 LIMIT 1`,
		cityName, country,
	).Scan(&obs.ID, &obs.CityID, &obs.CityName, &obs.Temperature, &obs.Humidity, &obs.Description, &obs.WindSpeed, &obs.ObservedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no observations for city: %s, %s", cityName, country)
		}
		return nil, fmt.Errorf("query observation: %w", err)
	}
	return &obs, nil
}

func (r *PostgresRepository) StreamObservations(ctx context.Context, cityName, country string, start, end time.Time) (<-chan *domain.WeatherObservation, <-chan error) {
	obsChan := make(chan *domain.WeatherObservation)
	errChan := make(chan error, 1)

	go func() {
		defer close(obsChan)
		defer close(errChan)

		rows, err := r.pool.Query(ctx,
			`SELECT wo.id, wo.city_id, c.name, wo.temperature, wo.humidity, wo.description, wo.wind_speed, wo.observed_at
			 FROM weather_observations wo
			 JOIN cities c ON c.id = wo.city_id
			 WHERE c.name = $1 AND c.country = $2 AND wo.observed_at >= $3 AND wo.observed_at <= $4
			 ORDER BY wo.observed_at ASC`,
			cityName, country, start, end,
		)
		if err != nil {
			errChan <- fmt.Errorf("query observations: %w", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var obs domain.WeatherObservation
			if err := rows.Scan(&obs.ID, &obs.CityID, &obs.CityName, &obs.Temperature, &obs.Humidity, &obs.Description, &obs.WindSpeed, &obs.ObservedAt); err != nil {
				errChan <- fmt.Errorf("scan observation: %w", err)
				return
			}
			select {
			case obsChan <- &obs:
			case <-ctx.Done():
				return
			}
		}
	}()

	return obsChan, errChan
}
