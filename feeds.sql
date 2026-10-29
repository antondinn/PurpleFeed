-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, category_id, last_fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetFeedByURL :one
SELECT *
FROM feeds
WHERE url = ?;

-- name: GetFeeds :many
SELECT
    feeds.id,
    feeds.created_at,
    feeds.updated_at,
    feeds.name,
    feeds.url,
    feeds.category_id,
    feeds.last_fetched_at,
    categories.name AS category_name,
    categories.is_active AS category_is_active
FROM feeds
JOIN categories ON feeds.category_id = categories.id
ORDER BY categories.name ASC, feeds.name ASC;

-- name: GetFeedByID :one
SELECT *
FROM feeds
WHERE id = ?;

-- name: GetFeedsByCategoryName :many
SELECT
    feeds.id,
    feeds.created_at,
    feeds.updated_at,
    feeds.name,
    feeds.url,
    feeds.category_id,
    feeds.last_fetched_at,
    categories.name AS category_name,
    categories.is_active AS category_is_active
FROM feeds
JOIN categories ON feeds.category_id = categories.id
WHERE categories.name = ?
ORDER BY feeds.name ASC;

-- name: GetFeedsForActiveCategories :many
SELECT
    feeds.id,
    feeds.created_at,
    feeds.updated_at,
    feeds.name,
    feeds.url,
    feeds.category_id,
    feeds.last_fetched_at,
    categories.name AS category_name
FROM feeds
JOIN categories ON feeds.category_id = categories.id
WHERE categories.is_active = 1
ORDER BY
    categories.name ASC,
    (feeds.last_fetched_at IS NOT NULL) ASC,
    feeds.last_fetched_at ASC,
    feeds.created_at ASC;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET last_fetched_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteFeedById :exec
DELETE FROM feeds
WHERE id = ?;