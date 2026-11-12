// Package sandbox replays a fixed Sunday of NFL data through the ordinary
// ingest clients. It owns the fail-closed replay transport, the named week 9
// response registry, the shared scenario clock, and the stage record the local
// sandbox is built from.
package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

// Stage names one set of replayed responses. Every fixture belongs to exactly
// one stage, so a scenario can swap its scoreboard and research snapshots
// without changing the clients or the database.
type Stage string

// The four scripted situations. Morning is the baseline; the other three are
// independent stages that rebuild morning before applying their manifest.
const (
	// Morning is the Sunday morning baseline: the full week 9 slate is scheduled.
	Morning Stage = "morning"
	// EarlyLoss is a coherent live afternoon: the featured game is in progress
	// and one scripted pick is already lost.
	EarlyLoss Stage = "early-loss"
	// AllClear is Sunday evening: every Sunday game is final and every scripted
	// pick won, while the Monday game is still upcoming.
	AllClear Stage = "all-clear"
	// SundayComplete is Monday night: every Sunday and Monday game is final.
	SundayComplete Stage = "sunday-complete"
)

// Provenance says where a fixture's bytes came from.
type Provenance string

// The two kinds of fixture provenance.
const (
	// Captured is a response saved from the real source and trimmed only by
	// removing whole events or rows.
	Captured Provenance = "captured"
	// Derived is a snapshot edited or synthesized to fill a gap in the
	// captures. Its note says what changed.
	Derived Provenance = "derived"
)

// The week 9 scenario's featured game. Its prop board is the only prop board
// the morning reset needs, because props load lazily when someone opens a game.
const (
	// FeaturedGameID is ESPN's ID for Cincinnati at Atlanta.
	FeaturedGameID = "401873038"
	// FeaturedEventID is Parlay API's event ID for the same game.
	FeaturedEventID = "dc56d980f527ffeef89940133b211b13"
	// FeaturedHomeESPNID and FeaturedAwayESPNID are ESPN's team IDs for the
	// featured game. The props sync reads both rosters before it matches
	// player names, so both are registered.
	FeaturedHomeESPNID = "1"
	FeaturedAwayESPNID = "4"
)

// week9PropAthletes are the ESPN athletes behind the featured prop board. Each
// one's game log is registered so the player research page has last-5 stats
// instead of failing closed. Joe Mixon has props but is not on the captured
// Bengals roster, so his lines stay unmatched until someone uses manual match.
var week9PropAthletes = []struct {
	Name      string
	AthleteID string
	Slug      string
}{
	{Name: "Joe Burrow", AthleteID: "3915511", Slug: "burrow"},
	{Name: "Ja'Marr Chase", AthleteID: "4362628", Slug: "chase"},
	{Name: "Tee Higgins", AthleteID: "4239993", Slug: "higgins"},
	{Name: "Evan McPherson", AthleteID: "4360234", Slug: "mcpherson"},
}

// Fallback answers an optional request whose exact response the scenario does
// not capture. Its file is a provider-shaped empty body, so an ordinary screen
// renders its empty state instead of failing the refresh. An exact fixture
// always wins, so a fallback can never mask a captured response.
type Fallback struct {
	// Name identifies the fallback in errors, tests, and docs.
	Name string
	// Stage is the scenario stage that selects this fallback.
	Stage Stage
	// Method is the HTTP method the request must use.
	Method string
	// URL is the request pattern the fallback answers. A '*' matches one path
	// segment, as in path.Match. Query parameters are ignored.
	URL string
	// File is the empty body's path in the registry's filesystem.
	File string
	// Header is the response header the fallback returns. The registry adds a
	// content type from the file extension when this leaves it empty.
	Header http.Header
	// Note explains why the scenario leaves this surface empty.
	Note string
}

// Fixture describes one registered replay response.
type Fixture struct {
	// Name identifies the fixture in errors, tests, and docs.
	Name string
	// Stage is the scenario stage that selects this response.
	Stage Stage
	// Method is the HTTP method the request must use.
	Method string
	// URL is the request the response answers, including query parameters.
	URL string
	// File is the fixture's path in the registry's filesystem.
	File string
	// Status is the saved HTTP status. Zero means 200.
	Status int
	// Header is the saved response header. The registry adds a content type
	// from the file extension when this leaves it empty.
	Header http.Header
	// Provenance marks the bytes as captured or derived.
	Provenance Provenance
	// Note explains where the fixture came from, or what a derived snapshot
	// was built from. A derived fixture must set it.
	Note string
}

// Registry holds every preloaded response for a scenario. Build one with
// NewRegistry or Week9; it is read-only after construction and safe to share.
type Registry struct {
	fixtures  []Fixture
	fallbacks []emptyResponse
	responses map[requestKey]response
}

// response is one loaded fixture: the bytes and metadata the transport returns.
type response struct {
	status int
	header http.Header
	body   []byte
	// empty marks a fallback body the scenario synthesized instead of
	// capturing. The transport labels it with FallbackHeader.
	empty bool
}

// emptyResponse is one loaded fallback: the path pattern it answers and the
// empty body it returns.
type emptyResponse struct {
	stage       Stage
	method      string
	host        string
	pathPattern string
	response    response
}

// NewRegistry validates every fixture, reads its file, and builds the lookup
// table. A missing file, a duplicate request, or an incomplete fixture is an
// error, so the scenario cannot start with a partial replay set.
func NewRegistry(fsys fs.FS, fixtures []Fixture) (*Registry, error) {
	return NewRegistryWithFallbacks(fsys, fixtures, nil)
}

// NewRegistryWithFallbacks adds empty fallbacks for optional requests the
// scenario does not capture. An exact fixture always wins over a fallback.
func NewRegistryWithFallbacks(fsys fs.FS, fixtures []Fixture, fallbacks []Fallback) (*Registry, error) {
	if fsys == nil {
		return nil, errors.New("sandbox: nil fixture filesystem")
	}
	if len(fixtures) == 0 {
		return nil, errors.New("sandbox: no fixtures registered")
	}

	registry := &Registry{
		fixtures:  make([]Fixture, 0, len(fixtures)),
		fallbacks: make([]emptyResponse, 0, len(fallbacks)),
		responses: make(map[requestKey]response, len(fixtures)),
	}
	for _, fixture := range fixtures {
		location, err := fixture.validate()
		if err != nil {
			return nil, err
		}

		body, err := fs.ReadFile(fsys, fixture.File)
		if err != nil {
			return nil, fmt.Errorf("fixture %q: read %s: %w", fixture.Name, fixture.File, err)
		}

		key, err := requestKeyFor(fixture.Stage, fixture.Method, location)
		if err != nil {
			return nil, fmt.Errorf("fixture %q: %w", fixture.Name, err)
		}
		if _, exists := registry.responses[key]; exists {
			return nil, fmt.Errorf(
				"fixture %q: another response already answers %s %s at stage %q",
				fixture.Name,
				fixture.Method,
				fixture.URL,
				fixture.Stage,
			)
		}

		status := fixture.Status
		if status == 0 {
			status = http.StatusOK
		}
		header := fixture.Header.Clone()
		if header == nil {
			header = make(http.Header)
		}
		if header.Get("Content-Type") == "" {
			header.Set("Content-Type", contentTypeFor(fixture.File))
		}
		fixture.Header = header

		registry.fixtures = append(registry.fixtures, fixture)
		registry.responses[key] = response{
			status: status,
			header: header.Clone(),
			body:   body,
		}
	}

	for _, fallback := range fallbacks {
		empty, err := loadFallback(fsys, fallback)
		if err != nil {
			return nil, err
		}
		for _, existing := range registry.fallbacks {
			if existing.stage == empty.stage &&
				existing.method == empty.method &&
				existing.host == empty.host &&
				existing.pathPattern == empty.pathPattern {
				return nil, fmt.Errorf(
					"fallback %q: another fallback already answers %s %s at stage %q",
					fallback.Name,
					fallback.Method,
					fallback.URL,
					fallback.Stage,
				)
			}
		}
		registry.fallbacks = append(registry.fallbacks, empty)
	}
	return registry, nil
}

// loadFallback validates one fallback and reads its empty body.
func loadFallback(fsys fs.FS, fallback Fallback) (emptyResponse, error) {
	location, err := fallback.validate()
	if err != nil {
		return emptyResponse{}, err
	}

	body, err := fs.ReadFile(fsys, fallback.File)
	if err != nil {
		return emptyResponse{}, fmt.Errorf("fallback %q: read %s: %w", fallback.Name, fallback.File, err)
	}

	header := fallback.Header.Clone()
	if header == nil {
		header = make(http.Header)
	}
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", contentTypeFor(fallback.File))
	}

	return emptyResponse{
		stage:       fallback.Stage,
		method:      strings.ToUpper(fallback.Method),
		host:        strings.ToLower(location.Host),
		pathPattern: location.EscapedPath(),
		response: response{
			status: http.StatusOK,
			header: header,
			body:   body,
			empty:  true,
		},
	}, nil
}

// fallback returns the empty response for a request the registry does not
// answer exactly. The first matching pattern wins.
func (r *Registry) fallback(key requestKey) (response, bool) {
	for _, empty := range r.fallbacks {
		if empty.stage != key.stage || empty.method != key.method || empty.host != key.host {
			continue
		}
		matched, err := path.Match(empty.pathPattern, key.path)
		if err != nil {
			continue // NewRegistryWithFallbacks already rejected a bad pattern.
		}
		if matched {
			return empty.response, true
		}
	}
	return response{}, false
}

// Fixtures returns the registered fixtures in registration order. The returned
// headers are copies, so a caller cannot change the registry.
func (r *Registry) Fixtures() []Fixture {
	fixtures := make([]Fixture, len(r.fixtures))
	for i, fixture := range r.fixtures {
		fixture.Header = fixture.Header.Clone()
		fixtures[i] = fixture
	}
	return fixtures
}

// validate checks a fixture and returns its parsed location.
func (f Fixture) validate() (*url.URL, error) {
	switch {
	case f.Name == "":
		return nil, errors.New("sandbox: fixture has no name")
	case f.Stage == "":
		return nil, fmt.Errorf("fixture %q: no stage", f.Name)
	case f.Method == "":
		return nil, fmt.Errorf("fixture %q: no method", f.Name)
	case f.URL == "":
		return nil, fmt.Errorf("fixture %q: no URL", f.Name)
	case f.File == "":
		return nil, fmt.Errorf("fixture %q: no file", f.Name)
	case f.Provenance != Captured && f.Provenance != Derived:
		return nil, fmt.Errorf("fixture %q: unknown provenance %q", f.Name, f.Provenance)
	case f.Provenance == Derived && strings.TrimSpace(f.Note) == "":
		return nil, fmt.Errorf("fixture %q: a derived fixture needs a note", f.Name)
	case f.Status != 0 && (f.Status < 100 || f.Status > 599):
		return nil, fmt.Errorf("fixture %q: status %d is not an HTTP status", f.Name, f.Status)
	}

	location, err := url.Parse(f.URL)
	if err != nil {
		return nil, fmt.Errorf("fixture %q: parse URL: %w", f.Name, err)
	}
	if location.Scheme != "http" && location.Scheme != "https" {
		return nil, fmt.Errorf("fixture %q: URL scheme %q is not HTTP", f.Name, location.Scheme)
	}
	if location.Host == "" {
		return nil, fmt.Errorf("fixture %q: URL has no host", f.Name)
	}
	return location, nil
}

// validate checks a fallback and returns its parsed location pattern.
func (f Fallback) validate() (*url.URL, error) {
	switch {
	case f.Name == "":
		return nil, errors.New("sandbox: fallback has no name")
	case f.Stage == "":
		return nil, fmt.Errorf("fallback %q: no stage", f.Name)
	case f.Method == "":
		return nil, fmt.Errorf("fallback %q: no method", f.Name)
	case f.URL == "":
		return nil, fmt.Errorf("fallback %q: no URL", f.Name)
	case f.File == "":
		return nil, fmt.Errorf("fallback %q: no file", f.Name)
	case strings.TrimSpace(f.Note) == "":
		return nil, fmt.Errorf("fallback %q: an empty response needs a note", f.Name)
	}

	location, err := url.Parse(f.URL)
	if err != nil {
		return nil, fmt.Errorf("fallback %q: parse URL: %w", f.Name, err)
	}
	if location.Scheme != "http" && location.Scheme != "https" {
		return nil, fmt.Errorf("fallback %q: URL scheme %q is not HTTP", f.Name, location.Scheme)
	}
	if location.Host == "" {
		return nil, fmt.Errorf("fallback %q: URL has no host", f.Name)
	}
	if location.Path == "" {
		return nil, fmt.Errorf("fallback %q: URL has no path", f.Name)
	}
	if _, err := path.Match(location.EscapedPath(), ""); err != nil {
		return nil, fmt.Errorf("fallback %q: path %q is not a match pattern: %w", f.Name, location.Path, err)
	}
	return location, nil
}

// contentTypeFor guesses the saved response's content type from its extension.
func contentTypeFor(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}

// Week9 preloads the week 9 morning registry. fsys must be rooted at the
// repository so the fixture paths resolve without copying the saved responses.
func Week9(fsys fs.FS) (*Registry, error) {
	fixtures, err := week9Fixtures()
	if err != nil {
		return nil, err
	}
	return NewRegistryWithFallbacks(fsys, fixtures, week9Fallbacks())
}

// week9Fallbacks answer optional research and prop requests for games the
// scenario does not capture. The bodies are provider-shaped empty payloads, so
// opening another slate game renders its normal empty state instead of failing
// the sync. The featured game's exact fixtures still win.
func week9Fallbacks() []Fallback {
	empties := []struct {
		name   string
		url    string
		file   string
		header http.Header
		note   string
	}{
		{
			name:   "props-empty",
			url:    odds.DefaultBaseURL + "/props",
			file:   "internal/ingest/odds/testdata/props_empty.json",
			header: creditHeader(614, 6, 3),
			note:   "An empty prop board for a slate game the scenario does not cover; only the featured game has captured props.",
		},
		{
			name: "summary-empty",
			url:  espn.DefaultBaseURL + "/summary",
			file: "internal/ingest/espn/testdata/summary_empty.json",
			note: "An empty summary for a slate game the scenario does not cover, so its research page shows no injuries.",
		},
		{
			name: "roster-empty",
			url:  espn.DefaultBaseURL + "/teams/*/roster",
			file: "internal/ingest/espn/testdata/roster_empty.json",
			note: "An empty roster for a team the scenario does not capture; only the featured game's two teams have real rosters.",
		},
		{
			name: "gamelog-empty",
			url:  espn.DefaultWebBaseURL + "/athletes/*/gamelog",
			file: "internal/ingest/espn/testdata/gamelog_empty.json",
			note: "An empty game log for an athlete the scenario does not capture, so player research shows no games.",
		},
	}

	fallbacks := make([]Fallback, 0, len(Stages())*len(empties))
	for _, stage := range Stages() {
		for _, empty := range empties {
			fallbacks = append(fallbacks, Fallback{
				Name:   empty.name + "-" + string(stage),
				Stage:  stage,
				Method: http.MethodGet,
				URL:    empty.url,
				File:   empty.file,
				Header: empty.header,
				Note:   empty.note,
			})
		}
	}
	return fallbacks
}

// week9Fixtures is the complete registry. The event and line fixtures were
// captured from The Odds API, whose wire format Parlay API matches. The morning
// set answers the baseline refresh; each later stage replays the same supporting
// responses with its own scoreboard snapshot.
func week9Fixtures() ([]Fixture, error) {
	morning, err := morningFixtures()
	if err != nil {
		return nil, err
	}

	fixtures := append([]Fixture(nil), morning...)
	for _, scoreboard := range week9StageScoreboards {
		fixtures = append(fixtures, stageFixtures(morning, scoreboard)...)
	}
	return fixtures, nil
}

// morningFixtures is the baseline response set: the full scheduled slate, game
// lines, nfelo projections, every outdoor forecast, and the featured game's
// lazy prop board.
func morningFixtures() ([]Fixture, error) {
	fixtures := []Fixture{
		{
			Name:       "espn-week-9-scoreboard",
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        espn.DefaultBaseURL + "/scoreboard",
			File:       "internal/ingest/espn/testdata/scoreboard_week.json",
			Provenance: Captured,
			Note:       "Captured from ESPN on 2026-11-07. Week 9 includes the Thursday game, which the parser drops.",
		},
		{
			Name:       "parlay-week-9-events",
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        odds.DefaultBaseURL + "/events",
			File:       "internal/ingest/odds/testdata/events_nfl.json",
			Header:     creditHeader(620, 0, 0),
			Provenance: Captured,
			Note:       "Captured from The Odds API on 2026-11-07; Parlay API returns the same event-list shape.",
		},
		{
			Name:   "parlay-week-9-lines",
			Stage:  Morning,
			Method: http.MethodGet,
			URL: endpoint(odds.DefaultBaseURL, "/odds", url.Values{
				"regions":      {"us"},
				"markets":      {"h2h,spreads,totals"},
				"oddsFormat":   {"american"},
				"includeLinks": {"true"},
				"includeSids":  {"true"},
			}),
			File:       "internal/ingest/odds/testdata/odds_nfl.json",
			Header:     creditHeader(617, 3, 3),
			Provenance: Captured,
			Note:       "Captured from The Odds API on 2026-11-07; Parlay API returns the same game-line shape.",
		},
		{
			Name:   "parlay-week-9-featured-props",
			Stage:  Morning,
			Method: http.MethodGet,
			URL: endpoint(odds.DefaultBaseURL, "/props", url.Values{
				"eventId":      {FeaturedEventID},
				"markets":      {week9PropMarkets},
				"oddsFormat":   {"american"},
				"includeLinks": {"true"},
				"includeSids":  {"true"},
				"limit":        {"10000"},
			}),
			File:       "internal/ingest/odds/testdata/props_sample.json",
			Header:     creditHeader(614, 6, 3),
			Provenance: Derived,
			Note:       "Hand-built fixture shaped like Parlay API's /props response, not a live capture.",
		},
		{
			Name:       "nfelo-week-9-projections",
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        nfelo.DefaultBaseURL + "/historic_projected_spreads.csv",
			File:       "internal/ingest/nfelo/testdata/historic_projected_spreads.csv",
			Provenance: Captured,
			Note:       "The real nfelo file saved on 2026-11-09, unedited; it includes 2026 week 9.",
		},
	}

	researchFixtures := week9FeaturedResearch()
	fixtures = append(fixtures, researchFixtures...)

	weatherFixtures, err := week9Weather()
	if err != nil {
		return nil, err
	}
	return append(fixtures, weatherFixtures...), nil
}

// week9FeaturedResearch answers the featured game's research surface: both team
// rosters, the game summary's injury report, and the game logs behind the prop
// board's players. The props sync reads the rosters before it matches names, so
// opening the featured game's props spends no errors and resolves its players.
func week9FeaturedResearch() []Fixture {
	fixtures := []Fixture{
		{
			Name:       "espn-featured-home-roster",
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        espn.DefaultBaseURL + "/teams/" + FeaturedHomeESPNID + "/roster",
			File:       "internal/ingest/espn/testdata/roster_falcons.json",
			Provenance: Captured,
			Note:       "Captured from ESPN on 2026-11-11. Atlanta's roster, read by the props sync before it matches player names.",
		},
		{
			Name:       "espn-featured-away-roster",
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        espn.DefaultBaseURL + "/teams/" + FeaturedAwayESPNID + "/roster",
			File:       "internal/ingest/espn/testdata/roster_bengals.json",
			Provenance: Captured,
			Note:       "Captured from ESPN on 2026-11-11. Cincinnati's roster, which resolves the featured prop board's players.",
		},
		{
			Name:   "espn-week-9-featured-summary",
			Stage:  Morning,
			Method: http.MethodGet,
			URL: endpoint(espn.DefaultBaseURL, "/summary", url.Values{
				"event": {FeaturedGameID},
			}),
			File:       "internal/ingest/espn/testdata/summary_week9_featured.json",
			Provenance: Captured,
			Note:       "Captured from ESPN on 2026-11-11. The featured game's summary, whose injury report the research page shows.",
		},
	}

	for _, athlete := range week9PropAthletes {
		fixtures = append(fixtures, Fixture{
			Name:       "espn-gamelog-" + athlete.Slug,
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        espn.DefaultWebBaseURL + "/athletes/" + athlete.AthleteID + "/gamelog",
			File:       "internal/ingest/espn/testdata/gamelog_" + athlete.Slug + ".json",
			Provenance: Captured,
			Note:       "Captured from ESPN on 2026-11-11. " + athlete.Name + "'s game log, behind the featured prop board's player research page.",
		})
	}

	return fixtures
}

// week9StageScoreboards overrides the scoreboard for each later stage. Every
// other supporting response is reused from the morning set, because a stage
// refresh still needs lines, projections, and forecasts that are due at its
// clock. The files are derived from the captured week 9 slate.
var week9StageScoreboards = []Fixture{
	{
		Name:       "espn-week-9-scoreboard-early-loss",
		Stage:      EarlyLoss,
		Method:     http.MethodGet,
		URL:        espn.DefaultBaseURL + "/scoreboard",
		File:       "internal/ingest/espn/testdata/scoreboard_week9_live.json",
		Provenance: Derived,
		Note:       "Derived from the captured week 9 slate: the Madrid game is live at Atlanta 17, Cincinnati 10 and every other slate game is scheduled.",
	},
	{
		Name:       "espn-week-9-scoreboard-all-clear",
		Stage:      AllClear,
		Method:     http.MethodGet,
		URL:        espn.DefaultBaseURL + "/scoreboard",
		File:       "internal/ingest/espn/testdata/scoreboard_week9_final_sunday.json",
		Provenance: Derived,
		Note:       "Derived from the captured week 9 slate: every Sunday game is final with a scripted score and the Monday game is scheduled.",
	},
	{
		Name:       "espn-week-9-scoreboard-sunday-complete",
		Stage:      SundayComplete,
		Method:     http.MethodGet,
		URL:        espn.DefaultBaseURL + "/scoreboard",
		File:       "internal/ingest/espn/testdata/scoreboard_week9_complete.json",
		Provenance: Derived,
		Note:       "Derived from the captured week 9 slate: every Sunday and Monday game is final with a scripted score.",
	},
}

// stageFixtures copies the morning supporting responses into one stage and adds
// that stage's scoreboard. The registry keys include the stage, so the same URL
// can answer with a different snapshot in each situation.
func stageFixtures(morning []Fixture, scoreboard Fixture) []Fixture {
	fixtures := make([]Fixture, 0, len(morning))
	for _, fixture := range morning {
		if fixture.Name == "espn-week-9-scoreboard" {
			continue
		}
		fixture.Name = fixture.Name + "-" + string(scoreboard.Stage)
		fixture.Stage = scoreboard.Stage
		fixtures = append(fixtures, fixture)
	}
	return append(fixtures, scoreboard)
}

// week9PropMarkets mirrors the market filter in odds.Client.Props. A change
// there must change here too, or the replayed props request stops matching.
const week9PropMarkets = "team_totals," +
	"player_pass_yds," +
	"player_pass_tds," +
	"player_rush_yds," +
	"player_rush_attempts," +
	"player_reception_yds," +
	"player_receptions," +
	"player_anytime_td," +
	"player_kicking_points"

// creditHeader builds the Parlay API credit headers. Every balance stays above
// the refresh floors, so the normal line and prop paths run.
func creditHeader(remaining, used, last int) http.Header {
	return http.Header{
		"X-Requests-Remaining": {strconv.Itoa(remaining)},
		"X-Requests-Used":      {strconv.Itoa(used)},
		"X-Requests-Last":      {strconv.Itoa(last)},
	}
}

// endpoint joins a base URL, path, and query the way the ingest clients do.
func endpoint(base, path string, query url.Values) string {
	return base + path + "?" + query.Encode()
}

// weatherWindow names one forecast the morning refresh needs.
type weatherWindow struct {
	name       string
	venueID    string
	kickoff    time.Time
	file       string
	provenance Provenance
	note       string
}

// week9Weather lists every outdoor game whose kickoff is inside the morning
// refresh's 48-hour weather window. Indoor games need no forecast.
func week9Weather() ([]Fixture, error) {
	windows := []weatherWindow{
		{
			name: "weather-arrowhead", venueID: "3622",
			kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_arrowhead.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-hard-rock", venueID: "3948",
			kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_hard_rock.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-lincoln-financial", venueID: "3806",
			kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_lincoln_financial.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-northwest", venueID: "3719",
			kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_northwest.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-bank-of-america", venueID: "3628",
			kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_bank_of_america.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-sofi", venueID: "7065",
			kickoff:    time.Date(2026, time.November, 8, 21, 5, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_sofi.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-gillette", venueID: "3738",
			kickoff:    time.Date(2026, time.November, 8, 21, 25, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_gillette.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-levis", venueID: "4738",
			kickoff:    time.Date(2026, time.November, 8, 21, 25, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_levis.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
		{
			name: "weather-lumen", venueID: "3673",
			kickoff:    time.Date(2026, time.November, 8, 21, 25, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_lumen.json",
			provenance: Captured,
			note:       "Captured from Open-Meteo on 2026-11-07, unedited.",
		},
		{
			name: "weather-soldier", venueID: "3933",
			kickoff:    time.Date(2026, time.November, 9, 1, 20, 0, 0, time.UTC),
			file:       "internal/ingest/weather/testdata/forecast_soldier.json",
			provenance: Derived,
			note:       "Derived from the week 9 kickoff window; Open-Meteo no longer serves this forecast.",
		},
	}

	fixtures := make([]Fixture, 0, len(windows))
	for _, window := range windows {
		venue, ok := weather.VenueByID(window.venueID)
		if !ok {
			return nil, fmt.Errorf("weather fixture %q: venue %s is not in the venue map", window.name, window.venueID)
		}
		fixtures = append(fixtures, Fixture{
			Name:       window.name,
			Stage:      Morning,
			Method:     http.MethodGet,
			URL:        weatherURL(venue, window.kickoff),
			File:       window.file,
			Provenance: window.provenance,
			Note:       window.note,
		})
	}
	return fixtures, nil
}

// weatherURL builds the request weather.Client.Forecast sends for one game.
func weatherURL(venue weather.Venue, kickoff time.Time) string {
	start := kickoff.UTC().Truncate(time.Hour)

	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(venue.Latitude, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(venue.Longitude, 'f', -1, 64))
	query.Set("hourly", "temperature_2m,precipitation,wind_speed_10m,wind_gusts_10m")
	query.Set("wind_speed_unit", "mph")
	query.Set("temperature_unit", "fahrenheit")
	query.Set("precipitation_unit", "inch")
	query.Set("timezone", "UTC")
	query.Set("start_hour", start.Format("2006-01-02T15:04"))
	query.Set("end_hour", start.Add(2*time.Hour).Format("2006-01-02T15:04"))
	return endpoint(weather.DefaultBaseURL, "/v1/forecast", query)
}
