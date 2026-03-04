(currently empty - no known bugs)

## Resolved Investigations

### "Postponed" games in game_statuses (investigated 2026-02-28)

Games marked as "postponed" in game_statuses may look like a bug, but they are expected behavior. When a game is postponed, Sportradar keeps the original game_id with a "postponed" status permanently and creates a new, separate game_id for the makeup game. The makeup game gets its own "closed" status once played.

Example: game_id 1062 (Bulls vs Heat, originally scheduled 2026-01-09 UTC 2026-01-08 PT) is permanently "postponed". The makeup game was assigned game_id 57976 (scheduled 2026-01-30 UTC 2026-01-29 PT) and has status "closed".

This means the ./grade_market_outcomes script will skip postponed games since they are never marked "closed", which is correct — any markets for the makeup game would be tied to the new game_id. If markets were placed against the original postponed game_id, those would need to be voided or manually reassigned.
