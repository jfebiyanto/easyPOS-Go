package crud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/jfebiyanto/easyPOS-Go/internal/audit"
	"github.com/jfebiyanto/easyPOS-Go/internal/db"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/validate"
)

type Record map[string]any

type Actor struct {
	CompanyID, UserID, StoreID uuid.UUID
	Name, Role                 string
}

// APIError adalah error yang diterjemahkan langsung ke respons HTTP.
type APIError struct {
	Status int
	Msg    string
	Errs   validate.Errors
}

func (e *APIError) Error() string { return e.Msg }

func Conflict(msg string) *APIError { return &APIError{Status: 409, Msg: msg} }
func NotFound() *APIError           { return &APIError{Status: 404, Msg: "Data tidak ditemukan."} }
func Invalid(errs validate.Errors) *APIError {
	return &APIError{Status: 422, Msg: validate.Message, Errs: errs}
}

// Ctx dibawa ke semua hook; Q = pool atau tx.
type Ctx struct {
	Ctx context.Context
	Q   db.DBTX
	A   Actor
	E   *Engine
}

type Expand struct {
	Field, As, Table string
	Cols             []string // default code,name
}

type Resource struct {
	Path, Table, Entity, Noun, Key string
	Show                           bool // GET detail memakai route key .show
	Fields                         []Field
	Search                         []string
	Filters                        []string
	Order                          string
	Expand                         []Expand
	SearchExtra                    string // SQL tambahan untuk q, memakai {q} (placeholder parameter) dan alias t
	NoDelete                       bool
	Singleton                      bool // baris tunggal: id = company_id (company_details)

	Validate    func(c *Ctx, merged, old Record, in map[string]any, errs validate.Errors) error
	BeforeWrite func(c *Ctx, merged, old Record, in map[string]any) (map[string]any, error)
	AfterSave   func(c *Ctx, id string, merged, old Record, in map[string]any) error
	Locked      func(c *Ctx, rec Record) string
	CanDelete   func(c *Ctx, rec Record) (string, error)
	Decorate    func(c *Ctx, recs []Record, detail bool) error
}

type Engine struct {
	Pool  *pgxpool.Pool
	usage usageCache
}

func NewEngine(pool *pgxpool.Pool) *Engine { return &Engine{Pool: pool} }

const tsFmt = `'YYYY-MM-DD"T"HH24:MI:SS'`

func (f Field) selectExpr(alias string) string {
	c := alias + `."` + f.col() + `"`
	switch f.Kind {
	case KDec:
		if f.Scale > 0 {
			return fmt.Sprintf("round(%s, %d)::text", c, f.Scale) // skala keluaran = skala validasi, bukan skala kolom
		}
		return c + "::text"
	case KUUID:
		return c + "::text"
	case KDate:
		return "to_char(" + c + ",'YYYY-MM-DD')"
	case KTime:
		return "to_char(" + c + "," + tsFmt + ")"
	}
	return c
}

func (f Field) cast() string {
	switch f.Kind {
	case KDec:
		return "::numeric"
	case KUUID:
		return "::uuid"
	case KDate:
		return "::date"
	case KTime:
		return "::timestamp"
	}
	return ""
}

func (r *Resource) selectList() string {
	cols := []string{`t.id::text`, `to_char(t.created_at,` + tsFmt + `)`, `to_char(t.updated_at,` + tsFmt + `)`}
	for _, f := range r.Fields {
		if !f.Virtual {
			cols = append(cols, f.selectExpr("t"))
		}
	}
	return strings.Join(cols, ", ")
}

func (r *Resource) scanTargets() ([]any, func() Record) {
	var id, ca, ua *string
	targets := []any{&id, &ca, &ua}
	type slot struct {
		name string
		kind Kind
		ptr  any
	}
	var slots []slot
	for _, f := range r.Fields {
		if f.Virtual {
			continue
		}
		var p any
		switch f.Kind {
		case KInt:
			p = new(*int64)
		case KBool:
			p = new(*bool)
		default:
			p = new(*string)
		}
		slots = append(slots, slot{f.Name, f.Kind, p})
		targets = append(targets, p)
	}
	return targets, func() Record {
		rec := Record{"id": deref(id), "created_at": deref(ca), "updated_at": deref(ua)}
		for _, s := range slots {
			switch p := s.ptr.(type) {
			case **int64:
				if *p != nil {
					rec[s.name] = **p
				} else {
					rec[s.name] = nil
				}
			case **bool:
				if *p != nil {
					rec[s.name] = **p
				} else {
					rec[s.name] = nil
				}
			case **string:
				if *p != nil {
					rec[s.name] = **p
				} else {
					rec[s.name] = nil
				}
			}
		}
		return rec
	}
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// Load membaca satu baris milik company; ErrNotFound bila tidak ada / milik company lain.
func (e *Engine) Load(c *Ctx, r *Resource, id string) (Record, error) {
	if r.Singleton {
		id = c.A.CompanyID.String()
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, NotFound()
	}
	targets, build := r.scanTargets()
	cond, args := "t.company_id=$1 AND t.id=$2", []any{c.A.CompanyID, id}
	if r.Singleton {
		cond, args = "t.id=$1", []any{c.A.CompanyID}
	}
	err := c.Q.QueryRow(c.Ctx, "SELECT "+r.selectList()+" FROM "+r.Table+" t WHERE "+cond, args...).Scan(targets...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, NotFound()
	}
	if err != nil {
		return nil, err
	}
	return build(), nil
}

func (e *Engine) expand(c *Ctx, r *Resource, recs []Record) error {
	for _, x := range r.Expand {
		cols := x.Cols
		if len(cols) == 0 {
			cols = []string{"code", "name"}
		}
		seen := map[string]bool{}
		var ids []string
		for _, rec := range recs {
			if v, ok := rec[x.Field].(string); ok && !seen[v] {
				seen[v] = true
				ids = append(ids, v)
			}
		}
		found := map[string]Record{}
		if len(ids) > 0 {
			sel := "id::text"
			for _, col := range cols {
				sel += "," + col + "::text"
			}
			rows, err := c.Q.Query(c.Ctx, "SELECT "+sel+" FROM "+x.Table+" WHERE company_id=$1 AND id = ANY($2::uuid[])", c.A.CompanyID, ids)
			if err != nil {
				return err
			}
			for rows.Next() {
				vals := make([]*string, len(cols)+1)
				ptrs := make([]any, len(vals))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					rows.Close()
					return err
				}
				m := Record{"id": *vals[0]}
				for i, col := range cols {
					m[col] = deref(vals[i+1])
				}
				found[*vals[0]] = m
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		for _, rec := range recs {
			if v, ok := rec[x.Field].(string); ok {
				if m, ok := found[v]; ok {
					rec[x.As] = m
				}
			}
			if _, ok := rec[x.As]; !ok {
				rec[x.As] = nil
			}
		}
	}
	return nil
}

func (e *Engine) finish(c *Ctx, r *Resource, recs []Record, detail bool) error {
	if err := e.expand(c, r, recs); err != nil {
		return err
	}
	if r.Decorate != nil {
		return r.Decorate(c, recs, detail)
	}
	return nil
}

// ---- parsing & validasi ----

func asString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	}
	return "", false
}

func (f Field) parse(v any) (any, string) {
	if v == nil {
		return nil, ""
	}
	switch f.Kind {
	case KStr:
		s, ok := v.(string)
		if !ok {
			return nil, "Harus berupa teks."
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, ""
		}
		if f.Upper {
			s = strings.ToUpper(s)
		}
		if f.Max > 0 && len([]rune(s)) > f.Max {
			return nil, fmt.Sprintf("Maksimal %d karakter.", f.Max)
		}
		if f.Email {
			if a, err := mail.ParseAddress(s); err != nil || a.Address != s {
				return nil, "Format email tidak valid."
			}
		}
		if f.Pattern != nil && !f.Pattern.MatchString(s) {
			return nil, f.PatternMsg
		}
		if len(f.Enum) > 0 && !contains(f.Enum, s) {
			return nil, "Pilihan tidak valid."
		}
		return s, ""
	case KInt:
		s, ok := asString(v)
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if !ok || err != nil {
			return nil, "Harus berupa bilangan bulat."
		}
		return n, f.checkRange(decimal.NewFromInt(n))
	case KBool:
		switch x := v.(type) {
		case bool:
			return x, ""
		case string:
			if b, err := strconv.ParseBool(x); err == nil {
				return b, ""
			}
		}
		return nil, "Harus berupa ya/tidak."
	case KDec:
		s, ok := asString(v)
		d, err := decimal.NewFromString(strings.TrimSpace(s))
		if !ok || err != nil {
			return nil, "Harus berupa angka."
		}
		if d.Exponent() < -int32(f.Scale) && !d.Equal(d.Round(int32(f.Scale))) {
			return nil, fmt.Sprintf("Maksimal %d angka desimal.", f.Scale)
		}
		if m := f.checkRange(d); m != "" {
			return nil, m
		}
		return d.StringFixed(int32(f.Scale)), ""
	case KUUID:
		s, ok := v.(string)
		if _, err := uuid.Parse(s); !ok || err != nil {
			return nil, "ID tidak valid."
		}
		return strings.ToLower(s), ""
	case KDate:
		s, ok := v.(string)
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(s)); !ok || err != nil {
			return nil, "Tanggal harus berformat YYYY-MM-DD."
		} else {
			return t.Format("2006-01-02"), ""
		}
	case KTime:
		s, _ := v.(string)
		for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02", time.RFC3339} {
			if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
				return t.Format("2006-01-02T15:04:05"), ""
			}
		}
		return nil, "Waktu tidak valid."
	}
	return v, ""
}

func (f Field) checkRange(d decimal.Decimal) string {
	if f.GT0 && !d.IsPositive() {
		return "Harus lebih dari 0."
	}
	if f.MinV != "" && d.LessThan(decimal.RequireFromString(f.MinV)) {
		return "Minimal " + f.MinV + "."
	}
	if f.MaxV != "" && d.GreaterThan(decimal.RequireFromString(f.MaxV)) {
		return "Maksimal " + f.MaxV + "."
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Parse memvalidasi input terhadap field; old = nil untuk create. Mengembalikan record gabungan.
func (e *Engine) Parse(c *Ctx, r *Resource, in map[string]any, old Record) (Record, validate.Errors, error) {
	errs := validate.Errors{}
	merged := Record{}
	for k, v := range old {
		merged[k] = v
	}
	provided := map[string]bool{}
	for _, f := range r.Fields {
		if f.ReadOnly {
			continue
		}
		raw, has := in[f.Name]
		if !has {
			if old == nil && f.HasDefault {
				merged[f.Name] = f.Default
			} else if old == nil {
				merged[f.Name] = nil
			}
			continue
		}
		provided[f.Name] = true
		val, msg := f.parse(raw)
		if msg != "" {
			errs.Add(f.Name, msg)
			continue
		}
		merged[f.Name] = val
	}
	for _, f := range r.Fields {
		if f.ReadOnly {
			continue
		}
		v := merged[f.Name]
		if f.Required && (v == nil) && !errs.Has(f.Name) {
			errs.Add(f.Name, "Wajib diisi.")
			continue
		}
		if v == nil || errs.Has(f.Name) || !(provided[f.Name] || old == nil) {
			continue
		}
		if f.Ref != nil {
			ok, err := e.refExists(c, f, v)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				errs.Add(f.Name, "Data tidak ditemukan.")
				continue
			}
		}
		if f.Unique {
			var id any = uuid.Nil.String()
			if old != nil {
				id = old["id"]
			}
			var n int
			sql := fmt.Sprintf(`SELECT count(*) FROM %s WHERE company_id=$1 AND "%s"=$2%s AND id<>$3`, r.Table, f.col(), f.cast())
			if err := c.Q.QueryRow(c.Ctx, sql, c.A.CompanyID, v, id).Scan(&n); err != nil {
				return nil, nil, err
			}
			if n > 0 {
				errs.Add(f.Name, "Sudah digunakan.")
			}
		}
	}
	if r.Validate != nil && !errs.Any() {
		if err := r.Validate(c, merged, old, in, errs); err != nil {
			return nil, nil, err
		}
	}
	return merged, errs, nil
}

func (e *Engine) refExists(c *Ctx, f Field, v any) (bool, error) {
	col := f.Ref.Col
	if col == "" {
		col = "id"
	}
	cast := ""
	if col == "id" {
		cast = "::uuid"
	}
	sql := fmt.Sprintf(`SELECT count(*) FROM %s WHERE company_id=$1 AND "%s"=$2%s`, f.Ref.Table, col, cast)
	if f.Ref.Active {
		sql += " AND is_active"
	}
	var n int
	err := c.Q.QueryRow(c.Ctx, sql, c.A.CompanyID, v).Scan(&n)
	return n > 0, err
}

// ---- tulis ----

func (e *Engine) writeCols(r *Resource, merged Record) (cols []string, casts []string, args []any) {
	for _, f := range r.Fields {
		if f.Virtual || f.ReadOnly {
			continue
		}
		cols = append(cols, f.col())
		casts = append(casts, f.cast())
		args = append(args, merged[f.Name])
	}
	return
}

func (e *Engine) diff(r *Resource, old, merged Record) map[string]any {
	oldM, newM := map[string]any{}, map[string]any{}
	for _, f := range r.Fields {
		if f.ReadOnly {
			continue
		}
		if old == nil || fmt.Sprint(old[f.Name]) != fmt.Sprint(merged[f.Name]) {
			if old != nil {
				oldM[f.Name] = old[f.Name]
			}
			newM[f.Name] = merged[f.Name]
		}
	}
	return map[string]any{"old": oldM, "new": newM}
}

// Save membuat (old == nil) atau mengubah satu baris dalam transaksi + audit. Mengembalikan id.
func (e *Engine) Save(c *Ctx, r *Resource, in map[string]any, old Record) (string, error) {
	merged, errs, err := e.Parse(c, r, in, old)
	if err != nil {
		return "", err
	}
	if errs.Any() {
		return "", Invalid(errs)
	}
	var id string
	err = pgx.BeginFunc(c.Ctx, e.Pool, func(tx pgx.Tx) error {
		tc := &Ctx{Ctx: c.Ctx, Q: tx, A: c.A, E: e}
		cols, casts, args := e.writeCols(r, merged)
		if r.BeforeWrite != nil {
			extra, err := r.BeforeWrite(tc, merged, old, in)
			if err != nil {
				return err
			}
			for k, v := range extra {
				cols, casts, args = append(cols, k), append(casts, ""), append(args, v)
			}
		}
		now := time.Now().UTC()
		if old == nil {
			nid, _ := uuid.NewV7()
			id = nid.String()
			names, ph, vals := []string{"id", "company_id", "created_at", "updated_at"}, []string{"$1::uuid", "$2", "$3", "$3"}, []any{id, c.A.CompanyID, now}
			for i, col := range cols {
				names = append(names, `"`+col+`"`)
				vals = append(vals, args[i])
				ph = append(ph, fmt.Sprintf("$%d%s", len(vals), casts[i]))
			}
			if _, err := tx.Exec(c.Ctx, "INSERT INTO "+r.Table+" ("+strings.Join(names, ",")+") VALUES ("+strings.Join(ph, ",")+")", vals...); err != nil {
				return err
			}
		} else {
			id = old["id"].(string)
			sets, vals := []string{"updated_at=$1"}, []any{now}
			for i, col := range cols {
				vals = append(vals, args[i])
				sets = append(sets, fmt.Sprintf(`"%s"=$%d%s`, col, len(vals), casts[i]))
			}
			vals = append(vals, c.A.CompanyID, id)
			sql := fmt.Sprintf("UPDATE %s SET %s WHERE company_id=$%d AND id=$%d::uuid", r.Table, strings.Join(sets, ","), len(vals)-1, len(vals))
			if r.Singleton {
				sql = fmt.Sprintf("UPDATE %s SET %s WHERE id=$%d::uuid", r.Table, strings.Join(sets, ","), len(vals)-1)
				vals = vals[:len(vals)-1]
			}
			if _, err := tx.Exec(c.Ctx, sql, vals...); err != nil {
				return err
			}
		}
		if r.AfterSave != nil {
			if err := r.AfterSave(tc, id, merged, old, in); err != nil {
				return err
			}
		}
		verb := "created"
		if old != nil {
			verb = "updated"
		}
		return audit.Log(c.Ctx, tx, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, StoreID: c.A.StoreID,
			Action: r.Entity + "." + verb, SubjectType: r.Entity, SubjectID: uuid.MustParse(id),
			Description: fmt.Sprintf("%s %s: %v", r.Noun, verb, merged[displayField(r)]), Properties: e.diff(r, old, merged)})
	})
	return id, mapPgErr(err)
}

func displayField(r *Resource) string {
	for _, n := range []string{"code", "name", "sku", "username"} {
		for _, f := range r.Fields {
			if f.Name == n {
				return n
			}
		}
	}
	return "id"
}

func mapPgErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23503":
			return Conflict("Data masih dipakai oleh data lain.")
		case "23505":
			return Invalid(validate.Errors{"code": {"Sudah digunakan."}})
		}
	}
	return err
}

// Delete menghapus baris bila tidak dipakai; ditolak 409 bila direferensikan.
func (e *Engine) Delete(c *Ctx, r *Resource, id string) error {
	rec, err := e.Load(c, r, id)
	if err != nil {
		return err
	}
	if r.Locked != nil {
		if msg := r.Locked(c, rec); msg != "" {
			return Conflict(msg)
		}
	}
	if r.CanDelete != nil {
		if msg, err := r.CanDelete(c, rec); err != nil || msg != "" {
			if err != nil {
				return err
			}
			return Conflict(msg)
		}
	}
	if msg, err := e.usageMessage(c, r, id); err != nil || msg != "" {
		if err != nil {
			return err
		}
		return Conflict(msg)
	}
	err = pgx.BeginFunc(c.Ctx, e.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(c.Ctx, "DELETE FROM "+r.Table+" WHERE company_id=$1 AND id=$2", c.A.CompanyID, id); err != nil {
			return err
		}
		return audit.Log(c.Ctx, tx, audit.Entry{CompanyID: c.A.CompanyID, ActorID: c.A.UserID, ActorName: c.A.Name, StoreID: c.A.StoreID,
			Action: r.Entity + ".deleted", SubjectType: r.Entity, SubjectID: uuid.MustParse(id),
			Description: fmt.Sprintf("%s dihapus: %v", r.Noun, rec[displayField(r)]), Properties: map[string]any{"old": rec}})
	})
	return mapPgErr(err)
}

// Parse mengekspor parser satu field (dipakai handler khusus).
func (f Field) Parse(v any) (any, string) { return f.parse(v) }

// InTx menjalankan fn dalam transaksi dengan Ctx baru.
func (e *Engine) InTx(c *Ctx, fn func(tc *Ctx) error) error {
	return pgx.BeginFunc(c.Ctx, e.Pool, func(tx pgx.Tx) error {
		return fn(&Ctx{Ctx: c.Ctx, Q: tx, A: c.A, E: e})
	})
}
