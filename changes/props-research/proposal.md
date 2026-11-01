---
slug: props-research
status: proposed
created: 2026-11-01
---

# Props and research

## Why

Player props are the group's most common legs, but they must be typed by hand, and nobody checks whether a line is
realistic.

## Problem

The app has no prop lines, no link between a sportsbook's player names and ESPN's players, and no recent game history to
judge a line.

## Decision

Load player props for a game when someone opens it, within the Odds API budget. Match players to ESPN, and add a
research view with last-5 hit bars, median against average, and key injuries. Add the median-versus-mean prop warning to
the analyzer.
