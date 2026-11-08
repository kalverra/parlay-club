# Phase 1 handoff: Grading by hand

## Done

- `internal/slips.Service.SetLegResult` grades one leg by hand. It checks membership, records the acting member in
  `graded_by`, recomputes the slip result and status from every leg, writes one activity line, and publishes
  `slip:{id}`. Any slip member can grade any leg, including one they do not own.
- `db.SetLegResult` and `db.SetSlipGrade` write the two rows, with `db.ErrLegResultInvalid` and
  `db.ErrSlipResultInvalid` guarding the schema values. `parlay.Result.IsLegResult` and `IsSlipResult` hold the
  allowed sets.
- Status follows the slip: `GRADED` once the result settles, otherwise `LOCKED` when every member has locked in, else
  `OPEN`. Resetting a leg to `PENDING` reopens a graded slip.
- The result chip on each leg opens a menu of `PENDING`, `WON`, `LOST`, `PUSH`, and `VOID` and sends
  `PUT /legs/{id}/result`. Non-members see a plain chip. An unknown result answers 422 with "that result isn't
  valid".
- The create-slip form takes a typed season, season type, and week number for a week the slate never stored, and
  upserts that week. The stored-week picker now labels each option with its season, and a typed week later than the
  latest stored week is refused. Past weeks skip kickoff locks as before, so a free-text `OTHER` leg with no game
  works.
- Tests cover every Grading case in `plan/product.md`: all won, an early loss, pending and needs-grading holds, a
  `PUSH` on an exact line, a `VOID` leg, all push/void, reopening and relocking, membership refusal, unknown result,
  repeated grade as a no-op, and an override by another member. The web tests cover the chip menu, the 422, the
  past-week `OTHER` leg, and the future-week refusal.
- Docs: grading added to `docs/concepts/slips.md`, the new route in `docs/reference/routes.md`, the summary in
  `docs/llms.txt`, and the manual grading rules removed from `plan/product.md` (the auto-grading rules stay).
- `go test ./...` and `golangci-lint run ./...` pass.

## Deviations

- The design said the create form accepts any past season and week. It now has an "Or add a past week" fieldset for
  season, season type, and week number, while the picker still lists stored weeks. Typed weeks ahead of the latest
  stored week are refused ("That week hasn't started yet."), and any typed week is allowed while no weeks exist.
- `SetLegResult` is idempotent for the same member and the same result: it writes no activity and publishes nothing.
  The design said it writes activity and publishes on every call. A repeated grade by a different member still writes
  activity and takes ownership of `graded_by`.
- Value checks live in `internal/db` and `internal/parlay`, not only in `internal/slips`, so the service and the
  coming seed loader share them.
- The menu offers `PENDING`, `WON`, `LOST`, `PUSH`, and `VOID`. `NEEDS_GRADING` is valid through the service and db
  for the auto-grader, but it is not a menu choice.

## Open issues

- Phase 2's seed loader should create each slip, then call `Service.SetLegResult` for each leg so the slip result and
  status compute the normal way. For a week with no slate it can use `db.WeekID(season, seasonType, weekNum)` and
  `Store.UpsertWeek`.
- The loader must skip an existing slip by (week, name) before adding legs, or a second boot duplicates legs,
  activity, and grades.
- The future-week guard compares against the latest stored week. An empty database allows any typed week, so the seed
  loader does not need a slate to create history.
- Resetting a leg to `PENDING` reopens a graded slip. Anything that imports or re-grades history must expect the
  status to follow the fresh leg results.
- `docs/concepts/slips.md` mentions `NEEDS_GRADING` and the `grading-record-book` auto-grader. That change has not
  landed; only the hand path exists now.
- Four docs-stale warnings remain for `analyzer.md`, `export.md`, `local-dev.md`, and `analyzer-rules.md`. They predate
  this phase and do not fail `good-vibes check`.

## Next-phase notes

- Reuse `Service.SetLegResult` and `Store.UpsertWeek`; do not insert legs or slip results directly, or results will not
  settle.
- Resolve each person through one function as the design says. Today it creates a user by name; later it will match by
  email or alias.
- Keep `seed/history.yaml` free of group fields, and load it once after migrations.

## Files touched

- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/db/legs.go`
- `internal/db/legs_test.go`
- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/parlay/parlay.go`
- `internal/parlay/parlay_test.go`
- `internal/slips/rules.go`
- `internal/slips/rules_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `plan/product.md`
- `web/static/app.css`
- `web/templates/partials.html`
- `web/templates/slip.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
