-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- get_active_exchange_state
-- Returns the active exchange state as a JSONB object with two arrays:
--   - orders: all exchange orders that are still active (not cancelled or fully
--     filled), each with remaining quantity and lineup/slate info
--   - cancel_requests: all cancel requests for active orders
-- Both arrays are sorted by created_at ascending for deterministic recovery.
-- =============================================================================
CREATE FUNCTION get_active_exchange_state()
RETURNS JSONB AS $$
BEGIN
    RETURN jsonb_build_object(
        'orders', COALESCE((
            SELECT jsonb_agg(row_data ORDER BY row_data->>'created_at')
            FROM (
                SELECT jsonb_build_object(
                    'order_id', o.id,
                    'user_id', o.user_id,
                    'order_type', o.order_type,
                    'portion', o.portion,
                    'quantity', o.quantity,
                    'remaining_quantity', o.quantity - calculate_total_filled_quantity(o.id),
                    'slate_id', l.slate_id,
                    'total_units', s.total_units,
                    'lineup_index', l.lineup_index,
                    'legs', l.legs,
                    'created_at', o.created_at
                ) AS row_data
                FROM exchange_orders o
                JOIN exchange_order_states os ON os.order_id = o.id
                JOIN exchange_lineups l ON l.id = o.lineup_id
                JOIN exchange_slates s ON s.id = l.slate_id
                WHERE os.status IN ('received_by_backend', 'submitted_to_exchange', 'resting_on_exchange')
            ) sub
        ), '[]'::jsonb),
        'cancel_requests', COALESCE((
            SELECT jsonb_agg(row_data ORDER BY row_data->>'created_at')
            FROM (
                SELECT jsonb_build_object(
                    'order_id', cr.order_id,
                    'created_at', cr.created_at
                ) AS row_data
                FROM exchange_cancel_requests cr
                JOIN exchange_order_states os ON os.order_id = cr.order_id
                WHERE os.status IN ('received_by_backend', 'submitted_to_exchange', 'resting_on_exchange')
            ) sub
        ), '[]'::jsonb)
    );
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION get_active_exchange_state();
-- +goose StatementEnd
