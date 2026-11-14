package web

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/record"
)

// RecordPage is the season Record Book screen.
type RecordPage struct {
	Page
	GroupName     string
	Season        int
	Seasons       []SeasonOption
	HasGroup      bool
	HasSlips      bool
	Record        string
	Wins          int
	Losses        int
	Pushes        int
	TotalSlips    int
	UnitsPL       string
	UnitsPositive bool
	UnitsNegative bool
	LongestWin    int
	LongestLoss   int
	Leaderboard   []RecordMemberView
}

// SeasonOption is one season in the Record Book season selector.
type SeasonOption struct {
	Year    int
	Current bool
}

// RecordMemberView is one member's row on the Record Book leaderboard.
type RecordMemberView struct {
	Rank            int
	Name            string
	Avatar          User
	LegsRecord      string
	TotalLegs       int
	DecidedLegs     int
	HitPct          string
	Qualified       bool
	AvgLegOdds      string
	TerroristCount  int
	ParlayTerrorist bool
	WagerCount      int
	DollarPL        string
	DollarPositive  bool
	DollarNegative  bool
}

// recordBook renders the active group's season Record Book at GET /record?season=.
func (h *slipHandlers) recordBook(w http.ResponseWriter, r *http.Request) {
	page := h.pageWithTab(r, "Record Book", TabRecord)
	if h.deps.Store == nil {
		h.renderPage(w, "record", RecordPage{Page: page})
		return
	}

	group, ok := h.group(r.Context())
	if !ok {
		h.renderPage(w, "record", RecordPage{Page: page})
		return
	}

	ctx := r.Context()
	allSeasons, err := h.deps.Store.SeasonsForGroup(ctx, group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	gradedSeasons, err := h.deps.Store.GradedSeasonsForGroup(ctx, group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	selectedSeason, ok := h.resolveRecordSeason(w, r, allSeasons, gradedSeasons)
	if !ok {
		return
	}

	slips, err := h.deps.Store.GradedSlipsForGroupSeason(ctx, group.ID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return
	}
	legs, err := h.deps.Store.GradedLegsForGroupSeason(ctx, group.ID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return
	}
	users, err := h.deps.Store.RecordUsersForGroupSeason(ctx, group.ID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return
	}
	wagers, err := h.deps.Store.WagersForGroupSeason(ctx, group.ID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return
	}

	book := buildRecordBook(selectedSeason, slips, legs, users, wagers)
	data := buildRecordPage(page, group, selectedSeason, allSeasons, users, book)
	h.renderPage(w, "record", data)
}

// resolveRecordSeason picks the season to display from ?season=, falling back
// to the latest season with GRADED slips, any season with slips, the latest
// loaded week, or the current year.
func (h *slipHandlers) resolveRecordSeason(
	w http.ResponseWriter,
	r *http.Request,
	allSeasons []int,
	gradedSeasons []int,
) (int, bool) {
	if raw := strings.TrimSpace(r.URL.Query().Get("season")); raw != "" {
		season, err := strconv.Atoi(raw)
		if err != nil || season <= 0 {
			http.Error(w, "invalid season", http.StatusBadRequest)
			return 0, false
		}
		return season, true
	}

	if len(gradedSeasons) > 0 {
		return gradedSeasons[0], true
	}
	if len(allSeasons) > 0 {
		return allSeasons[0], true
	}

	week, hasWeek, ok := h.latestWeek(w, r)
	if !ok {
		return 0, false
	}
	if hasWeek {
		return week.Season, true
	}
	return h.deps.now().Year(), true
}

func buildRecordBook(
	season int,
	slips []db.GradedSlip,
	legs []db.Leg,
	users []db.User,
	wagers []db.Wager,
) record.Book {
	recSlips := make([]record.Slip, 0, len(slips))
	for _, gs := range slips {
		recSlips = append(recSlips, record.Slip{
			ID:         gs.Slip.ID,
			Season:     gs.Week.Season,
			SeasonType: gs.Week.SeasonType,
			WeekNum:    gs.Week.WeekNum,
			CreatedAt:  gs.Slip.CreatedAt,
			Status:     string(gs.Slip.Status),
			Result:     gs.Slip.Result,
		})
	}

	recLegs := make([]record.Leg, 0, len(legs))
	for _, leg := range legs {
		recLegs = append(recLegs, record.Leg{
			SlipID: leg.SlipID,
			UserID: leg.UserID,
			Odds:   leg.Odds,
			Result: leg.Result,
		})
	}

	recMembers := make([]record.Member, 0, len(users))
	for _, u := range users {
		recMembers = append(recMembers, record.Member{
			ID:          u.ID,
			Name:        u.Name,
			AvatarColor: string(u.AvatarColor),
		})
	}

	recWagers := make([]record.Wager, 0, len(wagers))
	for _, wager := range wagers {
		recWagers = append(recWagers, record.Wager{
			SlipID:   wager.SlipID,
			UserID:   wager.UserID,
			Stake:    wager.Stake,
			Odds:     wager.Odds,
			BoostPct: wager.BoostPct,
		})
	}

	return record.Build(season, recSlips, recLegs, recMembers, recWagers)
}

func buildRecordPage(
	page Page,
	group db.Group,
	selectedSeason int,
	allSeasons []int,
	users []db.User,
	book record.Book,
) RecordPage {
	userByID := make(map[int64]db.User, len(users))
	for _, u := range users {
		userByID[u.ID] = u
	}

	seasonYears := slices.Clone(allSeasons)
	if len(seasonYears) > 0 && !slices.Contains(seasonYears, selectedSeason) {
		seasonYears = append(seasonYears, selectedSeason)
		slices.SortFunc(seasonYears, func(a, b int) int { return b - a })
	}
	seasons := make([]SeasonOption, 0, len(seasonYears))
	for _, year := range seasonYears {
		seasons = append(seasons, SeasonOption{
			Year:    year,
			Current: year == selectedSeason,
		})
	}

	leaderboard := make([]RecordMemberView, 0, len(book.Leaderboard))
	for i, row := range book.Leaderboard {
		u, ok := userByID[row.UserID]
		if !ok {
			u = db.User{ID: row.UserID, Name: row.Name, AvatarColor: db.AvatarColor(row.AvatarColor)}
		}
		legsRecord := fmt.Sprintf("%d-%d", row.WonLegs, row.LostLegs)
		if row.PushLegs > 0 {
			legsRecord = fmt.Sprintf("%d-%d-%d", row.WonLegs, row.LostLegs, row.PushLegs)
		}

		hitPct := "–"
		qualified := false
		if row.HitPct != nil {
			hitPct = fmt.Sprintf("%.1f%%", *row.HitPct)
			qualified = true
		}

		avgOdds := "–"
		if row.AvgLegOdds != 0 {
			avgOdds = formatOdds(row.AvgLegOdds)
		}

		dollarPL := "–"
		var dollarPos, dollarNeg bool
		if row.DollarPL != nil {
			dollarPL = formatSignedMoney(*row.DollarPL)
			dollarPos = *row.DollarPL > 0.005
			dollarNeg = *row.DollarPL < -0.005
		}

		leaderboard = append(leaderboard, RecordMemberView{
			Rank:            i + 1,
			Name:            row.Name,
			Avatar:          NewUser(u),
			LegsRecord:      legsRecord,
			TotalLegs:       row.TotalLegs,
			DecidedLegs:     row.DecidedLegs,
			HitPct:          hitPct,
			Qualified:       qualified,
			AvgLegOdds:      avgOdds,
			TerroristCount:  row.TerroristCount,
			ParlayTerrorist: row.ParlayTerrorist,
			WagerCount:      row.WagerCount,
			DollarPL:        dollarPL,
			DollarPositive:  dollarPos,
			DollarNegative:  dollarNeg,
		})
	}

	return RecordPage{
		Page:          page,
		GroupName:     group.Name,
		Season:        selectedSeason,
		Seasons:       seasons,
		HasGroup:      true,
		HasSlips:      book.TotalSlips > 0,
		Record:        fmt.Sprintf("%d-%d-%d", book.Wins, book.Losses, book.Pushes),
		Wins:          book.Wins,
		Losses:        book.Losses,
		Pushes:        book.Pushes,
		TotalSlips:    book.TotalSlips,
		UnitsPL:       formatUnits(book.UnitsPL),
		UnitsPositive: book.UnitsPL > 0.005,
		UnitsNegative: book.UnitsPL < -0.005,
		LongestWin:    book.LongestWin,
		LongestLoss:   book.LongestLoss,
		Leaderboard:   leaderboard,
	}
}

func formatUnits(value float64) string {
	if math.Abs(value) < 0.005 {
		return "0.00u"
	}
	return fmt.Sprintf("%+.2fu", value)
}

func formatSignedMoney(value float64) string {
	if math.Abs(value) < 0.005 {
		return "$0.00"
	}
	if value < 0 {
		return fmt.Sprintf("-$%.2f", -value)
	}
	return fmt.Sprintf("+$%.2f", value)
}
