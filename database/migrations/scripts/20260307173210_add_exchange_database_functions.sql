-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- Add unique constraint on exchange_slates for consistent lookups.
-- market_ids must always be sorted before insert (handled by the
-- ensure_exchange_slate_and_lineups function).
-- =============================================================================
ALTER TABLE exchange_slates
    ADD CONSTRAINT exchange_slates_market_type_market_ids_unique
    UNIQUE (market_type, market_ids);

-- =============================================================================
-- Trigger to reject inserts/updates with unsorted market_ids.
-- Ensures the unique constraint on (market_type, market_ids) works correctly
-- by preventing unsorted arrays from being stored.
-- =============================================================================
CREATE FUNCTION enforce_sorted_market_ids()
RETURNS TRIGGER AS $$
DECLARE
    v_sorted UUID[];
BEGIN
    SELECT ARRAY(SELECT unnest(NEW.market_ids) ORDER BY 1) INTO v_sorted;

    IF NEW.market_ids IS DISTINCT FROM v_sorted THEN
        RAISE EXCEPTION 'exchange_slates.market_ids must be sorted in ascending order. '
            'Received: %, expected: %. '
            'Use the ensure_exchange_slate_and_lineups() function which handles sorting automatically.',
            NEW.market_ids, v_sorted;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_exchange_slates_enforce_sorted_market_ids
    BEFORE INSERT OR UPDATE ON exchange_slates
    FOR EACH ROW
    EXECUTE FUNCTION enforce_sorted_market_ids();

-- =============================================================================
-- set_balance
-- Sets the user's balance to the given amount. Returns the updated balances row.
-- Raises exception if p_amount is negative or if balance record not found.
-- =============================================================================
CREATE FUNCTION set_balance(
    p_user_id UUID,
    p_amount BIGINT
) RETURNS balances AS $$
DECLARE
    v_row balances;
BEGIN
    IF p_amount < 0 THEN
        RAISE EXCEPTION 'p_amount must not be negative, got: %', p_amount;
    END IF;

    UPDATE balances
    SET total_balance = p_amount,
        updated_at = NOW()
    WHERE user_id = p_user_id
    RETURNING * INTO v_row;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Balance record not found for user_id: %', p_user_id;
    END IF;

    RETURN v_row;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- try_deduct_balance
-- Returns TRUE if deducted, FALSE if insufficient funds.
-- Raises exception if balance record not found.
-- =============================================================================
CREATE FUNCTION try_deduct_balance(
    p_user_id UUID,
    p_amount BIGINT
) RETURNS BOOLEAN AS $$
BEGIN
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'p_amount must be positive, got: %', p_amount;
    END IF;

    UPDATE balances
    SET total_balance = total_balance - p_amount,
        updated_at = NOW()
    WHERE user_id = p_user_id AND total_balance >= p_amount;

    IF FOUND THEN
        RETURN TRUE;
    END IF;

    -- Distinguish "insufficient funds" from "user not found"
    IF NOT EXISTS (SELECT 1 FROM balances WHERE user_id = p_user_id) THEN
        RAISE EXCEPTION 'Balance record not found for user_id: %', p_user_id;
    END IF;

    RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- ensure_exchange_slate_and_lineups
-- Finds or creates a slate with its lineups. Returns JSONB:
--   { "slate": {...}, "lineups": [{...}, ...] }
-- market_ids are sorted before storage/lookup for order-independent matching.
-- Lineups are generated as all 2^N combinations of over/under sides.
-- Raises exception if slate exists with different total_units.
-- =============================================================================
CREATE FUNCTION ensure_exchange_slate_and_lineups(
    p_market_type market_entity_type,
    p_market_ids UUID[],
    p_total_units BIGINT
) RETURNS JSONB AS $$
DECLARE
    v_sorted_ids UUID[];
    v_slate exchange_slates;
    v_num_markets INT;
    v_num_lineups INT;
    v_lineup_index INT;
    v_legs JSONB;
    v_result JSONB;
BEGIN
    IF array_length(p_market_ids, 1) IS NULL OR array_length(p_market_ids, 1) = 0 THEN
        RAISE EXCEPTION 'p_market_ids must not be empty';
    END IF;

    -- Sort market_ids for consistent storage and lookup
    SELECT ARRAY(SELECT unnest(p_market_ids) ORDER BY 1) INTO v_sorted_ids;

    -- Attempt insert; do nothing on conflict
    INSERT INTO exchange_slates (market_type, market_ids, total_units)
    VALUES (p_market_type, v_sorted_ids, p_total_units)
    ON CONFLICT (market_type, market_ids) DO NOTHING
    RETURNING * INTO v_slate;

    IF v_slate.id IS NOT NULL THEN
        -- New slate created; generate lineups (2^N combinations of over/under)
        v_num_markets := array_length(v_sorted_ids, 1);
        v_num_lineups := (1 << v_num_markets);

        FOR v_lineup_index IN 0..(v_num_lineups - 1) LOOP
            SELECT jsonb_agg(
                jsonb_build_object(
                    'market_id', v_sorted_ids[idx],
                    'side', CASE
                        WHEN (v_lineup_index >> (v_num_markets - idx)) & 1 = 0
                        THEN 'under' ELSE 'over'
                    END
                ) ORDER BY idx
            ) INTO v_legs
            FROM generate_series(1, v_num_markets) AS idx;

            INSERT INTO exchange_lineups (slate_id, lineup_index, legs)
            VALUES (v_slate.id, v_lineup_index, v_legs);
        END LOOP;
    ELSE
        -- Slate already exists; fetch and validate total_units
        SELECT * INTO v_slate
        FROM exchange_slates
        WHERE market_type = p_market_type AND market_ids = v_sorted_ids;

        IF v_slate.total_units != p_total_units THEN
            RAISE EXCEPTION 'Slate exists with total_units=%, expected %',
                v_slate.total_units, p_total_units;
        END IF;
    END IF;

    -- Return slate + lineups as JSONB
    SELECT jsonb_build_object(
        'slate', row_to_json(v_slate)::jsonb,
        'lineups', COALESCE((
            SELECT jsonb_agg(row_to_json(l)::jsonb ORDER BY l.lineup_index)
            FROM exchange_lineups l
            WHERE l.slate_id = v_slate.id
        ), '[]'::jsonb)
    ) INTO v_result;

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- create_exchange_order
-- Creates an order and its initial state. Returns JSONB:
--   { "order": {...}, "order_state": {...} }
-- Raises exception on FK violation (invalid lineup_id or user_id).
-- =============================================================================
CREATE FUNCTION create_exchange_order(
    p_lineup_id UUID,
    p_user_id UUID,
    p_order_type exchange_order_type,
    p_portion BIGINT,
    p_quantity BIGINT,
    p_client_order_id BIGINT,
    p_order_status exchange_order_status
) RETURNS JSONB AS $$
DECLARE
    v_order exchange_orders;
    v_order_state exchange_order_states;
BEGIN
    INSERT INTO exchange_orders (lineup_id, user_id, order_type, portion, quantity, client_order_id)
    VALUES (p_lineup_id, p_user_id, p_order_type, p_portion, p_quantity, p_client_order_id)
    RETURNING * INTO v_order;

    INSERT INTO exchange_order_states (order_id, status)
    VALUES (v_order.id, p_order_status)
    RETURNING * INTO v_order_state;

    RETURN jsonb_build_object(
        'order', row_to_json(v_order)::jsonb,
        'order_state', row_to_json(v_order_state)::jsonb
    );
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- create_exchange_match
-- Creates a match and its fills. Returns JSONB:
--   { "match": {...}, "fills": [{...}, ...] }
-- p_fills is a JSONB array: [{"order_id": uuid, "matched_portion": bigint}, ...]
-- Raises exception on FK violation (invalid order_id).
-- =============================================================================
CREATE FUNCTION create_exchange_match(
    p_aggressor_order_id UUID,
    p_matched_quantity BIGINT,
    p_fills JSONB
) RETURNS JSONB AS $$
DECLARE
    v_match exchange_matches;
    v_fills_result JSONB;
BEGIN
    INSERT INTO exchange_matches (aggressor_order_id, matched_quantity)
    VALUES (p_aggressor_order_id, p_matched_quantity)
    RETURNING * INTO v_match;

    WITH inserted_fills AS (
        INSERT INTO exchange_fills (match_id, order_id, matched_portion)
        SELECT v_match.id,
               (fill->>'order_id')::UUID,
               (fill->>'matched_portion')::BIGINT
        FROM jsonb_array_elements(p_fills) AS fill
        RETURNING *
    )
    SELECT COALESCE(jsonb_agg(row_to_json(f)::jsonb), '[]'::jsonb)
    INTO v_fills_result
    FROM inserted_fills f;

    RETURN jsonb_build_object(
        'match', row_to_json(v_match)::jsonb,
        'fills', v_fills_result
    );
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Signatures are specified to avoid ambiguity if overloads are ever added.
DROP FUNCTION create_exchange_match(UUID, BIGINT, JSONB);
DROP FUNCTION create_exchange_order(UUID, UUID, exchange_order_type, BIGINT, BIGINT, BIGINT, exchange_order_status);
DROP FUNCTION ensure_exchange_slate_and_lineups(market_entity_type, UUID[], BIGINT);
DROP FUNCTION try_deduct_balance(UUID, BIGINT);
DROP FUNCTION set_balance(UUID, BIGINT);
DROP TRIGGER trigger_exchange_slates_enforce_sorted_market_ids ON exchange_slates;
DROP FUNCTION enforce_sorted_market_ids;
ALTER TABLE exchange_slates DROP CONSTRAINT exchange_slates_market_type_market_ids_unique;
-- +goose StatementEnd
