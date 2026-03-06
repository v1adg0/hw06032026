CREATE TABLE cities (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    country VARCHAR(100) NOT NULL DEFAULT '',
    CONSTRAINT cities_name_country_key UNIQUE (name, country)
);

CREATE TABLE weather_observations (
    id SERIAL PRIMARY KEY,
    city_id INTEGER REFERENCES cities(id),
    temperature DECIMAL(5,2),
    humidity INTEGER,
    description VARCHAR(255),
    wind_speed DECIMAL(5,2),
    observed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_observations_city_time ON weather_observations(city_id, observed_at DESC);
