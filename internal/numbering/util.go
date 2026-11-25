package numbering

import "github.com/jackc/pgx/v5/pgtype"

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
