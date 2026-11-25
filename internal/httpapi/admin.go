package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/db"
	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/numbering"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/database"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
	"github.com/jfebiyanto/easyPOS-Go/internal/settings"
)

func serverError(w http.ResponseWriter) {
	httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
}

func entry(sess *identity.Session, action, desc string, props map[string]any) audit.Entry {
	return audit.Entry{CompanyID: sess.User.CompanyID, ActorID: sess.User.ID, ActorName: sess.User.Name,
		Action: action, Description: desc, Properties: props, StoreID: identity.StoreID(sess)}
}

// ---- Konfigurasi Sistem ----

func (s *server) getSettings(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	vals, err := settings.GetAll(r.Context(), s.Pool, sess.User.CompanyID)
	if err != nil {
		serverError(w)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"settings": vals, "definitions": settings.Definitions})
}

func (s *server) putSettings(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var in struct {
		Settings map[string]any `json:"settings"`
	}
	if err := httpx.Decode(r, &in); err != nil || len(in.Settings) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Body harus berisi objek settings.")
		return
	}
	if err := settings.Validate(in.Settings); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	err := database.InTx(r.Context(), s.Pool, func(tx pgx.Tx) error {
		if err := settings.Set(r.Context(), tx, sess.User.CompanyID, sess.User.ID, in.Settings, time.Now()); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, entry(sess, "settings.updated", "Konfigurasi Sistem diubah", map[string]any{"changes": in.Settings}))
	})
	if err != nil {
		serverError(w)
		return
	}
	s.getSettings(w, r)
}

// ---- Penomoran ----

type numberingDTO struct {
	TransactionType string  `json:"transaction_type"`
	Label           string  `json:"label"`
	Prefix          string  `json:"prefix"`
	Padding         int16   `json:"padding"`
	Separator       string  `json:"separator"`
	ResetPeriod     string  `json:"reset_period"`
	LastPeriod      *string `json:"last_period"`
	LastNumber      int64   `json:"last_number"`
	Example         string  `json:"example"`
}

func (s *server) getNumberings(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	now := time.Now().In(s.Loc)
	if err := numbering.EnsureAll(r.Context(), s.Pool, sess.User.CompanyID, now.UTC()); err != nil {
		serverError(w)
		return
	}
	rows, err := db.New(s.Pool).ListNumberings(r.Context(), sess.User.CompanyID)
	if err != nil {
		serverError(w)
		return
	}
	byType := map[string]db.NumberingConfig{}
	for _, c := range rows {
		byType[c.TransactionType] = c
	}
	out := []numberingDTO{}
	for _, t := range numbering.Types {
		c, ok := byType[t.Key]
		if !ok {
			continue
		}
		d := numberingDTO{TransactionType: t.Key, Label: t.Label, Prefix: c.Prefix, Padding: c.Padding, Separator: c.Separator,
			ResetPeriod: c.ResetPeriod, LastNumber: c.LastNumber, Example: numbering.Format(c, c.LastNumber+1, now)}
		if c.LastPeriod.Valid {
			d.LastPeriod = &c.LastPeriod.String
		}
		out = append(out, d)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"numberings": out})
}

func (s *server) putNumberings(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var in struct {
		Items []struct {
			TransactionType string `json:"transaction_type"`
			Prefix          string `json:"prefix"`
			Padding         int16  `json:"padding"`
			Separator       string `json:"separator"`
			ResetPeriod     string `json:"reset_period"`
		} `json:"items"`
	}
	if err := httpx.Decode(r, &in); err != nil || len(in.Items) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Body harus berisi items.")
		return
	}
	for _, it := range in.Items {
		if !numbering.Known(it.TransactionType) || len(it.Prefix) > 20 || it.Padding < 1 || it.Padding > 12 ||
			len(it.Separator) > 1 || !numbering.ResetPeriods[it.ResetPeriod] {
			httpx.Error(w, http.StatusUnprocessableEntity, "Konfigurasi penomoran tidak valid untuk "+it.TransactionType+".")
			return
		}
	}
	now := time.Now().UTC()
	err := database.InTx(r.Context(), s.Pool, func(tx pgx.Tx) error {
		if err := numbering.EnsureAll(r.Context(), tx, sess.User.CompanyID, now); err != nil {
			return err
		}
		q := db.New(tx)
		for _, it := range in.Items {
			if err := q.UpdateNumberingConfig(r.Context(), db.UpdateNumberingConfigParams{CompanyID: sess.User.CompanyID,
				TransactionType: it.TransactionType, Prefix: it.Prefix, Padding: it.Padding, Separator: it.Separator,
				ResetPeriod: it.ResetPeriod, UpdatedAt: nullTime(now)}); err != nil {
				return err
			}
		}
		return audit.Log(r.Context(), tx, entry(sess, "numberings.updated", "Penomoran diubah", map[string]any{"items": in.Items}))
	})
	if err != nil {
		serverError(w)
		return
	}
	s.getNumberings(w, r)
}

func (s *server) nextNumber(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	typ := chi.URLParam(r, "type")
	if !numbering.Known(typ) {
		httpx.Error(w, http.StatusNotFound, "Tipe transaksi tidak dikenal.")
		return
	}
	var number string
	err := database.InTx(r.Context(), s.Pool, func(tx pgx.Tx) error {
		var err error
		if number, err = numbering.Next(r.Context(), tx, sess.User.CompanyID, typ, time.Now().In(s.Loc)); err != nil {
			return err
		}
		return audit.Log(r.Context(), tx, entry(sess, "numberings.next", "Nomor "+typ+" diambil: "+number, map[string]any{"type": typ, "number": number}))
	})
	if errors.Is(err, numbering.ErrUnknownType) {
		httpx.Error(w, http.StatusNotFound, "Tipe transaksi tidak dikenal.")
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"number": number})
}
