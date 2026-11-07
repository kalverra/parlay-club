package weather

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Forecast is the weather across the three hours that contain kickoff. Wind and
// gust are the window's maximums, precipitation is the window's maximum hourly
// value, and temperature is the value at the kickoff hour.
type Forecast struct {
	TempF       float64
	WindMPH     float64
	WindGustMPH float64
	PrecipIN    float64
}

// windowHours is the kickoff hour plus the two after it.
const windowHours = 3

// hourLayout is Open-Meteo's local time format, and the format its start_hour
// and end_hour params accept.
const hourLayout = "2006-01-02T15:04"

// Forecast fetches the weather for venue across kickoff's three-hour window.
// Request or parse failures are errors; a window the source does not cover is
// also an error, so the caller can skip that game.
func (c *Client) Forecast(ctx context.Context, venue Venue, kickoff time.Time) (Forecast, error) {
	start := kickoff.UTC().Truncate(time.Hour)

	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(venue.Latitude, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(venue.Longitude, 'f', -1, 64))
	query.Set("hourly", "temperature_2m,precipitation,wind_speed_10m,wind_gusts_10m")
	query.Set("wind_speed_unit", "mph")
	query.Set("temperature_unit", "fahrenheit")
	query.Set("precipitation_unit", "inch")
	query.Set("timezone", "UTC")
	query.Set("start_hour", start.Format(hourLayout))
	query.Set("end_hour", start.Add((windowHours-1)*time.Hour).Format(hourLayout))

	var raw forecastResponse
	if err := c.get(ctx, forecastPath, query, &raw); err != nil {
		return Forecast{}, fmt.Errorf("forecast: %w", err)
	}

	forecast, err := parseForecast(raw, start)
	if err != nil {
		return Forecast{}, fmt.Errorf("forecast: %w", err)
	}
	return forecast, nil
}

// forecastResponse is the slice of Open-Meteo's forecast JSON this app reads.
type forecastResponse struct {
	Hourly hourlyResponse `json:"hourly"`
}

// hourlyResponse holds one value per hour for each series this app reads.
type hourlyResponse struct {
	Time          []string   `json:"time"`
	Temperature2m []*float64 `json:"temperature_2m"`
	Precipitation []*float64 `json:"precipitation"`
	WindSpeed10m  []*float64 `json:"wind_speed_10m"`
	WindGusts10m  []*float64 `json:"wind_gusts_10m"`
}

// consistent reports whether every hourly series lines up with the time list.
func (h hourlyResponse) consistent() bool {
	length := len(h.Time)
	return len(h.Temperature2m) == length &&
		len(h.Precipitation) == length &&
		len(h.WindSpeed10m) == length &&
		len(h.WindGusts10m) == length
}

// parseForecast takes the kickoff-hour window out of an hourly response. Every
// hour in the window must be present and set; a missing or null value is an
// error, not a zero.
func parseForecast(raw forecastResponse, start time.Time) (Forecast, error) {
	hourly := raw.Hourly
	if len(hourly.Time) == 0 {
		return Forecast{}, errors.New("no hourly data")
	}
	if !hourly.consistent() {
		return Forecast{}, errors.New("hourly series have different lengths")
	}

	index := -1
	hours := make([]time.Time, len(hourly.Time))
	for i, value := range hourly.Time {
		hour, err := time.Parse(hourLayout, value)
		if err != nil {
			return Forecast{}, fmt.Errorf("hour %q: %w", value, err)
		}
		hours[i] = hour
		if hour.Equal(start) {
			index = i
		}
	}
	if index < 0 {
		return Forecast{}, fmt.Errorf("no forecast for the %s kickoff hour", start.Format(hourLayout))
	}
	if index+windowHours > len(hours) {
		return Forecast{}, errors.New("forecast ends before the kickoff window")
	}

	forecast := Forecast{}
	for offset := range windowHours {
		at := index + offset
		want := start.Add(time.Duration(offset) * time.Hour)
		if !hours[at].Equal(want) {
			return Forecast{}, fmt.Errorf("hour %s is missing from the kickoff window", want.Format(hourLayout))
		}

		temperature, err := hourlyValue(hourly.Temperature2m, at, "temperature")
		if err != nil {
			return Forecast{}, err
		}
		wind, err := hourlyValue(hourly.WindSpeed10m, at, "wind speed")
		if err != nil {
			return Forecast{}, err
		}
		gust, err := hourlyValue(hourly.WindGusts10m, at, "wind gust")
		if err != nil {
			return Forecast{}, err
		}
		precipitation, err := hourlyValue(hourly.Precipitation, at, "precipitation")
		if err != nil {
			return Forecast{}, err
		}

		if offset == 0 {
			forecast.TempF = temperature
		}
		forecast.WindMPH = max(forecast.WindMPH, wind)
		forecast.WindGustMPH = max(forecast.WindGustMPH, gust)
		forecast.PrecipIN = max(forecast.PrecipIN, precipitation)
	}
	return forecast, nil
}

// hourlyValue returns one series value, or an error when the source sent null.
func hourlyValue(values []*float64, index int, field string) (float64, error) {
	value := values[index]
	if value == nil {
		return 0, fmt.Errorf("hour %d: %s is missing", index, field)
	}
	return *value, nil
}
