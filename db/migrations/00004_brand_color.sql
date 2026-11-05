-- +goose Up
ALTER TABLE companies
    ADD COLUMN brand_color text NOT NULL DEFAULT '#1E3A5F'
    CHECK (brand_color ~ '^#[0-9A-Fa-f]{6}$');

-- +goose Down
ALTER TABLE companies DROP COLUMN brand_color;
