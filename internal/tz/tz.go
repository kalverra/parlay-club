// Package tz holds the shared time zone locations used across the system.
package tz

import (
	"fmt"
	"time"

	// Embed the zone database so America/New_York resolves in a container
	// without a separate tzdata package.
	_ "time/tzdata"
)

// Eastern is America/New_York, the zone NFL schedules and kickoff windows use.
var Eastern = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("tz: load location %s: %v", name, err))
	}
	return location
}
