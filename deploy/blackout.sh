#!/bin/sh
# Refuse deploys during the game windows: Sunday 09:00–23:59 and Monday
# 20:00–23:59 in America/New_York. Outside them a deploy is clear.
#
# Reads NOW (an ISO-8601 timestamp with an offset or Z) when set, so tests can
# pin the clock. Exits 1 inside a window, 0 outside it, and 2 when NOW cannot
# be parsed.

set -eu

ISO="${NOW:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

# Turn the timestamp into epoch seconds. GNU date covers Linux and CI; the BSD
# fallback covers macOS. BSD's %z wants +0000, not +00:00.
epoch=""
if epoch=$(date -u -d "$ISO" +%s 2>/dev/null); then
	:
else
	compact=$(printf '%s' "$ISO" | sed -E 's/([+-][0-9]{2}):([0-9]{2})$/\1\2/')
	if ! epoch=$(date -j -u -f "%Y-%m-%dT%H:%M:%S%z" "$compact" +%s 2>/dev/null); then
		epoch=$(date -j -u -f "%Y-%m-%dT%H:%M:%SZ" "$compact" +%s 2>/dev/null) || {
			echo "blackout: cannot parse NOW=$ISO; use an ISO-8601 timestamp" >&2
			exit 2
		}
	fi
fi

# Format that instant in the game's timezone. The minutes are shown but do not
# affect the decision, because both windows start on the hour.
if stamp=$(TZ=America/New_York date -d "@$epoch" "+%w|%H|%M|%a %Y-%m-%d %H:%M %Z" 2>/dev/null); then
	:
else
	stamp=$(TZ=America/New_York date -r "$epoch" "+%w|%H|%M|%a %Y-%m-%d %H:%M %Z")
fi

dow=$(printf '%s' "$stamp" | cut -d'|' -f1)
hour=$(printf '%s' "$stamp" | cut -d'|' -f2)
pretty=$(printf '%s' "$stamp" | cut -d'|' -f4)

# "09" -ge 9 would trip octal parsing in some shells; strip the leading zero.
hour=${hour#0}
[ -n "$hour" ] || hour=0

if { [ "$dow" -eq 0 ] && [ "$hour" -ge 9 ]; } || { [ "$dow" -eq 1 ] && [ "$hour" -ge 20 ]; }; then
	echo "blackout: deploys paused, $pretty is inside a game window." >&2
	echo "blackout: deploying now can drop the old instance's last writes; re-run deploy.yml with force=true to override." >&2
	exit 1
fi

echo "blackout: clear to deploy at $pretty."
exit 0
