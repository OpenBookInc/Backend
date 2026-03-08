-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- update_exchange_order_status
-- Updates the status of an existing exchange order. Returns the updated
-- exchange_order_states row.
-- Raises exception if no order_state record exists for the given order_id.
-- =============================================================================
CREATE FUNCTION update_exchange_order_status(
    p_order_id UUID,
    p_status exchange_order_status
) RETURNS exchange_order_states AS $$
DECLARE
    v_row exchange_order_states;
BEGIN
    UPDATE exchange_order_states
    SET status = p_status,
        updated_at = NOW()
    WHERE order_id = p_order_id
    RETURNING * INTO v_row;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Order state not found for order_id: %', p_order_id;
    END IF;

    RETURN v_row;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION update_exchange_order_status(UUID, exchange_order_status);
-- +goose StatementEnd
