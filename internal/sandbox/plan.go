package sandbox

import (
	"time"

	"github.com/kalverra/parlay-club/internal/parlay"
)

// LegGrade is one scripted manual grade. A stage names the pick and the persona
// who records it, so activity and graded_by point at a real group member.
type LegGrade struct {
	Leg    LegID
	Actor  PersonaID
	Result parlay.Result
}

// StagePlan is the declarative half of a stage: the clock it runs at, whether
// the shared slip locks in, and which picks are graded by hand. The controller
// executes it through the real refresh and slip services.
type StagePlan struct {
	Stage  Stage
	Now    time.Time
	Lock   bool
	Grades []LegGrade
}

// PlanFor returns the scripted plan for a stage. The morning plan only sets the
// baseline clock; the other stages lock the shared slip and grade named legs.
func PlanFor(stage Stage) (StagePlan, bool) {
	now, ok := StageTime(stage)
	if !ok {
		return StagePlan{}, false
	}

	switch stage {
	case Morning:
		return StagePlan{Stage: stage, Now: now}, true
	case EarlyLoss:
		return StagePlan{
			Stage: stage,
			Now:   now,
			Lock:  true,
			Grades: []LegGrade{{
				Leg:    LegAdaBengalsML,
				Actor:  PersonaAda,
				Result: parlay.Lost,
			}},
		}, true
	case AllClear:
		return StagePlan{
			Stage:  stage,
			Now:    now,
			Lock:   true,
			Grades: everyLegWon(),
		}, true
	case SundayComplete:
		return StagePlan{
			Stage:  stage,
			Now:    now,
			Lock:   true,
			Grades: everyLegWon(),
		}, true
	default:
		return StagePlan{}, false
	}
}

// everyLegWon grades each seeded pick won in its owner's name.
func everyLegWon() []LegGrade {
	definitions := LegDefinitions()
	grades := make([]LegGrade, 0, len(definitions))
	for _, definition := range definitions {
		grades = append(grades, LegGrade{
			Leg:    definition.ID,
			Actor:  definition.Persona,
			Result: parlay.Won,
		})
	}
	return grades
}
