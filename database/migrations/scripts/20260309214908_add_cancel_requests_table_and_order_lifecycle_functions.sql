-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- exchange_cancel_requests
-- =============================================================================
CREATE TABLE exchange_cancel_requests (
    order_id UUID NOT NULL REFERENCES exchange_orders(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

ALTER TABLE public.exchange_cancel_requests ENABLE ROW LEVEL SECURITY;

-- =============================================================================
-- create_exchange_cancel_request
-- Records a user-initiated cancel request for an exchange order.
-- Raises exception if the order_id does not exist (FK constraint).
-- =============================================================================
CREATE FUNCTION create_exchange_cancel_request(
    p_order_id UUID
) RETURNS void AS $$
BEGIN
    INSERT INTO exchange_cancel_requests (order_id)
    VALUES (p_order_id);
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- calculate_total_filled_quantity
-- Returns the total filled quantity for an exchange order by summing
-- matched_quantity across all matches the order participated in.
-- Returns 0 if the order has no fills.
-- =============================================================================
CREATE FUNCTION calculate_total_filled_quantity(
    p_order_id UUID
) RETURNS BIGINT AS $$
DECLARE
    v_total BIGINT;
BEGIN
    SELECT COALESCE(SUM(m.matched_quantity), 0)
    INTO v_total
    FROM exchange_fills f
    JOIN exchange_matches m ON f.match_id = m.id
    WHERE f.order_id = p_order_id;

    RETURN v_total;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin

-- =============================================================================
-- create_exchange_order (replaces original)
-- Creates an order, its initial state, and atomically deducts the user's
-- balance. Raises exception if insufficient funds. Returns JSONB:
--   { "order": {...}, "order_state": {...} }
-- =============================================================================
DROP FUNCTION create_exchange_order(UUID, UUID, exchange_order_type, BIGINT, BIGINT, BIGINT, exchange_order_status);

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
    v_balance_ok BOOLEAN;
    v_order exchange_orders;
    v_order_state exchange_order_states;
BEGIN
    v_balance_ok := try_deduct_balance(p_user_id, p_quantity);
    IF NOT v_balance_ok THEN
        RAISE EXCEPTION 'Insufficient balance for user_id: %', p_user_id;
    END IF;

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

-- +goose StatementEnd
-- +goose StatementBegin

-- =============================================================================
-- create_exchange_match (replaces original)
-- Creates a match and its fills, then marks any fully-filled orders.
-- Returns JSONB:
--   { "match": {...}, "fills": [{...}, ...] }
-- p_fills is a JSONB array: [{"order_id": uuid, "matched_portion": bigint}, ...]
-- Raises exception on FK violation (invalid order_id).
-- =============================================================================
DROP FUNCTION create_exchange_match(UUID, BIGINT, JSONB);

CREATE FUNCTION create_exchange_match(
    p_aggressor_order_id UUID,
    p_matched_quantity BIGINT,
    p_fills JSONB
) RETURNS JSONB AS $$
DECLARE
    v_match exchange_matches;
    v_fills_result JSONB;
    v_fill_order_id UUID;
    v_order exchange_orders;
    v_total_filled BIGINT;
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

    -- Check each filled order for full completion
    FOR v_fill_order_id IN
        SELECT DISTINCT (fill->>'order_id')::UUID
        FROM jsonb_array_elements(p_fills) AS fill
    LOOP
        SELECT * INTO v_order FROM exchange_orders WHERE id = v_fill_order_id;
        v_total_filled := calculate_total_filled_quantity(v_fill_order_id);

        IF v_total_filled >= v_order.quantity THEN
            PERFORM update_exchange_order_status(v_fill_order_id, 'fully_filled');
        END IF;
    END LOOP;

    RETURN jsonb_build_object(
        'match', row_to_json(v_match)::jsonb,
        'fills', v_fills_result
    );
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin

-- =============================================================================
-- cancel_exchange_order_impl
-- Atomically cancels an exchange order: updates the order status, calculates
-- the total filled quantity, and refunds the remaining (unfilled) balance to
-- the user.
-- =============================================================================
CREATE FUNCTION cancel_exchange_order_impl(
    p_order_id UUID,
    p_cancel_reason exchange_order_status
) RETURNS void AS $$
DECLARE
    v_order exchange_orders;
    v_total_filled BIGINT;
    v_refund_amount BIGINT;
BEGIN
    SELECT * INTO v_order FROM exchange_orders WHERE id = p_order_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Order not found: %', p_order_id;
    END IF;

    PERFORM update_exchange_order_status(p_order_id, p_cancel_reason);

    v_total_filled := calculate_total_filled_quantity(p_order_id);
    v_refund_amount := v_order.quantity - v_total_filled;

    IF v_refund_amount > 0 THEN
        PERFORM add_balance(v_order.user_id, v_refund_amount);
    END IF;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin

-- =============================================================================
-- cancel_exchange_order_due_to_exchange
-- Cancels an exchange order due to exchange-initiated cancellation (e.g.,
-- matcher shutdown). Delegates to cancel_exchange_order_impl.
-- =============================================================================
CREATE FUNCTION cancel_exchange_order_due_to_exchange(
    p_order_id UUID
) RETURNS void AS $$
BEGIN
    PERFORM cancel_exchange_order_impl(p_order_id, 'cancelled_by_exchange');
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd
-- +goose StatementBegin

-- =============================================================================
-- cancel_exchange_order_due_to_user
-- Cancels an exchange order due to user-initiated cancellation. Delegates to
-- cancel_exchange_order_impl.
-- =============================================================================
CREATE FUNCTION cancel_exchange_order_due_to_user(
    p_order_id UUID
) RETURNS void AS $$
BEGIN
    PERFORM cancel_exchange_order_impl(p_order_id, 'cancelled_by_user');
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP FUNCTION cancel_exchange_order_due_to_user(UUID);
DROP FUNCTION cancel_exchange_order_due_to_exchange(UUID);
DROP FUNCTION cancel_exchange_order_impl(UUID, exchange_order_status);

-- Restore original create_exchange_match without fully-filled logic
DROP FUNCTION create_exchange_match(UUID, BIGINT, JSONB);

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

-- Restore original create_exchange_order without balance deduction
DROP FUNCTION create_exchange_order(UUID, UUID, exchange_order_type, BIGINT, BIGINT, BIGINT, exchange_order_status);

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

DROP FUNCTION calculate_total_filled_quantity(UUID);
DROP FUNCTION create_exchange_cancel_request(UUID);

DROP TABLE exchange_cancel_requests;

-- +goose StatementEnd
