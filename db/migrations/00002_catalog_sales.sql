-- +goose Up
ALTER TABLE companies
    ADD COLUMN ppn_enabled boolean NOT NULL DEFAULT true,
    ADD COLUMN ppn_rate numeric(7,4) NOT NULL DEFAULT 11,
    ADD COLUMN ppn_inclusive boolean NOT NULL DEFAULT true,
    ADD COLUMN allow_negative_stock boolean NOT NULL DEFAULT false;

CREATE TABLE units (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    code text NOT NULL,
    name text NOT NULL,
    UNIQUE (company_id, code)
);

CREATE TABLE product_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    code text NOT NULL,
    name text NOT NULL,
    UNIQUE (company_id, code)
);

CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    product_group_id uuid REFERENCES product_groups(id),
    unit_id uuid NOT NULL REFERENCES units(id),
    sku text NOT NULL,
    barcode text,
    name text NOT NULL,
    base_price numeric(18,2) NOT NULL CHECK (base_price >= 0),
    track_stock boolean NOT NULL DEFAULT true,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, sku)
);
CREATE UNIQUE INDEX products_barcode_uq ON products (company_id, barcode) WHERE barcode IS NOT NULL;

-- FR-A6-02: SKU is immutable.
-- +goose StatementBegin
CREATE FUNCTION forbid_sku_change() RETURNS trigger AS $$
BEGIN
    IF NEW.sku IS DISTINCT FROM OLD.sku THEN
        RAISE EXCEPTION 'SKU tidak dapat diubah' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER products_sku_immutable BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION forbid_sku_change();

CREATE TABLE payment_types (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    code text NOT NULL,
    name text NOT NULL,
    is_cash boolean NOT NULL DEFAULT false,
    is_active boolean NOT NULL DEFAULT true,
    UNIQUE (company_id, code)
);

CREATE TABLE numbering_configs (
    company_id uuid NOT NULL REFERENCES companies(id),
    doc_type text NOT NULL,
    prefix text NOT NULL,
    padding int NOT NULL DEFAULT 6,
    last_period text NOT NULL DEFAULT '',
    last_number bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (company_id, doc_type)
);

CREATE TABLE store_stocks (
    store_id uuid NOT NULL REFERENCES stores(id),
    product_id uuid NOT NULL REFERENCES products(id),
    on_hand numeric(18,4) NOT NULL DEFAULT 0,
    PRIMARY KEY (store_id, product_id)
);

-- FR-A9-04: append-only ledger.
CREATE TABLE stock_ledger (
    id bigserial PRIMARY KEY,
    store_id uuid NOT NULL REFERENCES stores(id),
    product_id uuid NOT NULL REFERENCES products(id),
    type text NOT NULL CHECK (type IN ('opening','sale','sale_return','adjustment','transfer_out','transfer_in')),
    qty numeric(18,4) NOT NULL,
    ref_type text NOT NULL DEFAULT '',
    ref_id text NOT NULL DEFAULT '',
    note text NOT NULL DEFAULT '',
    created_by uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stock_ledger_product_idx ON stock_ledger (store_id, product_id, id);
-- +goose StatementBegin
CREATE FUNCTION forbid_ledger_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'ledger bersifat append-only' USING ERRCODE = 'integrity_constraint_violation';
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER stock_ledger_append_only BEFORE UPDATE OR DELETE ON stock_ledger
    FOR EACH ROW EXECUTE FUNCTION forbid_ledger_change();

CREATE TABLE sales (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    store_id uuid NOT NULL REFERENCES stores(id),
    number text NOT NULL,
    client_uuid uuid NOT NULL UNIQUE,
    business_date date NOT NULL,
    cashier_id uuid NOT NULL REFERENCES users(id),
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed','void')),
    subtotal numeric(18,2) NOT NULL,
    discount_total numeric(18,2) NOT NULL DEFAULT 0,
    tax_total numeric(18,2) NOT NULL,
    total numeric(18,2) NOT NULL,
    paid_total numeric(18,2) NOT NULL,
    change_total numeric(18,2) NOT NULL DEFAULT 0,
    sold_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, number)
);
CREATE INDEX sales_store_date_idx ON sales (store_id, business_date);

CREATE TABLE sale_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id uuid NOT NULL REFERENCES sales(id),
    product_id uuid NOT NULL REFERENCES products(id),
    sku_snapshot text NOT NULL,
    name_snapshot text NOT NULL,
    qty numeric(18,4) NOT NULL CHECK (qty > 0),
    unit_price numeric(18,2) NOT NULL,
    discount numeric(18,2) NOT NULL DEFAULT 0,
    tax_rate numeric(7,4) NOT NULL DEFAULT 0,
    tax_amount numeric(18,6) NOT NULL DEFAULT 0,
    line_total numeric(18,2) NOT NULL
);
CREATE INDEX sale_lines_sale_idx ON sale_lines (sale_id);

CREATE TABLE sale_payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id uuid NOT NULL REFERENCES sales(id),
    payment_type_id uuid NOT NULL REFERENCES payment_types(id),
    amount numeric(18,2) NOT NULL CHECK (amount > 0),
    status text NOT NULL DEFAULT 'confirmed' CHECK (status IN ('pending','confirmed','void'))
);

-- +goose Down
DROP TABLE sale_payments, sale_lines, sales, stock_ledger, store_stocks, numbering_configs, payment_types, products, product_groups, units;
DROP FUNCTION forbid_ledger_change(), forbid_sku_change();
ALTER TABLE companies DROP COLUMN ppn_enabled, DROP COLUMN ppn_rate, DROP COLUMN ppn_inclusive, DROP COLUMN allow_negative_stock;
