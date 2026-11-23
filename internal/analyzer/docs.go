package analyzer

import (
	"fmt"
	"strconv"
)

// RuleDoc describes one analyzer warning rule for the in-app grading guide.
type RuleDoc struct {
	ID        string
	Title     string
	Severity  Severity
	Category  string
	Threshold string
	Rationale string
	Example   string
}

// GradeBand describes one letter grade score range.
type GradeBand struct {
	Letter      string
	Range       string
	MinScore    int
	Description string
}

// FactorDoc describes one scoring factor in the leg or slip grading rubric.
type FactorDoc struct {
	Rule        string
	Category    string
	Label       string
	Delta       string
	Description string
}

// RubricDoc describes the full A-F leg and slip grading rubric.
type RubricDoc struct {
	BaselineScore int
	Bands         []GradeBand
	LegFactors    []FactorDoc
	SlipFactors   []FactorDoc
}

// RuleDocs returns structured documentation for every active analyzer rule,
// keeping the in-app guide locked to the engine's live thresholds.
func RuleDocs() []RuleDoc {
	return []RuleDoc{
		{
			ID:        ruleBlowoutPassing,
			Title:     "Blowout script vs. passing volume",
			Severity:  Warn,
			Category:  "Script & Correlation",
			Threshold: "Team spread <= -7.0 paired with that team's PASS_YDS or REC_YDS Over",
			Rationale: "Big favorites that cover comfortably tend to run the ball and bleed clock in the fourth quarter, capping late passing volume.",
			Example:   "If BUF -7.5 covers, they'll likely run the clock in Q4. Josh Allen Over 265.5 Pass Yds pairs better with a closer spread, or try MIA's WR for garbage time.",
		},
		{
			ID:        ruleTrailingRB,
			Title:     "Trailing underdog vs. rushing volume",
			Severity:  Danger,
			Category:  "Script & Correlation",
			Threshold: "Underdog spread >= +4.5 or underdog ML paired with that team's RUSH_YDS or RUSH_ATT Over",
			Rationale: "Teams playing from behind abandon the run to catch up, starving rushing overs of carries.",
			Example:   "NYJ +6.5 means they're probably trailing, and trailing teams stop running. Breece Hall Over 64.5 Rush Yds fights that script.",
		},
		{
			ID:        ruleTotalContradiction,
			Title:     "Game total Under vs. point-scoring overs",
			Severity:  Danger,
			Category:  "Script & Correlation",
			Threshold: "TOTAL Under paired with 2+ legs in the same game needing points (passing/receiving Overs or Anytime TD)",
			Rationale: "A game total Under needs a quiet scoreboard, while multiple passing, receiving, or TD legs need drives and touchdowns.",
			Example:   "Under 44.5 needs a quiet game, but Josh Allen Over 265.5 Pass Yds and Stefon Diggs Anytime TD need points. One of these sides is going to lose.",
		},
		{
			ID:        ruleStackClockDrain,
			Title:     "Clock-drain positive stack",
			Severity:  Info,
			Category:  "Script & Correlation",
			Threshold: "Favorite spread/ML + same-team RUSH_YDS/RUSH_ATT Over + opponent TEAM_TOTAL Under",
			Rationale: "All three legs cash from the same game script: the favorite gets ahead early, leans on the ground game, and keeps the opponent off the field.",
			Example:   "Nice stack: SF -6.5 covers, Christian McCaffrey grinds the clock, ARI Team Total Under 17.5 stays quiet. These all win together.",
		},
		{
			ID:        ruleStackShootout,
			Title:     "Shootout positive stack",
			Severity:  Info,
			Category:  "Script & Correlation",
			Threshold: "PASS_YDS Over + same-team receiver REC_YDS/RECEPTIONS Over + game TOTAL Over",
			Rationale: "When a game turns into a back-and-forth track meet, the QB, his receiver, and the game total all rise together.",
			Example:   "Shootout stack: Joe Burrow Over 275.5 Pass Yds, Ja'Marr Chase Over 84.5 Rec Yds, game Over 49.5. If it's a track meet, all three hit.",
		},
		{
			ID:        ruleWind,
			Title:     "Outdoor wind headwind",
			Severity:  Warn,
			Category:  "Environment",
			Threshold: "Outdoor game with sustained wind >= 15 mph (Warn) or >= 20 mph (Danger), or gusts >= 25 mph (Warn), on passing/receiving/kicking/TOTAL Overs",
			Rationale: "High wind suppresses deep passing accuracy and drops field-goal make rates toward 77%.",
			Example:   "Heads up: 22 mph winds in BUF today. Passing and kicking overs take a real hit in that, so Josh Allen Over 265.5 Pass Yds gets tougher. Field goals drop to about 77%.",
		},
		{
			ID:        ruleFakeWeather,
			Title:     "Light rain scoring myth",
			Severity:  Info,
			Category:  "Environment",
			Threshold: "Outdoor game with light rain (0 < precip < 0.1 in) and wind < 10 mph on a TOTAL Under",
			Rationale: "Light rain without wind does not meaningfully lower NFL scoring, so it gives an Under no extra edge.",
			Example:   "FYI: light rain without wind doesn't actually lower NFL scoring. Under 43.5 gets no extra help here.",
		},
		{
			ID:        ruleHook,
			Title:     "Spread sitting on the hook",
			Severity:  Warn,
			Category:  "Line & Price",
			Threshold: "Spread on -3.5, +2.5, -7.5, or +6.5",
			Rationale: "NFL margins land on 3 (~15% of games) and 7 (~9% of games) far more often than any other numbers, making the half-point hook costly.",
			Example:   "BUF -3.5 is on the hook. About 15% of games land on exactly 3, and that's a loss here. Buying to -3 or switching to the ML is worth a look.",
		},
		{
			ID:        ruleTotalKeyNumber,
			Title:     "Game total near a key number",
			Severity:  Info,
			Category:  "Line & Price",
			Threshold: "TOTAL within 0.5 points of 37, 41, 43, 44, 47, or 51",
			Rationale: "Those six numbers are the most frequent combined final scores in NFL games, so half a point around them swings push/win rates.",
			Example:   "44 is one of the most common NFL final totals. A half point either way decides it.",
		},
		{
			ID:        ruleChalk,
			Title:     "Heavy chalk price",
			Severity:  Warn,
			Category:  "Line & Price",
			Threshold: "Leg odds <= -230",
			Rationale: "Heavy favorites add minimal payout multiplier to a parlay while still carrying real upset risk.",
			Example:   "This -300 leg only bumps the payout 1.33× but still busts the slip about 25% of the time.",
		},
		{
			ID:        ruleQBDrag,
			Title:     "nfelo QB downgrade",
			Severity:  Info,
			Category:  "Line & Price",
			Threshold: "nfelo QB adjustment <= -30 Elo (~1.2+ spread points) on a favorite spread/ML or that team's passing/receiving/team-total Over",
			Rationale: "When a backup or downgraded starter drops a team's QB rating by more than a point, both covering as a favorite and clearing passing overs get harder.",
			Example:   "nfelo marks down MIA's QB by about 1.4 points. That drags on MIA -3.5 and Tua Tagovailoa Over 245.5 Pass Yds.",
		},
		{
			ID:        ruleMedianTrap,
			Title:     "Last-5 median trap",
			Severity:  Warn,
			Category:  "Player Form & Matchup",
			Threshold: "Yardage Over (PASS_YDS, RUSH_YDS, REC_YDS) where line > player's last-5 game median (requires >= 3 games)",
			Rationale: "One spike game can inflate a season average above the line even when a player's typical weekly output sits below it.",
			Example:   "Khalil Shakir's last-5 median is 52 yards, so 58.5 is a stretch. His 64-yard average is propped up by one big game.",
		},
		{
			ID:        ruleToughMatchup,
			Title:     "Top-5 opposing defense (EPA allowed)",
			Severity:  Warn,
			Category:  "Player Form & Matchup",
			Threshold: "Yardage or TD Over facing an opposing defense ranked #1–#5 in matching DefPassEPAAllowed or DefRushEPAAllowed",
			Rationale: "Elite defenses in EPA allowed per play consistently compress opponent passing or rushing efficiency.",
			Example:   "BAL ranks #2 against the rush by EPA allowed, so James Cook Over 68.5 Rush Yds is a tough matchup.",
		},
		{
			ID:        ruleShrinkingRole,
			Title:     "Shrinking snap or target role",
			Severity:  Warn,
			Category:  "Player Form & Matchup",
			Threshold: "Player prop Over where last-2-game snap share dropped >= 15 percentage points or target share dropped >= 8 percentage points vs. season average",
			Rationale: "Season-long numbers lag behind rotation changes; a sharp two-week drop in snaps or targets signals fading volume.",
			Example:   "Dalton Kincaid's snap share dropped from 78% on the season to 55% over the last 2 games — recent volume is fading for Dalton Kincaid Over 45.5 Rec Yds.",
		},
		{
			ID:        rulePaceClash,
			Title:     "Pace & pass-rate clash on game total",
			Severity:  Warn,
			Category:  "Player Form & Matchup",
			Threshold: "TOTAL Over when both teams rank bottom-8 in PlaysPerGame (#25–#32) and bottom-10 in PassRate (#23–#32), or TOTAL Under when both rank top-8 (#1–#8) in PlaysPerGame and OffPassEPA",
			Rationale: "Two slow, run-heavy teams shorten the game for an Over, while two fast, pass-efficient offenses create extra drives and explosive plays against an Under.",
			Example:   "Both PIT and CLE sit bottom-8 in pace and bottom-10 in pass rate, so Over 42.5 needs a lot of efficiency.",
		},
		{
			ID:        ruleTDSpikeRisk,
			Title:     "TD spike outpacing opportunity",
			Severity:  Info,
			Category:  "Player Form & Matchup",
			Threshold: "ANYTIME_TD Yes with >= 3 TDs in last 3 games on < 15% target share and < 8 carries/game, or PASS_TDS Over with >= 6 TDs in last 3 games on <= 0 avg TotalEPA",
			Rationale: "Touchdowns are the noisiest box-score stat; high TD output on low share or negative EPA tends to regress toward opportunity.",
			Example:   "Dawson Knox has 3 TDs in the last 3 games on a 11% target share and 0 carries/game — TD luck can cool off.",
		},
	}
}

// GradeRubric returns the A-F scoring bands and factor weights used by
// ScoreLeg and ScoreSlip, reading directly from ActiveRubric.
func GradeRubric() RubricDoc {
	return GradeRubricFor(ActiveRubric)
}

// GradeRubricFor returns the A-F scoring bands and factor weights for r
// without reading or mutating ActiveRubric.
func GradeRubricFor(r Rubric) RubricDoc {
	return RubricDoc{
		BaselineScore: r.LegBaseline,
		Bands: []GradeBand{
			{
				Letter:      "A",
				Range:       fmt.Sprintf("%d–100", r.BandA),
				MinScore:    r.BandA,
				Description: "Strong form, matchup, and price with clean script alignment.",
			},
			{
				Letter:      "B",
				Range:       fmt.Sprintf("%d–%d", r.BandB, r.BandA-1),
				MinScore:    r.BandB,
				Description: "Solid leg or slip with minor or no red flags.",
			},
			{
				Letter:      "C",
				Range:       fmt.Sprintf("%d–%d", r.BandC, r.BandB-1),
				MinScore:    r.BandC,
				Description: "Mixed signals: playable, but fighting a hook, chalk, or tough spot.",
			},
			{
				Letter:      "D",
				Range:       fmt.Sprintf("%d–%d", r.BandD, r.BandC-1),
				MinScore:    r.BandD,
				Description: "Multiple headwinds or script friction dragging down the ticket.",
			},
			{
				Letter:      "F",
				Range:       fmt.Sprintf("0–%d", r.BandD-1),
				MinScore:    0,
				Description: "Severe script conflict, heavy chalk, or stacked environmental headwinds.",
			},
		},
		LegFactors: []FactorDoc{
			{
				Rule:        ruleHitRate,
				Category:    "Form & Usage",
				Label:       "Strong L5 hit rate",
				Delta:       formatSignedDelta(r.HitRate),
				Description: "Cleared the line in at least 4 of the last 5 games.",
			},
			{
				Rule:        ruleRisingUsage,
				Category:    "Form & Usage",
				Label:       "Rising usage",
				Delta:       formatSignedDelta(r.RisingUsage),
				Description: "Last-3-game snap share (+8 pp) or target share (+4 pp) trending above season average.",
			},
			{
				Rule:     ruleMedianTrap,
				Category: "Form & Usage",
				Label:    "L5 median trap",
				Delta: fmt.Sprintf(
					"%s (%s w/ teammate Out)",
					formatSignedDelta(r.MedianTrap),
					formatSignedDelta(r.MedianTrapSoftened),
				),
				Description: fmt.Sprintf(
					"Yardage Over line sits above the player's last-5 game median; softened to %s when a same-position teammate is Out.",
					formatSignedDelta(r.MedianTrapSoftened),
				),
			},
			{
				Rule:        ruleTeammateOutBump,
				Category:    "Form & Usage",
				Label:       "Teammate out (role bump)",
				Delta:       formatSignedDelta(r.TeammateOutBump),
				Description: "Same-team, same-position (WR, TE, RB) teammate is Out on an Over or Anytime TD prop.",
			},
			{
				Rule:        ruleShrinkingRole,
				Category:    "Form & Usage",
				Label:       "Shrinking role",
				Delta:       formatSignedDelta(r.ShrinkingRole),
				Description: "Last-2-game snap share (-15 pp) or target share (-8 pp) dropped below season average.",
			},
			{
				Rule:        ruleFavorableMatchup,
				Category:    "Matchup & Environment",
				Label:       "Weak opposing defense",
				Delta:       formatSignedDelta(r.FavorableMatchup),
				Description: "Opposing defense ranks #25–#32 in matching pass or rush EPA allowed.",
			},
			{
				Rule:        ruleToughMatchup,
				Category:    "Matchup & Environment",
				Label:       "Tough opposing defense",
				Delta:       formatSignedDelta(r.ToughMatchup),
				Description: "Opposing defense ranks #1–#5 in matching pass or rush EPA allowed.",
			},
			{
				Rule:     ruleWind,
				Category: "Matchup & Environment",
				Label:    "Wind headwind",
				Delta:    fmt.Sprintf("%s / %s", formatSignedDelta(r.WindWarn), formatSignedDelta(r.WindDanger)),
				Description: fmt.Sprintf(
					"%s for Warn wind/gusts; %s for Danger wind (>= 20 mph) on passing/kicking/total Overs.",
					formatSignedDelta(r.WindWarn),
					formatSignedDelta(r.WindDanger),
				),
			},
			{
				Rule:        ruleQBDrag,
				Category:    "Matchup & Environment",
				Label:       "nfelo QB drag",
				Delta:       formatSignedDelta(r.QBDrag),
				Description: "nfelo marks down the team's QB by <= -30 Elo on a favorite or passing Over.",
			},
			{
				Rule:        ruleFakeWeather,
				Category:    "Matchup & Environment",
				Label:       "Light rain myth",
				Delta:       formatSignedDelta(r.FakeWeather),
				Description: "Light rain without wind gives a game total Under no real scoring brake.",
			},
			{
				Rule:        ruleNfeloCushion,
				Category:    "Line & Price",
				Label:       "nfelo spread cushion",
				Delta:       formatSignedDelta(r.NfeloCushion),
				Description: "nfelo projects at least 2 points of cushion on a spread leg.",
			},
			{
				Rule:     ruleHook,
				Category: "Line & Price",
				Label:    "Spread on hook",
				Delta: fmt.Sprintf(
					"%s (%s w/ cushion)",
					formatSignedDelta(r.Hook),
					formatSignedDelta(r.HookSoftened),
				),
				Description: fmt.Sprintf(
					"Spread sits on -3.5, +2.5, -7.5, or +6.5; softened to %s with >= +2 nfelo cushion.",
					formatSignedDelta(r.HookSoftened),
				),
			},
			{
				Rule:        ruleTotalKeyNumber,
				Category:    "Line & Price",
				Label:       "Total on key number",
				Delta:       formatSignedDelta(r.TotalKeyNumber),
				Description: "Game total sits within 0.5 points of 37, 41, 43, 44, 47, or 51.",
			},
			{
				Rule:     ruleChalk,
				Category: "Line & Price",
				Label:    "Heavy chalk",
				Delta:    fmt.Sprintf("%s to %s", formatSignedDelta(r.ChalkMin), formatSignedDelta(r.ChalkMax)),
				Description: fmt.Sprintf(
					"Odds <= -230 scale from %s at -230 down to %s at -500 or worse.",
					formatSignedDelta(r.ChalkMin),
					formatSignedDelta(r.ChalkMax),
				),
			},
		},
		SlipFactors: []FactorDoc{
			{
				Rule:        ruleStackShootout,
				Category:    "Slip Correlation & Drag",
				Label:       "Shootout stack",
				Delta:       formatSignedDelta(r.StackBonus),
				Description: "Pass Over + same-team receiver Over + game Total Over in the same game.",
			},
			{
				Rule:        ruleStackClockDrain,
				Category:    "Slip Correlation & Drag",
				Label:       "Clock-drain stack",
				Delta:       formatSignedDelta(r.StackBonus),
				Description: "Favorite spread/ML + same-team Rush Over + opponent Team Total Under.",
			},
			{
				Rule:        ruleBlowoutPassing,
				Category:    "Slip Correlation & Drag",
				Label:       "Blowout script conflict",
				Delta:       formatSignedDelta(r.WarnConflict),
				Description: "Big favorite spread (<= -7.0) paired with same-team passing/receiving Over.",
			},
			{
				Rule:        ruleTrailingRB,
				Category:    "Slip Correlation & Drag",
				Label:       "Trailing RB danger",
				Delta:       formatSignedDelta(r.DangerConflict),
				Description: "Underdog spread (>= +4.5) or ML paired with same-team rushing Over.",
			},
			{
				Rule:        ruleTotalContradiction,
				Category:    "Slip Correlation & Drag",
				Label:       "Total contradiction",
				Delta:       formatSignedDelta(r.DangerConflict),
				Description: "Game Total Under paired with 2+ point-scoring Overs or Anytime TDs.",
			},
			{
				Rule:        ruleParlayDrag,
				Category:    "Slip Correlation & Drag",
				Label:       "Parlay length drag",
				Delta:       formatSignedDelta(r.ParlayDragPerLeg) + " / leg",
				Description: fmt.Sprintf("Applied for every leg beyond %d legs.", r.ParlayDragFreeLegs),
			},
			{
				Rule:        ruleChalkDrag,
				Category:    "Slip Correlation & Drag",
				Label:       "Extra chalk drag",
				Delta:       formatSignedDelta(r.ChalkDragPerExtraLeg) + " / leg",
				Description: "Applied for every heavy favorite leg (<= -230) beyond the first.",
			},
		},
	}
}

func formatSignedDelta(v int) string {
	if v > 0 {
		return fmt.Sprintf("+%d", v)
	}
	return strconv.Itoa(v)
}
