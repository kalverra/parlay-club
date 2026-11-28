package progress

import (
	"fmt"
	"strings"
)

// EvaluateSlip aggregates individual leg evaluations into a combined parlay
// progress summary. Pushed and voided legs (Counts == false) are excluded from
// probabilities, counts, and bar progress.
func EvaluateSlip(legs []LegProgress) SlipProgress {
	if len(legs) == 0 {
		return SlipProgress{
			State:     StateScheduled,
			Tone:      toneNeutral,
			ProbTrend: trendFlat,
		}
	}

	out := SlipProgress{
		PreGameProb: 1.0,
		LiveProb:    1.0,
	}

	var sumPct float64
	for _, leg := range legs {
		if leg.HasLive {
			out.Show = true
		}
		if !leg.Counts {
			continue
		}

		out.TotalCount++
		out.PreGameProb *= leg.PreGameProb

		switch leg.State {
		case StateBusted:
			out.LostCount++
			out.LiveProb = 0
		case StateCashed:
			out.HitCount++
			sumPct += 100
		case StateLive:
			out.LiveCount++
			out.LiveProb *= leg.LiveProb
			sumPct += clamp(leg.ProgressPct, 0, 100)
		default:
			out.ScheduledCount++
			out.LiveProb *= leg.LiveProb
		}
	}

	if out.TotalCount == 0 {
		out.State = StatePush
		out.Tone = toneNeutral
		out.Summary = "Push"
		out.ProbTrend = trendFlat
		return out
	}

	out.ProgressPct = sumPct / float64(out.TotalCount)

	switch {
	case out.LostCount > 0:
		out.State = StateBusted
		out.Tone = toneLost
		out.LiveProb = 0
		out.Summary = fmt.Sprintf("%d/%d hit · %d lost", out.HitCount, out.TotalCount, out.LostCount)
	case out.HitCount == out.TotalCount:
		out.State = StateCashed
		out.Tone = toneWon
		out.LiveProb = 1.0
		out.ProgressPct = 100
		out.Summary = fmt.Sprintf("%d/%d hit", out.HitCount, out.TotalCount)
	case out.Show:
		out.State = StateLive
		if out.LiveProb >= out.PreGameProb {
			out.Tone = toneOnTrack
		} else {
			out.Tone = toneTrailing
		}
		out.Summary = formatLiveSummary(out.HitCount, out.TotalCount, out.LiveCount, out.ScheduledCount)
	default:
		out.State = StateScheduled
		out.Tone = toneNeutral
		out.Summary = fmt.Sprintf("%d/%d hit", out.HitCount, out.TotalCount)
	}

	out.ProbDelta, out.ProbTrend = probDeltaAndTrend(out.PreGameProb, out.LiveProb, true)
	return out
}

func formatLiveSummary(hit, total, live, scheduled int) string {
	parts := []string{fmt.Sprintf("%d/%d hit", hit, total)}
	if live > 0 {
		parts = append(parts, fmt.Sprintf("%d live", live))
	}
	if scheduled > 0 {
		parts = append(parts, fmt.Sprintf("%d upcoming", scheduled))
	}
	return strings.Join(parts, " · ")
}
