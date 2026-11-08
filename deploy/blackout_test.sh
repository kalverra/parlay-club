#!/bin/sh
# Exercise deploy/blackout.sh around both game windows and their edges. Every
# case pins NOW, so this runs anywhere, any day, and covers both EST and EDT.

set -eu

here=$(cd "$(dirname "$0")" && pwd)
script="$here/blackout.sh"

pass=0
fail=0

check() {
	want=$1
	now=$2
	label=$3

	status=0
	NOW="$now" "$script" >/dev/null 2>&1 || status=$?

	if [ "$status" -eq "$want" ]; then
		printf 'ok   %-34s %s\n' "$label" "$now"
		pass=$((pass + 1))
	else
		printf 'FAIL %-34s %s: want exit %s, got %s\n' "$label" "$now" "$want" "$status" >&2
		fail=$((fail + 1))
	fi
}

# 2026-11-08 is a Sunday, in EST (UTC-5).
check 0 "2026-11-08T13:59:00Z" "Sunday 08:59 New York"
check 1 "2026-11-08T14:00:00Z" "Sunday 09:00 New York"
check 1 "2026-11-09T04:59:00Z" "Sunday 23:59 New York"
check 0 "2026-11-09T05:00:00Z" "Monday 00:00 New York"
check 0 "2026-11-10T00:59:00Z" "Monday 19:59 New York"
check 1 "2026-11-10T01:00:00Z" "Monday 20:00 New York"
check 1 "2026-11-10T04:59:00Z" "Monday 23:59 New York"
check 0 "2026-11-10T05:00:00Z" "Tuesday 00:00 New York"
check 0 "2026-11-07T17:00:00Z" "Saturday 12:00 New York"

# The window follows New York local time through daylight saving.
check 0 "2026-07-12T12:59:00Z" "Sunday 08:59 New York (EDT)"
check 1 "2026-07-12T13:00:00Z" "Sunday 09:00 New York (EDT)"

# Offsets with a colon work the same as Z.
check 1 "2026-11-08T09:00:00-05:00" "Sunday 09:00 New York (offset)"
check 0 "2026-11-08T08:59:00-05:00" "Sunday 08:59 New York (offset)"

# A bad timestamp is a usage error, not a silent pass.
check 2 "not-a-timestamp" "rejects a bad NOW"

printf '\n%s passed, %s failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
