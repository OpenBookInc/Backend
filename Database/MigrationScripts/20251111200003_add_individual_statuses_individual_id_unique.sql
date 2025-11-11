-- +goose Up
ALTER TABLE individual_statuses ADD CONSTRAINT individual_statuses_individual_id_unique UNIQUE (individual_id);

-- +goose Down
ALTER TABLE individual_statuses DROP CONSTRAINT individual_statuses_individual_id_unique;
