// Package audit writes append-only activity log entries (FR-A1-07).
package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

type Actor struct {
	ID   string
	Name string
}

type Entry struct {
	CompanyID   string
	Actor       Actor
	Action      string
	SubjectType string
	SubjectID   string
	Description string
	Properties  map[string]any
}

// Record must be called with the same transaction as the mutation it describes.
func Record(ctx context.Context, tx pgx.Tx, e Entry) error {
	props, err := json.Marshal(e.Properties)
	if err != nil || e.Properties == nil {
		props = []byte("{}")
	}
	var actorID any
	if e.Actor.ID != "" {
		actorID = e.Actor.ID
	}
	_, err = tx.Exec(ctx, `INSERT INTO activity_logs
		(company_id, actor_id, actor_name, action, subject_type, subject_id, description, properties)
		VALUES (NULLIF($1,'')::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)`,
		e.CompanyID, actorID, e.Actor.Name, e.Action, e.SubjectType, e.SubjectID, e.Description, props)
	return err
}
