-- +goose Up
CREATE TABLE individual_statuses (
    id SERIAL PRIMARY KEY,
    individual_id BIGINT NOT NULL,
    status VARCHAR(255) NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS individual_statuses;
