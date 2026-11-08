---
id: history-file
title: History file
type: reference
summary: The seed/history.yaml format, its slip and leg fields and defaults, how boot imports it into the first group, which slips are skipped, and how errors and users are handled.
tags: [history, seed, yaml, import, slips, legs, results, users, boot]
sources: [internal/seed/seed.go, seed/history.yaml, cmd/server/main.go, internal/db/groups.go]
last_verified: 2026-11-08
---

# History file

## Summary

`internal/seed` reads `seed/history.yaml` from the working directory once at boot, after migrations. Every slip in the
file is created in the app's first group, the one with the lowest id. The file carries the picks and their results; the
slip result and status are computed from the legs, the same way the app computes them for a live slip.

A slip is keyed by its week and name within the group. A slip already stored under the same key is skipped before any
of its legs are added, so a restart or a later edit to the file does not duplicate it. To change a slip that already
loaded, edit it in the app or give the file entry a new name.

The shipped file is comments only. An empty or comments-only file is valid and imports nothing. A missing file logs an
error and the app keeps serving.

## Shape

```yaml
slips:
  - season: 2026
    season_type: 2 # 2 regular, 3 postseason
    week: 7
    name: Week 7
    legs_per_member: 2
    legs:
      - user: Adam
        selection: BUF -3.5
        market: OTHER
        odds: -110
        result: WON
      - user: Sam
        selection: KC ML
        result: LOST
```

The top level holds one key, `slips`, a list. Unknown fields are refused, which catches typos and fields that belong to
a later change.

### Slip fields

| Field             | Required | Rules                                                                     |
| ----------------- | -------- | ------------------------------------------------------------------------- |
| `season`          | yes      | A positive year, e.g. `2026`.                                              |
| `season_type`     | yes      | `1` preseason, `2` regular, or `3` postseason.                             |
| `week`            | yes      | `1`–`99`.                                                                  |
| `name`            | yes      | Trimmed; must not be blank. With the week, the slip's key in the group.    |
| `legs_per_member` | no       | `1`–`5`. Defaults to `2`, the create-slip default.                         |
| `legs`            | yes      | At least one. Each user may appear at most `legs_per_member` times.        |

The week is upserted, so a past week the slate never stored imports without a slate or any games.

### Leg fields

| Field       | Required | Rules                                                                                           |
| ----------- | -------- | ----------------------------------------------------------------------------------------------- |
| `user`      | yes      | Matched to a user by name, ignoring case and surrounding space. An unknown name creates an active user. |
| `selection` | yes      | Trimmed, free text, e.g. `BUF -3.5` or `Chiefs win the Super Bowl`.                              |
| `market`    | no       | One market name from [Slips](../concepts/slips.md#legs). Defaults to `OTHER`.                    |
| `team`      | no       | ESPN abbreviation.                                                                              |
| `player`    | no       | Player name.                                                                                    |
| `player_id` | no       | ESPN athlete ID.                                                                                |
| `side`      | no       | `TEAM`, `OVER`, `UNDER`, or `YES`. An `ANYTIME_TD` leg without a side gets `YES`.                |
| `line`      | no       | Number. Leave it out for markets with no line, such as `ML` and `ANYTIME_TD`.                    |
| `game_id`   | no       | ESPN event ID. Past slips usually have none.                                                    |
| `odds`      | no       | American odds. Defaults to `-110`, the same as an unset price in the leg form.                   |
| `result`    | no       | A leg result. Defaults to `PENDING`.                                                            |

Leg results are `PENDING`, `NEEDS_GRADING`, `WON`, `LOST`, `PUSH`, and `VOID`. A leg left `PENDING` or
`NEEDS_GRADING` keeps the slip `PENDING`; the slip settles as described in
[Slips](../concepts/slips.md#grading). Historical picks are usually `OTHER` with free text and no game link.

## What a load does

1. The whole file is parsed and checked before any write. Problems are reported together with the slip or leg that
   caused them, and a file with a problem imports nothing.
2. Each slip is looked up by week and name within the first group. A match, ignoring case, counts as already present:
   the slip is skipped and its counters still count.
3. The slip's people are resolved by name. An unknown name becomes an active user with no Google identity, so a later
   sign-in can link to it.
4. The slip is created through `internal/slips`: the week is upserted, `CreateSlip` sets the members and limit, each leg
   is added through `AddLeg`, and each leg with a result other than `PENDING` is set through `SetLegResult`. The leg's
   owner is the grader, so `graded_by` holds their name. That writes the activity entries and computes the slip result
   and status the normal way. All of one slip's writes happen in one transaction.

The boot log reports the group, how many slips were created, how many were skipped, and how many users were created.
A failed load logs the error and lets the app serve.

With no group in the database there is nowhere to put the slips, so the import logs and waits for a later boot.

## Related

- [Slips](../concepts/slips.md): markets, leg results, slip result and status, and the editing rules.
- [Database schema](schema.md): the `slips`, `slip_members`, `legs`, and `weeks` tables.
