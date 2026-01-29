-- +goose Up
-- +goose StatementBegin
ALTER TABLE individuals ADD COLUMN vendor_unified_id VARCHAR(255) NOT NULL DEFAULT '';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE individuals ALTER COLUMN vendor_unified_id DROP DEFAULT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE teams ADD COLUMN vendor_unified_id VARCHAR(255) NOT NULL DEFAULT '';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE teams ALTER COLUMN vendor_unified_id DROP DEFAULT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE individuals DROP COLUMN vendor_unified_id;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE teams DROP COLUMN vendor_unified_id;
-- +goose StatementEnd
