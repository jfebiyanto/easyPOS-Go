-- Kunci sesi karena idle: sesi tetap ada, tetapi API ditolak sampai dibuka dengan password.

-- +goose Up
ALTER TABLE personal_access_tokens ADD COLUMN locked_at timestamp(0) without time zone NULL;

-- +goose Down
ALTER TABLE personal_access_tokens DROP COLUMN locked_at;
