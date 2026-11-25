-- name: InsertActivityLog :exec
INSERT INTO activity_logs
  (id, company_id, actor_id, actor_name, action, subject_type, subject_id, description, properties, created_at, store_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
