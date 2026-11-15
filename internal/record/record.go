// Package record computes the season Record Book for a group from its graded
// slips, legs, members, and logged wagers. It is pure and deterministic: no
// database, HTTP, or clock access.
package record

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/parlay"
)

// MinDecidedLegs is the minimum number of decided (WON + LOST) legs a member
// needs before their Hit% is ranked on the leaderboard.
const MinDecidedLegs = 6

// Slip is one graded slip in the season paired with its week coordinates.
type Slip struct {
	ID         int64
	Season     int
	SeasonType int
	WeekNum    int
	CreatedAt  time.Time
	Status     string
	Result     parlay.Result
}

// Leg is one pick on a slip.
type Leg struct {
	SlipID int64
	UserID int64
	Odds   int
	Result parlay.Result
}

// Member is one group member who may appear on the leaderboard.
type Member struct {
	ID          int64
	Name        string
	AvatarColor string
}

// Wager is one member's logged bet on a slip.
type Wager struct {
	SlipID   int64
	UserID   int64
	Stake    float64
	Odds     int
	BoostPct float64
}

// Book is the computed Record Book for one group and NFL season.
type Book struct {
	Season      int
	TotalSlips  int
	Wins        int
	Losses      int
	Pushes      int
	UnitsPL     float64
	LongestWin  int
	LongestLoss int
	Leaderboard []MemberRow
}

// MemberRow is one member's season summary on the Record Book leaderboard.
type MemberRow struct {
	UserID          int64
	Name            string
	AvatarColor     string
	TotalLegs       int
	WonLegs         int
	LostLegs        int
	PushLegs        int
	VoidLegs        int
	DecidedLegs     int
	HitPct          *float64
	AvgLegOdds      int
	TerroristCount  int
	ParlayTerrorist bool
	WagerCount      int
	DollarPL        *float64
}

// Build computes the Record Book for season from a group's graded slips,
// legs, members, and wagers. Non-graded slips (and legs or wagers on them) are
// ignored.
func Build(season int, slips []Slip, legs []Leg, members []Member, wagers []Wager) Book {
	book := Book{
		Season:      season,
		Leaderboard: make([]MemberRow, 0),
	}

	graded := filterAndSortSlips(slips)
	if len(graded) == 0 {
		return book
	}

	gradedByID := make(map[int64]Slip, len(graded))
	legsBySlip := make(map[int64][]Leg, len(graded))
	for _, s := range graded {
		gradedByID[s.ID] = s
	}
	for _, leg := range legs {
		if _, ok := gradedByID[leg.SlipID]; !ok {
			continue
		}
		legsBySlip[leg.SlipID] = append(legsBySlip[leg.SlipID], leg)
	}

	memberByID := make(map[int64]Member, len(members))
	for _, m := range members {
		memberByID[m.ID] = m
	}

	stats := make(map[int64]*memberStats)
	ensureMember := func(userID int64) *memberStats {
		if st, ok := stats[userID]; ok {
			return st
		}
		m, ok := memberByID[userID]
		if !ok {
			m = Member{
				ID:   userID,
				Name: fmt.Sprintf("Member %d", userID),
			}
		}
		st := &memberStats{member: m}
		stats[userID] = st
		return st
	}

	var curWin, curLoss int
	for _, s := range graded {
		book.TotalSlips++
		switch s.Result {
		case parlay.Won:
			book.Wins++
			curWin++
			curLoss = 0
			if curWin > book.LongestWin {
				book.LongestWin = curWin
			}
		case parlay.Lost:
			book.Losses++
			curLoss++
			curWin = 0
			if curLoss > book.LongestLoss {
				book.LongestLoss = curLoss
			}
		case parlay.Push:
			book.Pushes++
			curWin = 0
			curLoss = 0
		}

		slipLegs := legsBySlip[s.ID]
		parlayLegs := make([]parlay.Leg, 0, len(slipLegs))
		var lostLegUsers []int64
		for _, leg := range slipLegs {
			parlayLegs = append(parlayLegs, parlay.Leg{
				Decimal: parlay.AmericanToDecimal(leg.Odds),
				Result:  leg.Result,
			})

			st := ensureMember(leg.UserID)
			st.totalLegs++
			switch leg.Result {
			case parlay.Won:
				st.wonLegs++
				st.addOdds(leg.Odds)
			case parlay.Lost:
				st.lostLegs++
				st.addOdds(leg.Odds)
				lostLegUsers = append(lostLegUsers, leg.UserID)
			case parlay.Push:
				st.pushLegs++
				st.addOdds(leg.Odds)
			case parlay.Void:
				st.voidLegs++
			}
		}

		book.UnitsPL += parlay.Units(s.Result, parlay.CombinedOdds(parlayLegs))

		if s.Result == parlay.Lost && len(lostLegUsers) == 1 {
			ensureMember(lostLegUsers[0]).terroristCount++
		}
	}

	applyWagers(wagers, gradedByID, stats)

	maxTerrorist := 0
	for _, st := range stats {
		if st.terroristCount > maxTerrorist {
			maxTerrorist = st.terroristCount
		}
	}

	rows := make([]MemberRow, 0, len(stats))
	for _, st := range stats {
		decided := st.wonLegs + st.lostLegs
		var hitPct *float64
		if decided >= MinDecidedLegs {
			pct := float64(st.wonLegs) / float64(decided) * 100
			hitPct = &pct
		}

		var avgOdds int
		if st.oddsCount > 0 {
			avgOdds = parlay.DecimalToAmerican(st.oddsDecimalSum / float64(st.oddsCount))
		}

		var dollarPL *float64
		if st.wagerCount > 0 {
			val := st.dollarPL
			dollarPL = &val
		}

		rows = append(rows, MemberRow{
			UserID:          st.member.ID,
			Name:            st.member.Name,
			AvatarColor:     st.member.AvatarColor,
			TotalLegs:       st.totalLegs,
			WonLegs:         st.wonLegs,
			LostLegs:        st.lostLegs,
			PushLegs:        st.pushLegs,
			VoidLegs:        st.voidLegs,
			DecidedLegs:     decided,
			HitPct:          hitPct,
			AvgLegOdds:      avgOdds,
			TerroristCount:  st.terroristCount,
			ParlayTerrorist: maxTerrorist > 0 && st.terroristCount == maxTerrorist,
			WagerCount:      st.wagerCount,
			DollarPL:        dollarPL,
		})
	}

	sortLeaderboard(rows)
	book.Leaderboard = rows
	return book
}

type memberStats struct {
	member         Member
	totalLegs      int
	wonLegs        int
	lostLegs       int
	pushLegs       int
	voidLegs       int
	oddsDecimalSum float64
	oddsCount      int
	terroristCount int
	wagerCount     int
	dollarPL       float64
}

func (st *memberStats) addOdds(american int) {
	dec := parlay.AmericanToDecimal(american)
	if dec <= 1 {
		return
	}
	st.oddsDecimalSum += dec
	st.oddsCount++
}

func filterAndSortSlips(slips []Slip) []Slip {
	out := make([]Slip, 0, len(slips))
	for _, s := range slips {
		if s.Status != "" && s.Status != "GRADED" {
			continue
		}
		if s.Result != parlay.Won && s.Result != parlay.Lost && s.Result != parlay.Push {
			continue
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Slip) int {
		if c := cmp.Compare(a.Season, b.Season); c != 0 {
			return c
		}
		if c := cmp.Compare(a.SeasonType, b.SeasonType); c != 0 {
			return c
		}
		if c := cmp.Compare(a.WeekNum, b.WeekNum); c != 0 {
			return c
		}
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

func sortLeaderboard(rows []MemberRow) {
	slices.SortFunc(rows, func(a, b MemberRow) int {
		aQual := a.HitPct != nil
		bQual := b.HitPct != nil
		if aQual != bQual {
			if aQual {
				return -1
			}
			return 1
		}
		if aQual && bQual {
			if c := cmp.Compare(*b.HitPct, *a.HitPct); c != 0 {
				return c
			}
		}
		if c := cmp.Compare(b.WonLegs, a.WonLegs); c != 0 {
			return c
		}
		if c := cmp.Compare(b.DecidedLegs, a.DecidedLegs); c != 0 {
			return c
		}
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return cmp.Compare(a.UserID, b.UserID)
	})
}

func applyWagers(wagers []Wager, gradedByID map[int64]Slip, stats map[int64]*memberStats) {
	for _, w := range wagers {
		s, ok := gradedByID[w.SlipID]
		if !ok {
			continue
		}
		st, hasLeg := stats[w.UserID]
		if !hasLeg {
			continue
		}
		st.wagerCount++
		switch s.Result {
		case parlay.Won:
			dec := parlay.AmericanToDecimal(w.Odds)
			st.dollarPL += parlay.Payout(w.Stake, dec, w.BoostPct) - w.Stake
		case parlay.Lost:
			st.dollarPL -= w.Stake
		case parlay.Push:
			// Push returns stake; net dollar P/L is 0.
		}
	}
}
