-- +goose Up
-- +goose StatementBegin
ALTER TABLE teams ALTER COLUMN vendor_unified_id SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE teams ALTER COLUMN vendor_unified_id DROP NOT NULL;
-- +goose StatementEnd
