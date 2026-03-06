-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- New enum types
-- =============================================================================
CREATE TYPE exchange_order_type AS ENUM (
    'limit',
    'market'
);

CREATE TYPE exchange_order_status AS ENUM (
    'received_by_backend',
    'submitted_to_exchange',
    'resting_on_exchange',
    'cancelled_by_exchange',
    'cancelled_by_user',
    'fully_filled'
);

-- =============================================================================
-- exchange_slates
-- =============================================================================
CREATE TABLE exchange_slates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    market_type market_entity_type NOT NULL,
    market_ids UUID[] NOT NULL,
    total_units BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

-- =============================================================================
-- exchange_lineups
-- =============================================================================
CREATE TABLE exchange_lineups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slate_id UUID NOT NULL REFERENCES exchange_slates(id),
    lineup_index INTEGER NOT NULL,
    legs JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    CONSTRAINT exchange_lineups_slate_lineup_index_unique UNIQUE (slate_id, lineup_index)
);

CREATE INDEX idx_exchange_lineups_slate_id ON exchange_lineups (slate_id);

-- =============================================================================
-- exchange_orders
-- =============================================================================
CREATE TABLE exchange_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lineup_id UUID NOT NULL REFERENCES exchange_lineups(id),
    user_id UUID NOT NULL REFERENCES auth.users(id),
    order_type exchange_order_type NOT NULL,
    portion BIGINT NOT NULL,
    quantity BIGINT NOT NULL,
    client_order_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_exchange_orders_user_id ON exchange_orders (user_id);

-- =============================================================================
-- exchange_order_states
-- =============================================================================
CREATE TABLE exchange_order_states (
    order_id UUID PRIMARY KEY REFERENCES exchange_orders(id),
    status exchange_order_status NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

-- =============================================================================
-- exchange_matches
-- =============================================================================
CREATE TABLE exchange_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggressor_order_id UUID NOT NULL REFERENCES exchange_orders(id),
    matched_quantity BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

-- =============================================================================
-- exchange_fills
-- =============================================================================
CREATE TABLE exchange_fills (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID NOT NULL REFERENCES exchange_matches(id),
    order_id UUID NOT NULL REFERENCES exchange_orders(id),
    matched_portion BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_exchange_fills_match_id ON exchange_fills (match_id);
CREATE INDEX idx_exchange_fills_order_id ON exchange_fills (order_id);

-- =============================================================================
-- Enable row-level security
-- =============================================================================
ALTER TABLE public.exchange_slates ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.exchange_lineups ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.exchange_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.exchange_order_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.exchange_matches ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.exchange_fills ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE exchange_fills;
DROP TABLE exchange_matches;
DROP TABLE exchange_order_states;
DROP TABLE exchange_orders;
DROP TABLE exchange_lineups;
DROP TABLE exchange_slates;
DROP TYPE exchange_order_status;
DROP TYPE exchange_order_type;
-- +goose StatementEnd
