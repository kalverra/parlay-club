package web

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
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
	UserID          int64
	Rank            int
	Name            string
	Avatar          User
	LegsRecord      string
	TotalLegs       int
	DecidedLegs     int
	HitPct          string
	ImpliedLift     string
	Qualified       bool
	AvgLegOdds      string
	TerroristCount  int
	ParlayTerrorist bool
	WagerCount      int
	DollarPL        string
	DollarPositive  bool
	DollarNegative  bool
	Tags            []RecordTagView
}

// RecordTagView is one earned Record Book tag rendered on a member row or profile.
type RecordTagView struct {
	Key         string
	Label       string
	Tone        string
	IsPraise    bool
	Rule        string
	Explanation string
}

// RecordMemberPage is the member bet history screen at GET /record/members/{id}.
type RecordMemberPage struct {
	Page
	GroupName string
	Season    int
	Seasons   []SeasonOption
	HasGroup  bool
	Member    RecordMemberView
	Bets      []RecordMemberBetView
}

// RecordMemberBetView is one graded leg in a member's season bet history.
type RecordMemberBetView struct {
	WeekLabel        string
	SlipID           int64
	SlipName         string
	SlipResult       string
	SlipResultClass  string
	Selection        string
	MarketLabel      string
	Odds             string
	Grade            string
	GradeClass       string
	Result           string
	ResultClass      string
	SoleKiller       bool
	NetVotes         int
	NetVotesText     string
	NetVotesPositive bool
	NetVotesNegative bool
}

type recordSeasonData struct {
	season     int
	allSeasons []int
	slips      []db.GradedSlip
	legs       []db.Leg
	users      []db.User
	legGrades  map[int64]string
	votes      map[int64]int
	book       record.Book
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

	loaded, ok := h.loadRecordSeasonData(w, r, group.ID)
	if !ok {
		return
	}

	data := buildRecordPage(page, group, loaded.season, loaded.allSeasons, loaded.users, loaded.book)
	h.renderPage(w, "record", data)
}

// recordMember renders one group member's season summary, tags, and newest-first
// graded leg history at GET /record/members/{id}?season=.
func (h *slipHandlers) recordMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseID(r.PathValue("id"))
	if !ok {
		http.Error(w, "invalid member id", http.StatusBadRequest)
		return
	}
	if h.deps.Store == nil {
		http.NotFound(w, r)
		return
	}

	group, ok := h.group(r.Context())
	if !ok {
		http.NotFound(w, r)
		return
	}

	loaded, ok := h.loadRecordSeasonData(w, r, group.ID)
	if !ok {
		return
	}

	targetUser, found := findUserByID(loaded.users, userID)
	if !found {
		http.NotFound(w, r)
		return
	}

	page := h.pageWithTab(r, targetUser.Name+" · Record Book", TabRecord)
	data := buildRecordMemberPage(page, group, targetUser, loaded)
	h.renderPage(w, "record-member", data)
}

func (h *slipHandlers) loadRecordSeasonData(
	w http.ResponseWriter,
	r *http.Request,
	groupID int64,
) (*recordSeasonData, bool) {
	ctx := r.Context()
	allSeasons, err := h.deps.Store.SeasonsForGroup(ctx, groupID)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}
	gradedSeasons, err := h.deps.Store.GradedSeasonsForGroup(ctx, groupID)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}

	selectedSeason, ok := h.resolveRecordSeason(w, r, allSeasons, gradedSeasons)
	if !ok {
		return nil, false
	}

	slips, err := h.deps.Store.GradedSlipsForGroupSeason(ctx, groupID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}
	legs, err := h.deps.Store.GradedLegsForGroupSeason(ctx, groupID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}
	users, err := h.deps.Store.RecordUsersForGroupSeason(ctx, groupID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}
	wagers, err := h.deps.Store.WagersForGroupSeason(ctx, groupID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}
	votes, err := h.deps.Store.VotesForGroupSeason(ctx, groupID, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return nil, false
	}

	legGrades := h.loadRecordLegGrades(ctx, groupID, selectedSeason)
	book := buildRecordBook(selectedSeason, slips, legs, users, wagers, legGrades, votes)
	return &recordSeasonData{
		season:     selectedSeason,
		allSeasons: allSeasons,
		slips:      slips,
		legs:       legs,
		users:      users,
		legGrades:  legGrades,
		votes:      votes,
		book:       book,
	}, true
}

func (h *slipHandlers) loadRecordLegGrades(ctx context.Context, groupID int64, season int) map[int64]string {
	if h.deps.EvalCache == nil {
		return nil
	}
	suite, err := h.deps.EvalCache.GetOrCompute(ctx, h.deps.Store, groupID, season)
	if err != nil {
		h.deps.Logger.Warn().
			Err(err).
			Int64("group_id", groupID).
			Int("season", season).
			Msg("record book: compute eval leg grades")
		return nil
	}
	engineName := ""
	if h.deps.GradingEngine != nil {
		engineName = h.deps.GradingEngine.Name()
	}
	return suite.LegGrades(engineName)
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
	legGrades map[int64]string,
	votes map[int64]int,
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
			ID:       leg.ID,
			SlipID:   leg.SlipID,
			UserID:   leg.UserID,
			Odds:     leg.Odds,
			Result:   leg.Result,
			Grade:    legGrades[leg.ID],
			NetVotes: votes[leg.ID],
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
	return RecordPage{
		Page:          page,
		GroupName:     group.Name,
		Season:        selectedSeason,
		Seasons:       buildSeasonOptions(allSeasons, selectedSeason),
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
		Leaderboard:   buildLeaderboardViews(users, book),
	}
}

func buildRecordMemberPage(
	page Page,
	group db.Group,
	targetUser db.User,
	loaded *recordSeasonData,
) RecordMemberPage {
	leaderboard := buildLeaderboardViews(loaded.users, loaded.book)
	member := RecordMemberView{
		UserID:     targetUser.ID,
		Name:       targetUser.Name,
		Avatar:     NewUser(targetUser),
		LegsRecord: "0-0",
		HitPct:     "–",
		AvgLegOdds: "–",
		DollarPL:   "–",
	}
	for _, row := range leaderboard {
		if row.UserID == targetUser.ID {
			member = row
			break
		}
	}

	return RecordMemberPage{
		Page:      page,
		GroupName: group.Name,
		Season:    loaded.season,
		Seasons:   buildSeasonOptions(loaded.allSeasons, loaded.season),
		HasGroup:  true,
		Member:    member,
		Bets:      buildMemberBets(targetUser.ID, loaded),
	}
}

func buildMemberBets(userID int64, loaded *recordSeasonData) []RecordMemberBetView {
	legsBySlip := make(map[int64][]db.Leg, len(loaded.slips))
	for _, leg := range loaded.legs {
		legsBySlip[leg.SlipID] = append(legsBySlip[leg.SlipID], leg)
	}

	bets := make([]RecordMemberBetView, 0)
	// GradedSlipsForGroupSeason returns oldest first; iterate in reverse for newest first.
	for _, gs := range slices.Backward(loaded.slips) {
		slipLegs := legsBySlip[gs.Slip.ID]
		soleKillerLegID := soleKillerLeg(gs.Slip.Result, slipLegs)
		wLabel := recordWeekLabel(gs.Week)

		for _, leg := range slipLegs {
			if leg.UserID != userID {
				continue
			}
			grade := loaded.legGrades[leg.ID]
			var gClass string
			if grade != "" {
				gClass = gradeClass(grade)
			}
			netVotes := loaded.votes[leg.ID]
			var netVotesText string
			if netVotes != 0 {
				netVotesText = fmt.Sprintf("%+d", netVotes)
			}

			bets = append(bets, RecordMemberBetView{
				WeekLabel:        wLabel,
				SlipID:           gs.Slip.ID,
				SlipName:         gs.Slip.Name,
				SlipResult:       string(gs.Slip.Result),
				SlipResultClass:  resultClass(string(gs.Slip.Result)),
				Selection:        leg.Selection,
				MarketLabel:      researchMarketLabel(leg.MarketType),
				Odds:             formatOdds(leg.Odds),
				Grade:            grade,
				GradeClass:       gClass,
				Result:           string(leg.Result),
				ResultClass:      resultClass(string(leg.Result)),
				SoleKiller:       leg.ID == soleKillerLegID,
				NetVotes:         netVotes,
				NetVotesText:     netVotesText,
				NetVotesPositive: netVotes > 0,
				NetVotesNegative: netVotes < 0,
			})
		}
	}
	return bets
}

func soleKillerLeg(slipResult parlay.Result, slipLegs []db.Leg) int64 {
	if slipResult != parlay.Lost {
		return 0
	}
	var lostID int64
	lostCount := 0
	for _, leg := range slipLegs {
		if leg.Result == parlay.Lost {
			lostID = leg.ID
			lostCount++
			if lostCount > 1 {
				return 0
			}
		}
	}
	if lostCount == 1 {
		return lostID
	}
	return 0
}

func buildSeasonOptions(allSeasons []int, selectedSeason int) []SeasonOption {
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
	return seasons
}

func buildLeaderboardViews(users []db.User, book record.Book) []RecordMemberView {
	userByID := make(map[int64]db.User, len(users))
	for _, u := range users {
		userByID[u.ID] = u
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

		impliedLift := ""
		if row.ImpliedLift != nil {
			impliedLift = fmt.Sprintf("%+.1f pts", *row.ImpliedLift)
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

		tags := make([]RecordTagView, 0, len(row.Tags))
		for _, tag := range row.Tags {
			tags = append(tags, RecordTagView{
				Key:         tag.Key,
				Label:       tag.Label,
				Tone:        string(tag.Tone),
				IsPraise:    tag.Tone == record.TagTonePraise,
				Rule:        tag.Rule,
				Explanation: tag.Explanation,
			})
		}

		leaderboard = append(leaderboard, RecordMemberView{
			UserID:          row.UserID,
			Rank:            i + 1,
			Name:            row.Name,
			Avatar:          NewUser(u),
			LegsRecord:      legsRecord,
			TotalLegs:       row.TotalLegs,
			DecidedLegs:     row.DecidedLegs,
			HitPct:          hitPct,
			ImpliedLift:     impliedLift,
			Qualified:       qualified,
			AvgLegOdds:      avgOdds,
			TerroristCount:  row.TerroristCount,
			ParlayTerrorist: row.ParlayTerrorist,
			WagerCount:      row.WagerCount,
			DollarPL:        dollarPL,
			DollarPositive:  dollarPos,
			DollarNegative:  dollarNeg,
			Tags:            tags,
		})
	}
	return leaderboard
}

func findUserByID(users []db.User, userID int64) (db.User, bool) {
	for _, u := range users {
		if u.ID == userID {
			return u, true
		}
	}
	return db.User{}, false
}

func recordWeekLabel(week db.Week) string {
	switch week.SeasonType {
	case 1:
		return fmt.Sprintf("Preseason Week %d", week.WeekNum)
	case 3:
		switch week.WeekNum {
		case 1:
			return "Wild Card"
		case 2:
			return "Divisional"
		case 3:
			return "Conference"
		case 4, 5:
			return "Super Bowl"
		default:
			return fmt.Sprintf("Postseason Week %d", week.WeekNum)
		}
	default:
		return weekLabel(week)
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
