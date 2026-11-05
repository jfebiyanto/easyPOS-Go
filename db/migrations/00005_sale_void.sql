-- +goose Up
ALTER TABLE sales
    ADD COLUMN void_reason text,
    ADD COLUMN voided_by uuid REFERENCES users(id),
    ADD COLUMN voided_at timestamptz,
    ADD CONSTRAINT sales_void_info CHECK (status <> 'void' OR (void_reason IS NOT NULL AND voided_by IS NOT NULL AND voided_at IS NOT NULL));

ALTER TABLE stock_ledger DROP CONSTRAINT stock_ledger_type_check;
ALTER TABLE stock_ledger ADD CONSTRAINT stock_ledger_type_check
    CHECK (type IN ('opening','sale','sale_return','sale_void','adjustment','transfer_out','transfer_in'));

ALTER TABLE loyalty_ledger DROP CONSTRAINT loyalty_ledger_type_check;
ALTER TABLE loyalty_ledger ADD CONSTRAINT loyalty_ledger_type_check
    CHECK (type IN ('earn','adjust','reverse','void'));

-- +goose Down
ALTER TABLE loyalty_ledger DROP CONSTRAINT loyalty_ledger_type_check;
ALTER TABLE loyalty_ledger ADD CONSTRAINT loyalty_ledger_type_check CHECK (type IN ('earn','adjust','reverse'));
ALTER TABLE stock_ledger DROP CONSTRAINT stock_ledger_type_check;
ALTER TABLE stock_ledger ADD CONSTRAINT stock_ledger_type_check
    CHECK (type IN ('opening','sale','sale_return','adjustment','transfer_out','transfer_in'));
ALTER TABLE sales DROP CONSTRAINT sales_void_info, DROP COLUMN void_reason, DROP COLUMN voided_by, DROP COLUMN voided_at;
