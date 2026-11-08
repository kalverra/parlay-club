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

### D: Parlay math and props

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
| `median_trap`         | line > L5 median                | line ≤ median; < 3 games; unmatched   |

Plus: `OTHER` legs never warn; output order is stable; an empty slip returns no warnings.
