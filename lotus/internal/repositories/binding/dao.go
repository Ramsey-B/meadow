package binding

import (
	"database/sql"
	"time"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/lotus/pkg/models"
)

const (
	bindingsTable = "bindings"
)

// BindingRow represents the database row for a binding
type BindingRow struct {
	ID          sql.NullString                        `db:"id"`
	TenantID    sql.NullString                        `db:"tenant_id"`
	Name        sql.NullString                        `db:"name"`
	Description sql.NullString                        `db:"description"`
	MappingID   sql.NullString                        `db:"mapping_id"`
	IsEnabled   sql.NullBool                          `db:"is_enabled"`
	OutputTopic sql.NullString                        `db:"output_topic"`
	Filter      database.JSONB[models.BindingFilter]  `db:"filter"`
	CreatedAt   sql.NullTime                          `db:"created_at"`
	UpdatedAt   sql.NullTime                          `db:"updated_at"`
}

var bindingStruct = database.NewStruct(new(BindingRow))

// FromBinding converts a domain model to a database row
func FromBinding(b *models.Binding) *BindingRow {
	return &BindingRow{
		ID:          sql.NullString{String: b.ID, Valid: b.ID != ""},
		TenantID:    sql.NullString{String: b.TenantID, Valid: b.TenantID != ""},
		Name:        sql.NullString{String: b.Name, Valid: b.Name != ""},
		MappingID:   sql.NullString{String: b.MappingID, Valid: b.MappingID != ""},
		IsEnabled:   sql.NullBool{Bool: b.IsEnabled, Valid: true},
		OutputTopic: sql.NullString{String: b.OutputTopic, Valid: b.OutputTopic != ""},
		Filter:      database.JSONB[models.BindingFilter]{Data: b.Filter},
		CreatedAt:   sql.NullTime{Time: b.CreatedAt, Valid: !b.CreatedAt.IsZero()},
		UpdatedAt:   sql.NullTime{Time: b.UpdatedAt, Valid: !b.UpdatedAt.IsZero()},
	}
}

// ToBinding converts a database row to a domain model
func ToBinding(row *BindingRow) *models.Binding {
	return &models.Binding{
		ID:          row.ID.String,
		TenantID:    row.TenantID.String,
		Name:        row.Name.String,
		MappingID:   row.MappingID.String,
		IsEnabled:   row.IsEnabled.Bool,
		OutputTopic: row.OutputTopic.String,
		Filter:      row.Filter.Data,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

// ToBindings converts a slice of database rows to domain models
func ToBindings(rows []BindingRow) []*models.Binding {
	bindings := make([]*models.Binding, len(rows))
	for i, row := range rows {
		bindings[i] = ToBinding(&row)
	}
	return bindings
}

// Now returns the current time in UTC
func Now() time.Time {
	return time.Now().UTC()
}

