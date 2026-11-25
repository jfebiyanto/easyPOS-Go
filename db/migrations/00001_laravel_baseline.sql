-- Skema baseline dari migration Laravel pada commit 0d78191 (pg_dump --schema-only).

-- +goose Up
--
-- PostgreSQL database dump
--


-- Dumped from database version 16.16 (Debian 16.16-1.pgdg13+1)
-- Dumped by pg_dump version 16.16 (Debian 16.16-1.pgdg13+1)




--
-- Name: activity_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.activity_logs (
    id uuid NOT NULL,
    company_id uuid,
    actor_id uuid,
    actor_name character varying(255) NOT NULL,
    action character varying(60) NOT NULL,
    subject_type character varying(60),
    subject_id uuid,
    description text NOT NULL,
    properties json,
    created_at timestamp(0) without time zone NOT NULL,
    store_id uuid
);


--
-- Name: brands; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.brands (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: cache; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cache (
    key character varying(255) NOT NULL,
    value text NOT NULL,
    expiration bigint NOT NULL
);


--
-- Name: cache_locks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cache_locks (
    key character varying(255) NOT NULL,
    owner character varying(255) NOT NULL,
    expiration bigint NOT NULL
);


--
-- Name: cashier_shift_denominations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cashier_shift_denominations (
    id uuid NOT NULL,
    cashier_shift_id uuid NOT NULL,
    denomination_value numeric(15,2) NOT NULL,
    count integer DEFAULT 0 NOT NULL,
    subtotal numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: cashier_shifts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cashier_shifts (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    user_id uuid NOT NULL,
    business_date date NOT NULL,
    started_at timestamp(0) without time zone NOT NULL,
    ended_at timestamp(0) without time zone,
    ended_reason character varying(20),
    opened_store boolean DEFAULT false NOT NULL,
    opening_float numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    opening_float_expected numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    opening_float_variance numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    opening_float_remark text,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: company_details; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.company_details (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    address text,
    logo_path character varying(255),
    npwp_number character varying(30),
    phone character varying(30),
    email character varying(150),
    ppn_enabled boolean DEFAULT false NOT NULL,
    ppn_rate numeric(5,2) DEFAULT '11'::numeric NOT NULL,
    ppn_inclusive boolean DEFAULT false NOT NULL,
    base_currency_code character varying(10) DEFAULT 'IDR'::character varying NOT NULL,
    timezone character varying(40) DEFAULT 'Asia/Jakarta'::character varying NOT NULL,
    allow_negative_stock boolean DEFAULT false NOT NULL,
    return_window_days smallint DEFAULT '30'::smallint,
    return_require_receipt boolean DEFAULT true NOT NULL,
    sku_config json,
    payment_providers text,
    sap_enabled boolean DEFAULT false NOT NULL,
    sap_verify_ssl boolean DEFAULT true NOT NULL,
    sap_endpoint_url character varying(255),
    sap_db_name character varying(60),
    sap_username character varying(60),
    sap_password text,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    sap_sync_interval_minutes integer DEFAULT 30 NOT NULL,
    sap_auto_sync boolean DEFAULT true NOT NULL,
    sap_last_synced_at timestamp(0) without time zone,
    sap_initial_imported_at timestamp(0) without time zone,
    tax_group_id uuid,
    sap_stock_synced_at timestamp(0) without time zone,
    sap_stock_sync_error text,
    sap_sync_watermarks json,
    sap_stock_reconciled_at timestamp(0) without time zone,
    promotion_hierarchy json
);


--
-- Name: credit_cards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.credit_cards (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(20) NOT NULL,
    name character varying(255) NOT NULL,
    gl_account character varying(25),
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: currencies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.currencies (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(3) NOT NULL,
    symbol character varying(10) NOT NULL,
    has_rounding boolean DEFAULT false NOT NULL,
    decimal_places smallint DEFAULT '2'::smallint NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: currency_denominations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.currency_denominations (
    id uuid NOT NULL,
    currency_id uuid NOT NULL,
    value numeric(15,2) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: customer_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customer_groups (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    source character varying(10) DEFAULT 'manual'::character varying NOT NULL,
    is_default boolean DEFAULT false NOT NULL
);


--
-- Name: customers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.customers (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30),
    name character varying(255) NOT NULL,
    phone character varying(30),
    email character varying(150),
    address text,
    is_member boolean DEFAULT true NOT NULL,
    member_level_id uuid,
    points_balance numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    joined_at timestamp(0) without time zone,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    customer_group_id uuid,
    source character varying(10) DEFAULT 'manual'::character varying NOT NULL,
    sap_sync_status character varying(10),
    sap_sync_error text,
    sap_synced_at timestamp(0) without time zone,
    is_retail boolean DEFAULT false NOT NULL
);


--
-- Name: deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.deliveries (
    id uuid NOT NULL,
    sale_id uuid NOT NULL,
    recipient_name character varying(255) NOT NULL,
    phone character varying(30),
    address text NOT NULL,
    shipping_cost numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    status character varying(255) DEFAULT 'pending'::character varying NOT NULL,
    note text,
    shipped_at timestamp(0) without time zone,
    delivered_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    CONSTRAINT deliveries_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'shipped'::character varying, 'delivered'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: failed_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.failed_jobs (
    id bigint NOT NULL,
    uuid character varying(255) NOT NULL,
    connection character varying(255) NOT NULL,
    queue character varying(255) NOT NULL,
    payload text NOT NULL,
    exception text NOT NULL,
    failed_at timestamp(0) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: failed_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.failed_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: failed_jobs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.failed_jobs_id_seq OWNED BY public.failed_jobs.id;


--
-- Name: job_batches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.job_batches (
    id character varying(255) NOT NULL,
    name character varying(255) NOT NULL,
    total_jobs integer NOT NULL,
    pending_jobs integer NOT NULL,
    failed_jobs integer NOT NULL,
    failed_job_ids text NOT NULL,
    options text,
    cancelled_at integer,
    created_at integer NOT NULL,
    finished_at integer
);


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobs (
    id bigint NOT NULL,
    queue character varying(255) NOT NULL,
    payload text NOT NULL,
    attempts smallint NOT NULL,
    reserved_at integer,
    available_at integer NOT NULL,
    created_at integer NOT NULL
);


--
-- Name: jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: jobs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.jobs_id_seq OWNED BY public.jobs.id;


--
-- Name: loyalty_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.loyalty_config (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    min_spend numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    rounding character varying(255) DEFAULT 'down'::character varying NOT NULL,
    expiry_days integer,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    CONSTRAINT loyalty_config_rounding_check CHECK (((rounding)::text = ANY ((ARRAY['none'::character varying, 'down'::character varying, 'up'::character varying, 'nearest'::character varying])::text[])))
);


--
-- Name: loyalty_ledger; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.loyalty_ledger (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    type character varying(255) NOT NULL,
    points numeric(15,2) NOT NULL,
    balance_after numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    ref_type character varying(40),
    ref_id uuid,
    note text,
    created_by uuid,
    created_at timestamp(0) without time zone NOT NULL,
    CONSTRAINT loyalty_ledger_type_check CHECK (((type)::text = ANY ((ARRAY['earn'::character varying, 'return'::character varying, 'adjust'::character varying, 'expire'::character varying, 'redeem'::character varying])::text[])))
);


--
-- Name: member_levels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.member_levels (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    name character varying(60) NOT NULL,
    min_points numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    earn_amount_per_point numeric(15,2) DEFAULT '10000'::numeric NOT NULL,
    "order" integer DEFAULT 0 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: migrations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.migrations_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: numbering_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.numbering_configs (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    transaction_type character varying(60) NOT NULL,
    prefix character varying(20) DEFAULT ''::character varying NOT NULL,
    padding smallint DEFAULT '4'::smallint NOT NULL,
    separator character varying(1) DEFAULT '/'::character varying NOT NULL,
    reset_period character varying(255) DEFAULT 'monthly'::character varying NOT NULL,
    last_period character varying(20),
    last_number bigint DEFAULT '0'::bigint NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    CONSTRAINT numbering_configs_reset_period_check CHECK (((reset_period)::text = ANY ((ARRAY['daily'::character varying, 'monthly'::character varying, 'yearly'::character varying, 'never'::character varying])::text[])))
);


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    username character varying(255) NOT NULL,
    token character varying(255) NOT NULL,
    created_at timestamp(0) without time zone
);


--
-- Name: payment_type_store; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_type_store (
    payment_type_id uuid NOT NULL,
    store_id uuid NOT NULL
);


--
-- Name: payment_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_types (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    description text,
    order_number integer DEFAULT 1 NOT NULL,
    count_type character varying(255) DEFAULT 'summary'::character varying NOT NULL,
    is_offline_allowed boolean DEFAULT true NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    credit_card_id uuid,
    kind character varying(10) DEFAULT 'custom'::character varying NOT NULL,
    is_qr_payment boolean DEFAULT false NOT NULL,
    qr_image_path character varying(255),
    CONSTRAINT payment_types_count_type_check CHECK (((count_type)::text = ANY ((ARRAY['summary'::character varying, 'detail'::character varying])::text[])))
);


--
-- Name: personal_access_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.personal_access_tokens (
    id bigint NOT NULL,
    tokenable_type character varying(255) NOT NULL,
    tokenable_id uuid NOT NULL,
    name text NOT NULL,
    token character varying(64) NOT NULL,
    abilities text,
    last_used_at timestamp(0) without time zone,
    expires_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: personal_access_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.personal_access_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: personal_access_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.personal_access_tokens_id_seq OWNED BY public.personal_access_tokens.id;


--
-- Name: pos_active_carts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pos_active_carts (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    cashier_id uuid NOT NULL,
    payload json NOT NULL,
    item_count integer DEFAULT 0 NOT NULL,
    gross_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: pos_cancellations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pos_cancellations (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    type character varying(20) NOT NULL,
    store_id uuid,
    store_label character varying(255),
    cashier_id uuid,
    cashier_name character varying(255),
    customer_id uuid,
    customer_label character varying(255),
    sale_id uuid,
    sale_number character varying(60),
    reason text,
    item_count integer DEFAULT 0 NOT NULL,
    total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    items json,
    occurred_at timestamp(0) without time zone NOT NULL,
    ip_address character varying(45),
    user_agent text,
    browser character varying(120),
    pos_session_id uuid,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: pos_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pos_sessions (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    user_id uuid NOT NULL,
    store_id uuid,
    session_hash character varying(64) NOT NULL,
    ip_address character varying(45),
    user_agent text,
    browser character varying(120),
    started_at timestamp(0) without time zone NOT NULL,
    last_seen_at timestamp(0) without time zone NOT NULL,
    ended_at timestamp(0) without time zone,
    ended_reason character varying(20),
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: price_list_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.price_list_items (
    id uuid NOT NULL,
    price_list_id uuid NOT NULL,
    product_id uuid NOT NULL,
    price numeric(15,2) NOT NULL,
    min_qty integer DEFAULT 1 NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: price_lists; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.price_lists (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    effective_from date,
    effective_until date,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: product_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_groups (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: products; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.products (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    product_group_id uuid NOT NULL,
    brand_id uuid,
    unit_id uuid NOT NULL,
    sku character varying(40) NOT NULL,
    barcode character varying(60),
    name character varying(255) NOT NULL,
    description text,
    base_price numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    cost_price numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    track_stock boolean DEFAULT true NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    image_path character varying(255)
);


--
-- Name: promotions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.promotions (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    type character varying(30) DEFAULT 'discount_percent'::character varying NOT NULL,
    value numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    product_id uuid,
    min_qty integer DEFAULT 1 NOT NULL,
    starts_at timestamp(0) without time zone,
    ends_at timestamp(0) without time zone,
    is_active boolean DEFAULT true NOT NULL,
    description text,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    buy_qty integer,
    get_qty integer,
    get_product_id uuid,
    max_applications integer,
    source character varying(10) DEFAULT 'manual'::character varying NOT NULL,
    sap_ref character varying(100),
    brand_id uuid,
    product_group_id uuid,
    customer_scope character varying(20) DEFAULT 'all'::character varying NOT NULL,
    customer_ref character varying(50),
    price_list_code character varying(30),
    tiers json,
    group_key character varying(100),
    group_relation character varying(30)
);


--
-- Name: sale_drafts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sale_drafts (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    client_uuid uuid NOT NULL,
    cashier_id uuid,
    device_label character varying(255),
    payload json NOT NULL,
    status character varying(20) DEFAULT 'held'::character varying NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: sale_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sale_lines (
    id uuid NOT NULL,
    sale_id uuid NOT NULL,
    product_id uuid,
    sku_snapshot character varying(40),
    name_snapshot character varying(255) NOT NULL,
    unit_price numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    qty numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    discount numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    tax_amount numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    line_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    promotion_id uuid,
    promotion_name_snapshot character varying(255),
    promotion_discount numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    tax_group_code character varying(30),
    tax_rate numeric(7,4)
);


--
-- Name: sale_payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sale_payments (
    id uuid NOT NULL,
    sale_id uuid NOT NULL,
    payment_type_id uuid,
    amount numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    status character varying(255) DEFAULT 'confirmed'::character varying NOT NULL,
    provider character varying(40),
    provider_ref character varying(120),
    confirmed_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    approval_code character varying(50),
    CONSTRAINT sale_payments_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'confirmed'::character varying, 'void'::character varying])::text[])))
);


--
-- Name: sales; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    number character varying(60),
    client_uuid uuid,
    origin character varying(255) DEFAULT 'online'::character varying NOT NULL,
    sync_status character varying(255) DEFAULT 'synced'::character varying NOT NULL,
    business_date date NOT NULL,
    type character varying(255) DEFAULT 'sale'::character varying NOT NULL,
    cashier_id uuid,
    status character varying(255) DEFAULT 'completed'::character varying NOT NULL,
    stock_review_status character varying(255) DEFAULT 'none'::character varying NOT NULL,
    review_reason text,
    resolved_by uuid,
    resolved_at timestamp(0) without time zone,
    subtotal numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    discount_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    tax_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    shipping_cost numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    paid_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    change_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    currency_code character varying(10) DEFAULT 'IDR'::character varying NOT NULL,
    sold_at timestamp(0) without time zone,
    voided_at timestamp(0) without time zone,
    void_reason text,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    customer_id uuid,
    points_earned numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    sap_post_status character varying(10),
    sap_post_error text,
    sap_invoice_obj_type integer,
    sap_invoice_doc_num character varying(20),
    sap_invoice_doc_entry integer,
    sap_payment_obj_type integer,
    sap_payment_doc_num character varying(20),
    sap_payment_doc_entry integer,
    sap_posted_at timestamp(0) without time zone,
    CONSTRAINT sales_origin_check CHECK (((origin)::text = ANY ((ARRAY['online'::character varying, 'offline'::character varying])::text[]))),
    CONSTRAINT sales_status_check CHECK (((status)::text = ANY ((ARRAY['completed'::character varying, 'void'::character varying])::text[]))),
    CONSTRAINT sales_stock_review_status_check CHECK (((stock_review_status)::text = ANY ((ARRAY['none'::character varying, 'pending'::character varying, 'resolved'::character varying])::text[]))),
    CONSTRAINT sales_sync_status_check CHECK (((sync_status)::text = ANY ((ARRAY['pending'::character varying, 'synced'::character varying])::text[]))),
    CONSTRAINT sales_type_check CHECK (((type)::text = ANY ((ARRAY['sale'::character varying, 'return'::character varying])::text[])))
);


--
-- Name: sales_return_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_return_lines (
    id uuid NOT NULL,
    sales_return_id uuid NOT NULL,
    sale_line_id uuid,
    product_id uuid,
    qty numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    unit_price numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    line_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: sales_returns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sales_returns (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    number character varying(60),
    original_sale_id uuid,
    reason text,
    total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    refund_method character varying(30),
    receipt_verified boolean DEFAULT true NOT NULL,
    created_by uuid,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    sap_post_status character varying(10),
    sap_post_error text,
    sap_credit_memo_obj_type integer,
    sap_credit_memo_doc_num character varying(20),
    sap_credit_memo_doc_entry integer,
    sap_refund_obj_type integer,
    sap_refund_doc_num character varying(20),
    sap_refund_doc_entry integer,
    sap_posted_at timestamp(0) without time zone
);


--
-- Name: sap_sync_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sap_sync_runs (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    mode character varying(10) NOT NULL,
    trigger character varying(10) NOT NULL,
    status character varying(10) NOT NULL,
    summary json,
    error text,
    started_at timestamp(0) without time zone,
    finished_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sessions (
    id character varying(255) NOT NULL,
    user_id uuid,
    ip_address character varying(45),
    user_agent character varying(255),
    payload text NOT NULL,
    last_activity integer NOT NULL
);


--
-- Name: stock_adjustment_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_adjustment_lines (
    id uuid NOT NULL,
    stock_adjustment_id uuid NOT NULL,
    product_id uuid NOT NULL,
    qty_before numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    qty_after numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    delta numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_adjustments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_adjustments (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    number character varying(60),
    date date NOT NULL,
    reason text,
    status character varying(255) DEFAULT 'draft'::character varying NOT NULL,
    created_by uuid,
    posted_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    sap_post_status character varying(10),
    sap_post_error text,
    sap_receipt_obj_type integer,
    sap_receipt_doc_num character varying(20),
    sap_receipt_doc_entry integer,
    sap_issue_obj_type integer,
    sap_issue_doc_num character varying(20),
    sap_issue_doc_entry integer,
    sap_posted_at timestamp(0) without time zone,
    CONSTRAINT stock_adjustments_status_check CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'posted'::character varying])::text[])))
);


--
-- Name: stock_ledger; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_ledger (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    product_id uuid NOT NULL,
    type character varying(255) NOT NULL,
    qty numeric(15,3) NOT NULL,
    unit_cost numeric(15,2),
    ref_type character varying(40),
    ref_id uuid,
    note text,
    created_by uuid,
    created_at timestamp(0) without time zone NOT NULL,
    CONSTRAINT stock_ledger_type_check CHECK (((type)::text = ANY ((ARRAY['opening'::character varying, 'sale'::character varying, 'sale_return'::character varying, 'adjustment'::character varying, 'transfer_out'::character varying, 'transfer_in'::character varying])::text[])))
);


--
-- Name: stock_transfer_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_lines (
    id uuid NOT NULL,
    stock_transfer_id uuid NOT NULL,
    product_id uuid NOT NULL,
    qty_requested numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    qty_shipped numeric(15,3),
    qty_received numeric(15,3),
    variance_reason text,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_transfer_receipt_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_receipt_lines (
    id uuid NOT NULL,
    receipt_id uuid NOT NULL,
    shipment_line_id uuid NOT NULL,
    qty numeric(15,3) NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_transfer_receipts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_receipts (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    number character varying(30),
    store_id uuid NOT NULL,
    note text,
    received_at timestamp(0) without time zone,
    created_by uuid,
    sap_post_status character varying(10),
    sap_post_error text,
    sap_doc_num character varying(20),
    sap_doc_entry integer,
    sap_posted_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_transfer_return_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_return_lines (
    id uuid NOT NULL,
    return_id uuid NOT NULL,
    shipment_line_id uuid NOT NULL,
    qty numeric(15,3) NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_transfer_returns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_returns (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    number character varying(30),
    store_id uuid NOT NULL,
    note text,
    returned_at timestamp(0) without time zone,
    created_by uuid,
    sap_post_status character varying(10),
    sap_post_error text,
    sap_doc_num character varying(20),
    sap_doc_entry integer,
    sap_posted_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_transfer_shipment_lines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_shipment_lines (
    id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    product_id uuid NOT NULL,
    qty numeric(15,3) NOT NULL,
    qty_received numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    qty_returned numeric(15,3) DEFAULT '0'::numeric NOT NULL
);


--
-- Name: stock_transfer_shipments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfer_shipments (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    number character varying(30),
    from_store_id uuid NOT NULL,
    to_store_id uuid NOT NULL,
    from_warehouse_id uuid,
    transit_warehouse_id uuid,
    to_warehouse_id uuid,
    status character varying(10) DEFAULT 'draft'::character varying NOT NULL,
    note text,
    posted_at timestamp(0) without time zone,
    created_by uuid,
    sap_post_status character varying(10),
    sap_post_error text,
    sap_doc_num character varying(20),
    sap_doc_entry integer,
    sap_posted_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stock_transfers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_transfers (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    number character varying(60),
    from_store_id uuid NOT NULL,
    to_store_id uuid NOT NULL,
    status character varying(255) DEFAULT 'draft'::character varying NOT NULL,
    note text,
    created_by uuid,
    shipped_at timestamp(0) without time zone,
    received_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    CONSTRAINT stock_transfers_status_check CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'requested'::character varying, 'shipped'::character varying, 'received'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: store_closure_denominations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_closure_denominations (
    id uuid NOT NULL,
    store_closure_id uuid NOT NULL,
    denomination_value numeric(15,2) NOT NULL,
    count integer DEFAULT 0 NOT NULL,
    subtotal numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: store_closure_payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_closure_payments (
    id uuid NOT NULL,
    store_closure_id uuid NOT NULL,
    payment_type_id uuid,
    payment_code character varying(30) NOT NULL,
    kind character varying(10) NOT NULL,
    expected_amount numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    confirmed_at timestamp(0) without time zone,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: store_closures; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_closures (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid NOT NULL,
    business_date date NOT NULL,
    opened_at timestamp(0) without time zone,
    closed_at timestamp(0) without time zone,
    sales_total numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    cash_expected numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    cash_counted numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    cash_variance numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    closed_by uuid,
    note text,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    reopened_at timestamp(0) without time zone,
    reopened_by uuid,
    reopen_reason text,
    opened_by uuid
);


--
-- Name: store_price_lists; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.store_price_lists (
    store_id uuid NOT NULL,
    price_list_id uuid NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: stores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stores (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(20) NOT NULL,
    name character varying(255) NOT NULL,
    address text,
    pic_name character varying(255),
    pic_phone character varying(30),
    ppn_enabled boolean DEFAULT false NOT NULL,
    ppn_rate numeric(5,2) DEFAULT '11'::numeric NOT NULL,
    currency character varying(10) DEFAULT 'IDR'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    purchase_price_list_id uuid,
    warehouse_id uuid,
    default_customer_id uuid,
    opening_float numeric(15,2) DEFAULT '0'::numeric NOT NULL
);


--
-- Name: tax_groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tax_groups (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    rate numeric(7,4) DEFAULT '0'::numeric NOT NULL,
    effective_from date,
    source character varying(10) DEFAULT 'manual'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: units; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.units (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(20) NOT NULL,
    name character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    store_id uuid,
    name character varying(255) NOT NULL,
    username character varying(60) NOT NULL,
    email character varying(255),
    phone character varying(30),
    role character varying(255) DEFAULT 'cashier'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    password character varying(255) NOT NULL,
    remember_token character varying(100),
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    CONSTRAINT users_role_check CHECK (((role)::text = ANY ((ARRAY['owner'::character varying, 'store_manager'::character varying, 'cashier'::character varying])::text[])))
);


--
-- Name: warehouse_stock_ledgers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.warehouse_stock_ledgers (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    product_id uuid NOT NULL,
    type character varying(30) NOT NULL,
    qty numeric(15,3) NOT NULL,
    balance_after numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    unit_cost numeric(15,2),
    ref_type character varying(40),
    ref_id uuid,
    note text,
    created_by uuid,
    created_at timestamp(0) without time zone NOT NULL,
    sap_trans_seq bigint,
    sap_trans_type integer,
    sap_doc_entry integer,
    sap_doc_num character varying(20),
    sap_doc_line integer,
    doc_date date
);


--
-- Name: warehouse_stocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.warehouse_stocks (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    warehouse_id uuid NOT NULL,
    product_id uuid NOT NULL,
    on_hand numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    committed numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    on_order numeric(15,3) DEFAULT '0'::numeric NOT NULL,
    avg_price numeric(15,2) DEFAULT '0'::numeric NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    sap_on_hand numeric(15,3),
    sap_checked_at timestamp(0) without time zone
);


--
-- Name: warehouses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.warehouses (
    id uuid NOT NULL,
    company_id uuid NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(255) NOT NULL,
    profit_center character varying(50),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp(0) without time zone,
    updated_at timestamp(0) without time zone,
    is_retail boolean DEFAULT true NOT NULL,
    cash_account character varying(25),
    qris_account character varying(25),
    edc_account character varying(25),
    transit_warehouse_code character varying(30),
    is_transit boolean DEFAULT false NOT NULL
);


--
-- Name: failed_jobs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.failed_jobs ALTER COLUMN id SET DEFAULT nextval('public.failed_jobs_id_seq'::regclass);


--
-- Name: jobs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs ALTER COLUMN id SET DEFAULT nextval('public.jobs_id_seq'::regclass);


--
-- Name: personal_access_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens ALTER COLUMN id SET DEFAULT nextval('public.personal_access_tokens_id_seq'::regclass);


--
-- Name: activity_logs activity_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_logs
    ADD CONSTRAINT activity_logs_pkey PRIMARY KEY (id);


--
-- Name: brands brands_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT brands_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: brands brands_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT brands_pkey PRIMARY KEY (id);


--
-- Name: cache_locks cache_locks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cache_locks
    ADD CONSTRAINT cache_locks_pkey PRIMARY KEY (key);


--
-- Name: cache cache_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cache
    ADD CONSTRAINT cache_pkey PRIMARY KEY (key);


--
-- Name: cashier_shift_denominations cashier_shift_denominations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cashier_shift_denominations
    ADD CONSTRAINT cashier_shift_denominations_pkey PRIMARY KEY (id);


--
-- Name: cashier_shifts cashier_shifts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cashier_shifts
    ADD CONSTRAINT cashier_shifts_pkey PRIMARY KEY (id);


--
-- Name: company_details company_details_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.company_details
    ADD CONSTRAINT company_details_pkey PRIMARY KEY (id);


--
-- Name: credit_cards credit_cards_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_cards
    ADD CONSTRAINT credit_cards_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: credit_cards credit_cards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_cards
    ADD CONSTRAINT credit_cards_pkey PRIMARY KEY (id);


--
-- Name: currencies currencies_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currencies
    ADD CONSTRAINT currencies_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: currencies currencies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currencies
    ADD CONSTRAINT currencies_pkey PRIMARY KEY (id);


--
-- Name: currency_denominations currency_denominations_currency_id_value_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currency_denominations
    ADD CONSTRAINT currency_denominations_currency_id_value_unique UNIQUE (currency_id, value);


--
-- Name: currency_denominations currency_denominations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currency_denominations
    ADD CONSTRAINT currency_denominations_pkey PRIMARY KEY (id);


--
-- Name: customer_groups customer_groups_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_groups
    ADD CONSTRAINT customer_groups_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: customer_groups customer_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_groups
    ADD CONSTRAINT customer_groups_pkey PRIMARY KEY (id);


--
-- Name: customers customers_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: customers customers_company_id_phone_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_company_id_phone_unique UNIQUE (company_id, phone);


--
-- Name: customers customers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);


--
-- Name: deliveries deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.deliveries
    ADD CONSTRAINT deliveries_pkey PRIMARY KEY (id);


--
-- Name: failed_jobs failed_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.failed_jobs
    ADD CONSTRAINT failed_jobs_pkey PRIMARY KEY (id);


--
-- Name: failed_jobs failed_jobs_uuid_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.failed_jobs
    ADD CONSTRAINT failed_jobs_uuid_unique UNIQUE (uuid);


--
-- Name: job_batches job_batches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_batches
    ADD CONSTRAINT job_batches_pkey PRIMARY KEY (id);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: loyalty_config loyalty_config_company_id_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_config
    ADD CONSTRAINT loyalty_config_company_id_unique UNIQUE (company_id);


--
-- Name: loyalty_config loyalty_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_config
    ADD CONSTRAINT loyalty_config_pkey PRIMARY KEY (id);


--
-- Name: loyalty_ledger loyalty_ledger_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_ledger
    ADD CONSTRAINT loyalty_ledger_pkey PRIMARY KEY (id);


--
-- Name: member_levels member_levels_company_id_name_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_levels
    ADD CONSTRAINT member_levels_company_id_name_unique UNIQUE (company_id, name);


--
-- Name: member_levels member_levels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_levels
    ADD CONSTRAINT member_levels_pkey PRIMARY KEY (id);


--
-- Name: numbering_configs numbering_configs_company_id_transaction_type_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.numbering_configs
    ADD CONSTRAINT numbering_configs_company_id_transaction_type_unique UNIQUE (company_id, transaction_type);


--
-- Name: numbering_configs numbering_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.numbering_configs
    ADD CONSTRAINT numbering_configs_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (username);


--
-- Name: payment_type_store payment_type_store_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_type_store
    ADD CONSTRAINT payment_type_store_pkey PRIMARY KEY (payment_type_id, store_id);


--
-- Name: payment_types payment_types_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_types
    ADD CONSTRAINT payment_types_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: payment_types payment_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_types
    ADD CONSTRAINT payment_types_pkey PRIMARY KEY (id);


--
-- Name: personal_access_tokens personal_access_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT personal_access_tokens_pkey PRIMARY KEY (id);


--
-- Name: personal_access_tokens personal_access_tokens_token_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.personal_access_tokens
    ADD CONSTRAINT personal_access_tokens_token_unique UNIQUE (token);


--
-- Name: price_list_items pli_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_list_items
    ADD CONSTRAINT pli_unique UNIQUE (price_list_id, product_id, min_qty);


--
-- Name: pos_active_carts pos_active_carts_company_id_cashier_id_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_active_carts
    ADD CONSTRAINT pos_active_carts_company_id_cashier_id_unique UNIQUE (company_id, cashier_id);


--
-- Name: pos_active_carts pos_active_carts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_active_carts
    ADD CONSTRAINT pos_active_carts_pkey PRIMARY KEY (id);


--
-- Name: pos_cancellations pos_cancellations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_pkey PRIMARY KEY (id);


--
-- Name: pos_sessions pos_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_sessions
    ADD CONSTRAINT pos_sessions_pkey PRIMARY KEY (id);


--
-- Name: price_list_items price_list_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_list_items
    ADD CONSTRAINT price_list_items_pkey PRIMARY KEY (id);


--
-- Name: price_lists price_lists_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_lists
    ADD CONSTRAINT price_lists_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: price_lists price_lists_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_lists
    ADD CONSTRAINT price_lists_pkey PRIMARY KEY (id);


--
-- Name: product_groups product_groups_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_groups
    ADD CONSTRAINT product_groups_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: product_groups product_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_groups
    ADD CONSTRAINT product_groups_pkey PRIMARY KEY (id);


--
-- Name: products products_company_id_barcode_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_company_id_barcode_unique UNIQUE (company_id, barcode);


--
-- Name: products products_company_id_sku_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_company_id_sku_unique UNIQUE (company_id, sku);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);


--
-- Name: promotions promotions_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: promotions promotions_company_id_sap_ref_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_company_id_sap_ref_unique UNIQUE (company_id, sap_ref);


--
-- Name: promotions promotions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_pkey PRIMARY KEY (id);


--
-- Name: sale_drafts sale_drafts_client_uuid_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_drafts
    ADD CONSTRAINT sale_drafts_client_uuid_unique UNIQUE (client_uuid);


--
-- Name: sale_drafts sale_drafts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_drafts
    ADD CONSTRAINT sale_drafts_pkey PRIMARY KEY (id);


--
-- Name: sale_lines sale_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_lines
    ADD CONSTRAINT sale_lines_pkey PRIMARY KEY (id);


--
-- Name: sale_payments sale_payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_payments
    ADD CONSTRAINT sale_payments_pkey PRIMARY KEY (id);


--
-- Name: sales sales_client_uuid_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_client_uuid_unique UNIQUE (client_uuid);


--
-- Name: sales sales_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_pkey PRIMARY KEY (id);


--
-- Name: sales_return_lines sales_return_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_return_lines
    ADD CONSTRAINT sales_return_lines_pkey PRIMARY KEY (id);


--
-- Name: sales_returns sales_returns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_returns
    ADD CONSTRAINT sales_returns_pkey PRIMARY KEY (id);


--
-- Name: sap_sync_runs sap_sync_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sap_sync_runs
    ADD CONSTRAINT sap_sync_runs_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: stock_adjustment_lines stock_adjustment_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustment_lines
    ADD CONSTRAINT stock_adjustment_lines_pkey PRIMARY KEY (id);


--
-- Name: stock_adjustments stock_adjustments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustments
    ADD CONSTRAINT stock_adjustments_pkey PRIMARY KEY (id);


--
-- Name: stock_ledger stock_ledger_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_ledger
    ADD CONSTRAINT stock_ledger_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_lines stock_transfer_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_lines
    ADD CONSTRAINT stock_transfer_lines_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_receipt_lines stock_transfer_receipt_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipt_lines
    ADD CONSTRAINT stock_transfer_receipt_lines_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_receipts stock_transfer_receipts_company_id_number_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipts
    ADD CONSTRAINT stock_transfer_receipts_company_id_number_unique UNIQUE (company_id, number);


--
-- Name: stock_transfer_receipts stock_transfer_receipts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipts
    ADD CONSTRAINT stock_transfer_receipts_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_return_lines stock_transfer_return_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_return_lines
    ADD CONSTRAINT stock_transfer_return_lines_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_returns stock_transfer_returns_company_id_number_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_returns
    ADD CONSTRAINT stock_transfer_returns_company_id_number_unique UNIQUE (company_id, number);


--
-- Name: stock_transfer_returns stock_transfer_returns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_returns
    ADD CONSTRAINT stock_transfer_returns_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_shipment_lines stock_transfer_shipment_lines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipment_lines
    ADD CONSTRAINT stock_transfer_shipment_lines_pkey PRIMARY KEY (id);


--
-- Name: stock_transfer_shipments stock_transfer_shipments_company_id_number_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_company_id_number_unique UNIQUE (company_id, number);


--
-- Name: stock_transfer_shipments stock_transfer_shipments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_pkey PRIMARY KEY (id);


--
-- Name: stock_transfers stock_transfers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfers
    ADD CONSTRAINT stock_transfers_pkey PRIMARY KEY (id);


--
-- Name: store_closure_denominations store_closure_denominations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closure_denominations
    ADD CONSTRAINT store_closure_denominations_pkey PRIMARY KEY (id);


--
-- Name: store_closure_payments store_closure_payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closure_payments
    ADD CONSTRAINT store_closure_payments_pkey PRIMARY KEY (id);


--
-- Name: store_closures store_closures_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_pkey PRIMARY KEY (id);


--
-- Name: store_closures store_closures_store_id_business_date_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_store_id_business_date_unique UNIQUE (store_id, business_date);


--
-- Name: store_price_lists store_price_lists_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_price_lists
    ADD CONSTRAINT store_price_lists_pkey PRIMARY KEY (store_id, price_list_id);


--
-- Name: stores stores_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: stores stores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_pkey PRIMARY KEY (id);


--
-- Name: stores stores_warehouse_id_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_warehouse_id_unique UNIQUE (warehouse_id);


--
-- Name: tax_groups tax_groups_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tax_groups
    ADD CONSTRAINT tax_groups_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: tax_groups tax_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tax_groups
    ADD CONSTRAINT tax_groups_pkey PRIMARY KEY (id);


--
-- Name: units units_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units
    ADD CONSTRAINT units_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: units units_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units
    ADD CONSTRAINT units_pkey PRIMARY KEY (id);


--
-- Name: users users_email_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_unique UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_unique UNIQUE (username);


--
-- Name: warehouse_stock_ledgers warehouse_stock_ledgers_company_id_sap_trans_seq_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stock_ledgers
    ADD CONSTRAINT warehouse_stock_ledgers_company_id_sap_trans_seq_unique UNIQUE (company_id, sap_trans_seq);


--
-- Name: warehouse_stock_ledgers warehouse_stock_ledgers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stock_ledgers
    ADD CONSTRAINT warehouse_stock_ledgers_pkey PRIMARY KEY (id);


--
-- Name: warehouse_stocks warehouse_stocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stocks
    ADD CONSTRAINT warehouse_stocks_pkey PRIMARY KEY (id);


--
-- Name: warehouse_stocks warehouse_stocks_warehouse_id_product_id_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stocks
    ADD CONSTRAINT warehouse_stocks_warehouse_id_product_id_unique UNIQUE (warehouse_id, product_id);


--
-- Name: warehouses warehouses_company_id_code_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouses
    ADD CONSTRAINT warehouses_company_id_code_unique UNIQUE (company_id, code);


--
-- Name: warehouses warehouses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouses
    ADD CONSTRAINT warehouses_pkey PRIMARY KEY (id);


--
-- Name: activity_logs_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_logs_created_at_index ON public.activity_logs USING btree (created_at);


--
-- Name: activity_logs_store_id_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX activity_logs_store_id_created_at_index ON public.activity_logs USING btree (store_id, created_at);


--
-- Name: cache_expiration_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cache_expiration_index ON public.cache USING btree (expiration);


--
-- Name: cache_locks_expiration_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cache_locks_expiration_index ON public.cache_locks USING btree (expiration);


--
-- Name: cashier_shifts_store_id_business_date_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cashier_shifts_store_id_business_date_index ON public.cashier_shifts USING btree (store_id, business_date);


--
-- Name: cashier_shifts_user_id_ended_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cashier_shifts_user_id_ended_at_index ON public.cashier_shifts USING btree (user_id, ended_at);


--
-- Name: failed_jobs_connection_queue_failed_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX failed_jobs_connection_queue_failed_at_index ON public.failed_jobs USING btree (connection, queue, failed_at);


--
-- Name: jobs_queue_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX jobs_queue_index ON public.jobs USING btree (queue);


--
-- Name: loyalty_ledger_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX loyalty_ledger_created_at_index ON public.loyalty_ledger USING btree (created_at);


--
-- Name: loyalty_ledger_customer_id_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX loyalty_ledger_customer_id_created_at_index ON public.loyalty_ledger USING btree (customer_id, created_at);


--
-- Name: personal_access_tokens_expires_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX personal_access_tokens_expires_at_index ON public.personal_access_tokens USING btree (expires_at);


--
-- Name: personal_access_tokens_tokenable_type_tokenable_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX personal_access_tokens_tokenable_type_tokenable_id_index ON public.personal_access_tokens USING btree (tokenable_type, tokenable_id);


--
-- Name: pos_cancellations_company_id_occurred_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pos_cancellations_company_id_occurred_at_index ON public.pos_cancellations USING btree (company_id, occurred_at);


--
-- Name: pos_cancellations_store_id_occurred_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pos_cancellations_store_id_occurred_at_index ON public.pos_cancellations USING btree (store_id, occurred_at);


--
-- Name: pos_sessions_company_id_started_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pos_sessions_company_id_started_at_index ON public.pos_sessions USING btree (company_id, started_at);


--
-- Name: pos_sessions_user_id_ended_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX pos_sessions_user_id_ended_at_index ON public.pos_sessions USING btree (user_id, ended_at);


--
-- Name: sales_returns_sap_post_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sales_returns_sap_post_status_index ON public.sales_returns USING btree (sap_post_status);


--
-- Name: sales_sap_post_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sales_sap_post_status_index ON public.sales USING btree (sap_post_status);


--
-- Name: sales_store_id_business_date_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sales_store_id_business_date_index ON public.sales USING btree (store_id, business_date);


--
-- Name: sap_sync_runs_company_id_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sap_sync_runs_company_id_created_at_index ON public.sap_sync_runs USING btree (company_id, created_at);


--
-- Name: sessions_last_activity_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_last_activity_index ON public.sessions USING btree (last_activity);


--
-- Name: sessions_user_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_user_id_index ON public.sessions USING btree (user_id);


--
-- Name: stock_adjustments_sap_post_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_adjustments_sap_post_status_index ON public.stock_adjustments USING btree (sap_post_status);


--
-- Name: stock_ledger_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_ledger_created_at_index ON public.stock_ledger USING btree (created_at);


--
-- Name: stock_ledger_ref_type_ref_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_ledger_ref_type_ref_id_index ON public.stock_ledger USING btree (ref_type, ref_id);


--
-- Name: stock_ledger_store_id_product_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_ledger_store_id_product_id_index ON public.stock_ledger USING btree (store_id, product_id);


--
-- Name: stock_transfer_receipts_sap_post_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_transfer_receipts_sap_post_status_index ON public.stock_transfer_receipts USING btree (sap_post_status);


--
-- Name: stock_transfer_returns_sap_post_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_transfer_returns_sap_post_status_index ON public.stock_transfer_returns USING btree (sap_post_status);


--
-- Name: stock_transfer_shipments_sap_post_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_transfer_shipments_sap_post_status_index ON public.stock_transfer_shipments USING btree (sap_post_status);


--
-- Name: stock_transfer_shipments_status_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX stock_transfer_shipments_status_index ON public.stock_transfer_shipments USING btree (status);


--
-- Name: warehouse_stock_ledgers_created_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX warehouse_stock_ledgers_created_at_index ON public.warehouse_stock_ledgers USING btree (created_at);


--
-- Name: warehouse_stock_ledgers_ref_type_ref_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX warehouse_stock_ledgers_ref_type_ref_id_index ON public.warehouse_stock_ledgers USING btree (ref_type, ref_id);


--
-- Name: warehouse_stock_ledgers_warehouse_id_product_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX warehouse_stock_ledgers_warehouse_id_product_id_index ON public.warehouse_stock_ledgers USING btree (warehouse_id, product_id);


--
-- Name: warehouse_stocks_product_id_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX warehouse_stocks_product_id_index ON public.warehouse_stocks USING btree (product_id);


--
-- Name: activity_logs activity_logs_actor_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_logs
    ADD CONSTRAINT activity_logs_actor_id_foreign FOREIGN KEY (actor_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: activity_logs activity_logs_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_logs
    ADD CONSTRAINT activity_logs_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE SET NULL;


--
-- Name: activity_logs activity_logs_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_logs
    ADD CONSTRAINT activity_logs_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE SET NULL;


--
-- Name: brands brands_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT brands_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: cashier_shift_denominations cashier_shift_denominations_cashier_shift_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cashier_shift_denominations
    ADD CONSTRAINT cashier_shift_denominations_cashier_shift_id_foreign FOREIGN KEY (cashier_shift_id) REFERENCES public.cashier_shifts(id) ON DELETE CASCADE;


--
-- Name: cashier_shifts cashier_shifts_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cashier_shifts
    ADD CONSTRAINT cashier_shifts_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: cashier_shifts cashier_shifts_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cashier_shifts
    ADD CONSTRAINT cashier_shifts_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: cashier_shifts cashier_shifts_user_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cashier_shifts
    ADD CONSTRAINT cashier_shifts_user_id_foreign FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: company_details company_details_tax_group_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.company_details
    ADD CONSTRAINT company_details_tax_group_id_foreign FOREIGN KEY (tax_group_id) REFERENCES public.tax_groups(id) ON DELETE SET NULL;


--
-- Name: credit_cards credit_cards_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_cards
    ADD CONSTRAINT credit_cards_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: currencies currencies_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currencies
    ADD CONSTRAINT currencies_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: currency_denominations currency_denominations_currency_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.currency_denominations
    ADD CONSTRAINT currency_denominations_currency_id_foreign FOREIGN KEY (currency_id) REFERENCES public.currencies(id) ON DELETE CASCADE;


--
-- Name: customer_groups customer_groups_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customer_groups
    ADD CONSTRAINT customer_groups_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: customers customers_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: customers customers_customer_group_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_customer_group_id_foreign FOREIGN KEY (customer_group_id) REFERENCES public.customer_groups(id) ON DELETE SET NULL;


--
-- Name: customers customers_member_level_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_member_level_id_foreign FOREIGN KEY (member_level_id) REFERENCES public.member_levels(id) ON DELETE SET NULL;


--
-- Name: deliveries deliveries_sale_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.deliveries
    ADD CONSTRAINT deliveries_sale_id_foreign FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE CASCADE;


--
-- Name: loyalty_config loyalty_config_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_config
    ADD CONSTRAINT loyalty_config_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: loyalty_ledger loyalty_ledger_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_ledger
    ADD CONSTRAINT loyalty_ledger_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: loyalty_ledger loyalty_ledger_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_ledger
    ADD CONSTRAINT loyalty_ledger_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: loyalty_ledger loyalty_ledger_customer_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.loyalty_ledger
    ADD CONSTRAINT loyalty_ledger_customer_id_foreign FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;


--
-- Name: member_levels member_levels_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.member_levels
    ADD CONSTRAINT member_levels_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: numbering_configs numbering_configs_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.numbering_configs
    ADD CONSTRAINT numbering_configs_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: payment_type_store payment_type_store_payment_type_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_type_store
    ADD CONSTRAINT payment_type_store_payment_type_id_foreign FOREIGN KEY (payment_type_id) REFERENCES public.payment_types(id) ON DELETE CASCADE;


--
-- Name: payment_type_store payment_type_store_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_type_store
    ADD CONSTRAINT payment_type_store_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: payment_types payment_types_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_types
    ADD CONSTRAINT payment_types_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: payment_types payment_types_credit_card_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_types
    ADD CONSTRAINT payment_types_credit_card_id_foreign FOREIGN KEY (credit_card_id) REFERENCES public.credit_cards(id) ON DELETE SET NULL;


--
-- Name: pos_active_carts pos_active_carts_cashier_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_active_carts
    ADD CONSTRAINT pos_active_carts_cashier_id_foreign FOREIGN KEY (cashier_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: pos_active_carts pos_active_carts_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_active_carts
    ADD CONSTRAINT pos_active_carts_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: pos_active_carts pos_active_carts_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_active_carts
    ADD CONSTRAINT pos_active_carts_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: pos_cancellations pos_cancellations_cashier_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_cashier_id_foreign FOREIGN KEY (cashier_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: pos_cancellations pos_cancellations_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: pos_cancellations pos_cancellations_customer_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_customer_id_foreign FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;


--
-- Name: pos_cancellations pos_cancellations_pos_session_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_pos_session_id_foreign FOREIGN KEY (pos_session_id) REFERENCES public.pos_sessions(id) ON DELETE SET NULL;


--
-- Name: pos_cancellations pos_cancellations_sale_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_sale_id_foreign FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE SET NULL;


--
-- Name: pos_cancellations pos_cancellations_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_cancellations
    ADD CONSTRAINT pos_cancellations_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE SET NULL;


--
-- Name: pos_sessions pos_sessions_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_sessions
    ADD CONSTRAINT pos_sessions_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: pos_sessions pos_sessions_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_sessions
    ADD CONSTRAINT pos_sessions_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE SET NULL;


--
-- Name: pos_sessions pos_sessions_user_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pos_sessions
    ADD CONSTRAINT pos_sessions_user_id_foreign FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: price_list_items price_list_items_price_list_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_list_items
    ADD CONSTRAINT price_list_items_price_list_id_foreign FOREIGN KEY (price_list_id) REFERENCES public.price_lists(id) ON DELETE CASCADE;


--
-- Name: price_list_items price_list_items_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_list_items
    ADD CONSTRAINT price_list_items_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: price_lists price_lists_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_lists
    ADD CONSTRAINT price_lists_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: price_lists price_lists_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.price_lists
    ADD CONSTRAINT price_lists_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: product_groups product_groups_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_groups
    ADD CONSTRAINT product_groups_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: products products_brand_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_brand_id_foreign FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE SET NULL;


--
-- Name: products products_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: products products_product_group_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_product_group_id_foreign FOREIGN KEY (product_group_id) REFERENCES public.product_groups(id) ON DELETE CASCADE;


--
-- Name: products products_unit_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_unit_id_foreign FOREIGN KEY (unit_id) REFERENCES public.units(id) ON DELETE RESTRICT;


--
-- Name: promotions promotions_brand_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_brand_id_foreign FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE SET NULL;


--
-- Name: promotions promotions_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: promotions promotions_get_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_get_product_id_foreign FOREIGN KEY (get_product_id) REFERENCES public.products(id) ON DELETE SET NULL;


--
-- Name: promotions promotions_product_group_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_product_group_id_foreign FOREIGN KEY (product_group_id) REFERENCES public.product_groups(id) ON DELETE SET NULL;


--
-- Name: promotions promotions_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.promotions
    ADD CONSTRAINT promotions_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE SET NULL;


--
-- Name: sale_drafts sale_drafts_cashier_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_drafts
    ADD CONSTRAINT sale_drafts_cashier_id_foreign FOREIGN KEY (cashier_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: sale_drafts sale_drafts_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_drafts
    ADD CONSTRAINT sale_drafts_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: sale_drafts sale_drafts_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_drafts
    ADD CONSTRAINT sale_drafts_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: sale_lines sale_lines_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_lines
    ADD CONSTRAINT sale_lines_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE SET NULL;


--
-- Name: sale_lines sale_lines_promotion_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_lines
    ADD CONSTRAINT sale_lines_promotion_id_foreign FOREIGN KEY (promotion_id) REFERENCES public.promotions(id) ON DELETE SET NULL;


--
-- Name: sale_lines sale_lines_sale_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_lines
    ADD CONSTRAINT sale_lines_sale_id_foreign FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE CASCADE;


--
-- Name: sale_payments sale_payments_payment_type_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_payments
    ADD CONSTRAINT sale_payments_payment_type_id_foreign FOREIGN KEY (payment_type_id) REFERENCES public.payment_types(id) ON DELETE SET NULL;


--
-- Name: sale_payments sale_payments_sale_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sale_payments
    ADD CONSTRAINT sale_payments_sale_id_foreign FOREIGN KEY (sale_id) REFERENCES public.sales(id) ON DELETE CASCADE;


--
-- Name: sales sales_cashier_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_cashier_id_foreign FOREIGN KEY (cashier_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: sales sales_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: sales sales_customer_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_customer_id_foreign FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;


--
-- Name: sales sales_resolved_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_resolved_by_foreign FOREIGN KEY (resolved_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: sales_return_lines sales_return_lines_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_return_lines
    ADD CONSTRAINT sales_return_lines_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE SET NULL;


--
-- Name: sales_return_lines sales_return_lines_sale_line_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_return_lines
    ADD CONSTRAINT sales_return_lines_sale_line_id_foreign FOREIGN KEY (sale_line_id) REFERENCES public.sale_lines(id) ON DELETE SET NULL;


--
-- Name: sales_return_lines sales_return_lines_sales_return_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_return_lines
    ADD CONSTRAINT sales_return_lines_sales_return_id_foreign FOREIGN KEY (sales_return_id) REFERENCES public.sales_returns(id) ON DELETE CASCADE;


--
-- Name: sales_returns sales_returns_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_returns
    ADD CONSTRAINT sales_returns_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: sales_returns sales_returns_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_returns
    ADD CONSTRAINT sales_returns_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: sales_returns sales_returns_original_sale_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_returns
    ADD CONSTRAINT sales_returns_original_sale_id_foreign FOREIGN KEY (original_sale_id) REFERENCES public.sales(id) ON DELETE SET NULL;


--
-- Name: sales_returns sales_returns_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales_returns
    ADD CONSTRAINT sales_returns_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: sales sales_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sales
    ADD CONSTRAINT sales_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: sap_sync_runs sap_sync_runs_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sap_sync_runs
    ADD CONSTRAINT sap_sync_runs_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_adjustment_lines stock_adjustment_lines_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustment_lines
    ADD CONSTRAINT stock_adjustment_lines_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: stock_adjustment_lines stock_adjustment_lines_stock_adjustment_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustment_lines
    ADD CONSTRAINT stock_adjustment_lines_stock_adjustment_id_foreign FOREIGN KEY (stock_adjustment_id) REFERENCES public.stock_adjustments(id) ON DELETE CASCADE;


--
-- Name: stock_adjustments stock_adjustments_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustments
    ADD CONSTRAINT stock_adjustments_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_adjustments stock_adjustments_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustments
    ADD CONSTRAINT stock_adjustments_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: stock_adjustments stock_adjustments_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_adjustments
    ADD CONSTRAINT stock_adjustments_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_ledger stock_ledger_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_ledger
    ADD CONSTRAINT stock_ledger_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_ledger stock_ledger_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_ledger
    ADD CONSTRAINT stock_ledger_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: stock_ledger stock_ledger_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_ledger
    ADD CONSTRAINT stock_ledger_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: stock_ledger stock_ledger_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_ledger
    ADD CONSTRAINT stock_ledger_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_lines stock_transfer_lines_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_lines
    ADD CONSTRAINT stock_transfer_lines_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_lines stock_transfer_lines_stock_transfer_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_lines
    ADD CONSTRAINT stock_transfer_lines_stock_transfer_id_foreign FOREIGN KEY (stock_transfer_id) REFERENCES public.stock_transfers(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_receipt_lines stock_transfer_receipt_lines_receipt_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipt_lines
    ADD CONSTRAINT stock_transfer_receipt_lines_receipt_id_foreign FOREIGN KEY (receipt_id) REFERENCES public.stock_transfer_receipts(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_receipt_lines stock_transfer_receipt_lines_shipment_line_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipt_lines
    ADD CONSTRAINT stock_transfer_receipt_lines_shipment_line_id_foreign FOREIGN KEY (shipment_line_id) REFERENCES public.stock_transfer_shipment_lines(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_receipts stock_transfer_receipts_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipts
    ADD CONSTRAINT stock_transfer_receipts_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_receipts stock_transfer_receipts_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipts
    ADD CONSTRAINT stock_transfer_receipts_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: stock_transfer_receipts stock_transfer_receipts_shipment_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipts
    ADD CONSTRAINT stock_transfer_receipts_shipment_id_foreign FOREIGN KEY (shipment_id) REFERENCES public.stock_transfer_shipments(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_receipts stock_transfer_receipts_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_receipts
    ADD CONSTRAINT stock_transfer_receipts_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_return_lines stock_transfer_return_lines_return_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_return_lines
    ADD CONSTRAINT stock_transfer_return_lines_return_id_foreign FOREIGN KEY (return_id) REFERENCES public.stock_transfer_returns(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_return_lines stock_transfer_return_lines_shipment_line_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_return_lines
    ADD CONSTRAINT stock_transfer_return_lines_shipment_line_id_foreign FOREIGN KEY (shipment_line_id) REFERENCES public.stock_transfer_shipment_lines(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_returns stock_transfer_returns_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_returns
    ADD CONSTRAINT stock_transfer_returns_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_returns stock_transfer_returns_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_returns
    ADD CONSTRAINT stock_transfer_returns_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: stock_transfer_returns stock_transfer_returns_shipment_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_returns
    ADD CONSTRAINT stock_transfer_returns_shipment_id_foreign FOREIGN KEY (shipment_id) REFERENCES public.stock_transfer_shipments(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_returns stock_transfer_returns_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_returns
    ADD CONSTRAINT stock_transfer_returns_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_shipment_lines stock_transfer_shipment_lines_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipment_lines
    ADD CONSTRAINT stock_transfer_shipment_lines_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_shipment_lines stock_transfer_shipment_lines_shipment_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipment_lines
    ADD CONSTRAINT stock_transfer_shipment_lines_shipment_id_foreign FOREIGN KEY (shipment_id) REFERENCES public.stock_transfer_shipments(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_from_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_from_store_id_foreign FOREIGN KEY (from_store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_from_warehouse_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_from_warehouse_id_foreign FOREIGN KEY (from_warehouse_id) REFERENCES public.warehouses(id) ON DELETE SET NULL;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_to_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_to_store_id_foreign FOREIGN KEY (to_store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_to_warehouse_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_to_warehouse_id_foreign FOREIGN KEY (to_warehouse_id) REFERENCES public.warehouses(id) ON DELETE SET NULL;


--
-- Name: stock_transfer_shipments stock_transfer_shipments_transit_warehouse_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfer_shipments
    ADD CONSTRAINT stock_transfer_shipments_transit_warehouse_id_foreign FOREIGN KEY (transit_warehouse_id) REFERENCES public.warehouses(id) ON DELETE SET NULL;


--
-- Name: stock_transfers stock_transfers_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfers
    ADD CONSTRAINT stock_transfers_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stock_transfers stock_transfers_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfers
    ADD CONSTRAINT stock_transfers_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: stock_transfers stock_transfers_from_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfers
    ADD CONSTRAINT stock_transfers_from_store_id_foreign FOREIGN KEY (from_store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stock_transfers stock_transfers_to_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_transfers
    ADD CONSTRAINT stock_transfers_to_store_id_foreign FOREIGN KEY (to_store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: store_closure_denominations store_closure_denominations_store_closure_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closure_denominations
    ADD CONSTRAINT store_closure_denominations_store_closure_id_foreign FOREIGN KEY (store_closure_id) REFERENCES public.store_closures(id) ON DELETE CASCADE;


--
-- Name: store_closure_payments store_closure_payments_payment_type_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closure_payments
    ADD CONSTRAINT store_closure_payments_payment_type_id_foreign FOREIGN KEY (payment_type_id) REFERENCES public.payment_types(id) ON DELETE SET NULL;


--
-- Name: store_closure_payments store_closure_payments_store_closure_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closure_payments
    ADD CONSTRAINT store_closure_payments_store_closure_id_foreign FOREIGN KEY (store_closure_id) REFERENCES public.store_closures(id) ON DELETE CASCADE;


--
-- Name: store_closures store_closures_closed_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_closed_by_foreign FOREIGN KEY (closed_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: store_closures store_closures_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: store_closures store_closures_opened_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_opened_by_foreign FOREIGN KEY (opened_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: store_closures store_closures_reopened_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_reopened_by_foreign FOREIGN KEY (reopened_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: store_closures store_closures_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_closures
    ADD CONSTRAINT store_closures_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: store_price_lists store_price_lists_price_list_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_price_lists
    ADD CONSTRAINT store_price_lists_price_list_id_foreign FOREIGN KEY (price_list_id) REFERENCES public.price_lists(id) ON DELETE CASCADE;


--
-- Name: store_price_lists store_price_lists_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.store_price_lists
    ADD CONSTRAINT store_price_lists_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE CASCADE;


--
-- Name: stores stores_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: stores stores_default_customer_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_default_customer_id_foreign FOREIGN KEY (default_customer_id) REFERENCES public.customers(id) ON DELETE SET NULL;


--
-- Name: stores stores_purchase_price_list_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_purchase_price_list_id_foreign FOREIGN KEY (purchase_price_list_id) REFERENCES public.price_lists(id) ON DELETE SET NULL;


--
-- Name: stores stores_warehouse_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stores
    ADD CONSTRAINT stores_warehouse_id_foreign FOREIGN KEY (warehouse_id) REFERENCES public.warehouses(id) ON DELETE SET NULL;


--
-- Name: tax_groups tax_groups_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tax_groups
    ADD CONSTRAINT tax_groups_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: units units_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.units
    ADD CONSTRAINT units_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: users users_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: users users_store_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_store_id_foreign FOREIGN KEY (store_id) REFERENCES public.stores(id) ON DELETE SET NULL;


--
-- Name: warehouse_stock_ledgers warehouse_stock_ledgers_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stock_ledgers
    ADD CONSTRAINT warehouse_stock_ledgers_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: warehouse_stock_ledgers warehouse_stock_ledgers_created_by_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stock_ledgers
    ADD CONSTRAINT warehouse_stock_ledgers_created_by_foreign FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: warehouse_stock_ledgers warehouse_stock_ledgers_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stock_ledgers
    ADD CONSTRAINT warehouse_stock_ledgers_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: warehouse_stock_ledgers warehouse_stock_ledgers_warehouse_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stock_ledgers
    ADD CONSTRAINT warehouse_stock_ledgers_warehouse_id_foreign FOREIGN KEY (warehouse_id) REFERENCES public.warehouses(id) ON DELETE CASCADE;


--
-- Name: warehouse_stocks warehouse_stocks_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stocks
    ADD CONSTRAINT warehouse_stocks_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- Name: warehouse_stocks warehouse_stocks_product_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stocks
    ADD CONSTRAINT warehouse_stocks_product_id_foreign FOREIGN KEY (product_id) REFERENCES public.products(id) ON DELETE CASCADE;


--
-- Name: warehouse_stocks warehouse_stocks_warehouse_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouse_stocks
    ADD CONSTRAINT warehouse_stocks_warehouse_id_foreign FOREIGN KEY (warehouse_id) REFERENCES public.warehouses(id) ON DELETE CASCADE;


--
-- Name: warehouses warehouses_company_id_foreign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.warehouses
    ADD CONSTRAINT warehouses_company_id_foreign FOREIGN KEY (company_id) REFERENCES public.company_details(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--



-- +goose Down
SELECT 'baseline tidak bisa di-down';
