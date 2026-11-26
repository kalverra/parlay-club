// Package reminders schedules and delivers the one kickoff reminder each
// person gets per slip. It keys every reminder to the slip's fixed start, the
// earliest kickoff among the slip's scoped games, so a moved kickoff or a
// later kickoff wave cannot send a second one.
package reminders

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/push"
	"github.com/kalverra/parlay-club/internal/slips"
)

const (
	// LeadTime is how long before the slip starts the reminder is scheduled.
	LeadTime = 20 * time.Minute
	// WindowBefore is how early a fire may still remind.
	WindowBefore = 30 * time.Minute
	// WindowAfter is how late a fire may still remind.
	WindowAfter = 10 * time.Minute
	// MaxHorizon is the furthest a scheduler may place a task. Cloud Tasks
	// refuses anything beyond 30 days, so a later pass picks that slip up.
	MaxHorizon = 30 * 24 * time.Hour
)

// Reminder is one person the fire should reach.
type Reminder struct {
	UserID int64
	Step   slips.Step
	Need   int
}

// Sender delivers one reminder to one person's devices. *push.Service
// satisfies it.
type Sender interface {
	Enabled() bool
	SendToUsers(ctx context.Context, userIDs []int64, m push.Message)
}

// Report says what one fire did. A stale or no-op fire still answers 200, so a
// scheduler stops retrying it.
type Report struct {
	Sent    int    `json:"sent,omitempty"`
	Skipped string `json:"skipped,omitempty"`
}

// Deps are the services a reminder service works with.
type Deps struct {
	Store     *db.Store
	Sender    Sender
	Scheduler Scheduler
	Now       func() time.Time
	Logger    zerolog.Logger
}

// Service schedules reminders and delivers the ones a task fires.
type Service struct {
	store     *db.Store
	sender    Sender
	scheduler Scheduler
	now       func() time.Time
	logger    zerolog.Logger
}

// New builds a reminder service. A nil clock uses the wall clock, and a nil
// scheduler or sender makes scheduling a no-op.
func New(deps Deps) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		store:     deps.Store,
		sender:    deps.Sender,
		scheduler: deps.Scheduler,
		now:       now,
		logger:    deps.Logger,
	}
}

// SetScheduler installs the scheduler to enqueue with. A nil scheduler turns
// scheduling off, which is the sandbox and pre-Cloud-Tasks production case.
func (s *Service) SetScheduler(scheduler Scheduler) {
	s.scheduler = scheduler
}

// Due returns the members who should get a reminder for a fire at now. It is
// pure: the caller loads the rows and decides how to send. A fire is due only
// when the slip is OPEN, start is still its first kickoff, now is between 30
// and 10 minutes before start, and the member still has picking or locking in
// left.
func Due(
	slip db.Slip,
	members []db.SlipMember,
	legs []db.Leg,
	games []db.Game,
	start time.Time,
	now time.Time,
) []Reminder {
	reminders, _ := due(slip, members, legs, games, start, now, true)
	return reminders
}

// due is Due with an optional window check and a skip reason. The sandbox
// button sends at a frozen scenario time, so it skips the window but keeps the
// stale-start and slip-status checks.
func due(
	slip db.Slip,
	members []db.SlipMember,
	legs []db.Leg,
	games []db.Game,
	start time.Time,
	now time.Time,
	checkWindow bool,
) ([]Reminder, string) {
	if slip.Status != db.SlipOpen {
		return nil, "stale"
	}
	first, ok := slips.FirstKickoff(slip, games)
	if !ok || !first.Equal(start) {
		return nil, "stale"
	}
	if checkWindow && (now.Before(start.Add(-WindowBefore)) || now.After(start.Add(-WindowAfter))) {
		return nil, "stale"
	}

	reminders := make([]Reminder, 0, len(members))
	for _, member := range members {
		if !member.User.Active {
			continue
		}
		next := slips.NextStep(slip, members, legs, games, member.User.ID, now)
		switch next.Step {
		case slips.StepPick:
			reminders = append(reminders, Reminder{UserID: member.User.ID, Step: next.Step, Need: next.Need})
		case slips.StepLockIn:
			reminders = append(reminders, Reminder{UserID: member.User.ID, Step: next.Step})
		}
	}
	if len(reminders) == 0 {
		return nil, "nobody"
	}
	return reminders, ""
}

// Fire delivers the reminder for slipID's start, judging the fire window
// against the service clock.
func (s *Service) Fire(ctx context.Context, slipID int64, start time.Time) (Report, error) {
	return s.FireAt(ctx, slipID, start, s.now(), true)
}

// FireAt delivers the reminder for slipID's start against a supplied instant.
// It is Fire with a fixed clock, for tests and the sandbox.
func (s *Service) FireAt(
	ctx context.Context,
	slipID int64,
	start time.Time,
	now time.Time,
	checkWindow bool,
) (Report, error) {
	if s == nil || s.sender == nil || !s.sender.Enabled() {
		return Report{Skipped: "disabled"}, nil
	}

	slip, err := s.store.SlipByID(ctx, slipID)
	if errors.Is(err, db.ErrSlipNotFound) {
		return Report{Skipped: "stale"}, nil
	}
	if err != nil {
		return Report{}, fmt.Errorf("load slip %d: %w", slipID, err)
	}
	members, err := s.store.SlipMembers(ctx, slipID)
	if err != nil {
		return Report{}, fmt.Errorf("load slip %d members: %w", slipID, err)
	}
	legs, err := s.store.LegsForSlip(ctx, slipID)
	if err != nil {
		return Report{}, fmt.Errorf("load slip %d legs: %w", slipID, err)
	}
	games, err := s.store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return Report{}, fmt.Errorf("load week %d games: %w", slip.WeekID, err)
	}

	pending, reason := due(slip, members, legs, games, start, now, checkWindow)
	if len(pending) == 0 {
		return Report{Skipped: reason}, nil
	}

	sent := 0
	for _, reminder := range pending {
		// Reserve before the send, so a retried task never doubles a reminder.
		reserved, err := s.store.ReservePushLog(ctx, db.PushLogReminder, slipID, reminder.UserID, "")
		if err != nil {
			return Report{Sent: sent}, err
		}
		if !reserved {
			continue
		}
		s.sender.SendToUsers(ctx, []int64{reminder.UserID}, reminderMessage(slip, reminder))
		sent++
	}
	return Report{Sent: sent}, nil
}

// SendNow sends the reminder for slipID against the service clock, skipping
// the fire window. The sandbox button uses it because its clock is frozen.
func (s *Service) SendNow(ctx context.Context, slipID int64) (Report, error) {
	slip, err := s.store.SlipByID(ctx, slipID)
	if errors.Is(err, db.ErrSlipNotFound) {
		return Report{Skipped: "stale"}, nil
	}
	if err != nil {
		return Report{}, fmt.Errorf("load slip %d: %w", slipID, err)
	}
	games, err := s.store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return Report{}, fmt.Errorf("load week %d games: %w", slip.WeekID, err)
	}
	start, ok := slips.FirstKickoff(slip, games)
	if !ok {
		return Report{Skipped: "stale"}, nil
	}
	return s.FireAt(ctx, slipID, start, s.now(), false)
}

// Sync makes sure every open slip whose start is ahead has a task for that
// start. It never deletes a task: a stale one fires and skips. For a durable
// scheduler, a reminder_tasks row keeps the normal pass down to reads with no
// scheduler call.
func (s *Service) Sync(ctx context.Context, now time.Time) error {
	if s == nil || s.scheduler == nil || s.sender == nil || !s.sender.Enabled() {
		return nil
	}

	open, err := s.store.OpenSlips(ctx)
	if err != nil {
		return err
	}
	if len(open) == 0 {
		return nil
	}
	tasks, err := s.store.ReminderTasks(ctx)
	if err != nil {
		return err
	}

	gamesByWeek := make(map[int64][]db.Game)
	var errs []error
	for _, slip := range open {
		games, ok := gamesByWeek[slip.WeekID]
		if !ok {
			games, err = s.store.GamesForWeek(ctx, slip.WeekID)
			if err != nil {
				errs = append(errs, fmt.Errorf("week %d games: %w", slip.WeekID, err))
				continue
			}
			gamesByWeek[slip.WeekID] = games
		}

		start, ok := slips.FirstKickoff(slip, games)
		if !ok {
			continue
		}
		at := start.Add(-LeadTime)
		if !at.After(now) || start.Sub(now) > MaxHorizon {
			continue
		}
		// A durable scheduler keeps its tasks across restarts, so the row alone
		// says the wake-up exists. A non-durable one loses them with the
		// process, so re-enqueue and let the scheduler collapse a repeat.
		if s.scheduler.Durable() {
			if task, ok := tasks[slip.ID]; ok && task.Start.Equal(start) {
				continue
			}
		}

		err := s.scheduler.Enqueue(ctx, slip.ID, start, at)
		if err != nil && !errors.Is(err, ErrAlreadyExists) {
			s.logger.Warn().
				Err(err).
				Int64("slip", slip.ID).
				Time("start", start).
				Msg("reminders: enqueue")
			errs = append(errs, err)
			continue
		}
		if err := s.store.UpsertReminderTask(ctx, db.ReminderTask{
			SlipID:      slip.ID,
			Start:       start,
			ScheduledAt: at,
		}); err != nil {
			errs = append(errs, err)
			continue
		}
		s.logger.Info().
			Int64("slip", slip.ID).
			Time("start", start).
			Time("at", at).
			Msg("reminders: scheduled")
	}
	return errors.Join(errs...)
}

// SyncNow runs Sync against the service clock.
func (s *Service) SyncNow(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return s.Sync(ctx, s.now())
}

// reminderMessage builds the notification for one person. The tag replaces an
// earlier reminder for the same slip on the device.
func reminderMessage(slip db.Slip, reminder Reminder) push.Message {
	body := "Kickoff in 20 minutes. Your legs are in; lock them in."
	if reminder.Step == slips.StepPick {
		legs := "legs"
		if reminder.Need == 1 {
			legs = "leg"
		}
		body = fmt.Sprintf("Kickoff in 20 minutes. You still need %d %s.", reminder.Need, legs)
	}
	return push.Message{
		Title: slip.Name,
		Body:  body,
		URL:   fmt.Sprintf("/slips/%d", slip.ID),
		Tag:   fmt.Sprintf("slip-%d-reminder", slip.ID),
	}
}
