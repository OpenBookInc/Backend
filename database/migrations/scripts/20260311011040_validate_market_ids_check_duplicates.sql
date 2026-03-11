-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- validate_market_ids
-- Validates that all provided market IDs exist in at least one market table
-- (nba_markets or nfl_markets) and that there are no duplicate IDs.
-- =============================================================================
CREATE OR REPLACE FUNCTION validate_market_ids(p_market_ids UUID[])
RETURNS VOID AS $$
DECLARE
    v_missing UUID[];
    v_unique_count INT;
    v_total_count INT;
BEGIN
    -- Check for duplicate market IDs
    v_total_count := array_length(p_market_ids, 1);
    SELECT COUNT(DISTINCT id) INTO v_unique_count FROM unnest(p_market_ids) AS id;

    IF v_unique_count < v_total_count THEN
        RAISE EXCEPTION 'Duplicate market IDs found in input: %', p_market_ids;
    END IF;

    -- Check that all market IDs exist in at least one market table
    SELECT ARRAY(
        SELECT unnest(p_market_ids)
        EXCEPT
        (
            SELECT id FROM nba_markets WHERE id = ANY(p_market_ids)
            UNION
            SELECT id FROM nfl_markets WHERE id = ANY(p_market_ids)
        )
    ) INTO v_missing;

    IF array_length(v_missing, 1) IS NOT NULL THEN
        RAISE EXCEPTION 'Market IDs not found in any market table: %', v_missing;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- =============================================================================
-- validate_market_ids (revert to original without duplicate check)
-- =============================================================================
CREATE OR REPLACE FUNCTION validate_market_ids(p_market_ids UUID[])
RETURNS VOID AS $$
DECLARE
    v_missing UUID[];
BEGIN
    SELECT ARRAY(
        SELECT unnest(p_market_ids)
        EXCEPT
        (
            SELECT id FROM nba_markets WHERE id = ANY(p_market_ids)
            UNION
            SELECT id FROM nfl_markets WHERE id = ANY(p_market_ids)
        )
    ) INTO v_missing;

    IF array_length(v_missing, 1) IS NOT NULL THEN
        RAISE EXCEPTION 'Market IDs not found in any market table: %', v_missing;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
