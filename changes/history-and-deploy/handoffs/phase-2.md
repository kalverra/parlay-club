# Phase 2 handoff: History file

## Done

- `internal/seed.Load` reads `seed/history.yaml` and imports its slips into a group. It returns a `Report` with the
  slips created, slips skipped, and users created.
- The whole file is decoded with known fields only and validated before the first write: required season, season type,
  week, name, and at least one leg; `legs_per_member` from 1 to 5; each user at most that many legs; a known market and
  a valid leg result on every leg. Problems are joined and name the slip and leg. A file with any problem imports
  nothing.
- A slip is keyed by its week and name within the group. An existing slip is skipped before any users are resolved or
  legs added, so a second load creates no slips, users, legs, or activity.
- Each person resolves through `resolveUser`: a name match ignoring case, or a new active user with no Google identity.
  Names resolve through the new `db.Store.UserByName`; the target group comes from the new `db.Store.FirstGroup`.
- A missing slip is built in one transaction: `UpsertWeek`, `CreateSlip`, then `Service.AddLeg` and
  `Service.SetLegResult` for every leg whose result is not `PENDING`. The leg's owner grades it, so `graded_by` holds
  their name and the slip result and status settle the normal way.
- Defaults match the app: a missing market is `OTHER`, a missing result is `PENDING`, missing odds are `-110`, and a
  missing `legs_per_member` is 2. An `ANYTIME_TD` leg without a side gets `YES`.
- `cmd/server/main.go` calls `loadHistory` after migrations. It logs the group and the three counts. No group yet, a
  missing file, or a bad file logs and lets the app serve.
- `seed/history.yaml` ships comments only, with a commented example.
- Tests cover created slips, users, and computed results (won, lost, and a push that leaves the rest to win); loading
  twice; the shipped empty file; a missing file; and a validation table for unknown result, unknown market, missing
  selection, missing user, too many legs, missing name, a week out of range, a bad season type, and an unknown field.
- Docs: added `docs/reference/history-file.md`, added it to `docs/llms.txt`, and deleted History from
  `plan/product.md`.
- `go test ./...` and `golangci-lint run ./...` pass.

## Deviations

- The design said the loader goes through `internal/slips`. There is no service create, so the week and slip are
  inserted through the store inside one transaction, and only the legs and results go through `Service.AddLeg` and
  `Service.SetLegResult`.
- The file has no group field, so the loader targets the first group by id. The design only said groups would point it
  at a group later.
- Idempotence compares week and name ignoring case, not by exact name. A slip saved as "Week 7" blocks a file entry
  named "week 7".
- A bad file imports nothing because validation runs over the whole file first. This is stricter than "skip what is
  already there" and is what the reference doc describes.
- Users are created outside the slip transaction, so a failure while building a slip can leave the new users behind
  without their slip.

## Open issues

- Names are not unique. `UserByName` returns the oldest match, so two people with the same display name collapse into
  one in the history file. The later email or alias matching has to disambiguate them.
- On a fresh database there is no group on the first boot, so the import logs and waits. History only lands after a
  group exists and the instance boots again. Phase 3, and anyone testing a fresh deploy, should expect this.
- A skipped slip is never updated. Changing a result in the file after it loaded does nothing; the slip must be edited
  in the app or entered under a new name. Docs say so, but the deploy runbook should repeat it.
- Users created for a slip that then fails are not cleaned up. A corrected file on the next boot creates the slip and
  reuses those users, so the leftover is cosmetic rather than a data conflict.
- `NEEDS_GRADING` and the `grading-record-book` auto-grader still have not landed. History can carry `NEEDS_GRADING`,
  but nothing grades it automatically.

## Next-phase notes

- Keep `seed/history.yaml` in the image and in the process working directory; `DefaultPath` is relative to the working
  directory. The image must also run migrations before `loadHistory`.
- Keep importing through `seed.Load` and do not insert legs or slip results directly, or the slip result will not
  settle.
- If user resolution changes for Google sign-in or groups, change `resolveUser` only. The file stays free of group
  fields.
- A typed or stored past week needs no slate; `UpsertWeek` creates it.

## Files touched

- `cmd/server/main.go`
- `docs/llms.txt`
- `docs/reference/history-file.md`
- `go.mod`
- `internal/db/groups.go`
- `internal/db/groups_test.go`
- `internal/db/users.go`
- `internal/db/users_test.go`
- `internal/seed/seed.go`
- `internal/seed/seed_test.go`
- `plan/product.md`
- `seed/history.yaml`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
