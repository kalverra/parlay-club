package record

import "fmt"

// TagTone distinguishes praise badges from insult badges.
type TagTone string

const (
	// TagTonePraise marks tags earned for standout positive performance.
	TagTonePraise TagTone = "praise"
	// TagToneInsult marks tags earned for standout negative performance.
	TagToneInsult TagTone = "insult"
)

// Tag is a deterministic Record Book badge earned by a member for a season.
type Tag struct {
	Key         string
	Label       string
	Tone        TagTone
	Rule        string
	Explanation string
}

type tagDefinition struct {
	key   string
	label string
	tone  TagTone
	rule  string
	match func(row MemberRow, st *memberStats, meanImpliedPct float64) (string, bool)
}

var tagCatalog = []tagDefinition{
	// Praise tags (in display order).
	{
		key:   "natural",
		label: "Natural",
		tone:  TagTonePraise,
		rule:  "Win at least 60% of 3+ decided D/F-graded picks (min 3 wins).",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.dfDecided < 3 || st.dfWon < 3 {
				return "", false
			}
			rate := winRatePct(st.dfWon, st.dfDecided)
			if rate < 60.0 {
				return "", false
			}
			return fmt.Sprintf("Won %d of %d D/F-graded picks (%.1f%%).", st.dfWon, st.dfDecided, rate), true
		},
	},
	{
		key:   "ball_knower",
		label: "Ball Knower",
		tone:  TagTonePraise,
		rule:  "Beat sportsbook implied odds by at least +12.0 percentage points across 6+ decided picks.",
		match: func(row MemberRow, st *memberStats, meanImpliedPct float64) (string, bool) {
			if row.ImpliedLift == nil || row.HitPct == nil ||
				st.impliedCount < MinDecidedLegs || *row.ImpliedLift < 12.0 {
				return "", false
			}
			return fmt.Sprintf(
				"Hit %.1f%% vs %.1f%% implied odds (%+.1f pts across %d legs).",
				*row.HitPct, meanImpliedPct, *row.ImpliedLift, st.impliedCount,
			), true
		},
	},
	{
		key:   "film_room_god",
		label: "Film Room God",
		tone:  TagTonePraise,
		rule:  "Win at least 75% of 4+ decided A/B-graded picks.",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.abDecided < 4 {
				return "", false
			}
			rate := winRatePct(st.abWon, st.abDecided)
			if rate < 75.0 {
				return "", false
			}
			return fmt.Sprintf("Won %d of %d A/B-graded picks (%.1f%%).", st.abWon, st.abDecided, rate), true
		},
	},
	{
		key:   "sniper",
		label: "Sniper",
		tone:  TagTonePraise,
		rule:  "Win at least 50% of 3+ decided +120 or longer underdogs (min 2 wins).",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.dogDecided < 3 || st.dogWon < 2 {
				return "", false
			}
			rate := winRatePct(st.dogWon, st.dogDecided)
			if rate < 50.0 {
				return "", false
			}
			return fmt.Sprintf("Won %d of %d +120 or longer picks (%.1f%%).", st.dogWon, st.dogDecided, rate), true
		},
	},
	{
		key:   "the_backpack",
		label: "The Backpack",
		tone:  TagTonePraise,
		rule:  "Hit at least 65% across 8+ decided picks with zero solo-loss parlay kills.",
		match: func(row MemberRow, _ *memberStats, _ float64) (string, bool) {
			if row.DecidedLegs < 8 || row.HitPct == nil || *row.HitPct < 65.0 || row.TerroristCount != 0 {
				return "", false
			}
			return fmt.Sprintf(
				"Hit %.1f%% (%d of %d decided picks) with 0 solo parlay kills.",
				*row.HitPct, row.WonLegs, row.DecidedLegs,
			), true
		},
	},
	{
		key:   "heater",
		label: "Heater",
		tone:  TagTonePraise,
		rule:  "Active streak of 5+ consecutive decided leg wins.",
		match: func(row MemberRow, _ *memberStats, _ float64) (string, bool) {
			if row.CurrentWinStreak < 5 {
				return "", false
			}
			return fmt.Sprintf("On an active %d-leg win streak.", row.CurrentWinStreak), true
		},
	},
	{
		key:   "told_you_so",
		label: "Told You So",
		tone:  TagTonePraise,
		rule:  "Win at least 60% of 3+ net-downvoted decided picks (min 2 wins).",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.downvotedDecided < 3 || st.downvotedWon < 2 {
				return "", false
			}
			rate := winRatePct(st.downvotedWon, st.downvotedDecided)
			if rate < 60.0 {
				return "", false
			}
			return fmt.Sprintf(
				"Won %d of %d net-downvoted picks (%.1f%%).",
				st.downvotedWon, st.downvotedDecided, rate,
			), true
		},
	},

	// Insult tags (in display order, Parlay Terrorist first).
	{
		key:   "parlay_terrorist",
		label: "Parlay Terrorist",
		tone:  TagToneInsult,
		rule:  "Tied for the most solo losing legs on lost parlays (min 1).",
		match: func(row MemberRow, _ *memberStats, _ float64) (string, bool) {
			if !row.ParlayTerrorist || row.TerroristCount <= 0 {
				return "", false
			}
			noun := "slips"
			if row.TerroristCount == 1 {
				noun = "slip"
			}
			return fmt.Sprintf("Sole losing leg on %d lost %s.", row.TerroristCount, noun), true
		},
	},
	{
		key:   "certified_donkey",
		label: "Certified Donkey",
		tone:  TagToneInsult,
		rule:  "Trail sportsbook implied odds by at least 12.0 percentage points across 6+ decided picks.",
		match: func(row MemberRow, st *memberStats, meanImpliedPct float64) (string, bool) {
			if row.ImpliedLift == nil || row.HitPct == nil ||
				st.impliedCount < MinDecidedLegs || *row.ImpliedLift > -12.0 {
				return "", false
			}
			return fmt.Sprintf(
				"Hit %.1f%% vs %.1f%% implied odds (%+.1f pts across %d legs).",
				*row.HitPct, meanImpliedPct, *row.ImpliedLift, st.impliedCount,
			), true
		},
	},
	{
		key:   "glue_eater",
		label: "Glue Eater",
		tone:  TagToneInsult,
		rule:  "Win 35% or less of 4+ decided A/B-graded picks.",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.abDecided < 4 {
				return "", false
			}
			rate := winRatePct(st.abWon, st.abDecided)
			if rate > 35.0 {
				return "", false
			}
			return fmt.Sprintf("Won %d of %d A/B-graded picks (%.1f%%).", st.abWon, st.abDecided, rate), true
		},
	},
	{
		key:   "vegas_favorite_idiot",
		label: "Vegas's Favorite Idiot",
		tone:  TagToneInsult,
		rule:  "Win 20% or less of 4+ decided +120 or longer underdogs.",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.dogDecided < 4 {
				return "", false
			}
			rate := winRatePct(st.dogWon, st.dogDecided)
			if rate > 20.0 {
				return "", false
			}
			return fmt.Sprintf("Won %d of %d +120 or longer picks (%.1f%%).", st.dogWon, st.dogDecided, rate), true
		},
	},
	{
		key:   "scared_money",
		label: "Scared Money",
		tone:  TagToneInsult,
		rule:  "Average leg odds of -220 or heavier chalk across 6+ decided picks.",
		match: func(row MemberRow, st *memberStats, _ float64) (string, bool) {
			if row.DecidedLegs < 6 || st.oddsCount == 0 || row.AvgLegOdds > -220 {
				return "", false
			}
			return fmt.Sprintf("Averaged %d leg odds across %d decided picks.", row.AvgLegOdds, row.DecidedLegs), true
		},
	},
	{
		key:   "stinker",
		label: "Stinker",
		tone:  TagToneInsult,
		rule:  "Active streak of 5+ consecutive decided leg losses.",
		match: func(row MemberRow, _ *memberStats, _ float64) (string, bool) {
			if row.CurrentLossStreak < 5 {
				return "", false
			}
			return fmt.Sprintf("On an active %d-leg losing streak.", row.CurrentLossStreak), true
		},
	},
	{
		key:   "fraud",
		label: "Fraud",
		tone:  TagToneInsult,
		rule:  "Win 35% or less of 3+ net-upvoted decided picks.",
		match: func(_ MemberRow, st *memberStats, _ float64) (string, bool) {
			if st.upvotedDecided < 3 {
				return "", false
			}
			rate := winRatePct(st.upvotedWon, st.upvotedDecided)
			if rate > 35.0 {
				return "", false
			}
			return fmt.Sprintf("Won %d of %d net-upvoted picks (%.1f%%).", st.upvotedWon, st.upvotedDecided, rate), true
		},
	},
}

func evaluateTags(row MemberRow, st *memberStats, meanImpliedPct float64) []Tag {
	var tags []Tag
	for _, def := range tagCatalog {
		explanation, ok := def.match(row, st, meanImpliedPct)
		if !ok {
			continue
		}
		tags = append(tags, Tag{
			Key:         def.key,
			Label:       def.label,
			Tone:        def.tone,
			Rule:        def.rule,
			Explanation: explanation,
		})
	}
	return tags
}

func winRatePct(won, decided int) float64 {
	if decided <= 0 {
		return 0
	}
	return float64(won) * 100 / float64(decided)
}
