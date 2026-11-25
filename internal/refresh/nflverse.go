package refresh

import (
	"context"
	"fmt"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
)

// nflverseSource syncs the active season's nflverse player crosswalk, team
// efficiency, player usage, and snap counts.
func (s *Service) nflverseSource(week db.Week, games []db.Game, now time.Time) source {
	return source{
		key: NFLverseSource(week.Season),
		due: func(_ context.Context, state db.SyncState) (bool, error) {
			return nflverseStale(state.FetchedAt, now, week, games), nil
		},
		run: func(ctx context.Context) (bool, error) {
			return s.syncNFLverse(ctx, week, now)
		},
	}
}

// syncNFLverse fetches the four nflverse CSVs for week.Season, joins player
// usage and snap counts onto ESPN IDs, upserts the tables in one transaction,
// and records the sync. Any fetch failure leaves stored rows and sync_state
// untouched so the caller backs off and retries.
func (s *Service) syncNFLverse(ctx context.Context, week db.Week, now time.Time) (bool, error) {
	if s.nflverse == nil || week.Season <= 0 ||
		(week.SeasonType != nflverse.SeasonTypeRegular && week.SeasonType != nflverse.SeasonTypePostseason) {
		return false, nil
	}

	players, err := s.nflverse.Players(ctx)
	if err != nil {
		return false, fmt.Errorf("nflverse players: %w", err)
	}
	teamStats, err := s.nflverse.TeamStats(ctx, week.Season)
	if err != nil {
		return false, fmt.Errorf("nflverse team stats: %w", err)
	}
	playerStats, err := s.nflverse.PlayerStats(ctx, week.Season)
	if err != nil {
		return false, fmt.Errorf("nflverse player stats: %w", err)
	}
	snapCounts, err := s.nflverse.SnapCounts(ctx, week.Season)
	if err != nil {
		return false, fmt.Errorf("nflverse snap counts: %w", err)
	}

	playerIDRows, espnByGSIS, espnByPFR := buildNFLversePlayerIDs(players, now)
	teamRows := buildNFLverseTeamWeeks(teamStats, week.Season, now)
	playerWeekRows := buildNFLversePlayerWeeks(playerStats, snapCounts, espnByGSIS, espnByPFR, week.Season, now)

	changed := false
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		idChanged, err := tx.UpsertNFLversePlayerIDs(ctx, playerIDRows)
		if err != nil {
			return err
		}
		teamChanged, err := tx.UpsertNFLverseTeamWeeks(ctx, teamRows)
		if err != nil {
			return err
		}
		playerChanged, err := tx.UpsertNFLversePlayerWeeks(ctx, playerWeekRows)
		if err != nil {
			return err
		}
		changed = idChanged || teamChanged || playerChanged
		return tx.RecordSync(ctx, NFLverseSource(week.Season), now, nil)
	})
	if err != nil {
		return changed, err
	}
	if changed {
		s.publishWeek(ctx, week.ID)
		s.notifyMarketDataChange()
	}
	return changed, nil
}

func buildNFLversePlayerIDs(
	players []nflverse.Player,
	now time.Time,
) ([]db.NFLversePlayerID, map[string]string, map[string]string) {
	rows := make([]db.NFLversePlayerID, 0, len(players))
	byGSIS := make(map[string]string, len(players))
	byPFR := make(map[string]string, len(players))
	for _, p := range players {
		if p.ESPNID == "" || p.GSISID == "" {
			continue
		}
		rows = append(rows, db.NFLversePlayerID{
			ESPNID:    p.ESPNID,
			GSISID:    p.GSISID,
			PFRID:     p.PFRID,
			UpdatedAt: now,
		})
		byGSIS[p.GSISID] = p.ESPNID
		if p.PFRID != "" {
			byPFR[p.PFRID] = p.ESPNID
		}
	}
	return rows, byGSIS, byPFR
}

func buildNFLverseTeamWeeks(
	stats []nflverse.TeamWeek,
	season int,
	now time.Time,
) []db.NFLverseTeamWeek {
	rows := make([]db.NFLverseTeamWeek, 0, len(stats))
	for _, tw := range stats {
		if tw.Season != season ||
			(tw.SeasonType != nflverse.SeasonTypeRegular && tw.SeasonType != nflverse.SeasonTypePostseason) {
			continue
		}
		plays := float64(tw.Attempts + tw.Carries)
		var passRate float64
		if plays > 0 {
			passRate = float64(tw.Attempts) / plays
		}
		rows = append(rows, db.NFLverseTeamWeek{
			Season:            tw.Season,
			SeasonType:        tw.SeasonType,
			WeekNum:           tw.Week,
			Team:              tw.Team,
			OffPassEPA:        tw.PassingEPA,
			OffRushEPA:        tw.RushingEPA,
			DefPassEPAAllowed: tw.OppPassingEPA,
			DefRushEPAAllowed: tw.OppRushingEPA,
			PlaysPerGame:      plays,
			PassRate:          passRate,
			UpdatedAt:         now,
		})
	}
	return rows
}

type nflversePlayerWeekKey struct {
	espnID     string
	season     int
	seasonType int
	week       int
}

func buildNFLversePlayerWeeks(
	playerStats []nflverse.PlayerWeek,
	snapCounts []nflverse.SnapCount,
	espnByGSIS map[string]string,
	espnByPFR map[string]string,
	season int,
	now time.Time,
) []db.NFLversePlayerWeek {
	byKey := make(map[nflversePlayerWeekKey]db.NFLversePlayerWeek, len(playerStats))
	order := make([]nflversePlayerWeekKey, 0, len(playerStats))

	for _, ps := range playerStats {
		if ps.Season != season ||
			(ps.SeasonType != nflverse.SeasonTypeRegular && ps.SeasonType != nflverse.SeasonTypePostseason) {
			continue
		}
		espnID, ok := espnByGSIS[ps.PlayerID]
		if !ok {
			continue
		}
		key := nflversePlayerWeekKey{
			espnID:     espnID,
			season:     ps.Season,
			seasonType: ps.SeasonType,
			week:       ps.Week,
		}
		if _, exists := byKey[key]; !exists {
			order = append(order, key)
		}
		byKey[key] = db.NFLversePlayerWeek{
			ESPNID:        espnID,
			Season:        ps.Season,
			SeasonType:    ps.SeasonType,
			WeekNum:       ps.Week,
			Team:          ps.Team,
			Opponent:      ps.Opponent,
			PlayerName:    ps.PlayerName,
			Position:      ps.Position,
			TargetShare:   ps.TargetShare,
			AirYardsShare: ps.AirYardsShare,
			WOPR:          ps.WOPR,
			PassAtt:       ps.Attempts,
			PassYds:       ps.PassingYards,
			PassTDs:       ps.PassingTDs,
			Carries:       ps.Carries,
			RushYds:       ps.RushingYards,
			RushTDs:       ps.RushingTDs,
			Targets:       ps.Targets,
			Receptions:    ps.Receptions,
			RecYds:        ps.ReceivingYards,
			RecTDs:        ps.ReceivingTDs,
			KickerPts:     3*ps.FGMade + ps.PATMade,
			TotalEPA:      ps.PassingEPA + ps.RushingEPA + ps.ReceivingEPA,
			UpdatedAt:     now,
		}
	}

	for _, sc := range snapCounts {
		if sc.Season != season ||
			(sc.SeasonType != nflverse.SeasonTypeRegular && sc.SeasonType != nflverse.SeasonTypePostseason) {
			continue
		}
		espnID, ok := espnByPFR[sc.PFRPlayerID]
		if !ok {
			continue
		}
		key := nflversePlayerWeekKey{
			espnID:     espnID,
			season:     sc.Season,
			seasonType: sc.SeasonType,
			week:       sc.Week,
		}
		row, exists := byKey[key]
		if !exists {
			order = append(order, key)
			row = db.NFLversePlayerWeek{
				ESPNID:     espnID,
				Season:     sc.Season,
				SeasonType: sc.SeasonType,
				WeekNum:    sc.Week,
				Team:       sc.Team,
				UpdatedAt:  now,
			}
		} else if row.Team == "" {
			row.Team = sc.Team
		}
		row.OffenseSnaps = sc.OffenseSnaps
		row.OffensePct = sc.OffensePct
		byKey[key] = row
	}

	rows := make([]db.NFLversePlayerWeek, 0, len(order))
	for _, key := range order {
		rows = append(rows, byKey[key])
	}
	return rows
}
