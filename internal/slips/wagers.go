package slips

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/kalverra/parlay-club/internal/db"
)

// Errors returned when a wager fails validation.
var (
	// ErrWagerStakeInvalid is returned when a wager's stake is not positive or finite.
	ErrWagerStakeInvalid = errors.New("stake must be greater than $0")
	// ErrWagerOddsInvalid is returned when American odds are 0 or between -99 and 99.
	ErrWagerOddsInvalid = errors.New("odds must be -100 or lower, or +100 or higher")
	// ErrWagerBoostInvalid is returned when a boost percentage is negative or non-finite.
	ErrWagerBoostInvalid = errors.New("boost can't be negative")
	// ErrWagerBookTooLong is returned when the sportsbook name exceeds 40 characters.
	ErrWagerBookTooLong = errors.New("book name must be 40 characters or fewer")
)

const maxWagerBookRunes = 40

// WagerInput is one member's submitted wager on a slip.
type WagerInput struct {
	Book     string
	Stake    float64
	Odds     int
	BoostPct float64
}

// SetWager validates and saves actorID's personal wager on slipID. Any member
// of the slip's group can log their bet on the slip.
func (s *Service) SetWager(ctx context.Context, actorID, slipID int64, input WagerInput) (db.Wager, error) {
	wager, err := validateWager(slipID, actorID, input)
	if err != nil {
		return db.Wager{}, fmt.Errorf("set wager for user %d on slip %d: %w", actorID, slipID, err)
	}

	var saved db.Wager
	err = s.store.Tx(ctx, func(tx *db.Store) error {
		if err := requireSlipGroupMember(ctx, tx, slipID, actorID); err != nil {
			return err
		}
		var err error
		saved, err = tx.UpsertWager(ctx, wager)
		return err
	})
	if err != nil {
		return db.Wager{}, fmt.Errorf("set wager for user %d on slip %d: %w", actorID, slipID, err)
	}
	s.notify(slipID)
	return saved, nil
}

// DeleteWager removes actorID's personal wager on slipID.
func (s *Service) DeleteWager(ctx context.Context, actorID, slipID int64) error {
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		if err := requireSlipGroupMember(ctx, tx, slipID, actorID); err != nil {
			return err
		}
		return tx.DeleteWager(ctx, slipID, actorID)
	})
	if err != nil {
		return fmt.Errorf("delete wager for user %d on slip %d: %w", actorID, slipID, err)
	}
	s.notify(slipID)
	return nil
}

func validateWager(slipID, userID int64, input WagerInput) (db.Wager, error) {
	if math.IsNaN(input.Stake) || math.IsInf(input.Stake, 0) || input.Stake <= 0 {
		return db.Wager{}, ErrWagerStakeInvalid
	}
	if input.Odds == 0 || (input.Odds > -100 && input.Odds < 100) {
		return db.Wager{}, ErrWagerOddsInvalid
	}
	if math.IsNaN(input.BoostPct) || math.IsInf(input.BoostPct, 0) || input.BoostPct < 0 {
		return db.Wager{}, ErrWagerBoostInvalid
	}
	book := strings.TrimSpace(input.Book)
	if utf8.RuneCountInString(book) > maxWagerBookRunes {
		return db.Wager{}, ErrWagerBookTooLong
	}
	var bookPtr *string
	if book != "" {
		bookPtr = &book
	}
	return db.Wager{
		SlipID:   slipID,
		UserID:   userID,
		Book:     bookPtr,
		Stake:    input.Stake,
		Odds:     input.Odds,
		BoostPct: input.BoostPct,
	}, nil
}

func requireSlipGroupMember(ctx context.Context, tx *db.Store, slipID, actorID int64) error {
	slip, err := tx.SlipByID(ctx, slipID)
	if err != nil {
		return err
	}
	member, err := tx.IsMember(ctx, slip.GroupID, actorID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNotMember
	}
	return nil
}
