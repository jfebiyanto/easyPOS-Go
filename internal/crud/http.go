package crud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
)

// Binder menghubungkan mesin dengan sesi/izin milik httpapi.
type Binder struct {
	E     *Engine
	Actor func(r *http.Request) Actor
	Route func(key string) func(http.Handler) http.Handler
}

func (b *Binder) Ctx(r *http.Request) *Ctx {
	return &Ctx{Ctx: r.Context(), Q: b.E.Pool, A: b.Actor(r), E: b.E}
}

// DecodeMap membaca body JSON object dengan angka tetap presisi.
func DecodeMap(r *http.Request) (map[string]any, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil || m == nil {
		return nil, &APIError{Status: 422, Msg: "Body harus berupa objek JSON."}
	}
	return m, nil
}

func WriteErr(w http.ResponseWriter, err error) {
	var ae *APIError
	if errors.As(err, &ae) {
		body := map[string]any{"message": ae.Msg}
		if ae.Errs != nil {
			body["errors"] = ae.Errs
		}
		httpx.JSON(w, ae.Status, body)
		return
	}
	slog.Error("crud", "err", err)
	httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
}

func (b *Binder) Mount(r chi.Router, res *Resource) {
	read := res.Key + ".index"
	if res.Show {
		read = res.Key + ".show"
	}
	p := "/" + res.Path
	r.With(b.Route(res.Key+".index")).Get(p, b.list(res))
	r.With(b.Route(read)).Get(p+"/{id}", b.get(res))
	r.With(b.Route(res.Key+".store")).Post(p, b.create(res))
	r.With(b.Route(res.Key+".update")).Put(p+"/{id}", b.update(res))
	if !res.NoDelete {
		r.With(b.Route(res.Key+".destroy")).Delete(p+"/{id}", b.destroy(res))
	}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (b *Binder) list(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := b.Ctx(r)
		qs := r.URL.Query()
		page, _ := strconv.Atoi(qs.Get("page"))
		per, _ := strconv.Atoi(qs.Get("per_page"))
		if page < 1 {
			page = 1
		}
		if per < 1 {
			per = 25
		}
		if per > 100 {
			per = 100
		}
		where, args := []string{"t.company_id=$1"}, []any{c.A.CompanyID}
		if q := strings.TrimSpace(qs.Get("q")); q != "" {
			args = append(args, "%"+escapeLike(q)+"%")
			ph := fmt.Sprintf("$%d", len(args))
			var ors []string
			for _, col := range res.Search {
				ors = append(ors, "t."+col+" ILIKE "+ph)
			}
			if res.SearchExtra != "" {
				ors = append(ors, strings.ReplaceAll(res.SearchExtra, "{q}", ph))
			}
			if len(ors) > 0 {
				where = append(where, "("+strings.Join(ors, " OR ")+")")
			}
		}
		filters := append([]string{"is_active"}, res.Filters...)
		for _, name := range filters {
			v := qs.Get(name)
			if v == "" {
				continue
			}
			var f *Field
			for i := range res.Fields {
				if res.Fields[i].Name == name {
					f = &res.Fields[i]
				}
			}
			if f == nil {
				continue
			}
			var arg any = v
			if f.Kind == KBool {
				bv, err := strconv.ParseBool(v)
				if err != nil {
					continue
				}
				arg = bv
			}
			args = append(args, arg)
			where = append(where, fmt.Sprintf(`t."%s"=$%d%s`, f.col(), len(args), f.cast()))
		}
		w8 := strings.Join(where, " AND ")
		var total int
		if err := c.Q.QueryRow(c.Ctx, "SELECT count(*) FROM "+res.Table+" t WHERE "+w8, args...).Scan(&total); err != nil {
			WriteErr(w, mapPgErr(err))
			return
		}
		order := res.Order
		if order == "" {
			order = "t.created_at DESC"
		}
		// ?sort=<field>&dir=asc|desc: hanya field kolom nyata (bukan Virtual); selain itu diabaikan.
		for _, f := range res.Fields {
			if f.Name != qs.Get("sort") || f.Virtual {
				continue
			}
			col := fmt.Sprintf(`t."%s"`, f.col())
			if f.Kind == KStr {
				col = "lower(" + col + ")"
			}
			dir := "ASC"
			if qs.Get("dir") == "desc" {
				dir = "DESC"
			}
			order = col + " " + dir
			break
		}
		sql := fmt.Sprintf("SELECT %s FROM %s t WHERE %s ORDER BY %s, t.id LIMIT %d OFFSET %d", res.selectList(), res.Table, w8, order, per, (page-1)*per)
		rows, err := c.Q.Query(c.Ctx, sql, args...)
		if err != nil {
			WriteErr(w, mapPgErr(err))
			return
		}
		recs := []Record{}
		for rows.Next() {
			targets, build := res.scanTargets()
			if err := rows.Scan(targets...); err != nil {
				rows.Close()
				WriteErr(w, err)
				return
			}
			recs = append(recs, build())
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			WriteErr(w, err)
			return
		}
		if err := b.E.finish(c, res, recs, false); err != nil {
			WriteErr(w, err)
			return
		}
		last := (total + per - 1) / per
		if last < 1 {
			last = 1
		}
		httpx.JSON(w, 200, map[string]any{"data": recs, "meta": map[string]int{"page": page, "per_page": per, "total": total, "last_page": last}})
	}
}

// One memuat satu record lengkap (expand + decorate detail).
func (b *Binder) One(c *Ctx, res *Resource, id string) (Record, error) {
	rec, err := b.E.Load(c, res, id)
	if err != nil {
		return nil, err
	}
	if err := b.E.finish(c, res, []Record{rec}, true); err != nil {
		return nil, err
	}
	return rec, nil
}

func (b *Binder) get(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec, err := b.One(b.Ctx(r), res, chi.URLParam(r, "id"))
		if err != nil {
			WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": rec})
	}
}

func (b *Binder) create(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := DecodeMap(r)
		if err != nil {
			WriteErr(w, err)
			return
		}
		c := b.Ctx(r)
		id, err := b.E.Save(c, res, in, nil)
		if err != nil {
			WriteErr(w, err)
			return
		}
		rec, err := b.One(c, res, id)
		if err != nil {
			WriteErr(w, err)
			return
		}
		httpx.JSON(w, 201, map[string]any{"data": rec})
	}
}

func (b *Binder) update(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := b.Ctx(r)
		old, err := b.E.Load(c, res, chi.URLParam(r, "id"))
		if err != nil {
			WriteErr(w, err)
			return
		}
		if res.Locked != nil {
			if msg := res.Locked(c, old); msg != "" {
				WriteErr(w, Conflict(msg))
				return
			}
		}
		in, err := DecodeMap(r)
		if err != nil {
			WriteErr(w, err)
			return
		}
		id, err := b.E.Save(c, res, in, old)
		if err != nil {
			WriteErr(w, err)
			return
		}
		rec, err := b.One(c, res, id)
		if err != nil {
			WriteErr(w, err)
			return
		}
		httpx.JSON(w, 200, map[string]any{"data": rec})
	}
}

func (b *Binder) destroy(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := b.E.Delete(b.Ctx(r), res, chi.URLParam(r, "id")); err != nil {
			WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
