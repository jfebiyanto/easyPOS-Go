-- Fase 0: tabel untuk fitur baru (UoM Group, price per unit, alternate UPC,
-- batch/serial, Konfigurasi Sistem) dan kolom sesi token.

-- +goose Up

-- Sesi token: gerai aktif dan jenis klien (pengganti session Laravel).
ALTER TABLE personal_access_tokens
    ADD COLUMN active_store_id uuid NULL REFERENCES stores(id) ON DELETE SET NULL,
    ADD COLUMN client_kind varchar(20) NOT NULL DEFAULT 'api'
        CHECK (client_kind IN ('api', 'backoffice', 'pos'));

-- Konfigurasi Sistem per perusahaan.
CREATE TABLE system_settings (
    company_id uuid NOT NULL REFERENCES company_details(id) ON DELETE CASCADE,
    key        varchar(80) NOT NULL,
    value      jsonb NOT NULL,
    updated_by uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamp(0) without time zone NULL,
    PRIMARY KEY (company_id, key)
);

-- UoM Group: satuan dasar + konversi (pola SAP B1 OUGP/UGP1).
CREATE TABLE uom_groups (
    id            uuid PRIMARY KEY,
    company_id    uuid NOT NULL REFERENCES company_details(id) ON DELETE CASCADE,
    code          varchar(20) NOT NULL,
    name          varchar(100) NOT NULL,
    base_unit_id  uuid NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    sap_abs_entry integer NULL,
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamp(0) without time zone NULL,
    updated_at    timestamp(0) without time zone NULL,
    UNIQUE (company_id, code)
);

CREATE TABLE uom_group_lines (
    id           uuid PRIMARY KEY,
    uom_group_id uuid NOT NULL REFERENCES uom_groups(id) ON DELETE CASCADE,
    unit_id      uuid NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    alt_qty      numeric(15,3) NOT NULL DEFAULT 1 CHECK (alt_qty > 0),
    base_qty     numeric(15,3) NOT NULL CHECK (base_qty > 0),
    created_at   timestamp(0) without time zone NULL,
    updated_at   timestamp(0) without time zone NULL,
    UNIQUE (uom_group_id, unit_id)
);

-- Produk: mode batch/serial (dari SAP), UoM Group, satuan jual default.
-- products.unit_id tetap = satuan dasar (stok selalu dalam satuan ini).
ALTER TABLE products
    ADD COLUMN manage_by varchar(10) NOT NULL DEFAULT 'none'
        CHECK (manage_by IN ('none', 'batch', 'serial')),
    ADD COLUMN uom_group_id uuid NULL REFERENCES uom_groups(id) ON DELETE RESTRICT,
    ADD COLUMN sales_unit_id uuid NULL REFERENCES units(id) ON DELETE RESTRICT;

-- Price per unit: harga khusus per satuan; NULL = harga satuan dasar.
ALTER TABLE price_list_items
    ADD COLUMN unit_id uuid NULL REFERENCES units(id) ON DELETE RESTRICT;
ALTER TABLE price_list_items DROP CONSTRAINT IF EXISTS pli_unique;
DROP INDEX IF EXISTS pli_unique;
CREATE UNIQUE INDEX pli_unique_unit ON price_list_items
    (price_list_id, product_id, COALESCE(unit_id, '00000000-0000-0000-0000-000000000000'::uuid), min_qty);

-- Alternate UPC: barcode tambahan, selalu terikat ke satu satuan.
CREATE TABLE product_barcodes (
    id         uuid PRIMARY KEY,
    company_id uuid NOT NULL REFERENCES company_details(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    unit_id    uuid NOT NULL REFERENCES units(id) ON DELETE RESTRICT,
    code       varchar(60) NOT NULL,
    source     varchar(10) NOT NULL DEFAULT 'local' CHECK (source IN ('local', 'sap')),
    note       varchar(255) NULL,
    created_at timestamp(0) without time zone NULL,
    updated_at timestamp(0) without time zone NULL,
    UNIQUE (company_id, code)
);
CREATE INDEX product_barcodes_product_idx ON product_barcodes (product_id);

-- Batch.
CREATE TABLE product_batches (
    id                 uuid PRIMARY KEY,
    company_id         uuid NOT NULL REFERENCES company_details(id) ON DELETE CASCADE,
    product_id         uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    batch_number       varchar(36) NOT NULL,
    expiry_date        date NULL,
    manufacturing_date date NULL,
    sap_abs_entry      integer NULL,
    created_at         timestamp(0) without time zone NULL,
    updated_at         timestamp(0) without time zone NULL,
    UNIQUE (company_id, product_id, batch_number)
);
CREATE INDEX product_batches_fefo_idx ON product_batches (product_id, expiry_date);

CREATE TABLE warehouse_batch_stocks (
    warehouse_id uuid NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
    batch_id     uuid NOT NULL REFERENCES product_batches(id) ON DELETE CASCADE,
    qty          numeric(15,3) NOT NULL DEFAULT 0,
    updated_at   timestamp(0) without time zone NULL,
    PRIMARY KEY (warehouse_id, batch_id)
);

-- Serial.
CREATE TABLE product_serials (
    id            uuid PRIMARY KEY,
    company_id    uuid NOT NULL REFERENCES company_details(id) ON DELETE CASCADE,
    product_id    uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    serial_number varchar(36) NOT NULL,
    warehouse_id  uuid NULL REFERENCES warehouses(id) ON DELETE SET NULL,
    status        varchar(12) NOT NULL DEFAULT 'available'
        CHECK (status IN ('available', 'sold', 'returned', 'in_transit', 'adjusted_out')),
    sap_abs_entry integer NULL,
    created_at    timestamp(0) without time zone NULL,
    updated_at    timestamp(0) without time zone NULL,
    UNIQUE (company_id, product_id, serial_number)
);
CREATE INDEX product_serials_lookup_idx ON product_serials (company_id, serial_number);

-- Alokasi batch/serial ke baris dokumen (jual, retur, penyesuaian, transfer).
CREATE TABLE document_line_lots (
    id               uuid PRIMARY KEY,
    company_id       uuid NOT NULL REFERENCES company_details(id) ON DELETE CASCADE,
    document_type    varchar(40) NOT NULL,
    document_line_id uuid NOT NULL,
    batch_id         uuid NULL REFERENCES product_batches(id) ON DELETE RESTRICT,
    serial_id        uuid NULL REFERENCES product_serials(id) ON DELETE RESTRICT,
    base_qty         numeric(15,3) NOT NULL CHECK (base_qty > 0),
    created_at       timestamp(0) without time zone NULL,
    CHECK ((batch_id IS NULL) <> (serial_id IS NULL))
);
CREATE INDEX document_line_lots_line_idx ON document_line_lots (document_type, document_line_id);

-- +goose Down
DROP TABLE IF EXISTS document_line_lots;
DROP TABLE IF EXISTS product_serials;
DROP TABLE IF EXISTS warehouse_batch_stocks;
DROP TABLE IF EXISTS product_batches;
DROP TABLE IF EXISTS product_barcodes;
DROP INDEX IF EXISTS pli_unique_unit;
ALTER TABLE price_list_items DROP COLUMN IF EXISTS unit_id;
ALTER TABLE price_list_items ADD CONSTRAINT pli_unique UNIQUE (price_list_id, product_id, min_qty);
ALTER TABLE products DROP COLUMN IF EXISTS sales_unit_id, DROP COLUMN IF EXISTS uom_group_id, DROP COLUMN IF EXISTS manage_by;
DROP TABLE IF EXISTS uom_group_lines;
DROP TABLE IF EXISTS uom_groups;
DROP TABLE IF EXISTS system_settings;
ALTER TABLE personal_access_tokens DROP COLUMN IF EXISTS client_kind, DROP COLUMN IF EXISTS active_store_id;
