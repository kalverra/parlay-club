## Done

- Switched all user-facing board callers in `internal/web/pick.go` (5 sites), `internal/web/research.go` (2 sites), `internal/web/games.go` (1 site), and `internal/web/slips.go` (1 site) from `board.Build` to `board.BuildDualBook`.
- Updated single-book test fixtures and dual-book line/price expectations in `internal/web/games_test.go`, `internal/web/pick_test.go`, `internal/web/research_test.go`, and `internal/web/slips_test.go` so pickable rows include both `draftkings` and `fanduel` quotes and reflect non-averaged dual-book lines.
- Verified `go test ./internal/eval/... ./internal/analyzer/... ./internal/board/...`, `go test -short ./...`, and `golangci-lint run ./...` all pass with 0 issues.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 UI and template work can now rely on `Row.DK`, `Row.FD`, and `Row.LinesDiffer` being populated on every user-facing board row from `internal/web/pick.go`, `internal/web/games.go`, `internal/web/research.go`, and `internal/web/slips.go`.

## Commit

Switch web board callers to dual-book builder

User-facing pick, game, research, and slip-hub boards now use the DraftKings and FanDuel intersection builder so single-book props and synthetic averaged lines no longer reach users. All-book evaluation and sharp-book grading remain unchanged.
