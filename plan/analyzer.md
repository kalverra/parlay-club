# Analyzer

Package `internal/analyzer`. Pure and deterministic: no I/O, no clock, no LLM calls. It runs on every slip render; its
output is not stored.

## Interface

```go
type Severity int

const (
    Info Severity = iota
    Warn
    Danger
)

type Warning struct {
    Rule     string   // stable ID from the rule list below
    Severity Severity
    Message  string
    LegIDs   []int64  // legs involved, in slip order
}

type Leg struct {
    ID         int64
    Owner      string  // user name, for messages
    GameID     string  // "" when unknown
    Market     string  // product.md market names
    Team       string  // ESPN abbreviation; "" for TOTAL
    PlayerID   string  // ESPN athlete ID; "" when unmatched
    PlayerName string
    Side       string  // TEAM, OVER, UNDER, YES
    Line       *float64
    Odds       int     // American
}

type Game struct {
    ID       string
    Home     string
    Away     string
    Indoor   bool
    Weather  *Weather // nil when unknown
}

type Weather struct {
    WindMPH, GustMPH, PrecipIn, TempF float64
}

type PlayerStats struct {
    // Values per market, most recent game first, this season only.
    Games map[string][]float64
}

func AnalyzeSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) []Warning
```

- Legs with market `OTHER`, an empty `GameID`, or a `GameID` missing from `games` are skipped by every rule.
- Output is sorted by severity (Danger first), then by the first leg's position in the slip. Same input → same output.
- "Same team" means equal `Team`. "Opponent" is the other team in the leg's game.
- A **favorite** leg is a `SPREAD` with line < 0 or an `ML` with odds < 0. An **underdog** leg is a `SPREAD` with line
  > 0 or an `ML` with odds > 0.
- **Passing/receiving over**: side OVER on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, or `RECEPTIONS`.

## Tone

Messages read like a sharp friend's heads-up: short, specific, names the people and the numbers, suggests an
alternative, never says "don't." When a rule involves two people's legs, name both ("Adam's BUF -7.5 + Sam's Josh Allen
Over 245.5"). Each rule below gives a model message; keep the content and adjust the words freely.

## Rules

### B: Weather (outdoor games with known weather only)

**`wind`.** Danger when sustained ≥ 20 mph; else Warn when sustained ≥ 15 mph or gusts ≥ 25 mph. Flags every leg in that
game that is a `PASS_YDS`, `PASS_TDS`, `REC_YDS`, `RECEPTIONS`, `KICKER_PTS`, or `TOTAL` OVER. One warning per game.
> Heads up: 22 mph winds in Buffalo today. Passing and kicking overs take a real hit in that (field goals drop to about
> 77%).

**`fake_weather` (Info).** Precipitation > 0 and < 0.1 in/hr, wind < 10 mph, and a `TOTAL` UNDER in that game.
> FYI: light rain without wind doesn't actually lower NFL scoring.

### C: Key numbers

**`hook` (Warn).** A `SPREAD` leg at exactly −3.5, −7.5, +2.5, or +6.5.
> BUF -3.5 is on the hook. About 15% of games land on exactly 3, and that's a loss here. Buying to -3 or switching to
> the ML is worth a look.

For +2.5 / +6.5, say that landing on 3 or 7 loses by the hook and suggest buying to +3 / +7.

**`total_key_number` (Info).** A `TOTAL` leg whose line is within 0.5 of 37, 41, 43, 44, 47, or 51.
> 43.5 sits right between 43 and 44, two of the most common NFL final totals. The half point matters here.

### D: Parlay math and props

**`chalk` (Warn).** Any leg with odds ≤ −230. Multiplier = `1 + 100/|odds|`; bust chance = `1 − |odds|/(|odds| + 100)`.
> This -300 leg only bumps the payout 1.33× but still busts the slip about 1 in 4 times.

**`median_trap` (Warn, `props-research`).** A yardage OVER (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`) with a matched player who
has ≥ 3 games, where line > median of the last 5 games. When line < season mean, add the mean to the message.
> Shakir's last-5 median is 41 yards, so 52.5 is a stretch. His 58-yard average is propped up by one big game.

## Tests

Table-driven, one table per rule. Every threshold is tested on both sides:

| Rule                  | Cases that must trigger         | Cases that must not                   |
| --------------------- | ------------------------------- | ------------------------------------- |
| `blowout_passing`     | −7.0, −10.5                     | −6.5; opposing team's WR; other game  |
| `trailing_rb`         | +4.5, +6.5, ML +150             | +4.0; favorite; other team's RB       |
| `total_contradiction` | under + 2 overs/TDs             | under + 1; legs in another game       |
| `wind`                | 15.0 Warn, 19.9 Warn, gust 25 Warn, 20.0 Danger | 14.9 with gust 24.9; indoor; nil weather |
| `fake_weather`        | 0.05 in + 9 mph + under         | 0.1 in; 10 mph; over                  |
| `hook`                | −3.5, −7.5, +2.5, +6.5          | −3, −4, +3.5, −6.5                    |
| `total_key_number`    | 43, 43.5, 36.5, 51.5            | 45.5, 49                              |
| `chalk`               | −230, −500                      | −229, +150                            |
| `median_trap`         | line > L5 median                | line ≤ median; < 3 games; unmatched   |

Plus: `OTHER` legs never warn; output order is stable; an empty slip returns no warnings.
