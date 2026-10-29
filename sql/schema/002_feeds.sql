-- +goose Up
CREATE TABLE feeds (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL UNIQUE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    last_fetched_at TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS feeds;
