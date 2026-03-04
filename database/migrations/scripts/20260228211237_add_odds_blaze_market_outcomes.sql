-- +goose Up
-- +goose StatementBegin
CREATE TYPE market_outcome_result AS ENUM ('Win', 'Loss', 'Void');

CREATE TABLE odds_blaze_market_outcomes (
    odds_blaze_id VARCHAR(255) NOT NULL UNIQUE,
    result market_outcome_result NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE odds_blaze_market_outcomes;
DROP TYPE market_outcome_result;
-- +goose StatementEnd
