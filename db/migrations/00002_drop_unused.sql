-- Fase 0: buang fitur dan tabel yang tidak dipakai easyPOS-Go.
-- Loyalty dihapus (keputusan Januar 25 Nov 2026). Tabel bawaan Laravel tidak dipakai Go.

-- +goose Up
ALTER TABLE customers DROP COLUMN IF EXISTS member_level_id;
ALTER TABLE customers DROP COLUMN IF EXISTS points_balance;
ALTER TABLE sales DROP COLUMN IF EXISTS points_earned;

DROP TABLE IF EXISTS loyalty_ledger;
DROP TABLE IF EXISTS loyalty_config;
DROP TABLE IF EXISTS member_levels;

DROP TABLE IF EXISTS cache_locks;
DROP TABLE IF EXISTS cache;
DROP TABLE IF EXISTS job_batches;
DROP TABLE IF EXISTS failed_jobs;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS migrations;

-- +goose Down
SELECT 'drop tidak bisa dibatalkan';
