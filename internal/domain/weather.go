package domain

import "time"

type City struct {
	ID      int64
	Name    string
	Country string
}

type WeatherObservation struct {
	ID          int64
	CityID      int64
	CityName    string
	Temperature float64
	Humidity    int
	Description string
	WindSpeed   float64
	ObservedAt  time.Time
}
