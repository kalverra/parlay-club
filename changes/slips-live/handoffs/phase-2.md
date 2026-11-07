# Phase 2: Slip rules

## Done

- Added the `internal/db` store layer:
  - Weeks: `WeekID`, `UpsertWeek` (idempotent by season, type, and week), and `WeekByID`.
  - Slips: `CreateSlip` (trims names, enforces 1–5 legs and at least one member, ignores repeat
    members), `SlipByID`, `SlipsForWeek`, `NewestSlipForWeek`, `PreviousSlip` (same week first, then
    earlier weeks), `SlipMembers`, `SlipMemberIDs`, `AddSlipMember`, `RemoveSlipMember`,
    `SetSlipMemberLock`, `UpdateSlip`, and `SetSlipStatus`.
  - Legs: `CreateLeg` (0 odds becomes -110, new legs start `PENDING`), `LegByID`, `LegsForSlip`,
    `UpdateLeg` (pick fields only), and `DeleteLeg`.
  - Activity: `AddActivity` (blank messages refused) and `ActivityForSlip` (newest first).
  - Games: `GameKickoff` and `WeekPast` as the clock inputs for the kickoff rule.
- Added `internal/slips`:
  - `CreateDefaults` plus `DefaultLegsPerMember` (2), copying the previous slip's limit and member IDs.
  - `Service` with an injected publisher and clock. It enforces every rule in
    `docs/concepts/slips.md` "Editing and locking", writes each change and one activity entry in a
    transaction, updates the slip status on lock in and unlock, and publishes `slip:{id}` once after
    the commit.
  - Typed errors for every refusal, wrapped with context so `errors.Is` still works in handlers.
- Tests:
  - Store tests cover week upserts, slip validation, previous-slip selection, member and lock
    mutations, status changes, leg CRUD and defaults, activity ordering, and missing rows.
  - Rule table tests 1–9 run on a fixed injected clock: membership and ownership, the leg limit,
    kickoff locks (upcoming, started, exactly now, no game, past week), lock in with exactly and too
    many legs, no-op lock in, locked-in member editing, the slip locking when everyone is in, unlock,
    settings only while `OPEN`, the limit floor, removing a member with legs, past-week edits, one
    activity entry and one publish per mutation, and refused mutations leaving no trace.
- Docs: added `docs/concepts/slips.md`, deleted Slips, Legs, and Editing and locking from
  `plan/product.md`, and updated the `plan/README.md`, `plan/architecture.md`, and `docs/llms.txt`
  pointers.

## Deviations

- A slip's week counts as past when every game in it has kicked off (`WeekPast`), not when its week id
  is older than a current week. That is the only clock signal available before `slate-and-lines`, and
  it also makes a current week editable once its last game starts. Phase 4 can switch to an explicit
  current-week signal if that is too loose.
- `Service` holds `*db.Store` directly. The small `Store` interface in `slips.go` exists only for
  `CreateDefaults`, so its tests can fake `PreviousSlip` and `SlipMemberIDs` without a database.
- `UpdateLeg` checks kickoff on both the old and the new game. Moving a leg into a game that has
  started has to be refused too, not only editing a leg already in one.
- A `UpdateSlip` with the same trimmed name and limit writes no activity and publishes nothing.
  `AddMember` of someone already present and a second `LockIn` also no-op. This keeps "one activity
  entry each" honest instead of recording phantom edits.
- `UpdateLeg` writes only pick fields, so an edit cannot move a leg to another owner or slip or clear
  its result or grader.
- `Selection` is still supplied by the caller. No builder generates it from the leg fields yet; the
  phase 4 form will produce the display strings in the docs table.

## Open issues

- `AddMember` checks only that the user exists, not that they belong to the slip's group. Phase 4 must
  limit the picker and the request to group members, or add that check here.
- Nothing guards a `GRADED` slip. `LockIn` can overwrite `GRADED` with `LOCKED`, `Unlock` can reopen
  it, and `AddLeg` can add to a graded past-week slip. `grading-record-book` must freeze graded slips,
  or a status check belongs in the service before then.
- `WeekPast` treats a week as past once its last game kicks off (see Deviations). Decide in phase 4
  whether that is the intended meaning of "past weeks skip rule 3".
- Odd prices stay American integers in `legs.odds`. The store never converts; phase 4 converts at the
  form and handler edges and keeps `parlay` in decimal.
- The service does not require a game on a leg, does not build `selection`, and does not check group
  membership. Those are handler and form rules for phase 4.
- `SetSlipStatus` accepts `GRADED`, but this change never computes or writes it. Grading owns the
  transition, including the doc's "result no longer `PENDING`" rule.

## Next-phase notes

- `slips.New(store, hub, nil)` is the wiring shape. `Publisher` is one method, `Publish(topic string)`,
  so `internal/live.Hub` can satisfy it. A nil publisher and a nil clock both work, which keeps tests
  and early wiring simple.
- Published topics are `slip:{id}`. One mutation publishes once after its transaction commits.
- `CreateDefaults(ctx, store, groupID, weekID)` returns `Defaults{LegsPerMember, MemberIDs}` for the
  create form. Week and name defaults (`Week 9`, then `Week 9 #2`) are still handler work.
- Use `NewestSlipForWeek` for the home page, `SlipsForWeek` for the switcher (newest first), and
  `ActivityForSlip` for the feed (newest first).
- Handler errors are sentinels. Map them to 422 with the messages in `docs/concepts/slips.md`, using
  `errors.Is`; the service wraps every refusal with context.
- Keep `internal/db` a thin query layer. Conversion, selection building, and group membership checks
  belong outside it.

## Files touched

- `docs/concepts/slips.md`
- `docs/llms.txt`
- `internal/db/activity.go`
- `internal/db/activity_test.go`
- `internal/db/games.go`
- `internal/db/legs.go`
- `internal/db/legs_test.go`
- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/db/weeks.go`
- `internal/db/weeks_test.go`
- `internal/slips/rules.go`
- `internal/slips/rules_test.go`
- `internal/slips/slips.go`
- `internal/slips/slips_test.go`
- `plan/README.md`
- `plan/architecture.md`
- `plan/product.md`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
