-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- add_balance
-- Adds the given amount to the user's balance. Returns the updated balances row.
-- Raises exception if p_amount is not positive or if balance record not found.
-- =============================================================================
CREATE FUNCTION add_balance(
    p_user_id UUID,
    p_amount BIGINT
) RETURNS balances AS $$
DECLARE
    v_row balances;
BEGIN
    IF p_amount <= 0 THEN
        RAISE EXCEPTION 'p_amount must be positive, got: %', p_amount;
    END IF;

    UPDATE balances
    SET total_balance = total_balance + p_amount,
        updated_at = NOW()
    WHERE user_id = p_user_id
    RETURNING * INTO v_row;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Balance record not found for user_id: %', p_user_id;
    END IF;

    RETURN v_row;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION add_balance(UUID, BIGINT);
-- +goose StatementEnd
