-- name: ListNumberings :many
SELECT * FROM numbering_configs WHERE company_id = $1 ORDER BY transaction_type;

-- name: GetNumberingForUpdate :one
SELECT * FROM numbering_configs WHERE company_id = $1 AND transaction_type = $2 FOR UPDATE;

-- name: InsertNumberingIfMissing :exec
INSERT INTO numbering_configs (id, company_id, transaction_type, prefix, padding, separator, reset_period, last_number, created_at, updated_at)
VALUES ($1, $2, $3, $4, 4, '/', 'monthly', 0, $5, $5)
ON CONFLICT (company_id, transaction_type) DO NOTHING;

-- name: UpdateNumberingCounter :exec
UPDATE numbering_configs SET last_number = $2, last_period = $3, updated_at = $4 WHERE id = $1;

-- name: UpdateNumberingConfig :exec
UPDATE numbering_configs SET prefix = $3, padding = $4, separator = $5, reset_period = $6, updated_at = $7
WHERE company_id = $1 AND transaction_type = $2;
