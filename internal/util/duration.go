package util

import (
	"time"
)

var durations = []time.Duration{
	time.Nanosecond,
	time.Microsecond,
	time.Millisecond,
	time.Second,
	time.Minute,
	time.Hour,
}

// HumanElapsed returns the time elapsed since the given start time truncated
// to 100x the highest non-zero duration unit (ns, us, ms, ...). This tends to
// create very short duration strings when printed (e.g. 725.8ms) without having
// to fiddle too much with
func HumanElapsed(start time.Time) time.Duration {
	return humanElapsed(time.Since(start))
}

func humanElapsed(elapsed time.Duration) time.Duration {
	i := 0
	for i < len(durations) && elapsed >= durations[i] {
		i++
	}

	if i >= 2 {
		// Truncate to the next duration unit
		resolution := durations[i-2]

		if (durations[i-1] / durations[i-2]) > 100 {
			// If we're going from ns -> us, us -> ms, ms -> s,
			// then we want to have two decimal points of precision
			// here. Not doing this for s -> m or m -> h is fine as
			// there will already be this much precision.
			resolution *= 10
		}

		return elapsed.Truncate(resolution)
	}

	return elapsed
}
