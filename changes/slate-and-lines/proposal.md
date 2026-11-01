---
slug: slate-and-lines
status: designed
created: "2026-11-01"
---

# Slate, weather, and lines

## Why

Typing every leg by hand is slow and error-prone, and the group can't see the slate, lines, or weather in one place.

## Problem

The app has no NFL data: no games, no lines, no weather.

## Decision

Pull the Sunday and Monday slate from ESPN, stadium weather from Open-Meteo, and game lines from The Odds API, fetched
only when someone opens the app and the data is stale. Show a game board with lines and weather badges, and let friends
add a spread, moneyline, or total in one tap. Stay well inside the Odds API's 500 free credits a month.
