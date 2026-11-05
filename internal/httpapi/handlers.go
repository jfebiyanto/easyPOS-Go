package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/catalog"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/loyalty"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
	"github.com/jfebiyanto/easyPOS-Go/internal/returns"
	"github.com/jfebiyanto/easyPOS-Go/internal/sales"
)

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "permintaan tidak valid")
		return false
	}
	return true
}

func actorOf(u *identity.User) audit.Actor { return audit.Actor{ID: u.ID, Name: u.Name} }

// fail maps domain errors to HTTP statuses.
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, catalog.ErrInvalid), errors.Is(err, sales.ErrInvalid), errors.Is(err, sales.ErrUnderpaid),
		errors.Is(err, loyalty.ErrInvalid), errors.Is(err, returns.ErrInvalid), errors.Is(err, returns.ErrWindow),
		errors.Is(err, returns.ErrVoided), errors.Is(err, returns.ErrOverReturn),
		errors.Is(err, sales.ErrAlreadyVoid), errors.Is(err, sales.ErrHasReturns), errors.Is(err, sales.ErrVoidDay):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, catalog.ErrDuplicate), errors.Is(err, sales.ErrInsufficientStock), errors.Is(err, loyalty.ErrDuplicatePhone):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, catalog.ErrNotFound), errors.Is(err, sales.ErrNotFound), errors.Is(err, loyalty.ErrNotFound), errors.Is(err, returns.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "terjadi kesalahan")
	}
}

// requireStore: cashier/store manager must have an active store to use POS endpoints (FR-A1-09).
func requireStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := CurrentUser(r.Context()); u == nil || u.StoreID == nil {
			writeErr(w, http.StatusForbidden, "akun belum ditugaskan ke gerai")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// storeScope returns the store an admin request applies to: staff are pinned to their own store;
// an Owner may pass ?store_id= (empty = all stores for reports).
func storeScope(r *http.Request) string {
	u := CurrentUser(r.Context())
	if u.Role == identity.RoleOwner {
		return r.URL.Query().Get("store_id")
	}
	if u.StoreID != nil {
		return *u.StoreID
	}
	return ""
}

func (s *Server) mountPOS(r chi.Router) {
	r.Use(requireStore)
	r.Get("/products", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		list, err := s.catalog.List(r.Context(), u.CompanyID, r.URL.Query().Get("q"), true, *u.StoreID)
		if err == nil {
			err = s.catalog.WithTiers(r.Context(), *u.StoreID, numbering.BusinessDate(time.Now()), list)
		}
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		var en, inc bool
		var rate, store, company string
		if err := s.pool.QueryRow(r.Context(), `SELECT c.ppn_enabled, c.ppn_inclusive, c.ppn_rate::text, st.name, c.name
			FROM companies c, stores st WHERE c.id=$1 AND st.id=$2`, u.CompanyID, *u.StoreID).Scan(&en, &inc, &rate, &store, &company); err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ppn_enabled": en, "ppn_inclusive": inc, "ppn_rate": rate, "store": store, "company": company})
	})
	r.Get("/payment-types", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.sales.PaymentTypes(r.Context(), CurrentUser(r.Context()).CompanyID)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	r.Get("/customers", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.loyalty.Search(r.Context(), CurrentUser(r.Context()).CompanyID, r.URL.Query().Get("q"))
		respond(w, list, err, http.StatusOK)
	})
	r.Post("/customers", func(w http.ResponseWriter, r *http.Request) { // quick member registration at the till (FR-B5-02)
		var in loyalty.Input
		if !decode(w, r, &in) {
			return
		}
		in.IsMember = true
		u := CurrentUser(r.Context())
		c, err := s.loyalty.Create(r.Context(), actorOf(u), u.CompanyID, in)
		respond(w, c, err, http.StatusCreated)
	})
	r.Get("/returns/lookup", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		res, err := s.returns.Lookup(r.Context(), returns.Actor{ID: u.ID, Name: u.Name, CompanyID: u.CompanyID, StoreID: *u.StoreID}, r.URL.Query().Get("number"))
		respond(w, res, err, http.StatusOK)
	})
	r.Post("/returns", func(w http.ResponseWriter, r *http.Request) {
		var in returns.Input
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		res, err := s.returns.Create(r.Context(), returns.Actor{ID: u.ID, Name: u.Name, CompanyID: u.CompanyID, StoreID: *u.StoreID}, in)
		code := http.StatusCreated
		if res != nil && res.Replayed {
			code = http.StatusOK
		}
		respond(w, res, err, code)
	})
	r.Post("/sales", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		var in sales.CheckoutInput
		if !decode(w, r, &in) {
			return
		}
		sale, err := s.sales.Checkout(r.Context(), sales.Cashier{ID: u.ID, Name: u.Name, CompanyID: u.CompanyID, StoreID: *u.StoreID}, in)
		if err != nil {
			fail(w, err)
			return
		}
		code := http.StatusCreated
		if sale.Replayed {
			code = http.StatusOK
		}
		writeJSON(w, code, sale)
	})
	r.Get("/sales", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		list, err := s.sales.ListToday(r.Context(), u.CompanyID, *u.StoreID)
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	r.Get("/sales/{id}", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		sale, err := s.sales.Get(r.Context(), u.CompanyID, *u.StoreID, chi.URLParam(r, "id"))
		if err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, sale)
	})
}

var jakarta = func() *time.Location {
	l, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return l
}()

const defaultBrandColor = "#1E3A5F"

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// putBrandColor lets the Owner customise the UI accent color for the whole company.
func (s *Server) putBrandColor(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r.Context())
	var in struct {
		BrandColor string `json:"brand_color"`
	}
	if !decode(w, r, &in) {
		return
	}
	color := strings.ToUpper(strings.TrimSpace(in.BrandColor))
	if !hexColor.MatchString(color) {
		writeErr(w, http.StatusUnprocessableEntity, "warna harus berformat #RRGGBB")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var old string
	if err := tx.QueryRow(r.Context(), `SELECT brand_color FROM companies WHERE id=$1 FOR UPDATE`, u.CompanyID).Scan(&old); err != nil {
		fail(w, err)
		return
	}
	if _, err := tx.Exec(r.Context(), `UPDATE companies SET brand_color=$2 WHERE id=$1`, u.CompanyID, color); err != nil {
		fail(w, err)
		return
	}
	if err := audit.Record(r.Context(), tx, audit.Entry{CompanyID: u.CompanyID, Actor: actorOf(u), Action: "settings.brand_color",
		SubjectType: "company", SubjectID: u.CompanyID, Description: "Ubah warna tema",
		Properties: map[string]any{"from": old, "to": color}}); err != nil {
		fail(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"brand_color": color})
}

func (s *Server) mountAdmin(r chi.Router) {
	r.With(RequireRole(identity.RoleOwner)).Put("/settings/brand-color", s.putBrandColor)
	s.mountPriceLists(r)
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"pong": "ok"})
	})

	r.Get("/units", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.catalog.Units(r.Context(), CurrentUser(r.Context()).CompanyID)
		respond(w, list, err, http.StatusOK)
	})
	r.Post("/units", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Code, Name string }
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		ref, err := s.catalog.CreateUnit(r.Context(), actorOf(u), u.CompanyID, in.Code, in.Name)
		respond(w, ref, err, http.StatusCreated)
	})
	r.Get("/product-groups", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.catalog.Groups(r.Context(), CurrentUser(r.Context()).CompanyID)
		respond(w, list, err, http.StatusOK)
	})
	r.Post("/product-groups", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Code, Name string }
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		ref, err := s.catalog.CreateGroup(r.Context(), actorOf(u), u.CompanyID, in.Code, in.Name)
		respond(w, ref, err, http.StatusCreated)
	})

	r.Get("/products", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		list, err := s.catalog.List(r.Context(), u.CompanyID, r.URL.Query().Get("q"), false, storeScope(r))
		respond(w, list, err, http.StatusOK)
	})
	r.Post("/products", func(w http.ResponseWriter, r *http.Request) {
		var in catalog.ProductInput
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		id, err := s.catalog.Create(r.Context(), actorOf(u), u.CompanyID, in)
		respond(w, map[string]string{"id": id}, err, http.StatusCreated)
	})
	r.Put("/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in catalog.ProductInput
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		err := s.catalog.Update(r.Context(), actorOf(u), u.CompanyID, chi.URLParam(r, "id"), in)
		respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
	})

	r.Get("/customers", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.loyalty.Search(r.Context(), CurrentUser(r.Context()).CompanyID, r.URL.Query().Get("q"))
		respond(w, list, err, http.StatusOK)
	})
	r.Post("/customers", func(w http.ResponseWriter, r *http.Request) {
		var in loyalty.Input
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		c, err := s.loyalty.Create(r.Context(), actorOf(u), u.CompanyID, in)
		respond(w, c, err, http.StatusCreated)
	})
	r.Put("/customers/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in loyalty.Input
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		err := s.loyalty.Update(r.Context(), actorOf(u), u.CompanyID, chi.URLParam(r, "id"), in)
		respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
	})
	r.Post("/customers/{id}/points", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Delta  int    `json:"delta"`
			Reason string `json:"reason"`
		}
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		err := s.loyalty.Adjust(r.Context(), actorOf(u), u.CompanyID, chi.URLParam(r, "id"), in.Delta, in.Reason)
		respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
	})
	r.Get("/stores", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		rows, err := s.pool.Query(r.Context(), `SELECT id::text, code, name FROM stores WHERE company_id=$1 AND is_active
			AND ($2='' OR id::text=$2) ORDER BY code`, u.CompanyID, scopeFor(u))
		if err != nil {
			fail(w, err)
			return
		}
		defer rows.Close()
		out := []catalog.Ref{}
		for rows.Next() {
			var x catalog.Ref
			if err := rows.Scan(&x.ID, &x.Code, &x.Name); err != nil {
				fail(w, err)
				return
			}
			out = append(out, x)
		}
		writeJSON(w, http.StatusOK, out)
	})
	r.Post("/stock/adjustments", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			StoreID   string `json:"store_id"`
			ProductID string `json:"product_id"`
			QtyAfter  string `json:"qty_after"`
			Reason    string `json:"reason"`
		}
		if !decode(w, r, &in) {
			return
		}
		u := CurrentUser(r.Context())
		if u.Role != identity.RoleOwner { // Store Manager only acts on own store (FR-A1-04)
			in.StoreID = *u.StoreID
		}
		err := s.catalog.Adjust(r.Context(), actorOf(u), u.CompanyID, in.StoreID, in.ProductID, in.QtyAfter, in.Reason)
		respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
	})

	r.Get("/sales", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		from, to, ok := dateRange(w, r)
		if !ok {
			return
		}
		list, err := s.sales.List(r.Context(), u.CompanyID, storeScope(r), from, to)
		respond(w, list, err, http.StatusOK)
	})
	r.Post("/sales/{number}/void", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		var in struct{ Reason string }
		if !decode(w, r, &in) {
			return
		}
		err := s.sales.Void(r.Context(), actorOf(u), u.CompanyID, scopeFor(u), chi.URLParam(r, "number"), in.Reason)
		respond(w, map[string]string{"status": "void"}, err, http.StatusOK)
	})
	r.Get("/reports/sales.xlsx", s.exportSales)
	r.Get("/reports/sales", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		from, to, ok := dateRange(w, r)
		if !ok {
			return
		}
		sum, err := s.reports.Sales(r.Context(), u.CompanyID, storeScope(r), from, to)
		respond(w, sum, err, http.StatusOK)
	})
}

// dateRange reads ?from&to (YYYY-MM-DD), defaulting both to today in Jakarta.
func dateRange(w http.ResponseWriter, r *http.Request) (from, to string, ok bool) {
	today := numbering.BusinessDate(time.Now())
	from, to = r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if from == "" {
		from = today
	}
	if to == "" {
		to = today
	}
	for _, d := range []string{from, to} {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "format tanggal harus YYYY-MM-DD")
			return "", "", false
		}
	}
	return from, to, true
}

func scopeFor(u *identity.User) string {
	if u.Role == identity.RoleOwner || u.StoreID == nil {
		return ""
	}
	return *u.StoreID
}

func respond(w http.ResponseWriter, v any, err error, code int) {
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, code, v)
}

// mountPriceLists: everyone in the admin area can read; only the Owner changes lists and store assignment (FR-A7-04).
func (s *Server) mountPriceLists(r chi.Router) {
	owner := RequireRole(identity.RoleOwner)
	r.Get("/price-lists", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		lists, err := s.catalog.PriceLists(r.Context(), u.CompanyID)
		if err != nil {
			fail(w, err)
			return
		}
		stores, err := s.catalog.StorePriceLists(r.Context(), u.CompanyID)
		respond(w, map[string]any{"lists": lists, "stores": stores}, err, http.StatusOK)
	})
	save := func(id func(*http.Request) string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u := CurrentUser(r.Context())
			var in catalog.PriceListInput
			if !decode(w, r, &in) {
				return
			}
			got, err := s.catalog.SavePriceList(r.Context(), actorOf(u), u.CompanyID, id(r), in)
			respond(w, map[string]string{"id": got}, err, http.StatusOK)
		}
	}
	r.With(owner).Post("/price-lists", save(func(*http.Request) string { return "" }))
	r.With(owner).Put("/price-lists/{id}", save(func(r *http.Request) string { return chi.URLParam(r, "id") }))
	r.Get("/price-lists/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		list, err := s.catalog.PriceItems(r.Context(), CurrentUser(r.Context()).CompanyID, chi.URLParam(r, "id"))
		respond(w, list, err, http.StatusOK)
	})
	r.With(owner).Put("/price-lists/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		var in catalog.PriceItemInput
		if !decode(w, r, &in) {
			return
		}
		err := s.catalog.SetPriceItem(r.Context(), actorOf(u), u.CompanyID, chi.URLParam(r, "id"), in)
		respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
	})
	r.With(owner).Put("/stores/{id}/price-list", func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r.Context())
		var in struct {
			PriceListID string `json:"price_list_id"`
		}
		if !decode(w, r, &in) {
			return
		}
		err := s.catalog.AssignStorePriceList(r.Context(), actorOf(u), u.CompanyID, chi.URLParam(r, "id"), in.PriceListID)
		respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
	})
}
