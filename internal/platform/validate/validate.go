// Package validate: pengumpul error validasi bersama (422 dengan pesan Bahasa Indonesia).
package validate

// Errors memetakan nama field ke daftar pesan.
type Errors map[string][]string

func (e Errors) Add(field, msg string) { e[field] = append(e[field], msg) }
func (e Errors) Any() bool             { return len(e) > 0 }

// Has: apakah field sudah punya error (supaya tidak menumpuk pesan).
func (e Errors) Has(field string) bool { return len(e[field]) > 0 }

const Message = "Data tidak valid"
