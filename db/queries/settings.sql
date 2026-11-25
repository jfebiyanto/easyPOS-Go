-- name: ListSettings :many
SELECT key, value FROM system_settings WHERE company_id = $1;

-- name: UpsertSetting :exec
INSERT INTO system_settings (company_id, key, value, updated_by, updated_at) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (company_id, key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at;
