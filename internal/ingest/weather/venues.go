// Package weather fetches stadium forecasts from Open-Meteo. Venue coordinates
// come from a map built from the season's scoreboards, because ESPN's
// scoreboard response does not include latitude or longitude.
package weather

// Venue is an NFL venue and the stadium's coordinates, not its city center.
// ID and Name are ESPN's values, so the map keys match games.venue_id.
type Venue struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
}

// venues holds every venue on the 2026 regular-season scoreboards, weeks 1
// through 18, including the international games. Add a venue when a new
// stadium appears; an unknown ID means the app has no coordinates for it, so
// the game gets no weather.
var venues = map[string]Venue{
	"3687":  {ID: "3687", Name: "AT&T Stadium", Latitude: 32.74785, Longitude: -97.09283},
	"3752":  {ID: "3752", Name: "Acrisure Stadium", Latitude: 40.44672, Longitude: -80.01572},
	"6501":  {ID: "6501", Name: "Allegiant Stadium", Latitude: 36.09074, Longitude: -115.18333},
	"3622":  {ID: "3622", Name: "Arrowhead Stadium", Latitude: 39.04894, Longitude: -94.48400},
	"3628":  {ID: "3628", Name: "Bank of America Stadium", Latitude: 35.22577, Longitude: -80.85283},
	"3493":  {ID: "3493", Name: "Caesars Superdome", Latitude: 29.95105, Longitude: -90.08231},
	"3937":  {ID: "3937", Name: "Empower Field at Mile High", Latitude: 39.74395, Longitude: -105.01922},
	"8219":  {ID: "8219", Name: "Estadio Banorte", Latitude: 19.30298, Longitude: -99.15047},
	"3712":  {ID: "3712", Name: "EverBank Stadium", Latitude: 30.32398, Longitude: -81.63730},
	"11930": {ID: "11930", Name: "FC Bayern Munich Stadium", Latitude: 48.21879, Longitude: 11.62362},
	"3727":  {ID: "3727", Name: "Ford Field", Latitude: 42.33996, Longitude: -83.04562},
	"3738":  {ID: "3738", Name: "Gillette Stadium", Latitude: 42.09089, Longitude: -71.26417},
	"3948":  {ID: "3948", Name: "Hard Rock Stadium", Latitude: 25.95790, Longitude: -80.23885},
	"11938": {ID: "11938", Name: "Highmark Stadium", Latitude: 42.77303, Longitude: -78.79219},
	"3679":  {ID: "3679", Name: "Huntington Bank Field", Latitude: 41.50606, Longitude: -81.69971},
	"3798":  {ID: "3798", Name: "Lambeau Field", Latitude: 44.50097, Longitude: -88.06197},
	"4738":  {ID: "4738", Name: "Levi's Stadium", Latitude: 37.40299, Longitude: -121.96965},
	"3806":  {ID: "3806", Name: "Lincoln Financial Field", Latitude: 39.90091, Longitude: -75.16749},
	"3812":  {ID: "3812", Name: "Lucas Oil Stadium", Latitude: 39.75906, Longitude: -86.16384},
	"3673":  {ID: "3673", Name: "Lumen Field", Latitude: 47.59535, Longitude: -122.33164},
	"3814":  {ID: "3814", Name: "M&T Bank Stadium", Latitude: 39.27797, Longitude: -76.62267},
	"11931": {ID: "11931", Name: "Maracanã Stadium", Latitude: -22.91216, Longitude: -43.23119},
	"9119":  {ID: "9119", Name: "Melbourne Cricket Ground", Latitude: -37.81991, Longitude: 144.98323},
	"5348":  {ID: "5348", Name: "Mercedes-Benz Stadium", Latitude: 33.75541, Longitude: -84.40085},
	"3839":  {ID: "3839", Name: "MetLife Stadium", Latitude: 40.81351, Longitude: -74.07434},
	"3810":  {ID: "3810", Name: "Nissan Stadium", Latitude: 36.16652, Longitude: -86.77131},
	"3719":  {ID: "3719", Name: "Northwest Stadium", Latitude: 38.90767, Longitude: -76.86453},
	"3874":  {ID: "3874", Name: "Paycor Stadium", Latitude: 39.09546, Longitude: -84.51605},
	"3886":  {ID: "3886", Name: "Raymond James Stadium", Latitude: 27.97598, Longitude: -82.50335},
	"3891":  {ID: "3891", Name: "Reliant Stadium", Latitude: 29.68486, Longitude: -95.41080},
	"1353":  {ID: "1353", Name: "Santiago Bernabéu", Latitude: 40.45304, Longitude: -3.68820},
	"7065":  {ID: "7065", Name: "SoFi Stadium", Latitude: 33.95341, Longitude: -118.33901},
	"3933":  {ID: "3933", Name: "Soldier Field", Latitude: 41.86234, Longitude: -87.61765},
	"1781":  {ID: "1781", Name: "Stade de France", Latitude: 48.92447, Longitude: 2.36013},
	"3970":  {ID: "3970", Name: "State Farm Stadium", Latitude: 33.52704, Longitude: -112.26171},
	"5534":  {ID: "5534", Name: "Tottenham Hotspur Stadium", Latitude: 51.60416, Longitude: -0.06626},
	"5239":  {ID: "5239", Name: "U.S. Bank Stadium", Latitude: 44.97362, Longitude: -93.25747},
	"2455":  {ID: "2455", Name: "Wembley Stadium", Latitude: 51.55607, Longitude: -0.27960},
}

// VenueByID returns the venue with ESPN's ID. The bool is false when the map
// does not know the venue.
func VenueByID(id string) (Venue, bool) {
	venue, ok := venues[id]
	return venue, ok
}
