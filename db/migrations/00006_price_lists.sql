-- +goose Up
CREATE TABLE price_lists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies(id),
    code text NOT NULL,
    name text NOT NULL,
    effective_from date,
    effective_until date,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company_id, code),
    CHECK (effective_until IS NULL OR effective_from IS NULL OR effective_until >= effective_from)
);

CREATE TABLE price_list_items (
    price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id),
    min_qty numeric(18,4) NOT NULL CHECK (min_qty > 0),
    price numeric(18,2) NOT NULL CHECK (price >= 0),
    PRIMARY KEY (price_list_id, product_id, min_qty)
);

ALTER TABLE stores ADD COLUMN sales_price_list_id uuid REFERENCES price_lists(id);

-- +goose Down
ALTER TABLE stores DROP COLUMN sales_price_list_id;
DROP TABLE price_list_items, price_lists;
