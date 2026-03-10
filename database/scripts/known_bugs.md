### Incorrect fill quantities when matching across all lineups (reported 2026-03-10)

When a pool has 40 total_units and 2 legs (4 lineups), placing 3 resting orders on 3 different lineup indices each with portion 250,000, then sending an aggressor order on the remaining lineup index with portion 20 to complete the match: the fill quantities are wrong. The aggressor receives only 1 matched quantity and a resting order receives 37 matched quantity, rather than the expected distribution.

**To reproduce (via tester → matching server):**
1. DefinePool: total_units=40, num_lineups=4
2. NewOrder on lineup 0: portion=250,000, quantity=N
3. NewOrder on lineup 1: portion=250,000, quantity=N
4. NewOrder on lineup 2: portion=250,000, quantity=N
5. NewOrder on lineup 3 (aggressor): portion=20, quantity=N → triggers match

**Expected:** Fill quantities distributed correctly across all 4 orders.
**Actual:** Aggressor gets matched_quantity=1, one resting order gets matched_quantity=37.

**Resolution:** Add a unit test in `matching_server/src/entry_pool.rs` reproducing this exact scenario and fix the matching logic.

## Resolved Investigations

### "Postponed" games in game_statuses (investigated 2026-02-28)

Games marked as "postponed" in game_statuses may look like a bug, but they are expected behavior. When a game is postponed, Sportradar keeps the original game_id with a "postponed" status permanently and creates a new, separate game_id for the makeup game. The makeup game gets its own "closed" status once played.

Example: game_id 1062 (Bulls vs Heat, originally scheduled 2026-01-09 UTC 2026-01-08 PT) is permanently "postponed". The makeup game was assigned game_id 57976 (scheduled 2026-01-30 UTC 2026-01-29 PT) and has status "closed".

This means the ./grade_market_outcomes script will skip postponed games since they are never marked "closed", which is correct — any markets for the makeup game would be tied to the new game_id. If markets were placed against the original postponed game_id, those would need to be voided or manually reassigned.
