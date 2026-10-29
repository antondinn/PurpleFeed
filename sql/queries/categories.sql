-- name: CreateCategory :one
INSERT INTO categories (id, created_at, updated_at, name, is_active)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCategory :one
SELECT *
FROM categories
WHERE name = ?;

-- name: GetCategories :many
SELECT *
FROM categories
ORDER BY is_active DESC, name ASC;

-- name: GetActiveCategories :many
SELECT *
FROM categories
WHERE is_active = 1
ORDER BY name ASC;

-- name: ActivateCategory :exec
UPDATE categories
SET is_active = 1,
    updated_at = CURRENT_TIMESTAMP
WHERE name = ?;

-- name: DeactivateCategory :exec
UPDATE categories
SET is_active = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE name = ?;

-- name: DeleteCategory :exec
DELETE FROM categories
WHERE name = ?;
