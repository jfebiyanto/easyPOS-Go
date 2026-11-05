-- +goose Up
ALTER TABLE companies ADD COLUMN return_window_days int NOT NULL DEFAULT 30 CHECK (return_window_days >= 0);

CREATE TABLE member_levels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    name text NOT NULL,
    min_points int NOT NULL,
    earn_amount_per_point numeric(18,2) NOT NULL CHECK (earn_amount_per_point > 0),
    UNIQUE (company_id, name)
);
-- Defaults from PRD FR-A2-08: Silver 0 / Gold 100 / Platinum 500; Rp 10.000 / 8.000 / 6.000 per point.
INSERT INTO member_levels (company_id, name, min_points, earn_amount_per_point)
SELECT c.id, v.name, v.min_points, v.earn FROM companies c,
  (VALUES ('Silver',0,10000),('Gold',100,8000),('Platinum',500,6000)) AS v(name,min_points,earn);

CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    code text NOT NULL,
    name text NOT NULL,
    phone text,
    email text,
    address text,
    is_member boolean NOT NULL DEFAULT true,
    member_level_id uuid REFERENCES member_levels(id),
    points_balance int NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    joined_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, code)
);
CREATE UNIQUE INDEX customers_phone_uq ON customers (company_id, phone) WHERE phone IS NOT NULL;

CREATE TABLE loyalty_ledger (
    id bigserial PRIMARY KEY,
    customer_id uuid NOT NULL REFERENCES customers(id),
    type text NOT NULL CHECK (type IN ('earn','adjust','reverse')),
    points int NOT NULL,
    balance_after int NOT NULL,
    ref_type text NOT NULL DEFAULT '',
    ref_id text NOT NULL DEFAULT '',
    note text NOT NULL DEFAULT '',
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX loyalty_ledger_customer_idx ON loyalty_ledger (customer_id, id);
CREATE TRIGGER loyalty_ledger_append_only BEFORE UPDATE OR DELETE ON loyalty_ledger
    FOR EACH ROW EXECUTE FUNCTION forbid_ledger_change();

ALTER TABLE sales
    ADD COLUMN customer_id uuid REFERENCES customers(id),
    ADD COLUMN points_earned int NOT NULL DEFAULT 0;

CREATE TABLE sales_returns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    store_id uuid NOT NULL REFERENCES stores(id),
    number text NOT NULL,
    client_uuid uuid NOT NULL UNIQUE,
    original_sale_id uuid NOT NULL REFERENCES sales(id),
    customer_id uuid REFERENCES customers(id),
    reason text NOT NULL,
    total numeric(18,2) NOT NULL CHECK (total >= 0),
    refund_payment_type_id uuid NOT NULL REFERENCES payment_types(id),
    points_deducted int NOT NULL DEFAULT 0,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, number)
);
CREATE INDEX sales_returns_sale_idx ON sales_returns (original_sale_id);

CREATE TABLE sales_return_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sales_return_id uuid NOT NULL REFERENCES sales_returns(id),
    sale_line_id uuid NOT NULL REFERENCES sale_lines(id),
    product_id uuid NOT NULL REFERENCES products(id),
    qty numeric(18,4) NOT NULL CHECK (qty > 0),
    line_total numeric(18,2) NOT NULL
);
CREATE INDEX sales_return_lines_line_idx ON sales_return_lines (sale_line_id);

-- +goose Down
DROP TABLE sales_return_lines, sales_returns;
ALTER TABLE sales DROP COLUMN customer_id, DROP COLUMN points_earned;
DROP TABLE loyalty_ledger, customers, member_levels;
ALTER TABLE companies DROP COLUMN return_window_days;
