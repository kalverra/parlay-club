package refresh

import (
	"context"
	"fmt"
)

// publishWeek signals that a week's board changed and that every slip in the
// week needs a fresh view. Call it only after the transaction that changed rows
// has committed, so a listener never refetches a write that may roll back.
func (s *Service) publishWeek(ctx context.Context, weekID int64) {
	if s.publisher == nil {
		return
	}

	s.publisher.Publish(fmt.Sprintf("week:%d", weekID))

	slipIDs, err := s.store.SlipIDsForWeek(ctx, weekID)
	if err != nil {
		s.logger.Error().Err(err).Int64("week", weekID).Msg("list slips to publish")
		return
	}
	for _, slipID := range slipIDs {
		s.publisher.Publish(fmt.Sprintf("slip:%d", slipID))
	}
}
