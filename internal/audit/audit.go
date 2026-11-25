// Package audit menulis activity_logs (skema Laravel).
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jfebiyanto/easyPOS-Go/internal/db"
)

type Entry struct {
	CompanyID   uuid.UUID // uuid.Nil = tidak diketahui
	ActorID     uuid.UUID // uuid.Nil = tanpa aktor
	ActorName   string
	Action      string
	SubjectType string
	SubjectID   uuid.UUID
	Description string
	Properties  map[string]any
	StoreID     uuid.UUID
}

func nullUUID(u uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: u, Valid: u != uuid.Nil} }

// Log menulis satu baris log. tx boleh pgx.Tx atau pool.
func Log(ctx context.Context, tx db.DBTX, e Entry) error {
	var props []byte
	if e.Properties != nil {
		var err error
		if props, err = json.Marshal(e.Properties); err != nil {
			return err
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if e.ActorName == "" {
		e.ActorName = "-"
	}
	return db.New(tx).InsertActivityLog(ctx, db.InsertActivityLogParams{
		ID:          id,
		CompanyID:   nullUUID(e.CompanyID),
		ActorID:     nullUUID(e.ActorID),
		ActorName:   e.ActorName,
		Action:      e.Action,
		SubjectType: pgtype.Text{String: e.SubjectType, Valid: e.SubjectType != ""},
		SubjectID:   nullUUID(e.SubjectID),
		Description: e.Description,
		Properties:  props,
		CreatedAt:   time.Now().UTC(),
		StoreID:     nullUUID(e.StoreID),
	})
}
