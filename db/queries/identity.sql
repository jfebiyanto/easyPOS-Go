-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetStore :one
SELECT id, company_id, code, name, is_active FROM stores WHERE id = $1;

-- name: FirstStoreByCompany :one
SELECT id FROM stores WHERE company_id = $1 AND is_active ORDER BY code LIMIT 1;

-- name: ListStoresByCompany :many
SELECT id, code, name FROM stores WHERE company_id = $1 AND is_active ORDER BY code;

-- name: GetCompany :one
SELECT id, name FROM company_details WHERE id = $1;

-- name: InsertToken :one
INSERT INTO personal_access_tokens
  (tokenable_type, tokenable_id, name, token, abilities, client_kind, active_store_id, created_at, updated_at)
VALUES ('App\Models\User', $1, $2, $3, '["*"]', $4, $5, $6, $6)
RETURNING id;

-- name: GetToken :one
SELECT id, tokenable_id, token, expires_at, last_used_at, client_kind, active_store_id
FROM personal_access_tokens WHERE id = $1 AND tokenable_type = 'App\Models\User';

-- name: DeleteToken :exec
DELETE FROM personal_access_tokens WHERE id = $1;

-- name: TouchToken :exec
UPDATE personal_access_tokens SET last_used_at = $2
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < $2::timestamp - interval '1 minute');

-- name: SetTokenStore :exec
UPDATE personal_access_tokens SET active_store_id = $2, updated_at = $3 WHERE id = $1;

-- name: InsertCompany :exec
INSERT INTO company_details (id, name, created_at, updated_at) VALUES ($1, $2, $3, $3);

-- name: InsertStore :exec
INSERT INTO stores (id, company_id, code, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5);

-- name: InsertUser :exec
INSERT INTO users (id, company_id, store_id, name, username, role, password, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8);
