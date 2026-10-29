-- +goose Up
CREATE TABLE categories (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    name TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT 1
);

-- +goose Down
DROP TABLE IF EXISTS categories;
