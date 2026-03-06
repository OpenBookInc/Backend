-- +goose Up
-- +goose StatementBegin

-- =============================================================================
-- balances
-- =============================================================================
CREATE TABLE balances (
    user_id UUID PRIMARY KEY REFERENCES auth.users(id),
    total_balance BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

-- =============================================================================
-- Insert balance rows for all existing auth.users
-- =============================================================================
INSERT INTO balances (user_id, total_balance)
SELECT id, 0 FROM auth.users;

-- =============================================================================
-- Trigger to auto-create balance row when a new auth.user is created
-- =============================================================================
CREATE FUNCTION public.handle_new_user_balance()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO public.balances (user_id, total_balance)
    VALUES (NEW.id, 0);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_auth_user_created_balance
    AFTER INSERT ON auth.users
    FOR EACH ROW
    EXECUTE FUNCTION public.handle_new_user_balance();

-- =============================================================================
-- Enable row-level security
-- =============================================================================
ALTER TABLE public.balances ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER on_auth_user_created_balance ON auth.users;
DROP FUNCTION public.handle_new_user_balance();
DROP TABLE balances;
-- +goose StatementEnd
