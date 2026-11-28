## Commit

Fix combo matching, switch warnings, and rule docs

Same-player receiving prop pairs were unreachable due to string-sorting order, and tap-to-switch board options falsely warned against the picked leg they would replace. Fixing both ensures inline and slip combo checks match sportsbook rules accurately, while updating the reference doc keeps all 18 rules in sync with the engine.
