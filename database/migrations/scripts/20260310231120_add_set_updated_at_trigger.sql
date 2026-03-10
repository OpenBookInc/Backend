-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- set_updated_at
-- Generic trigger function that sets updated_at = NOW() on every UPDATE.
-- Attach to any table with an updated_at column via a BEFORE UPDATE trigger.
-- =============================================================================
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_balances_set_updated_at
    BEFORE UPDATE ON balances
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_cancel_requests_set_updated_at
    BEFORE UPDATE ON exchange_cancel_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_fills_set_updated_at
    BEFORE UPDATE ON exchange_fills
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_lineups_set_updated_at
    BEFORE UPDATE ON exchange_lineups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_matches_set_updated_at
    BEFORE UPDATE ON exchange_matches
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_orders_set_updated_at
    BEFORE UPDATE ON exchange_orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_order_states_set_updated_at
    BEFORE UPDATE ON exchange_order_states
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_exchange_slates_set_updated_at
    BEFORE UPDATE ON exchange_slates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_game_statuses_set_updated_at
    BEFORE UPDATE ON game_statuses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nba_box_scores_set_updated_at
    BEFORE UPDATE ON nba_box_scores
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nba_markets_set_updated_at
    BEFORE UPDATE ON nba_markets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nba_plays_set_updated_at
    BEFORE UPDATE ON nba_plays
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nfl_box_scores_set_updated_at
    BEFORE UPDATE ON nfl_box_scores
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nfl_drives_set_updated_at
    BEFORE UPDATE ON nfl_drives
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nfl_markets_set_updated_at
    BEFORE UPDATE ON nfl_markets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_nfl_plays_set_updated_at
    BEFORE UPDATE ON nfl_plays
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trigger_odds_blaze_market_outcomes_set_updated_at
    BEFORE UPDATE ON odds_blaze_market_outcomes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER trigger_odds_blaze_market_outcomes_set_updated_at ON odds_blaze_market_outcomes;
DROP TRIGGER trigger_nfl_plays_set_updated_at ON nfl_plays;
DROP TRIGGER trigger_nfl_markets_set_updated_at ON nfl_markets;
DROP TRIGGER trigger_nfl_drives_set_updated_at ON nfl_drives;
DROP TRIGGER trigger_nfl_box_scores_set_updated_at ON nfl_box_scores;
DROP TRIGGER trigger_nba_plays_set_updated_at ON nba_plays;
DROP TRIGGER trigger_nba_markets_set_updated_at ON nba_markets;
DROP TRIGGER trigger_nba_box_scores_set_updated_at ON nba_box_scores;
DROP TRIGGER trigger_game_statuses_set_updated_at ON game_statuses;
DROP TRIGGER trigger_exchange_slates_set_updated_at ON exchange_slates;
DROP TRIGGER trigger_exchange_order_states_set_updated_at ON exchange_order_states;
DROP TRIGGER trigger_exchange_orders_set_updated_at ON exchange_orders;
DROP TRIGGER trigger_exchange_matches_set_updated_at ON exchange_matches;
DROP TRIGGER trigger_exchange_lineups_set_updated_at ON exchange_lineups;
DROP TRIGGER trigger_exchange_fills_set_updated_at ON exchange_fills;
DROP TRIGGER trigger_exchange_cancel_requests_set_updated_at ON exchange_cancel_requests;
DROP TRIGGER trigger_balances_set_updated_at ON balances;

DROP FUNCTION set_updated_at();

-- +goose StatementEnd
