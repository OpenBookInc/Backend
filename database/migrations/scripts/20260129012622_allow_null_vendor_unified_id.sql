-- +goose Up
-- +goose StatementBegin
ALTER TABLE individuals ALTER COLUMN vendor_unified_id DROP NOT NULL;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE teams ALTER COLUMN vendor_unified_id DROP NOT NULL;
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE individuals SET vendor_unified_id = NULL WHERE vendor_unified_id = '';
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE teams SET vendor_unified_id = NULL WHERE vendor_unified_id = '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE individuals SET vendor_unified_id = '' WHERE vendor_unified_id IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE teams SET vendor_unified_id = '' WHERE vendor_unified_id IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE individuals ALTER COLUMN vendor_unified_id SET NOT NULL;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE teams ALTER COLUMN vendor_unified_id SET NOT NULL;
-- +goose StatementEnd
