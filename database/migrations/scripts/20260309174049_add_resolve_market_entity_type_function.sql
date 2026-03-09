-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- resolve_market_entity_type
-- Determines which market entity type a given market ID belongs to by checking
-- all market tables (nba_markets, nfl_markets).
-- Returns the corresponding market_entity_type enum value.
-- Raises exception if the market ID is not found in any table or is found in
-- more than one table.
-- =============================================================================
CREATE FUNCTION resolve_market_entity_type(
    p_market_id UUID
) RETURNS market_entity_type AS $$
DECLARE
    v_found_in_nba BOOLEAN;
    v_found_in_nfl BOOLEAN;
    v_match_count INT := 0;
BEGIN
    SELECT EXISTS(SELECT 1 FROM nba_markets WHERE id = p_market_id) INTO v_found_in_nba;
    SELECT EXISTS(SELECT 1 FROM nfl_markets WHERE id = p_market_id) INTO v_found_in_nfl;

    v_match_count := v_found_in_nba::INT + v_found_in_nfl::INT;

    IF v_match_count = 0 THEN
        RAISE EXCEPTION 'Market ID not found in any market table: %', p_market_id;
    END IF;

    IF v_match_count > 1 THEN
        RAISE EXCEPTION 'Market ID found in multiple market tables: %', p_market_id;
    END IF;

    IF v_found_in_nba THEN
        RETURN 'nba_market';
    END IF;

    RETURN 'nfl_market';
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION resolve_market_entity_type(UUID);
-- +goose StatementEnd
