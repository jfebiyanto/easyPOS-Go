-- +goose Up
CREATE TABLE companies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    timezone text NOT NULL DEFAULT 'Asia/Jakarta',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE stores (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    code text NOT NULL,
    name text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, code)
);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    store_id uuid REFERENCES stores(id),
    name text NOT NULL,
    username text NOT NULL,
    email text,
    phone text,
    role text NOT NULL CHECK (role IN ('owner', 'store_manager', 'cashier')),
    is_active boolean NOT NULL DEFAULT true,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, username),
    -- FR-A1-08: cashier and store manager must belong to exactly one store.
    CONSTRAINT users_store_required CHECK (role = 'owner' OR store_id IS NOT NULL)
);

CREATE TABLE activity_logs (
    id bigserial PRIMARY KEY,
    company_id uuid REFERENCES companies(id),
    actor_id uuid,
    actor_name text NOT NULL DEFAULT '',
    action text NOT NULL,
    subject_type text NOT NULL DEFAULT '',
    subject_id text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    properties jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX activity_logs_created_idx ON activity_logs (created_at DESC);

-- Session store for alexedwards/scs pgxstore.
CREATE TABLE sessions (
    token text PRIMARY KEY,
    data bytea NOT NULL,
    expiry timestamptz NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- +goose Down
DROP TABLE sessions;
DROP TABLE activity_logs;
DROP TABLE users;
DROP TABLE stores;
DROP TABLE companies;
