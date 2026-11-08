# Design: Sharp-friend analyzer

## Context

Slips now have legs tied to games with weather and lines. This change adds the rules engine and the banner under the
slip. `median_trap` waits for `props-research`, which brings player stats. Read before starting:

- `plan/analyzer.md`: all of it except `median_trap`.
- `plan/product.md`: Screens (analyzer banner).
- `docs/concepts/slips.md`.

## Approach

- `internal/analyzer` is pure: `AnalyzeSlip(legs, games, stats) []Warning` with the types in `plan/analyzer.md`. One
  unexported function per rule, each `func(ctx) []Warning`, run in a fixed order, then sorted by severity and first leg
  position.
- A small shared helper set covers "same game", "same team", "opponent", "favorite", "underdog", and "passing/receiving
  over", so rules read like their spec.
- Messages are built with `fmt.Sprintf` from the model messages, naming owners when legs belong to different people.
- `internal/web` maps slip legs and games into analyzer inputs when rendering the slip panel. The banner is part of the
  panel, so it refreshes on `slip-updated` and `board-updated` (weather changes) with no new wiring.
- `stats` is passed through but unread until `props-research` adds player numbers; a nil map is fine.

## Verification

- `go test ./...` and `golangci-lint run ./...` pass, and the analyzer package stays at or above 95% coverage.
- Table tests cover every row in `plan/analyzer.md` "Tests" on both sides of each threshold, plus `OTHER` legs, an empty
  slip, and stable output order.
- A handler test with BUF -7.5 owned by one user and Josh Allen Over 245.5 owned by another shows `blowout_passing` and
  `hook` with both owners named.
- The slip panel shows a green "Looks clean" banner with no warnings, otherwise cards sorted Danger, Warn, Info with the
  message and the legs involved.
- Every message is read against `plan/analyzer.md` "Tone" before closing.

## Risks

- Rules that read odds or lines from legs depend on hand-entered data being sane. Validate line and odds ranges in the
  leg form (in `internal/slips`), not in the analyzer.
- Tone drift: review every message against the Tone section before closing.

## TODO

### Phase 1: Engine and correlation rules

- [x] Add analyzer types, the rule runner, stable sorting, the shared leg and game helpers, and blowout_passing,
      trailing_rb, total_contradiction, stack_clock_drain, and stack_shootout with table tests covering every threshold
      row in plan/analyzer.md
- [x] Docs: add docs/reference/analyzer-rules.md with Rule Set A; delete it from plan/analyzer.md

### Phase 2: Weather, key numbers, chalk

- [x] Add wind, fake_weather, hook, total_key_number, and chalk with table tests covering every threshold row, plus
  OTHER legs and empty slips, and keep package coverage at or above 95%
- [ ] Docs: add Rule Sets B, C, and chalk to docs/reference/analyzer-rules.md; delete them from plan/analyzer.md

### Phase 3: Banner

- [ ] Map slip legs and games into analyzer inputs and render the banner: Looks clean, or cards sorted by severity with
  message, legs, and owners, with a handler test where BUF -7.5 (one user) and Josh Allen Over 245.5 (another) shows
  blowout_passing and hook to both
- [ ] Docs: add docs/concepts/analyzer.md (purpose, tone, inputs); delete the banner text from plan/product.md
