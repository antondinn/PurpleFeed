-- name: CreatePost :exec
INSERT OR IGNORE INTO posts (
    id, created_at, updated_at, title, url, description, published_at, feed_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetPostByID :one
SELECT *
FROM posts
WHERE id = ?;

-- name: GetPostsForCategory :many
SELECT
    posts.id,
    posts.created_at,
    posts.updated_at,
    posts.title,
    posts.url,
    posts.description,
    posts.published_at,
    posts.feed_id,
    feeds.name AS feed_name,
    categories.name AS category_name
FROM posts
JOIN feeds ON posts.feed_id = feeds.id
JOIN categories ON feeds.category_id = categories.id
WHERE categories.name = ?
ORDER BY posts.published_at DESC, posts.created_at DESC
LIMIT ?;

-- name: GetPostsForActiveCategories :many
SELECT
    posts.id,
    posts.created_at,
    posts.updated_at,
    posts.title,
    posts.url,
    posts.description,
    posts.published_at,
    posts.feed_id,
    feeds.name AS feed_name,
    feeds.url AS feed_url,
    categories.id AS category_id,
    categories.name AS category_name
FROM posts
JOIN feeds ON posts.feed_id = feeds.id
JOIN categories ON feeds.category_id = categories.id
WHERE categories.is_active = 1
ORDER BY categories.name ASC, posts.published_at DESC, posts.created_at DESC;
