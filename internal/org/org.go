// Package org: profil perusahaan, pajak, mata uang, gudang, gerai, dan pengguna.
package org

import (
	"fmt"
	"net/http"
	"regexp"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/crud"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/validate"
)

var Company = crud.Resource{
	Table: "company_details", Entity: "company", Noun: "Perusahaan", Key: "admin.company", Singleton: true,
	Fields: []crud.Field{
		crud.S("name", "Nama", 150).Req(),
		crud.S("address", "Alamat", 500),
		crud.S("npwp_number", "NPWP", 30),
		crud.S("phone", "Telepon", 30),
		crud.S("email", "Email", 150).Mail(),
		crud.B("ppn_enabled", "PPN aktif", false),
		crud.U("tax_group_id", "Grup pajak", "tax_groups").Active(),
		crud.B("ppn_inclusive", "Harga termasuk PPN", false),
		crud.S("base_currency_code", "Mata uang dasar", 10).Req(),
		crud.S("timezone", "Zona waktu", 40).Req(),
		crud.B("allow_negative_stock", "Izinkan stok negatif", false),
		crud.I("return_window_days", "Batas retur (hari)").Range("0", "3650").Def(int64(30)),
		crud.B("return_require_receipt", "Retur wajib struk", true),
	},
	Validate: func(c *crud.Ctx, m, old crud.Record, in map[string]any, errs validate.Errors) error {
		if m["ppn_enabled"] == true && m["tax_group_id"] == nil {
			errs.Add("tax_group_id", "Wajib diisi bila PPN aktif.")
		}
		if tz, _ := m["timezone"].(string); tz != "" {
			if _, err := time.LoadLocation(tz); err != nil {
				errs.Add("timezone", "Zona waktu tidak dikenal (gunakan nama IANA, mis. Asia/Jakarta).")
			}
		}
		if cur, _ := m["base_currency_code"].(string); cur != "" {
			var n int
			if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM currencies WHERE company_id=$1 AND code=$2 AND is_active", c.A.CompanyID, cur).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				errs.Add("base_currency_code", "Mata uang tidak ditemukan atau nonaktif.")
			}
		}
		return nil
	},
}

var TaxGroups = crud.Resource{
	Path: "tax-groups", Table: "tax_groups", Entity: "tax_group", Noun: "Grup pajak", Key: "admin.tax-groups",
	Search: []string{"code", "name"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 30).Req().Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.D("rate", "Tarif (%)", 2).Req().Range("0", "100"),
		crud.Dt("effective_from", "Berlaku mulai"),
		crud.B("is_active", "Aktif", true),
		crud.S("source", "Sumber", 10).RO(),
	},
	Locked: func(_ *crud.Ctx, r crud.Record) string {
		if r["source"] == "sap" {
			return "Data dari SAP tidak bisa diubah di easyPOS."
		}
		return ""
	},
}

var Currencies = crud.Resource{
	Path: "currencies", Table: "currencies", Entity: "currency", Noun: "Mata uang", Key: "admin.currencies",
	Search: []string{"code"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 3).Req().Uniq().Up().Match(regexp.MustCompile(`^[A-Z]{3}$`), "Harus tepat 3 huruf A-Z."),
		crud.S("symbol", "Simbol", 10).Req(),
		crud.I("decimal_places", "Desimal").Range("0", "4").Def(int64(2)),
		crud.B("is_active", "Aktif", true),
	},
	Decorate: func(c *crud.Ctx, recs []crud.Record, detail bool) error {
		if !detail {
			return nil
		}
		for _, r := range recs {
			rows, err := c.Q.Query(c.Ctx, "SELECT id::text, value::text FROM currency_denominations d WHERE currency_id=$1 ORDER BY d.value", r["id"])
			if err != nil {
				return err
			}
			dens := []map[string]string{}
			for rows.Next() {
				var id, v string
				if err := rows.Scan(&id, &v); err != nil {
					rows.Close()
					return err
				}
				dens = append(dens, map[string]string{"id": id, "value": v})
			}
			rows.Close()
			r["denominations"] = dens
		}
		return nil
	},
	CanDelete: func(c *crud.Ctx, r crud.Record) (string, error) {
		var base, stores int
		if err := c.Q.QueryRow(c.Ctx, "SELECT (SELECT count(*) FROM company_details WHERE id=$1 AND base_currency_code=$2), (SELECT count(*) FROM stores WHERE company_id=$1 AND currency=$2)",
			c.A.CompanyID, r["code"]).Scan(&base, &stores); err != nil {
			return "", err
		}
		if base > 0 {
			return "Mata uang masih menjadi mata uang dasar perusahaan.", nil
		}
		if stores > 0 {
			return fmt.Sprintf("Mata uang masih dipakai %d gerai.", stores), nil
		}
		return "", nil
	},
}

var Warehouses = crud.Resource{
	Path: "warehouses", Table: "warehouses", Entity: "warehouse", Noun: "Gudang", Key: "admin.warehouses",
	Search: []string{"code", "name"}, Order: "t.code", Filters: []string{"is_retail"},
	Fields: []crud.Field{
		crud.S("code", "Kode", 30).Req().Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.S("profit_center", "Profit center", 50),
		crud.B("is_active", "Aktif", true),
		crud.B("is_retail", "Gudang retail", true),
		crud.S("cash_account", "Akun kas", 25),
		crud.S("qris_account", "Akun QRIS", 25),
		crud.S("edc_account", "Akun EDC", 25),
		crud.S("transit_warehouse_code", "Kode gudang transit", 30),
		crud.B("is_transit", "Gudang transit", false),
	},
}

var Stores = crud.Resource{
	Path: "stores", Table: "stores", Entity: "store", Noun: "Gerai", Key: "admin.stores", Show: true,
	Search: []string{"code", "name"}, Order: "t.code",
	Fields: []crud.Field{
		crud.S("code", "Kode", 20).Req().Uniq(),
		crud.S("name", "Nama", 150).Req(),
		crud.S("address", "Alamat", 500),
		crud.S("pic_name", "Nama PIC", 150).Req(),
		crud.S("pic_phone", "Telepon PIC", 30),
		crud.B("ppn_enabled", "PPN aktif", false),
		crud.D("ppn_rate", "Tarif PPN (%)", 2).Range("0", "100").Def("11.00"),
		{Name: "currency", Label: "Mata uang", Kind: crud.KStr, Max: 10, Required: true, Ref: &crud.Ref{Table: "currencies", Col: "code", Active: true}},
		crud.D("opening_float", "Modal awal kas", 2).Range("0", "").Def("0.00"),
		crud.B("is_active", "Aktif", true),
		crud.U("purchase_price_list_id", "Price list pembelian", "price_lists"),
		crud.U("default_customer_id", "Pelanggan default", "customers"),
		crud.U("warehouse_id", "Gudang", "warehouses").Uniq(),
		crud.U("sales_price_list_id", "Price list penjualan", "price_lists").Virt(),
	},
	Expand: []crud.Expand{{Field: "warehouse_id", As: "warehouse", Table: "warehouses"}},
	AfterSave: func(c *crud.Ctx, id string, _, _ crud.Record, in map[string]any) error {
		v, ok := in["sales_price_list_id"]
		if !ok {
			return nil
		}
		if _, err := c.Q.Exec(c.Ctx, "DELETE FROM store_price_lists WHERE store_id=$1::uuid", id); err != nil {
			return err
		}
		if s, _ := v.(string); s != "" {
			_, err := c.Q.Exec(c.Ctx, "INSERT INTO store_price_lists (store_id, price_list_id, is_default, created_at, updated_at) VALUES ($1::uuid,$2::uuid,true,$3,$3)", id, s, time.Now().UTC())
			return err
		}
		return nil
	},
	Decorate: func(c *crud.Ctx, recs []crud.Record, detail bool) error {
		for _, r := range recs {
			var pl *string
			err := c.Q.QueryRow(c.Ctx, "SELECT price_list_id::text FROM store_price_lists WHERE store_id=$1::uuid ORDER BY is_default DESC LIMIT 1", r["id"]).Scan(&pl)
			if err != nil && err.Error() != "no rows in result set" {
				return err
			}
			if pl != nil {
				r["sales_price_list_id"] = *pl
			} else {
				r["sales_price_list_id"] = nil
			}
		}
		return nil
	},
}

var alphaDash = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// CheckPassword: min 10 karakter berisi huruf dan angka. Mengembalikan pesan error atau "".
func CheckPassword(pw string) string {
	if len([]rune(pw)) < 10 {
		return "Minimal 10 karakter."
	}
	var letter, digit bool
	for _, r := range pw {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	if !letter || !digit {
		return "Harus berisi huruf dan angka."
	}
	return ""
}

func otherActiveOwners(c *crud.Ctx, exceptID string) (int, error) {
	var n int
	err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM users WHERE company_id=$1 AND role='owner' AND is_active AND id<>$2::uuid", c.A.CompanyID, exceptID).Scan(&n)
	return n, err
}

var Users = crud.Resource{
	Path: "users", Table: "users", Entity: "user", Noun: "Pengguna", Key: "admin.users",
	Search: []string{"name", "username", "email"}, Order: "lower(t.name)", Filters: []string{"role", "store_id"},
	Fields: []crud.Field{
		crud.S("name", "Nama", 150).Req(),
		crud.S("username", "Username", 60).Req().Match(alphaDash, "Hanya huruf, angka, strip, dan underscore."),
		crud.S("email", "Email", 150).Mail(),
		crud.S("phone", "Telepon", 30),
		crud.S("role", "Peran", 20).Req().In(identity.RoleOwner, identity.RoleStoreManager, identity.RoleCashier),
		crud.U("store_id", "Gerai", "stores"),
		crud.B("is_active", "Aktif", true),
		crud.S("password", "Password", 0).Virt(),
	},
	Expand: []crud.Expand{{Field: "store_id", As: "store", Table: "stores"}},
	Validate: func(c *crud.Ctx, m, old crud.Record, in map[string]any, errs validate.Errors) error {
		if m["role"] != identity.RoleOwner && m["store_id"] == nil {
			errs.Add("store_id", "Wajib diisi untuk Store Manager dan Kasir.")
		}
		for _, f := range []string{"username", "email"} {
			v, _ := m[f].(string)
			if v == "" {
				continue
			}
			var oid any = uuid.Nil.String()
			if old != nil {
				oid = old["id"]
			}
			var n int
			if err := c.Q.QueryRow(c.Ctx, fmt.Sprintf("SELECT count(*) FROM users WHERE lower(%s)=lower($1) AND id<>$2::uuid", f), v, oid).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				errs.Add(f, "Sudah digunakan.")
			}
		}
		if old == nil {
			pw, _ := in["password"].(string)
			if pw == "" {
				errs.Add("password", "Wajib diisi.")
			} else if msg := CheckPassword(pw); msg != "" {
				errs.Add("password", msg)
			}
		}
		if errs.Any() || old == nil {
			return nil
		}
		id := old["id"].(string)
		if id == c.A.UserID.String() && m["is_active"] == false {
			return crud.Conflict("Anda tidak bisa menonaktifkan akun sendiri.")
		}
		if old["role"] == identity.RoleOwner && old["is_active"] == true && (m["role"] != identity.RoleOwner || m["is_active"] == false) {
			if n, err := otherActiveOwners(c, id); err != nil {
				return err
			} else if n == 0 {
				return crud.Conflict("Owner aktif terakhir tidak boleh dinonaktifkan atau diturunkan perannya.")
			}
		}
		return nil
	},
	BeforeWrite: func(c *crud.Ctx, m, old crud.Record, in map[string]any) (map[string]any, error) {
		if old != nil {
			return nil, nil
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in["password"].(string)), 12)
		return map[string]any{"password": string(hash)}, err
	},
	AfterSave: func(c *crud.Ctx, id string, m, old crud.Record, _ map[string]any) error {
		if old != nil && old["is_active"] == true && m["is_active"] == false {
			return identity.RevokeAll(c.Ctx, c.Q, uuid.MustParse(id))
		}
		return nil
	},
	CanDelete: func(c *crud.Ctx, r crud.Record) (string, error) {
		id := r["id"].(string)
		if id == c.A.UserID.String() {
			return "Anda tidak bisa menghapus akun sendiri.", nil
		}
		if r["role"] == identity.RoleOwner && r["is_active"] == true {
			if n, err := otherActiveOwners(c, id); err != nil || n == 0 {
				return "Owner aktif terakhir tidak boleh dihapus.", err
			}
		}
		return "", nil
	},
}

// Mount mendaftarkan semua rute org: company, pajak, mata uang (+denominasi), gudang, gerai, pengguna.
func Mount(b *crud.Binder, r chi.Router) {
	for _, res := range []*crud.Resource{&TaxGroups, &Currencies, &Warehouses, &Stores, &Users} {
		b.Mount(r, res)
	}

	r.With(b.Route("admin.company.edit")).Get("/company", func(w http.ResponseWriter, req *http.Request) {
		rec, err := b.One(b.Ctx(req), &Company, "")
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": rec})
	})
	r.With(b.Route("admin.company.update")).Put("/company", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		old, err := b.E.Load(c, &Company, "")
		if err == nil {
			var in map[string]any
			if in, err = crud.DecodeMap(req); err == nil {
				if _, err = b.E.Save(c, &Company, in, old); err == nil {
					var rec crud.Record
					if rec, err = b.One(c, &Company, ""); err == nil {
						httpx.JSON(w, 200, map[string]any{"data": rec})
						return
					}
				}
			}
		}
		crud.WriteErr(w, err)
	})

	// Denominasi mata uang.
	r.With(b.Route("admin.currencies.denominations.store")).Post("/currencies/{id}/denominations", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		cur, err := b.E.Load(c, &Currencies, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		in, err := crud.DecodeMap(req)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		f := crud.D("value", "Nilai", 2).Req().Range("0.01", "")
		raw, _ := in["value"]
		val, msg := f.Parse(raw)
		if msg == "" && val == nil {
			msg = "Wajib diisi."
		}
		if msg != "" {
			crud.WriteErr(w, crud.Invalid(validate.Errors{"value": {msg}}))
			return
		}
		var n int
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM currency_denominations WHERE currency_id=$1::uuid AND value=$2::numeric", cur["id"], val).Scan(&n); err != nil {
			crud.WriteErr(w, err)
			return
		}
		if n > 0 {
			crud.WriteErr(w, crud.Invalid(validate.Errors{"value": {"Nominal sudah ada."}}))
			return
		}
		nid, _ := uuid.NewV7()
		now := time.Now().UTC()
		if _, err := c.Q.Exec(c.Ctx, "INSERT INTO currency_denominations (id, currency_id, value, is_active, created_at, updated_at) VALUES ($1,$2::uuid,$3::numeric,true,$4,$4)", nid, cur["id"], val, now); err != nil {
			crud.WriteErr(w, err)
			return
		}
		_ = audit.Log(c.Ctx, c.Q, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, Action: "currency.denomination_added",
			SubjectType: "currency", SubjectID: uuid.MustParse(cur["id"].(string)), Description: fmt.Sprintf("Denominasi %v ditambahkan ke %v", val, cur["code"])})
		rec, err := b.One(c, &Currencies, cur["id"].(string))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		httpx.JSON(w, 201, map[string]any{"data": rec})
	})
	r.With(b.Route("admin.currencies.denominations.destroy")).Delete("/currencies/{id}/denominations/{denId}", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		cur, err := b.E.Load(c, &Currencies, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		den, perr := uuid.Parse(chi.URLParam(req, "denId"))
		if perr != nil {
			crud.WriteErr(w, crud.NotFound())
			return
		}
		tag, err := c.Q.Exec(c.Ctx, "DELETE FROM currency_denominations WHERE id=$1 AND currency_id=$2::uuid", den, cur["id"])
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		if tag.RowsAffected() == 0 {
			crud.WriteErr(w, crud.NotFound())
			return
		}
		_ = audit.Log(c.Ctx, c.Q, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, Action: "currency.denomination_removed",
			SubjectType: "currency", SubjectID: uuid.MustParse(cur["id"].(string)), Description: "Denominasi dihapus dari " + fmt.Sprint(cur["code"])})
		w.WriteHeader(http.StatusNoContent)
	})

	// Reset password pengguna.
	r.With(b.Route("admin.users.reset-password")).Post("/users/{id}/reset-password", func(w http.ResponseWriter, req *http.Request) {
		c := b.Ctx(req)
		u, err := b.E.Load(c, &Users, chi.URLParam(req, "id"))
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		in, err := crud.DecodeMap(req)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		errs := validate.Errors{}
		pw, _ := in["password"].(string)
		if msg := CheckPassword(pw); msg != "" {
			errs.Add("password", msg)
		} else if conf, _ := in["password_confirmation"].(string); conf != pw {
			errs.Add("password_confirmation", "Konfirmasi tidak sama dengan password.")
		}
		if errs.Any() {
			crud.WriteErr(w, crud.Invalid(errs))
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		uid := uuid.MustParse(u["id"].(string))
		err = b.E.InTx(c, func(tc *crud.Ctx) error {
			if _, err := tc.Q.Exec(tc.Ctx, "UPDATE users SET password=$1, updated_at=$2 WHERE id=$3 AND company_id=$4", string(hash), time.Now().UTC(), uid, c.A.CompanyID); err != nil {
				return err
			}
			if err := identity.RevokeAll(tc.Ctx, tc.Q, uid); err != nil {
				return err
			}
			return audit.Log(tc.Ctx, tc.Q, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, StoreID: c.A.StoreID,
				Action: "user.password_reset", SubjectType: "user", SubjectID: uid, Description: "Password direset: " + fmt.Sprint(u["username"])})
		})
		if err != nil {
			crud.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
