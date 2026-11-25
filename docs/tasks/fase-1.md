# Fase 1: Master data (spesifikasi Opus untuk Sonnet)

Repo: `D:\Devs\easyPOS-go`. Buat branch `fase-1` dari `fase-0`. Commit lokal per task, **jangan push**.
Acuan perilaku: Laravel di `D:\Devs\easyPOS` (read-only, jangan diubah). Bila spesifikasi ini dan Laravel berbeda, ikuti spesifikasi ini lalu catat di laporan.
Laporan akhir: `docs/tasks/fase-1-laporan.md`.

Tidak ada impor data di fase 1. Tarik master data dari SAP B1 adalah fase 2, dan Laravel belum punya data produksi. Semua data dibuat lewat Backoffice/API.

## Aturan umum (berlaku untuk semua task)

1. **Struktur modul.** Satu paket per domain di `internal/` (mis. `internal/catalog`, `internal/pricing`, `internal/partner`, `internal/payment`, `internal/org`). Handler HTTP tipis; validasi + aturan bisnis di service; query di `db/queries/*.sql` lalu `sqlc generate`.
2. **Multi-company.** Setiap query baca/tulis WAJIB difilter `company_id` dari token. ID milik company lain → 404 (bukan 403). Uji ini sekali per entitas.
3. **ID & waktu.** UUID v7 dari Go. `created_at`/`updated_at` diisi server (UTC, presisi detik, sama seperti Laravel).
4. **Angka.** Uang, qty, rate, faktor konversi memakai `decimal.Decimal`; JSON keluar sebagai string (`"12500.00"`). Jangan pakai float.
5. **Bentuk API** (samakan dengan `routes/api.php` Laravel):
   - List: `GET /api/{res}?q=&page=&per_page=&is_active=` → `{data:[...], meta:{page, per_page, total, last_page}}`. Default per_page 25, maks 100. `q` mencari code/name (ILIKE).
   - Detail: `GET /api/{res}/{id}` → `{data:{...}}`.
   - Buat: `POST` → 201 `{data}`; ubah: `PUT` → 200 `{data}`; hapus: `DELETE` → 204.
   - Error validasi: 422 `{message:"Data tidak valid", errors:{field:["pesan"]}}` (pesan Bahasa Indonesia, nama field sama dengan request). Satu helper validator bersama di `internal/platform/validate`.
   - Konflik bisnis (mis. hapus data yang masih dipakai): 409 `{message}`.
6. **Hapus.** Hard delete hanya bila tidak direferensikan; kalau dipakai → 409 dengan pesan jelas ("Satuan masih dipakai 3 produk"). Cek dulu, tetap tangkap error Postgres `23503` sebagai cadangan.
7. **Akses.** Setiap rute didaftarkan dengan route key Laravel (tabel per task). Matriks dari fase 0 (`Allows`) yang memutuskan. Hasilnya: store_manager hanya bisa membaca semua master data fase 1; `admin.company.*` dan `admin.settings.*` tertutup baginya. Kasir tidak bisa memakai API admin. Tambahkan route key baru ke `me.permissions`.
8. **Audit.** Setiap create/update/delete menulis `activity_logs` (`{entity}.created|updated|deleted`, properties `{old:{}, new:{}}` berisi field yang berubah), dalam transaksi yang sama.
9. **Field SAP.** Kolom `sap_*` di tabel yang ada ditampilkan read-only dan tidak bisa diisi lewat API fase 1 (diabaikan bila dikirim). Fase 2 yang mengisinya.
10. **Frontend Backoffice.** Halaman per entitas di `/backoffice/`: list (tabel, pencarian, filter aktif, paginasi) + form (drawer atau halaman). Navy `#1E3A5F`, Inter, tanpa hijau, dark/light wajib. Buat komponen bersama dulu (DataTable, FormField, Select async, Switch, MoneyInput, ConfirmDialog, Toast). Tombol tulis disembunyikan bila `permissions[key].readOnly`. Bahasa UI Indonesia.
11. **Tes.** Per entitas minimal: tes service (validasi + aturan bisnis), satu tes integrasi HTTP (create→list→update→delete), tes isolasi company, tes akses store_manager (GET 200, POST 403). Frontend: tes form untuk Produk, Price List, UoM Group, Payment Type.

## F1.0 Perbaikan dari fase 0 (kerjakan pertama)

1. **Cookie terpisah.** Backoffice memakai cookie `easypos_bo_token`, POS `easypos_pos_token`. Login menentukan cookie dari `client_kind`. Middleware membaca cookie sesuai prefix rute: `/api/pos/*` → cookie POS, lainnya → cookie backoffice; header Bearer tetap didahulukan. Tes: login dua-duanya, keduanya tetap valid.
2. **Revalidasi gerai aktif.** Di middleware auth, untuk non-owner: bila `active_store_id` ≠ `users.store_id` saat ini → set `active_store_id = users.store_id` (atau NULL bila user tak punya store, lalu 403 untuk rute yang butuh gerai). Owner: bila gerai aktif sudah dihapus/nonaktif → NULL. Tes kedua kasus.
3. Saat user dinonaktifkan atau password-nya di-reset, hapus semua token miliknya.

## F1.1 Profil perusahaan

Tabel `company_details`. Field SAP (`sap_enabled`, `sap_endpoint_url`, dst.) tidak termasuk; dikerjakan di fase 2.
Rute: `GET /api/company` (`admin.company.edit`), `PUT /api/company` (`admin.company.update`).
Validasi: name wajib ≤150; address ≤500; npwp_number ≤30; phone ≤30; email valid ≤150; ppn_enabled bool; tax_group_id wajib bila ppn_enabled dan harus tax group aktif milik company; ppn_inclusive bool; base_currency_code wajib, harus kode currency aktif milik company; timezone wajib, nama IANA valid (`time.LoadLocation`); allow_negative_stock bool; return_window_days 0–3650; return_require_receipt bool.
Halaman **Profil Perusahaan** (owner saja).

## F1.2 Pajak dan mata uang

**Tax groups** `admin.tax-groups.{index,store,update,destroy}`: code wajib ≤30 unik per company; name ≤150; rate 0–100 (2 desimal); effective_from tanggal opsional; is_active. Hapus ditolak bila dipakai company/produk/store.

**Currencies** `admin.currencies.{index,store,update,destroy}`: code tepat 3 huruf A–Z (simpan uppercase) unik per company; symbol wajib ≤10; decimal_places 0–4; is_active. Hapus ditolak bila base currency company atau dipakai store.
Denominasi: `POST /api/currencies/{id}/denominations` (`admin.currencies.denominations.store`) value ≥0.01 unik per currency; `DELETE /api/currencies/{id}/denominations/{denId}` (`admin.currencies.denominations.destroy`). Detail currency mengembalikan `denominations` terurut naik.
Halaman **Grup Pajak**, **Mata Uang** (panel denominasi di detail).

## F1.3 Organisasi: gudang, gerai, pengguna

**Warehouses** `admin.warehouses.{index,store,update,destroy}`: code wajib ≤30 unik per company; name ≤150; profit_center ≤50; is_retail; plus kolom lain yang ada di tabel (transit, akun QRIS/EDC) — baca migration baseline, tampilkan semua yang bukan `sap_*`. Hapus ditolak bila dipakai store atau punya stok.

**Stores** `admin.stores.{index,show,store,update,destroy}`: code wajib ≤20 unik per company; name ≤150; address ≤500; pic_name wajib ≤150; pic_phone ≤30; ppn_enabled; ppn_rate 0–100; currency wajib = kode currency aktif; opening_float ≥0; is_active; sales_price_list_id, purchase_price_list_id (milik company); default_customer_id (milik company); warehouse_id (milik company, unik antar store).
Price list penjualan disinkron ke pivot `store_price_lists` dengan `is_default=true` (hapus yang lain), seperti `StoreController` Laravel. Hapus ditolak bila store punya transaksi atau user. Price list dan customer baru dibuat di F1.6/F1.8, jadi field-nya opsional (Select async).

**Users** `admin.users.{index,store,update,destroy}` + `POST /api/users/{id}/reset-password` (`admin.users.reset-password`): name wajib ≤150; username wajib ≤60 alpha_dash unik global; email valid ≤150 unik global; phone ≤30; role ∈ owner/store_manager/cashier; store_id wajib untuk store_manager & cashier; is_active. Password wajib saat create, min 10 karakter berisi huruf dan angka; bcrypt cost 12. Reset: `{password, password_confirmation}`.
Tambahan: user tidak boleh menghapus/menonaktifkan dirinya sendiri; owner aktif terakhir tidak boleh dihapus/dinonaktifkan/diturunkan (409). Jangan pernah kembalikan password di JSON. Lihat F1.0 butir 3.
Halaman **Gudang**, **Gerai**, **Pengguna**.

## F1.4 Satuan dan katalog dasar

**Units** `admin.units.{index,store,update,destroy}`: code wajib ≤20 unik; name ≤100; is_active. Hapus ditolak bila dipakai produk, UoM group, barcode, atau price list item.

**UoM Groups** (baru; tabel `uom_groups` + `uom_group_lines`) `admin.uom-groups.{index,show,store,update,destroy}`:
- code wajib ≤20 unik; name ≤100; base_unit_id wajib; is_active.
- `lines`: array `{unit_id, alt_qty, base_qty}` dikirim utuh di POST/PUT (replace dalam satu transaksi). Artinya alt_qty satuan ini = base_qty satuan dasar (Dus: alt_qty 1, base_qty 12).
- Server selalu memastikan baris base unit 1 = 1; baris base tidak bisa diubah/dihapus. unit_id unik per group. alt_qty, base_qty > 0, maks 3 desimal.
- Faktor = base_qty / alt_qty. Buat paket `internal/uom` dengan `Factor(group, unitID)` dan `ToBase(qty, unitID)` + tes (dipakai POS & inventori nanti).
- Baris unit yang dipakai barcode/price list item produk ber-group ini tidak bisa dihapus (409). Group yang dipakai produk tidak bisa dihapus dan base_unit_id-nya tidak bisa diganti.
- Halaman **UoM Group**: tabel baris yang bisa ditambah, tampilkan kalimat "1 Dus = 12 Pcs".

**Product groups** `admin.groups.{index,store,update,destroy}` (API `/api/product-groups`): code ≤30 unik; name ≤150; description ≤500; is_active.
**Brands** `admin.brands.{index,store,update,destroy}`: code ≤30 unik; name ≤150; is_active.

## F1.5 Produk

`admin.products.{index,store,update,destroy}`, API `/api/products` (+ `GET /api/products/{id}`).
Validasi Laravel: product_group_id wajib; brand_id opsional; unit_id wajib (= satuan dasar, stok selalu dalam satuan ini); sku wajib ≤40 unik per company; barcode ≤60 unik per company; name wajib ≤150; description ≤500; base_price ≥0 (per satuan dasar); track_stock; is_active.
Tambahan:
- `uom_group_id` opsional. Bila diisi, unit_id wajib = base_unit_id group (422 bila beda).
- `sales_unit_id` opsional: satuan default; harus unit dasar atau anggota group.
- `manage_by` ∈ none/batch/serial. Bisa diubah lewat API hanya bila `company_details.sap_enabled = false`; bila SAP aktif read-only (dari SAP di fase 2). Tidak boleh diubah bila produk sudah punya batch/serial/stok (409).
- **Barcode.** `products.barcode` = barcode utama (kelak dari SAP `BarCode`, satuan dasar). Alternate UPC di `product_barcodes`, dikirim sebagai `barcodes: [{id?, code, unit_id, note}]` di POST/PUT produk (replace-set). code ≤60; unit_id harus satuan dasar atau anggota group; code unik di seluruh company lintas `products.barcode` DAN `product_barcodes.code` (cek di service, tes dua arah). Baris `source='sap'` tidak bisa diubah/dihapus.
- **Lookup** `GET /api/products/lookup?code=` (route key `admin.products.index`): cari `products.barcode` lalu `product_barcodes.code` → `{product, unit, factor}`. Tulis sebagai service dengan tes (dipakai POS fase 3).
- **Gambar.** `POST /api/products/{id}/image` (multipart, jpg/jpeg/png/webp ≤2 MB, cek tipe dari isi file) dan `DELETE /api/products/{id}/image`. Simpan di `STORAGE_DIR` (env, default `./storage`) sebagai `products/{uuid}.{ext}`; sajikan via `GET /media/...` tanpa directory listing dan aman dari path traversal. Simpan path relatif di kolom gambar yang ada.
- List: filter product_group_id, brand_id, manage_by, is_active, q (sku/name/barcode/alternate code).
- Hapus ditolak bila dipakai price list, promosi, stok, atau transaksi (409).
Halaman **Produk** sesuai desain (gambar kecil, SKU, nama, grup, satuan, harga, status, badge Batch/Serial). Form tab: Umum, Satuan & Barcode (pilih group, tabel alternate UPC per satuan), Gambar.

## F1.6 Price list dan price per unit

`admin.price-lists.{index,show,store,update,destroy}`; item: `GET /api/price-lists/{id}/items`, `POST .../items` (`admin.price-lists.items.store`), `PUT .../items/{itemId}` (`admin.price-lists.items.update`), `DELETE .../items/{itemId}` (`admin.price-lists.items.destroy`).
Header: store_id opsional; code ≤30 unik; name ≤150; is_default (satu per company, set yang lain false dalam transaksi); effective_from/until, until ≥ from; is_active. Hapus ditolak bila dipakai store.
Item: product_id wajib; price ≥0; min_qty integer ≥1; unit_id opsional (price per unit; NULL = satuan dasar; bila diisi harus anggota group produk). Unik (price_list, product, unit, min_qty) — tangkap index `pli_unique_unit` → 422 di field min_qty.
**Resolver** paket `internal/pricing`, fungsi murni + tes table-driven: `ResolvePrice(priceList, product, unitID, qty, date)`:
1. Price list tidak aktif / di luar effective range → base_price × factor.
2. Cari item unit_id = unit diminta dan min_qty ≤ qty, ambil min_qty terbesar.
3. Bila tidak ada, item unit_id NULL dengan min_qty ≤ qty×factor, harga × factor.
4. Bila tidak ada → base_price × factor.
Kembalikan `{unit_price, source: price_list_unit|price_list_base|base_price}`. Endpoint uji `GET /api/price-lists/{id}/resolve?product_id=&unit_id=&qty=` (`admin.price-lists.show`).
Halaman **Price List**: header + tabel item (produk, satuan, min qty, harga) inline, plus info harga efektif per satuan dasar.

## F1.7 Promosi dan hierarki

`admin.promotions.{index,store,update,destroy}`. Port validasi `app/Models/Promotion.php::rules`: code ≤30 unik; name ≤150; type manual: discount_amount, special_price, buy_x_get_y (discount_percent & volume_price dari SAP, read-only); discount_amount > 0; special_price ≥0; product_id wajib untuk special_price & buy_x_get_y; buy_qty/get_qty ≥1 wajib untuk buy_x_get_y; get_product_id opsional; min_qty ≥1; max_applications ≥1; ends_at ≥ starts_at; is_active; description ≤500. Target pelanggan (CUSTOMER_ALL / CUSTOMER_GROUP / CUSTOMER_SPECIFIC): port apa adanya dari Laravel. Promosi source='sap' read-only.
Hierarki: `GET /api/promotion-hierarchy` (`admin.promotion-hierarchy.edit`), `PUT` (`admin.promotion-hierarchy.update`) body `{order:[...]}` berisi semua `HIERARCHY_SOURCES` tepat sekali; simpan di tempat yang sama dengan Laravel. Halaman drag-and-drop. Store_manager boleh melihat (pengecualian .edit).
Mesin perhitungan promosi BUKAN fase 1 (fase 4).

## F1.8 Pelanggan

**Customer groups** `admin.customer-groups.{index,store,update,destroy}`: code ≤30 unik; name ≤150; is_default (satu per company). Hapus ditolak bila dipakai customer atau default.
**Customers** `admin.customers.{index,show,store,update,destroy}`: name wajib ≤150; phone ≤30; code ≤30 (unik per company bila diisi; bila kosong dan ada numbering tipe customer, buat otomatis; bila tidak, biarkan kosong dan catat); email ≤150; address ≤500; customer_group_id (default = group default); is_member; is_active. Tanpa poin/loyalty. Rute adjust-points & sap-sync tidak dibuat.

## F1.9 Kartu kredit dan tipe pembayaran

**Credit cards** (Laravel tanpa UI; kelak dari SAP) `admin.credit-cards.{index,store,update,destroy}`: code ≤20 unik; name wajib; gl_account ≤25. Bila SAP aktif → read-only.
**Payment types** `admin.payment-types.{index,store,update,destroy}`: code ≤30 unik; description ≤500; order_number ≥1; kind ∈ cash/custom (kode CASH → cash, port `booted()`); count_type ∈ summary/detail; credit_card_id wajib untuk custom bila SAP aktif; is_qr_payment; qr_image wajib bila is_qr_payment dan belum ada gambar (upload terpisah `POST/DELETE /api/payment-types/{id}/qr-image`, aturan seperti gambar produk); store_ids wajib min 1, sinkron ke `payment_type_store`; is_active. QRIS dan EDC metode manual (tanpa Xendit).
Halaman **Kartu Kredit**, **Tipe Pembayaran**.

## Menu Backoffice

**Perusahaan** (Profil, Gerai, Gudang, Pengguna), **Katalog** (Produk, Grup Produk, Merek, Satuan, UoM Group), **Harga & Promo** (Price List, Promosi, Hierarki Promosi), **Pelanggan** (Grup, Pelanggan), **Keuangan** (Grup Pajak, Mata Uang, Kartu Kredit, Tipe Pembayaran), **Sistem** (Penomoran, Konfigurasi Sistem). Disaring dari `me.permissions`.

## Seed demo

`easypos seed-demo` (idempotent, hanya `APP_ENV=dev`): 1 company, tax group PPN 11%, IDR + denominasi, 2 gudang, 2 gerai, owner/manager/kasir (password acak dicetak ke stdout sekali, tidak ditulis ke file), satuan Pcs/Dus/Pak, UoM group Dus-Pcs (1 Dus = 12 Pcs), 3 grup, 3 merek, 20 produk (2 batch, 1 serial, 5 ber-UoM group dengan alternate UPC per Dus), 1 price list default dengan beberapa harga per Dus, 2 promosi, 1 customer group default + 5 pelanggan, tipe pembayaran CASH/QRIS/EDC.

## Urutan kerja dan commit

F1.0 → komponen frontend bersama → F1.1 … F1.9 → menu → seed. Satu commit per task (`fase-1: F1.x ...`). Setelah tiap task: `go vet ./...`, `go test ./...`, di `web/`: `npm run build`, `npm test`.

## Definition of done

- Semua cek hijau; `easypos migrate up` dari DB kosong lalu `seed-demo` berhasil.
- Uji manual di browser (light & dark): buat UoM group Dus-Pcs, produk dengan alternate UPC Dus, lookup barcode Dus → factor 12, harga per Dus diresolve benar, store_manager hanya bisa membaca.
- Migration lama tidak diubah; migration baru (bila perlu) bernomor 00004+ dengan Down.
- Laporan `docs/tasks/fase-1-laporan.md`: endpoint + route key, keputusan di luar spesifikasi, perbedaan dengan Laravel, yang belum selesai, cara uji manual singkat.
- Jangan push. Opus akan me-review diff fase-0..fase-1.
