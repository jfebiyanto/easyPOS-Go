// Package crud: mesin CRUD deklaratif untuk master data (validasi, multi-company, audit, cek pemakaian).
package crud

import "regexp"

type Kind int

const (
	KStr Kind = iota
	KInt
	KBool
	KDec
	KUUID
	KDate
	KTime
)

type Ref struct {
	Table  string
	Col    string // default "id"
	Active bool   // wajib is_active = true
}

// Field mendeskripsikan satu kolom/field request. Dibangun lewat builder (S, I, B, ...).
type Field struct {
	Name, Label string
	Kind        Kind
	Col         string // default = Name
	Required    bool
	Max         int    // panjang string maksimum
	MinV, MaxV  string // batas numerik (desimal/int), "" = tanpa batas
	GT0         bool   // harus > 0
	Scale       int    // desimal
	Enum        []string
	Ref         *Ref
	Unique      bool // unik per company
	Upper       bool
	Email       bool
	Pattern     *regexp.Regexp
	PatternMsg  string
	Virtual     bool // divalidasi tapi tidak ditulis ke kolom tabel (ditangani hook)
	ReadOnly    bool // hanya keluar (mis. sap_*), tidak pernah ditulis dari API
	Default     any
	HasDefault  bool
}

func (f Field) col() string {
	if f.Col != "" {
		return f.Col
	}
	return f.Name
}

func S(name, label string, max int) Field {
	return Field{Name: name, Label: label, Kind: KStr, Max: max}
}
func I(name, label string) Field { return Field{Name: name, Label: label, Kind: KInt} }
func B(name, label string, def bool) Field {
	return Field{Name: name, Label: label, Kind: KBool, Default: def, HasDefault: true}
}
func D(name, label string, scale int) Field {
	return Field{Name: name, Label: label, Kind: KDec, Scale: scale}
}
func U(name, label, refTable string) Field {
	return Field{Name: name, Label: label, Kind: KUUID, Ref: &Ref{Table: refTable}}
}
func Dt(name, label string) Field { return Field{Name: name, Label: label, Kind: KDate} }
func Ts(name, label string) Field { return Field{Name: name, Label: label, Kind: KTime} }

func (f Field) Req() Field                { f.Required = true; return f }
func (f Field) Uniq() Field               { f.Unique = true; return f }
func (f Field) Up() Field                 { f.Upper = true; return f }
func (f Field) Mail() Field               { f.Email = true; return f }
func (f Field) Virt() Field               { f.Virtual = true; return f }
func (f Field) RO() Field                 { f.ReadOnly = true; return f }
func (f Field) Active() Field             { f.Ref.Active = true; return f }
func (f Field) RefCol(c string) Field     { f.Ref.Col = c; return f }
func (f Field) Range(lo, hi string) Field { f.MinV, f.MaxV = lo, hi; return f }
func (f Field) Pos() Field                { f.GT0 = true; return f }
func (f Field) In(v ...string) Field      { f.Enum = v; return f }
func (f Field) Def(v any) Field           { f.Default, f.HasDefault = v, true; return f }
func (f Field) Column(c string) Field     { f.Col = c; return f }
func (f Field) Match(re *regexp.Regexp, msg string) Field {
	f.Pattern, f.PatternMsg = re, msg
	return f
}
